package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

//go:embed all:dist
var embedded embed.FS

type server struct {
	cfg   config
	store *store
	art   *artCache
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	cfg := loadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := newStore(cfg)
	if err != nil {
		log.Fatalf("state %s: %v", cfg.statePath, err)
	}

	s := &server{cfg: cfg, store: st, art: newArtCache(cfg.artDir)}

	srv := &http.Server{
		Addr:              cfg.listen,
		Handler:           s.routes(),
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		lookup := "anilist only"
		if cfg.tmdbKey != "" {
			lookup = "anilist + tmdb"
		}
		log.Printf("listening on %s, %d entries in %s, lookup: %s",
			cfg.listen, len(st.snapshot().Entries), cfg.statePath, lookup)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Print("shutting down")

	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("POST /api/entries", s.handleCreate)
	mux.HandleFunc("PATCH /api/entries/{id}", s.handlePatch)
	mux.HandleFunc("POST /api/entries/{id}/move", s.handleMove)
	mux.HandleFunc("DELETE /api/entries/{id}", s.handleDelete)
	mux.HandleFunc("PUT /api/settings", s.handleSettings)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/export", s.handleExport)
	mux.HandleFunc("GET /api/art/{id}", s.handleArt)
	mux.HandleFunc("GET /api/preview", s.handlePreview)
	mux.Handle("GET /", s.static())
	return mux
}

func (s *server) static() http.Handler {
	dist, err := fs.Sub(embedded, "dist")
	if err != nil {
		log.Fatalf("embedded ui: %v", err)
	}
	files := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_astro/") || strings.HasPrefix(r.URL.Path, "/fonts/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	snap := s.store.snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"entries":  snap.Entries,
		"settings": snap.Settings,
		"meta": map[string]any{
			"kinds":    kinds,
			"statuses": statuses,
			"tiers":    tiers,
			"tmdb":     s.cfg.tmdbKey != "",
		},
	})
}

func (s *server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var e entry
	if err := decode(w, r, &e); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := e.normalize(); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.add(&e); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	// warm the poster so the card is not blank on the first render
	if e.Poster != "" {
		go func(id, poster string) {
			if _, err := s.art.fetch(id, poster); err != nil {
				log.Printf("poster for %s: %v", id, err)
			}
		}(e.ID, e.Poster)
	}
	writeJSON(w, http.StatusCreated, &e)
}

func (s *server) handlePatch(w http.ResponseWriter, r *http.Request) {
	var fields map[string]json.RawMessage
	if err := decode(w, r, &fields); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	// these are the store's business, not the client's
	for _, locked := range []string{"id", "rank", "added", "updated"} {
		delete(fields, locked)
	}

	updated, err := s.store.patch(r.PathValue("id"), fields)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}

	if _, changed := fields["poster"]; changed {
		s.art.drop(updated.ID)
		if updated.Poster != "" {
			go func(id, poster string) {
				if _, err := s.art.fetch(id, poster); err != nil {
					log.Printf("poster for %s: %v", id, err)
				}
			}(updated.ID, updated.Poster)
		}
	}

	writeJSON(w, http.StatusOK, updated)
}

type moveRequest struct {
	Status string `json:"status"`
	Tier   string `json:"tier"`
	Index  int    `json:"index"`
}

func (s *server) handleMove(w http.ResponseWriter, r *http.Request) {
	var req moveRequest
	if err := decode(w, r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if !validStatus(req.Status) {
		fail(w, http.StatusBadRequest, errors.New("that is not a status"))
		return
	}
	if req.Status == statusRanked && !validTier(strings.ToUpper(req.Tier)) {
		fail(w, http.StatusBadRequest, errors.New("that is not a tier"))
		return
	}

	updated, err := s.store.move(r.PathValue("id"), req.Status, strings.ToUpper(req.Tier), req.Index)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.drop(id); err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	s.art.drop(id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var next settings
	if err := decode(w, r, &next); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	saved, err := s.store.saveSettings(next)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "anime"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	results, err := s.search(ctx, kind, r.URL.Query().Get("q"))
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *server) handleExport(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	body := exportMarkdown(s.store.snapshot(), kind)

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="watchlist.md"`)
	}
	_, _ = w.Write([]byte(body))
}

// handleArt serves the cached poster, fetching it on the first miss. the ui
// only ever points at this, never at the provider's cdn.
func (s *server) handleArt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	e, ok := s.store.get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	path, found := s.art.find(id)
	if !found {
		fetched, err := s.art.fetch(id, e.Poster)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		path = fetched
	}

	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeFile(w, r, path)
}

// handlePreview streams a provider thumbnail for the search dialog. nothing
// is cached here: only the poster of whatever gets added is worth keeping, and
// that goes through handleArt instead.
func (s *server) handlePreview(w http.ResponseWriter, r *http.Request) {
	target, ok := previewAllowed(r.URL.Query().Get("url"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	req.Header.Set("User-Agent", userAgent)

	res, err := artClient.Do(req)
	if err != nil {
		http.Error(w, "upstream poster host is unreachable", http.StatusBadGateway)
		return
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		http.NotFound(w, r)
		return
	}

	ct := res.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = io.Copy(w, io.LimitReader(res.Body, maxArtBytes))
}

func decode(w http.ResponseWriter, r *http.Request, into any) error {
	defer func() { _ = r.Body.Close() }()

	limited := http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(limited).Decode(into); err != nil {
		return errors.New("that request body is not valid json")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func fail(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
