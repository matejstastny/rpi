package main

import (
	"errors"
	"net/http"
	"strings"

	"rpi/internal/ids"
	"rpi/internal/web"
)

type repeatInput struct {
	Freq     string `json:"freq"`
	Weekdays []int  `json:"weekdays"`
	Until    string `json:"until"`
}

type itemInput struct {
	Title    string       `json:"title"`
	Notes    string       `json:"notes"`
	Date     string       `json:"date"`
	Start    string       `json:"start"`
	End      string       `json:"end"`
	Category string       `json:"category"`
	Kind     string       `json:"kind"`
	Repeat   *repeatInput `json:"repeat"`
}

// clean validates one submitted item into the fields that get stored
func (s *server) clean(in itemInput) (*Item, error) {
	name, err := title(in.Title)
	if err != nil {
		return nil, err
	}
	if _, err := parseDate(in.Date, s.cfg.zone); err != nil {
		return nil, err
	}
	start, err := clock(in.Start)
	if err != nil {
		return nil, err
	}
	end, err := clock(in.End)
	if err != nil {
		return nil, err
	}
	if start == "" {
		end = ""
	}
	if end != "" && end < start {
		return nil, errors.New("that ends before it starts")
	}

	notes := strings.TrimSpace(in.Notes)
	if len(notes) > 2000 {
		notes = notes[:2000]
	}

	kind := strings.TrimSpace(in.Kind)
	if !validItemKind(kind) {
		kind = kindTask
	}

	return &Item{
		Title:    name,
		Notes:    notes,
		Date:     in.Date,
		Start:    start,
		End:      end,
		Category: strings.TrimSpace(in.Category),
		Kind:     kind,
	}, nil
}

// handleCreate takes one item, or a whole repeating series when the form had
// its repeat switched on. The two are the same form in the ui, so they are
// the same endpoint here.
func (s *server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var in itemInput
	if err := web.Read(w, r, &in); err != nil {
		fail(w, err)
		return
	}

	item, err := s.clean(in)
	if err != nil {
		fail(w, err)
		return
	}

	err = s.store.write(func(st *state) error {
		if item.Category != "" && st.category(item.Category) == nil {
			return errors.New("no such category")
		}
		if in.Repeat == nil {
			item.ID = ids.New(8)
			item.Status = statusOpen
			item.Made = now()
			st.Items = append(st.Items, item)
			return nil
		}

		series, err := s.series(item, *in.Repeat)
		if err != nil {
			return err
		}
		st.Series = append(st.Series, series)
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.answer(w, r)
}

// series builds the repeat an item described. An end date is required: a
// repeat with no horizon is how a planner silently fills up with things
// nobody chose.
func (s *server) series(item *Item, in repeatInput) (*Series, error) {
	if !validFreq(in.Freq) {
		return nil, errors.New("that is not a repeat this understands")
	}

	from, err := parseDate(item.Date, s.cfg.zone)
	if err != nil {
		return nil, err
	}
	until, err := parseDate(in.Until, s.cfg.zone)
	if err != nil {
		return nil, errors.New("a repeat needs an end date")
	}
	if until.Before(from) {
		return nil, errors.New("that repeat ends before it starts")
	}

	days := in.Weekdays
	if in.Freq == freqWeekly {
		if len(days) == 0 {
			days = []int{int(from.Weekday())}
		}
		if _, err := weekdaysFrom(days); err != nil {
			return nil, err
		}
	} else {
		days = nil
	}

	return &Series{
		ID:       ids.New(8),
		Title:    item.Title,
		Notes:    item.Notes,
		Start:    item.Start,
		End:      item.End,
		Category: item.Category,
		Kind:     item.Kind,
		Freq:     in.Freq,
		Weekdays: days,
		From:     item.Date,
		Until:    in.Until,
		Made:     now(),
	}, nil
}

// handleEdit replaces the editable half of one item. Status, carry count and
// which series it came from are the server's to keep.
func (s *server) handleEdit(w http.ResponseWriter, r *http.Request) {
	var in itemInput
	if err := web.Read(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	edit, err := s.clean(in)
	if err != nil {
		fail(w, err)
		return
	}

	id := r.PathValue("id")
	err = s.store.write(func(st *state) error {
		item := st.item(id)
		if item == nil {
			return errNotFound
		}
		if edit.Category != "" && st.category(edit.Category) == nil {
			return errors.New("no such category")
		}
		item.Title, item.Notes = edit.Title, edit.Notes
		item.Date, item.Start, item.End = edit.Date, edit.Start, edit.End
		item.Category, item.Kind = edit.Category, edit.Kind
		// an event that becomes work starts out unfinished, and work that
		// becomes an event stops being something to finish
		if item.event() {
			item.Status, item.Resolved = statusOpen, ""
		}
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.answer(w, r)
}

// handleStatus is the whole point of the app: done, or honestly not done.
func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status string `json:"status"`
	}
	if err := web.Read(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if !validStatus(in.Status) {
		fail(w, errors.New("that is not a status"))
		return
	}

	id := r.PathValue("id")
	err := s.store.write(func(st *state) error {
		item := st.item(id)
		if item == nil {
			return errNotFound
		}
		if item.event() {
			return errors.New("an event is not something you finish")
		}
		item.Status = in.Status
		if in.Status == statusOpen {
			item.Resolved = ""
		} else {
			item.Resolved = now()
		}
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.answer(w, r)
}

// handleDelete removes one item. A single occurrence of a repeat leaves a
// skip behind, or the next sweep would helpfully put it back.
func (s *server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.write(func(st *state) error {
		item := st.item(id)
		if item == nil {
			return errNotFound
		}
		if item.Series != "" && item.Occurs != "" {
			if series := st.series(item.Series); series != nil {
				series.Skips = append(series.Skips, item.Occurs)
			}
		}
		if item.Feed != "" && item.Source != "" {
			if feed := st.feed(item.Feed); feed != nil {
				feed.Skips = append(feed.Skips, item.Source+"@"+item.Occurs)
			}
		}
		st.dropItem(id)
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.answer(w, r)
}

// handleEditSeries changes a repeat. The date in the payload is the series'
// own start, not the occurrence the edit was opened from: an edit made from
// the third week of a repeat must not quietly move its beginning to there and
// take every occurrence in between with it.
//
// What already happened keeps the shape it had when it happened; only
// untouched future occurrences are dropped, to be rebuilt from the new
// schedule on the next load.
func (s *server) handleEditSeries(w http.ResponseWriter, r *http.Request) {
	var in itemInput
	if err := web.Read(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if in.Repeat == nil {
		fail(w, errors.New("a repeat needs its schedule"))
		return
	}
	edit, err := s.clean(in)
	if err != nil {
		fail(w, err)
		return
	}
	next, err := s.series(edit, *in.Repeat)
	if err != nil {
		fail(w, err)
		return
	}

	id := r.PathValue("id")
	cut := stamp(today(s.cfg.zone))
	err = s.store.write(func(st *state) error {
		series := st.series(id)
		if series == nil {
			return errNotFound
		}
		next.ID, next.Made, next.Skips = series.ID, series.Made, series.Skips
		*series = *next

		kept := st.Items[:0]
		for _, item := range st.Items {
			if item.Series == id && item.open() && item.Date >= cut {
				continue
			}
			kept = append(kept, item)
		}
		st.Items = kept
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.answer(w, r)
}

// handleDeleteSeries stops a repeat and takes its future with it, leaving
// everything already done or dropped as the record of what happened
func (s *server) handleDeleteSeries(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cut := stamp(today(s.cfg.zone))

	err := s.store.write(func(st *state) error {
		if st.series(id) == nil {
			return errNotFound
		}
		kept := make([]*Series, 0, len(st.Series))
		for _, series := range st.Series {
			if series.ID != id {
				kept = append(kept, series)
			}
		}
		st.Series = kept

		items := st.Items[:0]
		for _, item := range st.Items {
			if item.Series == id && item.open() && item.Date >= cut {
				continue
			}
			items = append(items, item)
		}
		st.Items = items
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.answer(w, r)
}
