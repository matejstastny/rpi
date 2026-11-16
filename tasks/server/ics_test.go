package main

import (
	"sort"
	"strings"
	"testing"
	"time"
)

const sample = `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:one@example.com
SUMMARY:Standup
LOCATION:Room 2
DTSTART;TZID=Europe/Prague:20261116T090000
DTEND;TZID=Europe/Prague:20261116T091500
RRULE:FREQ=WEEKLY;BYDAY=MO,WE;UNTIL=20261130T000000Z
EXDATE;TZID=Europe/Prague:20261118T090000
END:VEVENT
BEGIN:VEVENT
UID:two@example.com
SUMMARY:Trip to the sea
DTSTART;VALUE=DATE:20261120
DTEND;VALUE=DATE:20261123
END:VEVENT
BEGIN:VEVENT
UID:three@example.com
SUMMARY:Cancelled thing
STATUS:CANCELLED
DTSTART:20261117T100000Z
END:VEVENT
BEGIN:VEVENT
UID:four@example.com
SUMMARY:A very long title that the publisher
  wrapped over two lines
DTSTART:20261117T140000Z
DTEND:20261117T150000Z
END:VEVENT
END:VCALENDAR
`

func parse(t *testing.T, raw string) []*entry {
	t.Helper()
	zone, err := time.LoadLocation("Europe/Prague")
	if err != nil {
		t.Fatalf("tzdata: %v", err)
	}
	entries, err := parseICS(strings.NewReader(raw), zone)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return entries
}

func TestParseICS(t *testing.T) {
	entries := parse(t, sample)
	if len(entries) != 3 {
		t.Fatalf("got %d events, want 3 (the cancelled one is dropped)", len(entries))
	}

	standup := entries[0]
	if standup.title != "Standup" || standup.where != "Room 2" {
		t.Fatalf("got %q at %q", standup.title, standup.where)
	}
	if standup.allDay {
		t.Fatal("a 09:00 standup is not an all day event")
	}
	if standup.span != 15*time.Minute {
		t.Fatalf("got a span of %s, want 15m", standup.span)
	}
	if standup.repeat == nil || standup.repeat.freq != freqWeekly || len(standup.repeat.weekdays) != 2 {
		t.Fatalf("rrule did not survive: %+v", standup.repeat)
	}
	if !standup.skip["2026-11-18"] {
		t.Fatalf("the exdate was not kept: %v", standup.skip)
	}

	trip := entries[1]
	if !trip.allDay || trip.span != 72*time.Hour {
		t.Fatalf("got allDay=%v span=%s, want a three day all day event", trip.allDay, trip.span)
	}

	if got := entries[2].title; got != "A very long title that the publisher wrapped over two lines" {
		t.Fatalf("folded line was not joined: %q", got)
	}
}

func TestExpand(t *testing.T) {
	zone, _ := time.LoadLocation("Europe/Prague")
	entries := parse(t, sample)
	feed := &Feed{ID: "f1", Name: "work", Color: "#89b4fa"}

	from := time.Date(2026, 11, 16, 0, 0, 0, 0, zone)
	days := expand(entries, from, from.AddDate(0, 0, 6), feed)

	if got := len(days["2026-11-16"]); got != 1 {
		t.Fatalf("monday has %d events, want the standup", got)
	}
	if got := days["2026-11-16"][0]; got.Start != "09:00" || got.End != "09:15" || got.Feed != "f1" {
		t.Fatalf("standup came out as %+v", got)
	}
	if len(days["2026-11-18"]) != 0 {
		t.Fatal("wednesday was excluded by EXDATE and should be empty")
	}
	for _, day := range []string{"2026-11-20", "2026-11-21", "2026-11-22"} {
		if len(days[day]) != 1 {
			t.Fatalf("%s should carry the multi-day trip, got %v", day, days[day])
		}
	}
	// the trip's exclusive end date means it does not reach monday
	if len(days["2026-11-23"]) != 0 {
		t.Fatal("an all day DTEND is exclusive")
	}
}

func TestFeedURL(t *testing.T) {
	got, err := feedURL(" webcal://calendar.example.com/basic.ics ")
	if err != nil || got != "https://calendar.example.com/basic.ics" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, bad := range []string{"", "file:///etc/passwd", "not a url", "ftp://host/cal.ics"} {
		if _, err := feedURL(bad); err == nil {
			t.Fatalf("%q should not be accepted", bad)
		}
	}
}

func TestPlanReadsTheSameCalendarAsTasks(t *testing.T) {
	zone, _ := time.LoadLocation("Europe/Prague")
	entries := parse(t, sample)
	feed := &Feed{ID: "f1", Name: "school", Color: "#89b4fa", Kind: feedTasks}

	from := time.Date(2026, 11, 16, 0, 0, 0, 0, zone)
	tasks := plan(entries, from, from.AddDate(0, 0, 6), feed)

	var dates []string
	for _, task := range tasks {
		dates = append(dates, task.date+" "+task.title)
	}
	sort.Strings(dates)

	want := []string{
		// wednesday's standup is excluded by the calendar's own EXDATE, and
		// the cancelled event never becomes work at all
		"2026-11-16 Standup",
		"2026-11-17 A very long title that the publisher wrapped over two lines",
		// the trip is one task on the day it starts, not one per day it covers
		"2026-11-20 Trip to the sea",
	}
	sort.Strings(want)

	if len(dates) != len(want) {
		t.Fatalf("got %v, want %v", dates, want)
	}
	for n := range want {
		if dates[n] != want[n] {
			t.Fatalf("got %v, want %v", dates, want)
		}
	}
}

func TestImportTasksIsIdempotentAndHonoursSkips(t *testing.T) {
	st := &state{Feeds: []*Feed{{ID: "f1", Name: "school", Kind: feedTasks, Skips: []string{"exam@2026-11-18"}}}}
	tasks := []due{
		{feed: "f1", uid: "lecture", title: "Lecture", date: "2026-11-17", start: "10:00", end: "11:30"},
		{feed: "f1", uid: "exam", title: "Midterm", date: "2026-11-18"},
		{feed: "missing", uid: "x", title: "Orphan", date: "2026-11-18"},
	}

	importTasks(st, tasks)
	if len(st.Items) != 1 {
		t.Fatalf("want only the unskipped task of a known feed, got %+v", st.Items)
	}
	if got := st.Items[0]; got.Title != "Lecture" || got.Start != "10:00" || got.Status != statusOpen {
		t.Fatalf("imported task came out as %+v", got)
	}

	// refetching the calendar must not import the same occurrence twice
	importTasks(st, tasks)
	if len(st.Items) != 1 {
		t.Fatalf("a second import made %d items", len(st.Items))
	}

	// and once imported it carries like anything else the planner owns
	st.Items[0].Date = "2026-11-17"
	if moved := carry(st, time.UTC, date(t, "2026-11-20")); moved != 1 {
		t.Fatalf("an imported task should carry forward, moved %d", moved)
	}
	if st.Items[0].Date != "2026-11-20" || st.Items[0].Carried != 1 {
		t.Fatalf("carry left it at %+v", st.Items[0])
	}
}

const canvasFeed = `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:a1@instructure
SUMMARY:LinkedList [CSS 143 A]
DTSTART;VALUE=DATE:20261118
DTEND;VALUE=DATE:20261119
END:VEVENT
BEGIN:VEVENT
UID:a2@instructure
SUMMARY:12.1 HW - Parametric Equations [STMATH 126 B]
DTSTART;TZID=Europe/Prague:20261119T235900
DTEND;TZID=Europe/Prague:20261120T000000
END:VEVENT
BEGIN:VEVENT
UID:a3@instructure
SUMMARY:Quiz Chapter 4b/5- Lewis Structures [B CHEM 143 A]
DTSTART;VALUE=DATE:20261120
END:VEVENT
BEGIN:VEVENT
UID:a4@instructure
SUMMARY:Office hours
DTSTART;TZID=Europe/Prague:20261117T100000
END:VEVENT
END:VCALENDAR
`

func TestCanvasTitlesSplitIntoCourses(t *testing.T) {
	cases := map[string][2]string{
		"LinkedList [CSS 143 A]":                  {"LinkedList", "CSS 143 A"},
		"Quiz Chapter 4b/5- Lewis [B CHEM 143 A]": {"Quiz Chapter 4b/5- Lewis", "B CHEM 143 A"},
		"12.1 HW - Parametric [STMATH 126 B]":     {"12.1 HW - Parametric", "STMATH 126 B"},
		"Office hours":                            {"Office hours", ""},
		"[CSS 143 A]":                             {"[CSS 143 A]", ""},
		"Reading [chapter 3] [CSS 143 A]":         {"Reading [chapter 3]", "CSS 143 A"},
	}
	for title, want := range cases {
		name, code := splitCourse(title)
		if name != want[0] || code != want[1] {
			t.Fatalf("%q split to %q / %q, want %q / %q", title, name, code, want[0], want[1])
		}
	}
}

func TestCanvasFeedLearnsAndHidesCourses(t *testing.T) {
	zone, _ := time.LoadLocation("Europe/Prague")
	entries := parse(t, canvasFeed)

	feed := &Feed{ID: "f1", Name: "canvas", Color: "#89b4fa", Kind: feedCanvas}
	if !feed.learn(courses(entries)) {
		t.Fatal("a canvas feed should discover its courses")
	}
	if len(feed.Classes) != 3 {
		t.Fatalf("want three courses, got %+v", feed.Classes)
	}
	// discovering the same feed again adds nothing
	if feed.learn(courses(entries)) {
		t.Fatal("courses should only be learned once")
	}
	for n, c := range feed.Classes {
		if c.Color != classColors[n%len(classColors)] {
			t.Fatalf("course %s got colour %s", c.Code, c.Color)
		}
	}

	from := time.Date(2026, 11, 16, 0, 0, 0, 0, zone)
	tasks := plan(entries, from, from.AddDate(0, 0, 6), feed)
	if len(tasks) != 4 {
		t.Fatalf("want every entry as work, got %d", len(tasks))
	}
	for _, task := range tasks {
		if strings.Contains(task.title, "[") {
			t.Fatalf("the course should be taken out of the title: %q", task.title)
		}
		if task.uid == "a1@instructure" && task.class != "CSS 143 A" {
			t.Fatalf("want the course kept beside the title, got %q", task.class)
		}
	}

	// switching one off keeps its assignments out of the week entirely
	feed.class("CSS 143 A").Hidden = true
	tasks = plan(entries, from, from.AddDate(0, 0, 6), feed)
	if len(tasks) != 3 {
		t.Fatalf("a hidden course should bring nothing in, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.class == "CSS 143 A" {
			t.Fatalf("a hidden course leaked in: %+v", task)
		}
	}
}
