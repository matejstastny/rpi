package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// dates are carried as plain YYYY-MM-DD strings end to end: the ui never has
// to think about timezones, and a stored week reads the same in any of them
const dateLayout = "2006-01-02"

const (
	statusOpen    = "open"
	statusDone    = "done"
	statusDropped = "dropped"
)

// a task is work: it waits for you, and follows you forward until it is
// finished or written off. an event is a thing that happens at a time: it
// cannot be ticked, and when its day is over it is over.
const (
	kindTask  = "task"
	kindEvent = "event"
)

func validItemKind(k string) bool { return k == kindTask || k == kindEvent }

// Category colours one item. The set is the planner's own, seeded on first
// run and editable from the ui.
type Category struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Item is one thing on one day, either work or an appointment. Both are the
// same record, because both live in the same column and get moved around the
// same way; only carrying and completing tell them apart.
type Item struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Notes    string `json:"notes,omitempty"`
	Date     string `json:"date"`
	Start    string `json:"start,omitempty"`
	End      string `json:"end,omitempty"`
	Category string `json:"category,omitempty"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	Resolved string `json:"resolved,omitempty"`
	// how many days this has been pushed forward, so the ui can say so
	Carried int `json:"carried,omitempty"`
	// set when this was materialised from a recurrence
	Series string `json:"series,omitempty"`
	// set when this came out of a calendar subscribed as tasks, with the
	// event's own uid, so the same occurrence is never imported twice
	Feed   string `json:"feed,omitempty"`
	Source string `json:"source,omitempty"`
	Occurs string `json:"occurs,omitempty"`
	// what "keep it on this day" was called before an item knew whether it
	// was work or an appointment; read once on load and then dropped
	Pinned bool   `json:"pinned,omitempty"`
	Made   string `json:"made"`
}

func (i *Item) timed() bool { return i.Start != "" }

func (i *Item) open() bool { return i.Status == statusOpen }

func (i *Item) event() bool { return i.Kind == kindEvent }

// Series is a repeating item. Occurrences are materialised into real items as
// the weeks holding them are opened, so completing or moving one is the same
// operation as for anything else.
type Series struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Notes    string `json:"notes,omitempty"`
	Start    string `json:"start,omitempty"`
	End      string `json:"end,omitempty"`
	Category string `json:"category,omitempty"`
	Kind     string `json:"kind"`
	Freq     string `json:"freq"`
	// weekdays, 0 sunday through 6 saturday, for the weekly frequency
	Weekdays []int  `json:"weekdays,omitempty"`
	From     string `json:"from"`
	Until    string `json:"until"`
	// occurrences deleted one at a time, which the sweep must not rebuild
	Skips []string `json:"skips,omitempty"`
	Made  string   `json:"made"`
}

// Feed is a subscribed ics calendar, read either way round. As events it is
// context: refetched on a timer, drawn beside the day, never completed. As
// tasks every occurrence becomes a real item that can be finished, written
// off and carried forward, because plenty of calendars are really just
// deadlines wearing a calendar's clothes.
type Feed struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Color string `json:"color"`
	Kind  string `json:"kind"`
	// occurrences deleted one at a time, as "uid@date", which the next import
	// must not bring back
	Skips []string `json:"skips,omitempty"`
	Made  string   `json:"made"`

	// refreshed in place by the sync loop, never read from disk
	Synced string `json:"synced,omitempty"`
	Error  string `json:"error,omitempty"`
	Events int    `json:"events"`
}

// Event is one occurrence from a feed, already expanded out of its rrule.
type Event struct {
	Feed  string `json:"feed"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Title string `json:"title"`
	Where string `json:"where,omitempty"`
	Date  string `json:"date"`
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

func parseDate(s string, zone *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation(dateLayout, strings.TrimSpace(s), zone)
	if err != nil {
		return time.Time{}, fmt.Errorf("%q is not a date", s)
	}
	return t, nil
}

func stamp(t time.Time) string { return t.Format(dateLayout) }

// clock validates an HH:MM, which is the only time format the ui sends
func clock(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		return "", fmt.Errorf("%q is not a time", s)
	}
	return t.Format("15:04"), nil
}

func validStatus(s string) bool {
	return s == statusOpen || s == statusDone || s == statusDropped
}

const (
	feedEvents = "events"
	feedTasks  = "tasks"
)

func validKind(k string) bool { return k == feedEvents || k == feedTasks }

var errNoTitle = errors.New("that needs a title")

func title(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errNoTitle
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s, nil
}
