package main

import (
	"testing"
	"time"
)

func TestMonday(t *testing.T) {
	for in, want := range map[string]string{
		"2026-11-16": "2026-11-16", // a monday is its own week
		"2026-11-22": "2026-11-16", // sunday closes it
		"2026-11-23": "2026-11-23",
	} {
		if got := stamp(monday(date(t, in))); got != want {
			t.Fatalf("monday(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestCarryMovesUnfinishedTasksOntoToday(t *testing.T) {
	st := &state{Items: []*Item{
		{ID: "a", Title: "bins", Date: "2026-11-14", Kind: kindTask, Status: statusOpen},
		{ID: "b", Title: "paid", Date: "2026-11-14", Kind: kindTask, Status: statusDone},
		{ID: "c", Title: "the match", Date: "2026-11-14", Kind: kindEvent, Status: statusOpen},
		{ID: "d", Title: "today", Date: "2026-11-16", Kind: kindTask, Status: statusOpen},
	}}

	moved := carry(st, time.UTC, date(t, "2026-11-16"))
	if moved != 1 {
		t.Fatalf("moved %d items, want 1", moved)
	}
	if st.Items[0].Date != "2026-11-16" || st.Items[0].Carried != 1 {
		t.Fatalf("the unfinished task did not carry: %+v", st.Items[0])
	}
	if st.Items[1].Date != "2026-11-14" {
		t.Fatal("a finished task must stay where it was")
	}
	// saturday's match does not become sunday's match
	if st.Items[2].Date != "2026-11-14" {
		t.Fatalf("an event must stay on its day: %+v", st.Items[2])
	}
}

func TestCarryDropsAStackingRepeat(t *testing.T) {
	st := &state{Items: []*Item{
		{ID: "a", Title: "gym", Date: "2026-11-13", Status: statusOpen, Series: "s1", Occurs: "2026-11-13"},
		{ID: "b", Title: "gym", Date: "2026-11-16", Status: statusOpen, Series: "s1", Occurs: "2026-11-16"},
	}}

	carry(st, time.UTC, date(t, "2026-11-16"))

	if st.Items[0].Status != statusDropped {
		t.Fatalf("a missed repeat should be dropped, not stacked: %+v", st.Items[0])
	}
	if st.Items[1].Date != "2026-11-16" || st.Items[1].Carried != 0 {
		t.Fatalf("today's occurrence should be untouched: %+v", st.Items[1])
	}
}

func TestStreak(t *testing.T) {
	today := date(t, "2026-11-16")

	t.Run("counts back over days with something done", func(t *testing.T) {
		st := &state{Items: []*Item{
			{Date: "2026-11-16", Status: statusDone},
			{Date: "2026-11-15", Status: statusDone},
			{Date: "2026-11-14", Status: statusDone},
			{Date: "2026-11-13", Status: statusDropped},
		}}
		if got := streak(st, time.UTC, today); got != 3 {
			t.Fatalf("got %d, want 3", got)
		}
	})

	t.Run("an empty day is neutral, not a break", func(t *testing.T) {
		st := &state{Items: []*Item{
			{Date: "2026-11-16", Status: statusDone},
			{Date: "2026-11-14", Status: statusDone},
		}}
		if got := streak(st, time.UTC, today); got != 2 {
			t.Fatalf("got %d, want 2", got)
		}
	})

	t.Run("today being unfinished does not break yesterday's", func(t *testing.T) {
		st := &state{Items: []*Item{
			{Date: "2026-11-16", Status: statusOpen},
			{Date: "2026-11-15", Status: statusDone},
		}}
		if got := streak(st, time.UTC, today); got != 1 {
			t.Fatalf("got %d, want 1", got)
		}
	})
}

func TestMaterialise(t *testing.T) {
	st := &state{Series: []*Series{{
		ID:       "s1",
		Title:    "gym",
		Freq:     freqWeekly,
		Weekdays: []int{int(time.Monday), int(time.Thursday)},
		From:     "2026-11-16",
		Until:    "2026-12-31",
		Skips:    []string{"2026-11-19"},
	}}}

	from, to := date(t, "2026-11-16"), date(t, "2026-11-22")
	materialise(st, time.UTC, from, to)
	if len(st.Items) != 1 || st.Items[0].Date != "2026-11-16" {
		t.Fatalf("want one occurrence on the monday, got %+v", st.Items)
	}

	// the same week opened twice must not double anything up
	materialise(st, time.UTC, from, to)
	if len(st.Items) != 1 {
		t.Fatalf("materialising twice made %d items", len(st.Items))
	}
}
