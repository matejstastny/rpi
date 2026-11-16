package main

import (
	"fmt"
	"sort"
	"time"
)

const (
	freqDaily   = "daily"
	freqWeekly  = "weekly"
	freqMonthly = "monthly"
	freqYearly  = "yearly"
)

// the planner's own repeat options. daily is not offered in the ui, but an
// imported calendar is full of it, and the engine is the same one.
func validFreq(f string) bool {
	switch f {
	case freqDaily, freqWeekly, freqMonthly, freqYearly:
		return true
	}
	return false
}

// rule is the slice of RFC 5545 recurrence this planner needs: one of four
// frequencies, an interval, weekdays for the weekly case, and an end as
// either a last date or a number of occurrences. Both the ui's own repeating
// items and the RRULE lines in a subscribed calendar expand through it.
type rule struct {
	freq     string
	interval int
	weekdays []time.Weekday
	until    time.Time
	count    int
}

// a repeat that produced nothing for this many steps in a row is either
// malformed or so sparse it is not worth walking any further
const maxSteps = 4000

// occurrences returns the dates the rule fires on within [from, to], given it
// starts on start. All four are midnights in the same location.
func (r rule) occurrences(start, from, to time.Time) []time.Time {
	if r.interval < 1 {
		r.interval = 1
	}
	if to.Before(start) || to.Before(from) {
		return nil
	}

	var out []time.Time
	seen := 0

	// emit records one candidate date against both ends of the rule and the
	// window, and reports whether walking further can still produce anything
	emit := func(d time.Time) bool {
		if !r.until.IsZero() && d.After(r.until) {
			return false
		}
		if r.count > 0 && seen >= r.count {
			return false
		}
		seen++
		if !d.Before(from) && !d.After(to) {
			out = append(out, d)
		}
		return !d.After(to)
	}

	switch r.freq {
	case freqWeekly:
		days := r.weekdays
		if len(days) == 0 {
			days = []time.Weekday{start.Weekday()}
		}
		sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })

		// walk whole weeks from the one the series starts in, so BYDAY days
		// before the start weekday in that first week are skipped, not shifted
		week := start.AddDate(0, 0, -int(start.Weekday()))
		for step := 0; step < maxSteps; step++ {
			for _, wd := range days {
				d := week.AddDate(0, 0, int(wd))
				if d.Before(start) {
					continue
				}
				if !emit(d) {
					return out
				}
			}
			week = week.AddDate(0, 0, 7*r.interval)
			if week.After(to) {
				return out
			}
		}

	case freqMonthly, freqYearly:
		day := start.Day()
		for step := 0; step < maxSteps; step++ {
			var first time.Time
			if r.freq == freqMonthly {
				first = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location()).
					AddDate(0, step*r.interval, 0)
			} else {
				first = time.Date(start.Year()+step*r.interval, start.Month(), 1, 0, 0, 0, 0, start.Location())
			}
			// a 31st or a 29 february simply does not happen in a month that
			// is too short, the same way every calendar treats it
			if day <= daysIn(first) {
				d := first.AddDate(0, 0, day-1)
				if !emit(d) {
					return out
				}
			} else if first.After(to) {
				return out
			}
		}

	default:
		for step := 0; step < maxSteps; step++ {
			if !emit(start.AddDate(0, 0, step*r.interval)) {
				return out
			}
		}
	}

	return out
}

func daysIn(month time.Time) int {
	return time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location()).
		AddDate(0, 1, -1).Day()
}

// weekdaysFrom turns the ui's 0-6 numbers into weekdays, rejecting anything
// else so a bad payload cannot make a series that never fires
func weekdaysFrom(nums []int) ([]time.Weekday, error) {
	var out []time.Weekday
	seen := map[int]bool{}
	for _, n := range nums {
		if n < 0 || n > 6 {
			return nil, fmt.Errorf("%d is not a weekday", n)
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, time.Weekday(n))
	}
	return out, nil
}
