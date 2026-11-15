package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// the three libraries the ui is split into
var kinds = []string{"anime", "series", "movie"}

// a ranked entry sits in a tier; everything else sits in a flat ordered list
const (
	statusWatching = "watching"
	statusRanked   = "ranked"
	statusQueue    = "queue"
	statusBacklog  = "backlog"
	statusSkipped  = "skipped"
)

var statuses = []string{statusWatching, statusRanked, statusQueue, statusBacklog, statusSkipped}

// N is the top-five shelf, so entries there keep a meaningful order; the rest
// are unordered sets the ui sorts alphabetically unless dragged
var tiers = []string{"N", "S", "A", "B", "C", "D"}

type entry struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Status string `json:"status"`

	Title    string `json:"title"`
	AltTitle string `json:"altTitle,omitempty"`

	Tier string `json:"tier,omitempty"`
	Rank int    `json:"rank"`

	// progress and the guess at where it will land, for things being watched
	Progress  string `json:"progress,omitempty"`
	Predicted string `json:"predicted,omitempty"`

	Finished string `json:"finished,omitempty"`
	Note     string `json:"note,omitempty"`

	Poster   string   `json:"poster,omitempty"`
	Year     int      `json:"year,omitempty"`
	Episodes int      `json:"episodes,omitempty"`
	Format   string   `json:"format,omitempty"`
	Studio   string   `json:"studio,omitempty"`
	Genres   []string `json:"genres,omitempty"`
	Synopsis string   `json:"synopsis,omitempty"`
	Score    int      `json:"score,omitempty"`

	Provider   string `json:"provider,omitempty"`
	ProviderID string `json:"providerId,omitempty"`
	Link       string `json:"link,omitempty"`

	Added   time.Time `json:"added"`
	Updated time.Time `json:"updated"`
}

type settings struct {
	ClaudeInstructions string            `json:"claudeInstructions"`
	TierDefinitions    map[string]string `json:"tierDefinitions"`
}

type state struct {
	Entries  []*entry `json:"entries"`
	Settings settings `json:"settings"`
}

func defaultSettings() settings {
	return settings{
		TierDefinitions: map[string]string{
			"N": "Top 5 shows ever. My beloved.",
			"S": "The peak of everything. I wish to die to relive this again.",
			"A": "Amazing series. I cried tons & would rewatch.",
			"B": "Really good series. Loved them.",
			"C": "Okay, I'd watch something else though.",
			"D": "Did not like these.",
		},
	}
}

func validKind(k string) bool   { return contains(kinds, k) }
func validStatus(s string) bool { return contains(statuses, s) }
func validTier(t string) bool   { return contains(tiers, t) }

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// normalize fills in what the ui may have left out and rejects what cannot be
// repaired, so nothing invalid ever reaches the store
func (e *entry) normalize() error {
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		return errors.New("that needs a title")
	}

	if e.Kind == "" {
		e.Kind = "anime"
	}
	if !validKind(e.Kind) {
		return fmt.Errorf("%q is not one of anime, series, movie", e.Kind)
	}

	if e.Status == "" {
		e.Status = statusQueue
	}
	if !validStatus(e.Status) {
		return fmt.Errorf("%q is not a status", e.Status)
	}

	e.Tier = strings.ToUpper(strings.TrimSpace(e.Tier))
	if e.Status == statusRanked {
		if e.Tier == "" {
			e.Tier = "B"
		}
		if !validTier(e.Tier) {
			return fmt.Errorf("%q is not a tier", e.Tier)
		}
	} else {
		// a tier only means something on a ranked entry; a prediction is the
		// equivalent on something still being watched
		e.Tier = ""
	}

	e.Predicted = strings.ToUpper(strings.TrimSpace(e.Predicted))
	if e.Predicted != "" && !validTier(e.Predicted) {
		e.Predicted = ""
	}

	e.AltTitle = strings.TrimSpace(e.AltTitle)
	e.Progress = strings.TrimSpace(e.Progress)
	e.Note = strings.TrimSpace(e.Note)
	e.Finished = strings.TrimSpace(e.Finished)
	e.Provider = strings.TrimSpace(e.Provider)
	if e.Provider == "" {
		e.Provider = "manual"
	}

	if e.Genres == nil {
		e.Genres = []string{}
	}
	return nil
}
