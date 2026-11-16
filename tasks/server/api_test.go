package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newServer(t *testing.T) (*server, http.Handler) {
	t.Helper()
	cfg := config{dir: t.TempDir(), zone: time.UTC, interval: time.Hour}
	st, err := newStore(cfg)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	s := &server{cfg: cfg, store: st, feeds: newFeeds(cfg, st)}
	return s, s.routes()
}

func call(t *testing.T, h http.Handler, method, path, body string) weekView {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)

	if res.Code >= 300 {
		t.Fatalf("%s %s: %d %s", method, path, res.Code, res.Body.String())
	}
	var week weekView
	if err := json.Unmarshal(res.Body.Bytes(), &week); err != nil {
		t.Fatalf("decode %s %s: %v", method, path, err)
	}
	return week
}

func titlesOf(week weekView) []string {
	var out []string
	for _, day := range week.Days {
		for _, item := range day.Items {
			out = append(out, item.Date+" "+item.Title)
		}
	}
	return out
}

// editing a repeat from one of its later occurrences used to move the whole
// series' start to that date, which deleted every occurrence between today
// and the one being edited
func TestEditingASeriesKeepsTheOccurrencesBeforeIt(t *testing.T) {
	s, h := newServer(t)
	start := monday(today(s.cfg.zone))
	week := stamp(start)
	until := stamp(start.AddDate(0, 0, 20))

	call(t, h, "POST", "/api/items?week="+week, `{
		"title": "gym", "notes": "", "date": "`+week+`", "start": "", "end": "",
		"category": "", "kind": "task",
		"repeat": {"freq": "weekly", "weekdays": [0,1,2,3,4,5,6], "until": "`+until+`"}
	}`)

	before := call(t, h, "GET", "/api/week?week="+week, "")
	if len(titlesOf(before)) != 7 {
		t.Fatalf("want an occurrence every day of the week, got %v", titlesOf(before))
	}
	id := before.Series[0].ID

	// the ui opens the edit from an occurrence late in the series, and sends
	// the series' own start date with it
	after := call(t, h, "PATCH", "/api/series/"+id+"?week="+week, `{
		"title": "gym and sauna", "notes": "", "date": "`+before.Series[0].From+`",
		"start": "", "end": "", "category": "", "kind": "task",
		"repeat": {"freq": "weekly", "weekdays": [0,1,2,3,4,5,6], "until": "`+until+`"}
	}`)

	got := titlesOf(after)
	if len(got) != 7 {
		t.Fatalf("the week should still be full after the edit, got %v", got)
	}
	for _, title := range got {
		if !strings.HasSuffix(title, "gym and sauna") {
			t.Fatalf("every occurrence should carry the edit, got %v", got)
		}
	}
	if after.Series[0].From != before.Series[0].From {
		t.Fatalf("the repeat moved its start from %s to %s", before.Series[0].From, after.Series[0].From)
	}
}

func TestAnEventCannotBeFinished(t *testing.T) {
	_, h := newServer(t)
	week := call(t, h, "GET", "/api/week", "")
	day := week.Days[0].Date

	call(t, h, "POST", "/api/items?week="+week.Start, `{
		"title": "dentist", "notes": "", "date": "`+day+`", "start": "14:30", "end": "15:00",
		"category": "", "kind": "event", "repeat": null
	}`)

	after := call(t, h, "GET", "/api/week?week="+week.Start, "")
	item := after.Days[0].Items[0]
	if item.Kind != kindEvent {
		t.Fatalf("want an event, got %q", item.Kind)
	}
	// an event is not work, so it is not part of what the week counts
	if after.Stats.Total != 0 {
		t.Fatalf("an event should not count towards the week's work: %+v", after.Stats)
	}

	req := httptest.NewRequest("POST", "/api/items/"+item.ID+"/status?week="+week.Start,
		strings.NewReader(`{"status": "done"}`))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("finishing an event should be refused, got %d %s", res.Code, res.Body.String())
	}
}

// a planner carried into a different timezone must not end up holding both
// the old copy of an imported deadline and the corrected one
func TestChangingTimezoneDropsUnfinishedImports(t *testing.T) {
	dir := t.TempDir()
	prague, err := time.LoadLocation("Europe/Prague")
	if err != nil {
		t.Fatalf("tzdata: %v", err)
	}

	st, err := newStore(config{dir: dir, zone: prague})
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	ahead := stamp(today(prague).AddDate(0, 0, 3))
	err = st.write(func(s *state) error {
		s.Items = append(s.Items,
			&Item{ID: "a", Title: "LinkedList", Date: ahead, Kind: kindTask, Status: statusOpen,
				Feed: "f1", Source: "a1", Occurs: ahead},
			&Item{ID: "b", Title: "handed in", Date: ahead, Kind: kindTask, Status: statusDone,
				Feed: "f1", Source: "a2", Occurs: ahead},
			&Item{ID: "c", Title: "my own", Date: ahead, Kind: kindTask, Status: statusOpen},
		)
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// reopened somewhere else entirely
	reopened, err := newStore(config{dir: dir, zone: time.UTC})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}

	var left []string
	reopened.read(func(s *state) {
		for _, i := range s.Items {
			left = append(left, i.ID)
		}
	})
	if len(left) != 2 || left[0] != "b" || left[1] != "c" {
		t.Fatalf("want the finished import and the hand-written item kept, got %v", left)
	}

	// and it only happens once
	again, err := newStore(config{dir: dir, zone: time.UTC})
	if err != nil {
		t.Fatalf("reopen twice: %v", err)
	}
	again.read(func(s *state) {
		if len(s.Items) != 2 {
			t.Fatalf("a second open at the same zone changed things: %d items", len(s.Items))
		}
	})
}
