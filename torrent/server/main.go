package main

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"rpi/internal/web"
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

	ctx, stop := web.Signals()
	defer stop()

	s := &server{cfg: cfg, tx: newTorrentClient(cfg)}

	web.Serve(ctx, cfg.listen, s.routes(), fmt.Sprintf("listening on %s, talking to transmission at %s",
		cfg.listen, cfg.transmissionURL))
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/torrents", s.handleList)
	mux.HandleFunc("POST /api/torrents", s.handleAdd)
	mux.HandleFunc("POST /api/torrents/{id}/pause", s.handlePause)
	mux.HandleFunc("POST /api/torrents/{id}/resume", s.handleResume)
	mux.HandleFunc("DELETE /api/torrents/{id}", s.handleRemove)
	mux.Handle("GET /", web.Static(embedded))
	return mux
}

func (s *server) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := s.tx.list(r.Context())
	if err != nil {
		web.Fail(w, http.StatusBadGateway, err)
		return
	}
	web.JSON(w, http.StatusOK, map[string]any{"torrents": list})
}

type addRequest struct {
	Magnet   string `json:"magnet"`
	Metainfo string `json:"metainfo"`
}

func (s *server) handleAdd(w http.ResponseWriter, r *http.Request) {
	var req addRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		web.Fail(w, http.StatusBadRequest, errors.New("could not read that request"))
		return
	}
	req.Magnet = strings.TrimSpace(req.Magnet)
	req.Metainfo = strings.TrimSpace(req.Metainfo)
	if req.Metainfo != "" {
		if _, err := base64.StdEncoding.DecodeString(req.Metainfo); err != nil {
			web.Fail(w, http.StatusBadRequest, errors.New("that .torrent file did not decode"))
			return
		}
	}

	added, err := s.tx.add(r.Context(), req.Magnet, req.Metainfo)
	if err != nil {
		web.Fail(w, http.StatusBadGateway, err)
		return
	}
	web.JSON(w, http.StatusCreated, added)
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
		web.Fail(w, http.StatusBadRequest, errors.New("that is not a torrent id"))
		return
	}
	if err := s.tx.setActive(r.Context(), id, active); err != nil {
		web.Fail(w, http.StatusBadGateway, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleRemove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		web.Fail(w, http.StatusBadRequest, errors.New("that is not a torrent id"))
		return
	}
	deleteData := r.URL.Query().Get("deleteData") == "true"
	if err := s.tx.remove(r.Context(), id, deleteData); err != nil {
		web.Fail(w, http.StatusBadGateway, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
