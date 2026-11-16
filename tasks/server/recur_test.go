package main

import (
	"strings"
	"testing"
	"time"
)

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation(dateLayout, s, time.UTC)
	if err != nil {
		t.Fatalf("bad test date %q: %v", s, err)
	}
	return d
}

func dates(times []time.Time) string {
	out := make([]string, len(times))
	for n, t := range times {
		out[n] = stamp(t)
	}
	return strings.Join(out, " ")
}

func TestOccurrences(t *testing.T) {
	cases := []struct {
		name     string
		rule     rule
		start    string
		from, to string
		want     string
	}{
		{
			name:  "weekly on two weekdays",
			rule:  rule{freq: freqWeekly, weekdays: []time.Weekday{time.Monday, time.Thursday}},
			start: "2026-11-16", from: "2026-11-16", to: "2026-11-29",
			want: "2026-11-16 2026-11-19 2026-11-23 2026-11-26",
		},
		{
			name:  "weekly skips the days before it starts",
			rule:  rule{freq: freqWeekly, weekdays: []time.Weekday{time.Monday, time.Friday}},
			start: "2026-11-18", from: "2026-11-16", to: "2026-11-27",
			want: "2026-11-20 2026-11-23 2026-11-27",
		},
		{
			name:  "weekly honours its end date",
			rule:  rule{freq: freqWeekly, weekdays: []time.Weekday{time.Monday}, until: date(t, "2026-11-30")},
			start: "2026-11-16", from: "2026-11-16", to: "2026-12-31",
			want: "2026-11-16 2026-11-23 2026-11-30",
		},
		{
			name:  "fortnightly",
			rule:  rule{freq: freqWeekly, interval: 2, weekdays: []time.Weekday{time.Monday}},
			start: "2026-11-16", from: "2026-11-16", to: "2026-12-20",
			want: "2026-11-16 2026-11-30 2026-12-14",
		},
		{
			name:  "monthly skips months that are too short",
			rule:  rule{freq: freqMonthly},
			start: "2027-01-31", from: "2027-01-01", to: "2027-05-01",
			want: "2027-01-31 2027-03-31",
		},
		{
			name:  "yearly",
			rule:  rule{freq: freqYearly},
			start: "2026-03-09", from: "2026-01-01", to: "2029-01-01",
			want: "2026-03-09 2027-03-09 2028-03-09",
		},
		{
			name:  "leap day only lands on leap years",
			rule:  rule{freq: freqYearly},
			start: "2028-02-29", from: "2028-01-01", to: "2033-01-01",
			want: "2028-02-29 2032-02-29",
		},
		{
			name:  "daily with a count stops counting from the start",
			rule:  rule{freq: freqDaily, count: 3},
			start: "2026-11-16", from: "2026-11-01", to: "2026-12-01",
			want: "2026-11-16 2026-11-17 2026-11-18",
		},
		{
			name:  "a window later than the count sees nothing",
			rule:  rule{freq: freqDaily, count: 2},
			start: "2026-11-16", from: "2026-11-20", to: "2026-12-01",
			want: "",
		},
		{
			name:  "a window that opens mid-series",
			rule:  rule{freq: freqWeekly, weekdays: []time.Weekday{time.Wednesday}},
			start: "2026-01-07", from: "2026-11-16", to: "2026-11-22",
			want: "2026-11-18",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := dates(c.rule.occurrences(date(t, c.start), date(t, c.from), date(t, c.to)))
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestOccurrencesStopsOnAnImpossibleWindow(t *testing.T) {
	r := rule{freq: freqWeekly, weekdays: []time.Weekday{time.Monday}}
	if got := r.occurrences(date(t, "2026-11-16"), date(t, "2026-01-01"), date(t, "2026-01-07")); got != nil {
		t.Fatalf("a window before the start should be empty, got %v", dates(got))
	}
}

func TestWeekdaysFrom(t *testing.T) {
	got, err := weekdaysFrom([]int{1, 4, 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != time.Monday || got[1] != time.Thursday {
		t.Fatalf("got %v, want monday thursday", got)
	}
	if _, err := weekdaysFrom([]int{9}); err == nil {
		t.Fatal("9 is not a weekday")
	}
}
