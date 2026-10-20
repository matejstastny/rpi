package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//go:embed all:dist
var embedded embed.FS

type server struct {
	cfg config
	tx  *torrentClient
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	cfg := loadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s := &server{cfg: cfg, tx: newTorrentClient(cfg)}

	srv := &http.Server{
		Addr:              cfg.listen,
		Handler:           s.routes(),
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("listening on %s, talking to transmission at %s", cfg.listen, cfg.transmissionURL)
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
	mux.HandleFunc("GET /api/torrents", s.handleList)
	mux.HandleFunc("POST /api/torrents", s.handleAdd)
	mux.HandleFunc("POST /api/torrents/{id}/pause", s.handlePause)
	mux.HandleFunc("POST /api/torrents/{id}/resume", s.handleResume)
	mux.HandleFunc("DELETE /api/torrents/{id}", s.handleRemove)
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
		// astro fingerprints everything under _astro, so those can be pinned
		// forever while the page itself must always be revalidated
		if strings.HasPrefix(r.URL.Path, "/_astro/") || strings.HasPrefix(r.URL.Path, "/fonts/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

func (s *server) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := s.tx.list(r.Context())
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"torrents": list})
}

type addRequest struct {
	Magnet   string `json:"magnet"`
	Metainfo string `json:"metainfo"`
}

func (s *server) handleAdd(w http.ResponseWriter, r *http.Request) {
	var req addRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, errors.New("could not read that request"))
		return
	}
	req.Magnet = strings.TrimSpace(req.Magnet)
	req.Metainfo = strings.TrimSpace(req.Metainfo)
	if req.Metainfo != "" {
		if _, err := base64.StdEncoding.DecodeString(req.Metainfo); err != nil {
			fail(w, http.StatusBadRequest, errors.New("that .torrent file did not decode"))
			return
		}
	}

	added, err := s.tx.add(r.Context(), req.Magnet, req.Metainfo)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusCreated, added)
}

func (s *server) handlePause(w http.ResponseWriter, r *http.Request) {
	s.setActive(w, r, false)
}

func (s *server) handleResume(w http.ResponseWriter, r *http.Request) {
	s.setActive(w, r, true)
}

func (s *server) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, errors.New("that is not a torrent id"))
		return
	}
	if err := s.tx.setActive(r.Context(), id, active); err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleRemove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, errors.New("that is not a torrent id"))
		return
	}
	deleteData := r.URL.Query().Get("deleteData") == "true"
	if err := s.tx.remove(r.Context(), id, deleteData); err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func fail(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
