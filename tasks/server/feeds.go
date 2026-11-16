package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// at some point a calendar is not a calendar any more
const maxFeedSize = 10 << 20

// calendar is one subscription as last fetched: the feed's own colour and
// name alongside its parsed events, so drawing a week never has to reach back
// into the store while the store is already locked
type calendar struct {
	feed    Feed
	entries []*entry
}

// feeds owns every subscribed ics url. They are refetched on a timer and
// never written into the planner itself: an imported event is something that
// is happening to you, not something you keep.
type feeds struct {
	cfg    config
	store  *store
	client *http.Client

	mu    sync.RWMutex
	cache map[string]*calendar
}

func newFeeds(cfg config, st *store) *feeds {
	return &feeds{
		cfg:    cfg,
		store:  st,
		client: &http.Client{Timeout: 30 * time.Second},
		cache:  map[string]*calendar{},
	}
}

// run refreshes everything on start and then on the configured interval
func (f *feeds) run(ctx context.Context) {
	f.refreshAll(ctx)

	tick := time.NewTicker(f.cfg.interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			f.refreshAll(ctx)
		}
	}
}

func (f *feeds) refreshAll(ctx context.Context) {
	var ids []string
	f.store.read(func(st *state) {
		for _, feed := range st.Feeds {
			ids = append(ids, feed.ID)
		}
	})
	for _, id := range ids {
		if err := f.refresh(ctx, id); err != nil {
			log.Printf("calendar %s: %v", id, err)
		}
	}
}

// refresh fetches one feed and swaps its parsed copy in. The fetch happens
// outside both locks, so a slow calendar cannot stall the week.
func (f *feeds) refresh(ctx context.Context, id string) error {
	var feed Feed
	found := false
	f.store.read(func(st *state) {
		if got := st.feed(id); got != nil {
			feed, found = *got, true
		}
	})
	if !found {
		return errNotFound
	}

	entries, err := f.fetch(ctx, feed.URL)

	f.mu.Lock()
	if err == nil {
		f.cache[id] = &calendar{feed: feed, entries: entries}
	} else if held, ok := f.cache[id]; ok {
		// a failed refresh keeps the last good copy on screen rather than
		// blanking a week because the wifi blinked
		held.feed = feed
	}
	f.mu.Unlock()

	var learned bool
	writeErr := f.store.write(func(st *state) error {
		got := st.feed(id)
		if got == nil {
			return errNotFound
		}
		got.Synced = now()
		if err != nil {
			got.Error = err.Error()
			return nil
		}
		got.Error = ""
		got.Events = len(entries)
		if got.Kind == feedCanvas {
			learned = got.learn(courses(entries))
			feed = *got
		}
		return nil
	})

	// a course discovered during this sync has to reach the cached copy too,
	// or its colour and its switch would not exist until the next refetch
	if learned {
		f.restate(feed)
	}
	return writeErr
}

func (f *feeds) fetch(ctx context.Context, raw string) ([]*entry, error) {
	target, err := feedURL(raw)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/calendar, text/plain")

	res, err := f.client.Do(req)
	if err != nil {
		return nil, errors.New("could not reach that calendar")
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the calendar answered %s", res.Status)
	}
	return parseICS(io.LimitReader(res.Body, maxFeedSize), f.cfg.zone)
}

// feedURL normalises what gets pasted in. webcal is the same fetch over
// https, and it is what every calendar app hands you when you ask to share.
func feedURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if after, ok := strings.CutPrefix(raw, "webcal://"); ok {
		raw = "https://" + after
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("that is not an http or webcal calendar url")
	}
	return u.String(), nil
}

// between expands the subscriptions read as events into the days they cover
func (f *feeds) between(from, to time.Time) map[string][]Event {
	f.mu.RLock()
	defer f.mu.RUnlock()

	out := map[string][]Event{}
	for _, cal := range f.cache {
		if works(cal.feed.Kind) {
			continue
		}
		for date, events := range expand(cal.entries, from, to, &cal.feed) {
			out[date] = append(out[date], events...)
		}
	}
	for _, events := range out {
		sort.SliceStable(events, func(a, b int) bool { return events[a].Start < events[b].Start })
	}
	return out
}

// due lists every occurrence of the subscriptions read as work, for the week
// to turn into real items
func (f *feeds) due(from, to time.Time) []due {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var out []due
	for _, cal := range f.cache {
		if !works(cal.feed.Kind) {
			continue
		}
		out = append(out, plan(cal.entries, from, to, &cal.feed)...)
	}
	return out
}

// courses lists every course code a canvas feed's entries mention
func courses(entries []*entry) []string {
	var out []string
	seen := map[string]bool{}
	for _, e := range entries {
		if _, code := splitCourse(e.title); code != "" && !seen[code] {
			seen[code] = true
			out = append(out, code)
		}
	}
	return out
}

// restate swaps in a feed's own settings after they are edited. The cache
// holds its own copy of them so that drawing a week never has to reach back
// into the store, which means a rename, a recolour or a change of kind has to
// be handed over explicitly rather than waiting for the next refetch.
func (f *feeds) restate(feed Feed) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if cal, ok := f.cache[feed.ID]; ok {
		cal.feed = feed
	}
}

func (f *feeds) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.cache, id)
}
