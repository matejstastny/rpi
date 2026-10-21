package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *store {
	t.Helper()

	dir := t.TempDir()
	s, err := newStore(config{
		dir:       dir,
		statePath: filepath.Join(dir, "state.json"),
		artDir:    filepath.Join(dir, "art"),
		seedPath:  filepath.Join(dir, "missing.json"),
	})
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	return s
}

func addRanked(t *testing.T, s *store, title, tier string) *entry {
	t.Helper()

	e := &entry{Kind: "anime", Status: statusRanked, Title: title, Tier: tier}
	if err := e.normalize(); err != nil {
		t.Fatalf("normalize %s: %v", title, err)
	}
	if err := s.add(e); err != nil {
		t.Fatalf("add %s: %v", title, err)
	}
	return e
}

func titlesIn(s *store, status, tier string) []string {
	out := []string{}
	for _, e := range s.bucket("anime", status, tier) {
		out = append(out, e.Title)
	}
	return out
}

func TestAddPutsNewEntriesOnTop(t *testing.T) {
	s := newTestStore(t)
	addRanked(t, s, "first", "A")
	addRanked(t, s, "second", "A")

	got := strings.Join(titlesIn(s, statusRanked, "A"), ",")
	if got != "second,first" {
		t.Fatalf("want second,first; got %s", got)
	}
}

func TestMoveInsertsAtIndexAndRenumbersBothBuckets(t *testing.T) {
	s := newTestStore(t)
	a := addRanked(t, s, "a", "A")
	addRanked(t, s, "b", "A")
	addRanked(t, s, "c", "B")

	// a from the bottom of A to the middle of B
	if _, err := s.move(a.ID, statusRanked, "B", 1); err != nil {
		t.Fatalf("move: %v", err)
	}

	if got := strings.Join(titlesIn(s, statusRanked, "B"), ","); got != "c,a" {
		t.Fatalf("destination: want c,a; got %s", got)
	}
	if got := strings.Join(titlesIn(s, statusRanked, "A"), ","); got != "b" {
		t.Fatalf("source: want b; got %s", got)
	}

	// the bucket it left must not be holding a gap at rank 1
	for _, e := range s.bucket("anime", statusRanked, "A") {
		if e.Rank != 0 {
			t.Fatalf("source rank not compacted: %s is %d", e.Title, e.Rank)
		}
	}
}

func TestMoveOutOfRankedClearsTheTier(t *testing.T) {
	s := newTestStore(t)
	a := addRanked(t, s, "a", "S")

	moved, err := s.move(a.ID, statusQueue, "S", 0)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if moved.Tier != "" {
		t.Fatalf("queued entry kept tier %q", moved.Tier)
	}
}

func TestPatchKeepsUntouchedFields(t *testing.T) {
	s := newTestStore(t)
	a := addRanked(t, s, "a", "A")

	updated, err := s.patch(a.ID, map[string]json.RawMessage{"note": json.RawMessage(`"rewatch this"`)})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if updated.Title != "a" || updated.Tier != "A" {
		t.Fatalf("patch clobbered fields: %+v", updated)
	}
	if updated.Note != "rewatch this" {
		t.Fatalf("note not applied: %q", updated.Note)
	}
}

func TestNormalizeRejectsAnEmptyTitle(t *testing.T) {
	e := &entry{Kind: "anime", Status: statusQueue, Title: "   "}
	if err := e.normalize(); err == nil {
		t.Fatal("expected an error for a blank title")
	}
}

func TestExportCarriesInstructionsTiersAndLists(t *testing.T) {
	s := newTestStore(t)
	addRanked(t, s, "Witch Hat Atelier", "N")

	queued := &entry{Kind: "anime", Status: statusQueue, Title: "Angel Beats"}
	if err := queued.normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if err := s.add(queued); err != nil {
		t.Fatalf("add: %v", err)
	}

	if _, err := s.saveSettings(settings{
		ClaudeInstructions: "she cries at everything by evan call",
		TierDefinitions:    defaultSettings().TierDefinitions,
	}); err != nil {
		t.Fatalf("saveSettings: %v", err)
	}

	out := exportMarkdown(s.snapshot(), "all")

	for _, want := range []string{
		"she cries at everything by evan call",
		"**N**: Top 5 shows ever",
		"| N | Witch Hat Atelier | NA |",
		"- Angel Beats",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("export missing %q:\n%s", want, out)
		}
	}
}

func TestExportCanBeScopedToOneLibrary(t *testing.T) {
	s := newTestStore(t)
	addRanked(t, s, "Witch Hat Atelier", "N")

	film := &entry{Kind: "movie", Status: statusRanked, Title: "Arrival", Tier: "S"}
	if err := film.normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if err := s.add(film); err != nil {
		t.Fatalf("add: %v", err)
	}

	out := exportMarkdown(s.snapshot(), "movie")
	if strings.Contains(out, "Witch Hat Atelier") {
		t.Fatal("movie-scoped export leaked the anime library")
	}
	if !strings.Contains(out, "Arrival") {
		t.Fatal("movie-scoped export dropped the movie")
	}
}

func TestPreviewOnlyAllowsKnownPosterHosts(t *testing.T) {
	cases := map[string]bool{
		"https://s4.anilist.co/file/x.jpg":   true,
		"https://image.tmdb.org/t/p/w500/x":  true,
		"https://evil.example.com/x.jpg":     false,
		"http://s4.anilist.co/file/x.jpg":    false,
		"https://127.0.0.1/admin":            false,
		"file:///etc/passwd":                 false,
		"https://s4.anilist.co.evil.com/x.j": false,
	}

	for raw, want := range cases {
		if _, ok := previewAllowed(raw); ok != want {
			t.Fatalf("previewAllowed(%q) = %v, want %v", raw, ok, want)
		}
	}
}

func TestSearchRankingPrefersTheObviousTitle(t *testing.T) {
	results := []result{
		{Title: "Ouch, Chou Chou", AltTitle: "Aitata Bocchi", weight: 300},
		{Title: "BOCCHI THE ROCK! Recap", weight: 2000},
		{Title: "BOCCHI THE ROCK!", weight: 400000},
	}

	rank(results, "bocchi the rock")

	if results[0].Title != "BOCCHI THE ROCK!" {
		t.Fatalf("want BOCCHI THE ROCK! first; got %s", results[0].Title)
	}
}

func TestStripHTMLFlattensAniListDescriptions(t *testing.T) {
	got := stripHTML("A girl<br>\n<br><i>and</i> her &quot;band&quot;")
	if got != "A girl\n\nand her \"band\"" {
		t.Fatalf("got %q", got)
	}
}
