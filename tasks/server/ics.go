package main

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	// alpine ships no tzdata, and a calendar is nothing but timezones
	_ "time/tzdata"
)

// entry is one VEVENT, kept in the shape it was published in: a first
// occurrence plus the rule that repeats it. Occurrences are expanded per
// request instead of up front, so a subscription that repeats forever costs
// nothing to hold.
type entry struct {
	uid    string
	title  string
	where  string
	allDay bool
	start  time.Time
	span   time.Duration
	repeat *rule
	skip   map[string]bool
}

// parseICS reads a calendar into its events. Anything it cannot make sense of
// is skipped rather than failing the feed: published calendars are full of
// properties nothing but their own client reads.
func parseICS(r io.Reader, zone *time.Location) ([]*entry, error) {
	var (
		out      []*entry
		current  *entry
		inEvent  bool
		uid      string
		cancel   bool
		override = map[string][]string{}
		byUID    = map[string]*entry{}
	)

	lines, err := unfold(r)
	if err != nil {
		return nil, err
	}

	for _, line := range lines {
		name, params, value := property(line)
		switch name {
		case "BEGIN":
			if value == "VEVENT" {
				inEvent, current, uid, cancel = true, &entry{skip: map[string]bool{}}, "", false
			}
		case "END":
			if value != "VEVENT" || !inEvent {
				continue
			}
			inEvent = false
			if cancel || current.title == "" || current.start.IsZero() {
				continue
			}
			out = append(out, current)
			if uid != "" {
				byUID[uid] = current
			}
		}
		if !inEvent || current == nil {
			continue
		}

		switch name {
		case "UID":
			uid, current.uid = value, value
		case "SUMMARY":
			current.title = unescape(value)
		case "LOCATION":
			current.where = unescape(value)
		case "STATUS":
			cancel = value == "CANCELLED"
		case "DTSTART":
			t, allDay, err := moment(value, params, zone)
			if err == nil {
				current.start, current.allDay = t, allDay
			}
		case "DTEND":
			t, _, err := moment(value, params, zone)
			if err == nil && !current.start.IsZero() && t.After(current.start) {
				// an all-day DTEND is exclusive: a one day event ends at the
				// following midnight, which is one span of 24h
				current.span = t.Sub(current.start)
			}
		case "RRULE":
			if r, err := parseRRULE(value, zone); err == nil {
				current.repeat = r
			}
		case "EXDATE":
			for _, v := range strings.Split(value, ",") {
				if t, _, err := moment(v, params, zone); err == nil {
					current.skip[stamp(t)] = true
				}
			}
		case "RECURRENCE-ID":
			// a single moved or edited occurrence: it lands as its own event,
			// and the series it came from has to stop drawing the original
			if t, _, err := moment(value, params, zone); err == nil && uid != "" {
				override[uid] = append(override[uid], stamp(t))
			}
		}
	}

	for uid, dates := range override {
		if base, ok := byUID[uid]; ok {
			for _, d := range dates {
				base.skip[d] = true
			}
		}
	}
	return out, nil
}

// unfold joins the continuation lines ics wraps long values onto
func unfold(r io.Reader) ([]string, error) {
	var out []string
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for s.Scan() {
		line := strings.TrimRight(s.Text(), "\r")
		if line == "" {
			continue
		}
		if (line[0] == ' ' || line[0] == '\t') && len(out) > 0 {
			out[len(out)-1] += line[1:]
			continue
		}
		out = append(out, line)
	}
	return out, s.Err()
}

// property splits NAME;PARAM=VALUE:value into its three parts
func property(line string) (name string, params map[string]string, value string) {
	colon := strings.Index(line, ":")
	if colon < 0 {
		return "", nil, ""
	}
	head, value := line[:colon], line[colon+1:]
	params = map[string]string{}

	parts := strings.Split(head, ";")
	name = strings.ToUpper(parts[0])
	for _, p := range parts[1:] {
		if k, v, ok := strings.Cut(p, "="); ok {
			params[strings.ToUpper(k)] = strings.Trim(v, `"`)
		}
	}
	return name, params, value
}

func unescape(v string) string {
	r := strings.NewReplacer(`\n`, " ", `\N`, " ", `\,`, ",", `\;`, ";", `\\`, `\`)
	return strings.TrimSpace(r.Replace(v))
}

// moment reads one ics date or datetime into the planner's own zone. A value
// with no zone at all is floating by definition, which here means local.
func moment(value string, params map[string]string, zone *time.Location) (time.Time, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false, errors.New("empty date")
	}

	if params["VALUE"] == "DATE" || len(value) == 8 {
		t, err := time.ParseInLocation("20060102", value, zone)
		return t, true, err
	}

	if strings.HasSuffix(value, "Z") {
		t, err := time.Parse("20060102T150405Z", value)
		return t.In(zone), false, err
	}

	where := zone
	if tz := params["TZID"]; tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			where = loc
		}
	}
	t, err := time.ParseInLocation("20060102T150405", value, where)
	return t.In(zone), false, err
}

var icsWeekdays = map[string]time.Weekday{
	"SU": time.Sunday, "MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday,
	"TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday,
}

func parseRRULE(value string, zone *time.Location) (*rule, error) {
	r := &rule{interval: 1}
	for _, part := range strings.Split(value, ";") {
		key, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(key) {
		case "FREQ":
			r.freq = strings.ToLower(v)
		case "INTERVAL":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				r.interval = n
			}
		case "COUNT":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				r.count = n
			}
		case "UNTIL":
			if t, _, err := moment(v, nil, zone); err == nil {
				r.until = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone)
			}
		case "BYDAY":
			for _, d := range strings.Split(v, ",") {
				// BYDAY can be ordinal, as in 3RD friday of the month. the
				// day itself is the last two characters either way
				d = strings.ToUpper(strings.TrimSpace(d))
				if len(d) >= 2 {
					if wd, ok := icsWeekdays[d[len(d)-2:]]; ok {
						r.weekdays = append(r.weekdays, wd)
					}
				}
			}
		}
	}
	if !validFreq(r.freq) {
		return nil, errors.New("unsupported frequency " + r.freq)
	}
	// BYDAY only drives the expansion for weekly repeats here; on a monthly
	// rule it would need the ordinal too, so the plain monthly day is used
	if r.freq != freqWeekly {
		r.weekdays = nil
	}
	return r, nil
}

// each walks every occurrence of every entry that starts in the window,
// which is all both readings of a calendar need: an event fills the days it
// covers, a task lands on the day it starts
func each(entries []*entry, from, to time.Time, fn func(e *entry, day time.Time)) {
	for _, e := range entries {
		day := time.Date(e.start.Year(), e.start.Month(), e.start.Day(), 0, 0, 0, 0, e.start.Location())
		if e.repeat == nil {
			if !e.skip[stamp(day)] && !day.Before(from) && !day.After(to) {
				fn(e, day)
			}
			continue
		}
		// a multi-day all-day event can start before the window and still
		// cover part of it, so the search begins a fortnight early
		for _, d := range e.repeat.occurrences(day, from.AddDate(0, 0, -14), to) {
			if !e.skip[stamp(d)] {
				fn(e, d)
			}
		}
	}
}

// expand draws one parsed calendar as the events of the days it covers
func expand(entries []*entry, from, to time.Time, feed *Feed) map[string][]Event {
	out := map[string][]Event{}

	add := func(e *entry, d time.Time) {
		days := 1
		if e.allDay && e.span > 24*time.Hour {
			days = int(e.span.Hours() / 24)
		}
		for n := range days {
			on := d.AddDate(0, 0, n)
			if on.Before(from) || on.After(to) {
				continue
			}
			ev := Event{
				Feed:  feed.ID,
				Name:  feed.Name,
				Color: feed.Color,
				Title: e.title,
				Where: e.where,
				Date:  stamp(on),
			}
			if !e.allDay {
				ev.Start = e.start.Format("15:04")
				if e.span > 0 && e.span < 24*time.Hour {
					ev.End = e.start.Add(e.span).Format("15:04")
				}
			}
			out[ev.Date] = append(out[ev.Date], ev)
		}
	}

	each(entries, from, to, add)
	return out
}

// due is one occurrence of a calendar read as tasks, ready to become an item
type due struct {
	feed   string
	uid    string
	title  string
	where  string
	date   string
	start  string
	end    string
	allDay bool
}

// plan reads the same calendar as work to be done: one task per occurrence on
// the day it starts, rather than a band across every day it touches
func plan(entries []*entry, from, to time.Time, feed *Feed) []due {
	var out []due
	each(entries, from, to, func(e *entry, d time.Time) {
		if d.Before(from) || d.After(to) {
			return
		}
		task := due{feed: feed.ID, uid: e.uid, title: e.title, where: e.where, date: stamp(d), allDay: e.allDay}
		if !e.allDay {
			task.start = e.start.Format("15:04")
			if e.span > 0 && e.span < 24*time.Hour {
				task.end = e.start.Add(e.span).Format("15:04")
			}
		}
		out = append(out, task)
	})
	return out
}
