package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"rpi/internal/ids"
)

// the whole planner is one small json document: a few hundred items a year,
// rewritten atomically on every change. nothing here needs a database, and a
// file you can read with cat is worth more on a pi than one you cannot
type state struct {
	Categories []*Category `json:"categories"`
	Items      []*Item     `json:"items"`
	Series     []*Series   `json:"series"`
	Feeds      []*Feed     `json:"feeds"`
}

type store struct {
	path string
	mu   sync.Mutex
	st   state
}

var errNotFound = errors.New("no such thing")

// the first-run categories, in the palette the ui already paints with
var seedCategories = []*Category{
	{Name: "personal", Color: "#cba6f7"},
	{Name: "work", Color: "#89b4fa"},
	{Name: "health", Color: "#a6e3a1"},
	{Name: "social", Color: "#f5c2e7"},
}

func newStore(cfg config) (*store, error) {
	if err := os.MkdirAll(cfg.dir, 0o755); err != nil {
		return nil, err
	}
	s := &store{path: filepath.Join(cfg.dir, "planner.json")}

	raw, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		for _, c := range seedCategories {
			s.st.Categories = append(s.st.Categories, &Category{ID: ids.New(6), Name: c.Name, Color: c.Color})
		}
		if err := s.save(); err != nil {
			return nil, err
		}
		log.Printf("started a new planner at %s", s.path)
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(raw, &s.st); err != nil {
			return nil, err
		}
		s.settle()
		log.Printf("loaded %d items, %d repeats and %d calendars from %s",
			len(s.st.Items), len(s.st.Series), len(s.st.Feeds), s.path)
	}
	return s, nil
}

// settle fills in what a planner written by an older version of this service
// does not carry. an item that was pinned to its day is what is now called an
// event; everything else is work.
func (s *store) settle() {
	for _, i := range s.st.Items {
		if i.Kind == "" {
			i.Kind = kindTask
			if i.Pinned {
				i.Kind = kindEvent
			}
		}
		i.Pinned = false
	}
	for _, series := range s.st.Series {
		if series.Kind == "" {
			series.Kind = kindTask
		}
	}
}

// save writes through a temporary file so a power cut on a pi can only ever
// lose the newest change, never the whole planner
func (s *store) save() error {
	raw, err := json.MarshalIndent(&s.st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// read runs fn against the state under the lock. fn must not keep anything it
// is handed.
func (s *store) read(fn func(*state)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.st)
}

// write runs fn and persists the result, leaving the state untouched when fn
// fails so a rejected request cannot half-apply
func (s *store) write(fn func(*state) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(&s.st); err != nil {
		return err
	}
	return s.save()
}

func (st *state) item(id string) *Item {
	for _, i := range st.Items {
		if i.ID == id {
			return i
		}
	}
	return nil
}

func (st *state) series(id string) *Series {
	for _, s := range st.Series {
		if s.ID == id {
			return s
		}
	}
	return nil
}

func (st *state) feed(id string) *Feed {
	for _, f := range st.Feeds {
		if f.ID == id {
			return f
		}
	}
	return nil
}

func (st *state) category(id string) *Category {
	for _, c := range st.Categories {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func (st *state) dropItem(id string) bool {
	for n, i := range st.Items {
		if i.ID == id {
			st.Items = append(st.Items[:n], st.Items[n+1:]...)
			return true
		}
	}
	return false
}

// occurrence finds the item a series already materialised for one date, so
// opening the same week twice does not double it up
func (st *state) occurrence(series, date string) *Item {
	for _, i := range st.Items {
		if i.Series == series && i.Occurs == date {
			return i
		}
	}
	return nil
}

// imported finds the item a task calendar already produced for one event on
// one date, so refetching the same calendar never doubles anything up
func (st *state) imported(feed, source, date string) *Item {
	for _, i := range st.Items {
		if i.Feed == feed && i.Source == source && i.Occurs == date {
			return i
		}
	}
	return nil
}

func now() string { return time.Now().Format(time.RFC3339) }
