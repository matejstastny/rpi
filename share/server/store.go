package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"rpi/internal/human"
	"rpi/internal/ids"
)

type entry struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Created time.Time `json:"created"`
}

func (e entry) path(dir string) string {
	return filepath.Join(dir, e.ID, e.Name)
}

type store struct {
	cfg     config
	mu      sync.Mutex
	entries map[string]*entry
}

func newStore(cfg config) (*store, error) {
	s := &store{cfg: cfg, entries: map[string]*entry{}}
	if err := os.MkdirAll(cfg.dir, 0o755); err != nil {
		return nil, err
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *store) metaPath(id string) string {
	return filepath.Join(s.cfg.dir, id, "meta.json")
}

// load rebuilds the index from the upload dirs, which are the only source of
// truth. anything without a readable meta.json or its file is swept.
func (s *store) load() error {
	dirs, err := os.ReadDir(s.cfg.dir)
	if err != nil {
		return err
	}

	for _, d := range dirs {
		if !d.IsDir() || !ids.Valid(d.Name(), 8) {
			continue
		}
		dir := filepath.Join(s.cfg.dir, d.Name())

		raw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
		if err != nil {
			log.Printf("sweeping incomplete share dir %s", d.Name())
			_ = os.RemoveAll(dir)
			continue
		}

		var e entry
		if err := json.Unmarshal(raw, &e); err != nil || e.ID != d.Name() {
			log.Printf("sweeping unreadable share dir %s", d.Name())
			_ = os.RemoveAll(dir)
			continue
		}

		if _, err := os.Stat(e.path(s.cfg.dir)); err != nil {
			log.Printf("sweeping share dir with missing file %s", d.Name())
			_ = os.RemoveAll(dir)
			continue
		}

		s.entries[e.ID] = &e
	}

	log.Printf("share: %d of %d files kept", len(s.entries), s.cfg.maxFiles)
	return nil
}

func (s *store) save(e *entry) error {
	raw, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.cfg.dir, e.ID, "meta.json.tmp")
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.metaPath(e.ID))
}

func (s *store) add(e *entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.save(e); err != nil {
		return err
	}
	s.entries[e.ID] = e
	return nil
}

func (s *store) get(id string) (entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok {
		return entry{}, false
	}
	return *e, true
}

func (s *store) list() []entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]entry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

func (s *store) drop(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropLocked(id)
}

func (s *store) dropLocked(id string) error {
	if _, ok := s.entries[id]; !ok {
		return errors.New("no such file")
	}
	delete(s.entries, id)
	return os.RemoveAll(filepath.Join(s.cfg.dir, id))
}

// enforceLimit evicts the oldest files once more than maxFiles are kept. keep
// is the upload that just landed, so a fresh file is never the one evicted to
// make room for itself.
func (s *store) enforceLimit(keep string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for len(s.entries) > s.cfg.maxFiles {
		var victim *entry
		for _, e := range s.entries {
			if e.ID == keep {
				continue
			}
			if victim == nil || e.Created.Before(victim.Created) {
				victim = e
			}
		}
		if victim == nil {
			return
		}
		log.Printf("share: evicting %q (%s), over the %d file cap", victim.Name, human.Bytes(victim.Size), s.cfg.maxFiles)
		if err := s.dropLocked(victim.ID); err != nil {
			log.Printf("share: eviction failed for %s: %v", victim.ID, err)
			return
		}
	}
}
