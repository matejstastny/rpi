package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type store struct {
	path string
	mu   sync.RWMutex
	st   state
}

func newStore(cfg config) (*store, error) {
	if err := os.MkdirAll(cfg.dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.artDir, 0o755); err != nil {
		return nil, err
	}

	s := &store{path: cfg.statePath, st: state{Entries: []*entry{}, Settings: defaultSettings()}}

	raw, err := os.ReadFile(cfg.statePath)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		// first boot: take the seed if one shipped, otherwise stay empty
		if seed, serr := os.ReadFile(cfg.seedPath); serr == nil {
			log.Printf("no state yet, seeding from %s", cfg.seedPath)
			raw = seed
		} else {
			return s, s.save()
		}
	default:
		return nil, err
	}

	var loaded state
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return nil, fmt.Errorf("state is not valid json: %w", err)
	}

	if loaded.Entries == nil {
		loaded.Entries = []*entry{}
	}
	if loaded.Settings.TierDefinitions == nil {
		loaded.Settings.TierDefinitions = defaultSettings().TierDefinitions
	}

	seen := map[string]bool{}
	kept := make([]*entry, 0, len(loaded.Entries))
	for _, e := range loaded.Entries {
		if e == nil {
			continue
		}
		if e.ID == "" || seen[e.ID] {
			e.ID = newID()
		}
		seen[e.ID] = true
		if e.Added.IsZero() {
			e.Added = time.Now()
		}
		if err := e.normalize(); err != nil {
			log.Printf("dropping entry %s: %v", e.ID, err)
			continue
		}
		kept = append(kept, e)
	}
	loaded.Entries = kept

	s.st = loaded
	s.renumberAll()
	return s, s.save()
}

// snapshot hands back a copy safe to serialize while writers carry on
func (s *store) snapshot() state {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := state{Entries: make([]*entry, 0, len(s.st.Entries)), Settings: s.st.Settings}
	for _, e := range s.st.Entries {
		clone := *e
		clone.Genres = append([]string{}, e.Genres...)
		out.Entries = append(out.Entries, &clone)
	}
	return out
}

func (s *store) get(id string) (*entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.st.Entries {
		if e.ID == id {
			clone := *e
			return &clone, true
		}
	}
	return nil, false
}

func (s *store) add(e *entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e.ID = newID()
	e.Added = time.Now()
	e.Updated = e.Added
	// new things land at the top of their bucket, which is where you want to
	// see what you just added
	e.Rank = -1
	s.st.Entries = append(s.st.Entries, e)
	s.renumber(e.Kind, e.Status, e.Tier)
	return s.save()
}

// patch applies only the fields present in the request body, so the ui can
// send a single changed field without having to echo the whole entry back
func (s *store) patch(id string, fields map[string]json.RawMessage) (*entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, e := range s.st.Entries {
		if e.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, errors.New("no such entry")
	}

	merged := *s.st.Entries[idx]
	wasStatus, wasTier := merged.Status, merged.Tier

	body, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &merged); err != nil {
		return nil, errors.New("could not read those fields")
	}
	merged.ID = id
	merged.Added = s.st.Entries[idx].Added
	merged.Updated = time.Now()
	if err := merged.normalize(); err != nil {
		return nil, err
	}

	// moving bucket means the old rank is meaningless, so go to the top
	if merged.Status != wasStatus || merged.Tier != wasTier {
		merged.Rank = -1
	}

	s.st.Entries[idx] = &merged
	s.renumber(merged.Kind, merged.Status, merged.Tier)
	if merged.Status != wasStatus || merged.Tier != wasTier {
		s.renumber(merged.Kind, wasStatus, wasTier)
	}

	if err := s.save(); err != nil {
		return nil, err
	}
	clone := merged
	return &clone, nil
}

// move is the drag-and-drop path: put an entry in a bucket at a position
func (s *store) move(id, status, tier string, index int) (*entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var target *entry
	for _, e := range s.st.Entries {
		if e.ID == id {
			target = e
			break
		}
	}
	if target == nil {
		return nil, errors.New("no such entry")
	}

	fromStatus, fromTier := target.Status, target.Tier

	target.Status = status
	target.Tier = tier
	target.Updated = time.Now()
	if err := target.normalize(); err != nil {
		return nil, err
	}

	// renumber the destination with the entry spliced in at index
	bucket := s.bucket(target.Kind, target.Status, target.Tier)
	rest := make([]*entry, 0, len(bucket))
	for _, e := range bucket {
		if e.ID != id {
			rest = append(rest, e)
		}
	}
	if index < 0 || index > len(rest) {
		index = len(rest)
	}
	ordered := make([]*entry, 0, len(rest)+1)
	ordered = append(ordered, rest[:index]...)
	ordered = append(ordered, target)
	ordered = append(ordered, rest[index:]...)
	for i, e := range ordered {
		e.Rank = i
	}

	if fromStatus != target.Status || fromTier != target.Tier {
		s.renumber(target.Kind, fromStatus, fromTier)
	}

	if err := s.save(); err != nil {
		return nil, err
	}
	clone := *target
	return &clone, nil
}

func (s *store) drop(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, e := range s.st.Entries {
		if e.ID == id {
			kind, status, tier := e.Kind, e.Status, e.Tier
			s.st.Entries = append(s.st.Entries[:i], s.st.Entries[i+1:]...)
			s.renumber(kind, status, tier)
			return s.save()
		}
	}
	return errors.New("no such entry")
}

func (s *store) saveSettings(next settings) (settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if next.TierDefinitions == nil {
		next.TierDefinitions = s.st.Settings.TierDefinitions
	}
	s.st.Settings = next
	return s.st.Settings, s.save()
}

// bucket is every entry sharing a kind, status and tier, in rank order
func (s *store) bucket(kind, status, tier string) []*entry {
	out := []*entry{}
	for _, e := range s.st.Entries {
		if e.Kind == kind && e.Status == status && e.Tier == tier {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Rank < out[j].Rank })
	return out
}

func (s *store) renumber(kind, status, tier string) {
	for i, e := range s.bucket(kind, status, tier) {
		e.Rank = i
	}
}

func (s *store) renumberAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	type key struct{ kind, status, tier string }
	seen := map[key]bool{}
	for _, e := range s.st.Entries {
		seen[key{e.Kind, e.Status, e.Tier}] = true
	}
	for k := range seen {
		s.renumber(k.kind, k.status, k.tier)
	}
}

// save writes through a temp file in the same directory, so a crash mid-write
// leaves the previous watchlist intact rather than a truncated one
func (s *store) save() error {
	body, err := json.MarshalIndent(s.st, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()

	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Rename(name, s.path)
}

func newID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000"), ".", "")
	}
	return hex.EncodeToString(buf)
}
