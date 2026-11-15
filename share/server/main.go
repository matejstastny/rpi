package main

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rpi/internal/human"
	"rpi/internal/ids"
	"rpi/internal/web"
)

//go:embed all:dist
var embedded embed.FS

type server struct {
	cfg   config
	store *store
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	cfg := loadConfig()

	ctx, stop := web.Signals()
	defer stop()

	st, err := newStore(cfg)
	if err != nil {
		log.Fatalf("share dir %s: %v", cfg.dir, err)
	}

	s := &server{cfg: cfg, store: st}

	web.Serve(ctx, cfg.listen, s.routes(), fmt.Sprintf("listening on %s, up to %d files in %s, %s max each",
		cfg.listen, cfg.maxFiles, cfg.dir, human.Bytes(cfg.maxFileSize)))
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/files", s.handleUpload)
	mux.HandleFunc("GET /api/files", s.handleList)
	mux.HandleFunc("DELETE /api/files/{id}", s.handleDrop)
	mux.HandleFunc("GET /dl/{id}/{name}", s.handleDownload)
	mux.Handle("GET /", web.Static(embedded))
	return mux
}

func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// a little slack over the configured cap for the multipart boilerplate
	// itself, so the real limit error comes from the copy below, not this wrapper
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.maxFileSize+1<<20)

	reader, err := r.MultipartReader()
	if err != nil {
		web.Fail(w, http.StatusBadRequest, errors.New("expected a multipart upload"))
		return
	}

	var part *multipart.Part
	for {
		p, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			web.Fail(w, http.StatusBadRequest, errors.New("no file in that upload"))
			return
		}
		if err != nil {
			web.Fail(w, http.StatusBadRequest, errors.New("could not read that upload"))
			return
		}
		if p.FormName() == "file" {
			part = p
			break
		}
		_ = p.Close()
	}

	name := sanitizeName(part.FileName())
	if name == "" {
		web.Fail(w, http.StatusBadRequest, errors.New("that file has no usable name"))
		return
	}

	id := ids.New(8)
	dir := filepath.Join(s.cfg.dir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		web.Fail(w, http.StatusInternalServerError, err)
		return
	}

	out, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		_ = os.RemoveAll(dir)
		web.Fail(w, http.StatusInternalServerError, err)
		return
	}

	written, err := io.Copy(out, part)
	_ = out.Close()
	if err != nil {
		_ = os.RemoveAll(dir)
		if err.Error() == "http: request body too large" {
			web.Fail(w, http.StatusRequestEntityTooLarge, fmt.Errorf("that is over the %s limit", human.Bytes(s.cfg.maxFileSize)))
			return
		}
		web.Fail(w, http.StatusBadRequest, errors.New("upload did not finish"))
		return
	}

	e := &entry{ID: id, Name: name, Size: written, Created: time.Now()}
	if err := s.store.add(e); err != nil {
		_ = os.RemoveAll(dir)
		web.Fail(w, http.StatusInternalServerError, err)
		return
	}
	s.store.enforceLimit(id)

	web.JSON(w, http.StatusCreated, e)
}

func (s *server) handleList(w http.ResponseWriter, r *http.Request) {
	web.JSON(w, http.StatusOK, map[string]any{
		"files": s.store.list(),
		"limit": s.cfg.maxFiles,
	})
}

func (s *server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !ids.Valid(id, 8) {
		web.Fail(w, http.StatusBadRequest, errors.New("that is not a file id"))
		return
	}

	e, ok := s.store.get(id)
	if !ok {
		web.Fail(w, http.StatusNotFound, errors.New("no such file"))
		return
	}

	path := e.path(s.cfg.dir)
	if _, err := os.Stat(path); err != nil {
		web.Fail(w, http.StatusGone, errors.New("the file is gone from disk"))
		return
	}

	w.Header().Set("Content-Disposition", web.Disposition(e.Name))
	http.ServeFile(w, r, path)
}

func (s *server) handleDrop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !ids.Valid(id, 8) {
		web.Fail(w, http.StatusBadRequest, errors.New("that is not a file id"))
		return
	}
	if err := s.store.drop(id); err != nil {
		web.Fail(w, http.StatusNotFound, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// sanitizeName keeps the upload's own name for display and the download
// filename, but never lets it escape its own upload directory
func sanitizeName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == ".." || name == "" || name == string(filepath.Separator) {
		return ""
	}
	return name
}
