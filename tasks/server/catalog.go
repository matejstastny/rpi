package main

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"rpi/internal/ids"
	"rpi/internal/web"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type categoryInput struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (in categoryInput) clean() (string, string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", "", errors.New("a category needs a name")
	}
	if len(name) > 40 {
		name = name[:40]
	}
	color := strings.TrimSpace(in.Color)
	if !hexColor.MatchString(color) {
		return "", "", errors.New("that is not a colour")
	}
	return name, strings.ToLower(color), nil
}

func (s *server) handleAddCategory(w http.ResponseWriter, r *http.Request) {
	var in categoryInput
	if err := web.Read(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	name, color, err := in.clean()
	if err != nil {
		fail(w, err)
		return
	}

	err = s.store.write(func(st *state) error {
		if len(st.Categories) >= 24 {
			return errors.New("that is enough categories")
		}
		st.Categories = append(st.Categories, &Category{ID: ids.New(6), Name: name, Color: color})
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.answer(w, r)
}

func (s *server) handleEditCategory(w http.ResponseWriter, r *http.Request) {
	var in categoryInput
	if err := web.Read(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	name, color, err := in.clean()
	if err != nil {
		fail(w, err)
		return
	}

	id := r.PathValue("id")
	err = s.store.write(func(st *state) error {
		category := st.category(id)
		if category == nil {
			return errNotFound
		}
		category.Name, category.Color = name, color
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.answer(w, r)
}

// handleDeleteCategory drops a category and uncategorises whatever wore it,
// rather than leaving items pointing at something that no longer exists
func (s *server) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.write(func(st *state) error {
		if st.category(id) == nil {
			return errNotFound
		}
		kept := make([]*Category, 0, len(st.Categories))
		for _, c := range st.Categories {
			if c.ID != id {
				kept = append(kept, c)
			}
		}
		st.Categories = kept

		for _, item := range st.Items {
			if item.Category == id {
				item.Category = ""
			}
		}
		for _, series := range st.Series {
			if series.Category == id {
				series.Category = ""
			}
		}
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.answer(w, r)
}

type feedInput struct {
	Name    string  `json:"name"`
	URL     string  `json:"url"`
	Color   string  `json:"color"`
	Kind    string  `json:"kind"`
	Classes []Class `json:"classes"`
}

func (in feedInput) clean() (name, color, kind string) {
	name = strings.TrimSpace(in.Name)
	if name == "" {
		name = "calendar"
	}
	if len(name) > 40 {
		name = name[:40]
	}
	color = strings.ToLower(strings.TrimSpace(in.Color))
	if !hexColor.MatchString(color) {
		color = "#89b4fa"
	}
	kind = strings.TrimSpace(in.Kind)
	if !validKind(kind) {
		kind = feedEvents
	}
	return name, color, kind
}

func (s *server) handleAddFeed(w http.ResponseWriter, r *http.Request) {
	var in feedInput
	if err := web.Read(w, r, &in); err != nil {
		fail(w, err)
		return
	}

	target, err := feedURL(in.URL)
	if err != nil {
		fail(w, err)
		return
	}
	name, color, kind := in.clean()

	feed := &Feed{ID: ids.New(6), Name: name, URL: target, Color: color, Kind: kind, Made: now()}
	err = s.store.write(func(st *state) error {
		if len(st.Feeds) >= 12 {
			return errors.New("that is enough calendars")
		}
		st.Feeds = append(st.Feeds, feed)
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}

	// fetched before answering, so a bad url shows up as this feed's own
	// error line instead of an empty week with no explanation
	if err := s.feeds.refresh(r.Context(), feed.ID); err != nil {
		fail(w, err)
		return
	}
	s.answer(w, r)
}

// handleEditFeed renames, recolours, switches a course off, or flips a
// calendar between being context and being work. Demoting it back to events takes its untouched
// future with it, the same as unsubscribing would: saying it is not work any
// more and leaving the work on the board would only draw everything twice.
// What was already done or written off stays as the record of what happened.
func (s *server) handleEditFeed(w http.ResponseWriter, r *http.Request) {
	var in feedInput
	if err := web.Read(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	name, color, kind := in.clean()

	id := r.PathValue("id")
	cut := stamp(today(s.cfg.zone))

	var edited Feed
	err := s.store.write(func(st *state) error {
		feed := st.feed(id)
		if feed == nil {
			return errNotFound
		}
		demoted := works(feed.Kind) && !works(kind)
		feed.Name, feed.Color, feed.Kind = name, color, kind

		// courses are merged by code rather than replaced, so a sync that
		// discovers a new one while this dialog is open does not lose it
		hidden := map[string]bool{}
		for _, in := range in.Classes {
			got := feed.class(in.Code)
			if got == nil {
				continue
			}
			if hexColor.MatchString(in.Color) {
				got.Color = strings.ToLower(in.Color)
			}
			if in.Hidden && !got.Hidden {
				hidden[got.Code] = true
			}
			got.Hidden = in.Hidden
		}
		// a calendar switched to canvas after the fact still has items
		// carrying the course in their title, so take it out now rather than
		// waiting for each one to be reissued
		if feed.Kind == feedCanvas {
			var codes []string
			for _, item := range st.Items {
				if item.Feed != id || item.Class != "" {
					continue
				}
				if name, code := splitCourse(item.Title); code != "" {
					item.Title, item.Class = name, code
					codes = append(codes, code)
				}
			}
			feed.learn(codes)
		}
		edited = *feed

		if demoted || len(hidden) > 0 {
			items := st.Items[:0]
			for _, item := range st.Items {
				gone := demoted || hidden[item.Class]
				if item.Feed == id && gone && item.open() && item.Date >= cut {
					continue
				}
				items = append(items, item)
			}
			st.Items = items
		}
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}

	s.feeds.restate(edited)
	s.answer(w, r)
}

// handleDeleteFeed unsubscribes, taking the untouched future with it. What
// was already done or written off stays as the record of what happened.
func (s *server) handleDeleteFeed(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cut := stamp(today(s.cfg.zone))

	err := s.store.write(func(st *state) error {
		if st.feed(id) == nil {
			return errNotFound
		}

		items := st.Items[:0]
		for _, item := range st.Items {
			if item.Feed == id && item.open() && item.Date >= cut {
				continue
			}
			items = append(items, item)
		}
		st.Items = items

		kept := make([]*Feed, 0, len(st.Feeds))
		for _, f := range st.Feeds {
			if f.ID != id {
				kept = append(kept, f)
			}
		}
		st.Feeds = kept
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.feeds.forget(id)
	s.answer(w, r)
}

func (s *server) handleSyncFeed(w http.ResponseWriter, r *http.Request) {
	if err := s.feeds.refresh(r.Context(), r.PathValue("id")); err != nil {
		fail(w, err)
		return
	}
	s.answer(w, r)
}
