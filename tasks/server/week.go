package main

import (
	"log"
	"slices"
	"sort"
	"time"

	"rpi/internal/ids"
)

type dayView struct {
	Date    string  `json:"date"`
	Items   []*Item `json:"items"`
	Events  []Event `json:"events"`
	Today   bool    `json:"today"`
	Past    bool    `json:"past"`
	Planned bool    `json:"planned"`
}

type weekStats struct {
	Done    int `json:"done"`
	Dropped int `json:"dropped"`
	Open    int `json:"open"`
	Total   int `json:"total"`
	Streak  int `json:"streak"`
	// how many items the last sweep pushed onto today, so the ui can own up
	// to it instead of the week quietly rearranging itself
	Carried int `json:"carried"`
}

type weekView struct {
	Start      string      `json:"start"`
	End        string      `json:"end"`
	Today      string      `json:"today"`
	Days       []dayView   `json:"days"`
	Stats      weekStats   `json:"stats"`
	Categories []*Category `json:"categories"`
	Series     []*Series   `json:"series"`
	Feeds      []*Feed     `json:"feeds"`
}

// monday snaps any date back to the monday of its week, which is the only
// week boundary this app knows
func monday(d time.Time) time.Time {
	back := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -back)
}

func today(zone *time.Location) time.Time {
	n := time.Now().In(zone)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, zone)
}

// materialise turns every series occurrence between from and to into a real
// item, once. From there a repeat is indistinguishable from anything typed by
// hand: it can be completed, moved, carried or deleted on its own.
func materialise(st *state, zone *time.Location, from, to time.Time) {
	for _, s := range st.Series {
		start, err := parseDate(s.From, zone)
		if err != nil {
			continue
		}
		r := rule{freq: s.Freq, interval: 1}
		if s.Freq == freqWeekly {
			days, err := weekdaysFrom(s.Weekdays)
			if err != nil {
				continue
			}
			r.weekdays = days
		}
		if until, err := parseDate(s.Until, zone); err == nil {
			r.until = until
		}

		skipped := map[string]bool{}
		for _, date := range s.Skips {
			skipped[date] = true
		}

		for _, d := range r.occurrences(start, from, to) {
			date := stamp(d)
			if skipped[date] || st.occurrence(s.ID, date) != nil {
				continue
			}
			st.Items = append(st.Items, &Item{
				ID:       ids.New(8),
				Title:    s.Title,
				Notes:    s.Notes,
				Date:     date,
				Start:    s.Start,
				End:      s.End,
				Category: s.Category,
				Kind:     s.Kind,
				Status:   statusOpen,
				Series:   s.ID,
				Occurs:   date,
				Made:     now(),
			})
		}
	}
}

// importTasks turns the occurrences of a calendar subscribed as tasks into
// real items, once each. From there they are the planner's own: completable,
// droppable, and carried forward like anything else you did not get to.
func importTasks(st *state, tasks []due) {
	for _, task := range tasks {
		feed := st.feed(task.feed)
		if feed == nil || task.uid == "" || task.title == "" {
			continue
		}
		if slices.Contains(feed.Skips, task.uid+"@"+task.date) {
			continue
		}
		if st.imported(task.feed, task.uid, task.date) != nil {
			continue
		}
		st.Items = append(st.Items, &Item{
			ID:     ids.New(8),
			Title:  task.title,
			Notes:  task.where,
			Date:   task.date,
			Start:  task.start,
			End:    task.end,
			Kind:   kindTask,
			Status: statusOpen,
			Feed:   task.feed,
			Source: task.uid,
			Occurs: task.date,
			Made:   now(),
		})
	}
}

// carry sweeps every unfinished task in the past onto today. No work falls
// off the week silently: a task either gets done, gets explicitly dropped, or
// follows you forward wearing the number of days it has been dodged. Events
// stay where they were, because a thing that happens at a time cannot happen
// later just because you missed it.
//
// A repeat is the one exception. Yesterday's missed gym does not stack on top
// of today's gym, so the older occurrence is dropped instead of piling up.
func carry(st *state, zone *time.Location, day time.Time) int {
	cut := stamp(day)
	moved := 0

	repeats := map[string]bool{}
	for _, i := range st.Items {
		if i.Series != "" && i.open() && i.Date >= cut {
			repeats[i.Series] = true
		}
	}

	for _, i := range st.Items {
		if !i.open() || i.Date >= cut || i.event() {
			continue
		}
		if i.Series != "" && repeats[i.Series] {
			i.Status = statusDropped
			i.Resolved = now()
			continue
		}
		if i.Series != "" {
			repeats[i.Series] = true
		}
		i.Date = cut
		i.Carried++
		moved++
	}
	return moved
}

// streak counts back from today over days that had something done on them. A
// day with nothing planned is neutral rather than a break: an empty saturday
// is not a failure, and a planner that punishes rest does not get opened on
// monday.
func streak(st *state, zone *time.Location, day time.Time) int {
	done := map[string]bool{}
	any := map[string]bool{}
	for _, i := range st.Items {
		any[i.Date] = true
		if i.Status == statusDone {
			done[i.Date] = true
		}
	}

	count := 0
	d := day
	// today still in progress never breaks a streak, it just has not extended
	// it yet
	if !done[stamp(d)] {
		d = d.AddDate(0, 0, -1)
	}
	for range 400 {
		date := stamp(d)
		switch {
		case done[date]:
			count++
		case any[date]:
			return count
		}
		d = d.AddDate(0, 0, -1)
	}
	return count
}

// week assembles the seven columns the ui draws, after making sure every
// repeat in range exists and nothing open is still stranded in the past
func (s *server) week(start time.Time) weekView {
	end := start.AddDate(0, 0, 6)
	current := today(s.cfg.zone)

	view := weekView{
		Start: stamp(start),
		End:   stamp(end),
		Today: stamp(current),
	}

	err := s.store.write(func(st *state) error {
		// repeats are materialised a week either side as well, so moving
		// through the weeks never shows an empty column that fills in late
		materialise(st, s.cfg.zone, start.AddDate(0, 0, -7), end.AddDate(0, 0, 7))
		materialise(st, s.cfg.zone, current, current.AddDate(0, 0, 7))
		importTasks(st, s.feeds.due(start.AddDate(0, 0, -7), end.AddDate(0, 0, 7)))
		importTasks(st, s.feeds.due(current, current.AddDate(0, 0, 7)))
		view.Stats.Carried = carry(st, s.cfg.zone, current)
		view.Stats.Streak = streak(st, s.cfg.zone, current)

		byDay := map[string][]*Item{}
		for _, i := range st.Items {
			byDay[i.Date] = append(byDay[i.Date], i)
		}

		events := s.feeds.between(start, end)

		for n := range 7 {
			d := start.AddDate(0, 0, n)
			date := stamp(d)
			items := orEmpty(byDay[date])
			sort.SliceStable(items, func(a, b int) bool {
				ia, ib := items[a], items[b]
				if ia.timed() != ib.timed() {
					return ia.timed()
				}
				if ia.timed() && ia.Start != ib.Start {
					return ia.Start < ib.Start
				}
				return ia.Made < ib.Made
			})

			for _, i := range items {
				if i.event() {
					continue
				}
				view.Stats.Total++
				switch i.Status {
				case statusDone:
					view.Stats.Done++
				case statusDropped:
					view.Stats.Dropped++
				default:
					view.Stats.Open++
				}
			}

			day := orEmpty(events[date])

			view.Days = append(view.Days, dayView{
				Date:    date,
				Items:   items,
				Events:  day,
				Today:   date == view.Today,
				Past:    d.Before(current),
				Planned: len(items) > 0 || len(day) > 0,
			})
		}

		// a nil slice marshals to null, and the ui is promised lists
		view.Categories = orEmpty(st.Categories)
		view.Series = orEmpty(st.Series)
		view.Feeds = orEmpty(st.Feeds)
		return nil
	})
	if err != nil {
		// the view in hand is still right; only the write-back of the sweep failed
		log.Printf("week %s: %v", view.Start, err)
	}
	return view
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
