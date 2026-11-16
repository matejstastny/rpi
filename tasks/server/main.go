package main

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"rpi/internal/web"
)

//go:embed all:dist
var embedded embed.FS

type server struct {
	cfg   config
	store *store
	feeds *feeds
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	cfg := loadConfig()

	ctx, stop := web.Signals()
	defer stop()

	st, err := newStore(cfg)
	if err != nil {
		log.Fatalf("planner dir %s: %v", cfg.dir, err)
	}

	s := &server{cfg: cfg, store: st, feeds: newFeeds(cfg, st)}
	go s.feeds.run(ctx)

	web.Serve(ctx, cfg.listen, s.routes(), fmt.Sprintf(
		"listening on %s, planning in %s, calendars every %s", cfg.listen, cfg.zone, cfg.interval))
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/week", s.handleWeek)

	mux.HandleFunc("POST /api/items", s.handleCreate)
	mux.HandleFunc("PATCH /api/items/{id}", s.handleEdit)
	mux.HandleFunc("POST /api/items/{id}/status", s.handleStatus)
	mux.HandleFunc("DELETE /api/items/{id}", s.handleDelete)

	mux.HandleFunc("PATCH /api/series/{id}", s.handleEditSeries)
	mux.HandleFunc("DELETE /api/series/{id}", s.handleDeleteSeries)

	mux.HandleFunc("POST /api/categories", s.handleAddCategory)
	mux.HandleFunc("PATCH /api/categories/{id}", s.handleEditCategory)
	mux.HandleFunc("DELETE /api/categories/{id}", s.handleDeleteCategory)

	mux.HandleFunc("POST /api/feeds", s.handleAddFeed)
	mux.HandleFunc("PATCH /api/feeds/{id}", s.handleEditFeed)
	mux.HandleFunc("DELETE /api/feeds/{id}", s.handleDeleteFeed)
	mux.HandleFunc("POST /api/feeds/{id}/sync", s.handleSyncFeed)

	mux.Handle("GET /", web.Static(embedded))
	return mux
}

// handleWeek is the only read in the app: every mutation answers with the
// same shape, so the ui never has to merge a response into what it already
// holds and can never drift from the server
func (s *server) handleWeek(w http.ResponseWriter, r *http.Request) {
	start, err := s.askedWeek(r)
	if err != nil {
		web.Fail(w, http.StatusBadRequest, err)
		return
	}
	web.JSON(w, http.StatusOK, s.week(start))
}

// askedWeek reads the week a request is about, defaulting to the one
// happening now
func (s *server) askedWeek(r *http.Request) (time.Time, error) {
	raw := r.URL.Query().Get("week")
	if raw == "" {
		raw = r.URL.Query().Get("start")
	}
	if raw == "" {
		return monday(today(s.cfg.zone)), nil
	}
	d, err := parseDate(raw, s.cfg.zone)
	if err != nil {
		return time.Time{}, err
	}
	return monday(d), nil
}

// answer replies with the week the ui is looking at, after a change landed
func (s *server) answer(w http.ResponseWriter, r *http.Request) {
	start, err := s.askedWeek(r)
	if err != nil {
		web.Fail(w, http.StatusBadRequest, err)
		return
	}
	web.JSON(w, http.StatusOK, s.week(start))
}

func fail(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, errNotFound) {
		status = http.StatusNotFound
	}
	web.Fail(w, status, err)
}
