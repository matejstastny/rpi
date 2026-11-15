package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"rpi/internal/human"
	"rpi/internal/ids"
	"rpi/internal/web"
)

//go:embed all:dist
var embedded embed.FS

type server struct {
	cfg    config
	store  *store
	runner *runner
	probes *probeCache
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	cfg := loadConfig()

	ctx, stop := web.Signals()
	defer stop()

	st, err := newStore(cfg)
	if err != nil {
		log.Fatalf("cache dir %s: %v", cfg.cacheDir, err)
	}

	s := &server{cfg: cfg, store: st, runner: newRunner(ctx, cfg, st), probes: newProbeCache()}

	web.Serve(ctx, cfg.listen, s.routes(), fmt.Sprintf("listening on %s, cache %s in %s, %d workers",
		cfg.listen, human.Bytes(cfg.cacheLimit), cfg.cacheDir, cfg.workers))
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/probe", s.handleProbe)
	mux.HandleFunc("POST /api/jobs", s.handleCreateJob)
	mux.HandleFunc("GET /api/jobs/{id}/events", s.handleEvents)
	mux.HandleFunc("GET /api/files", s.handleFiles)
	mux.HandleFunc("GET /api/files/{id}/download", s.handleDownload)
	mux.HandleFunc("DELETE /api/files/{id}", s.handleDrop)
	mux.Handle("GET /", web.Static(embedded))
	return mux
}

type probeResponse struct {
	URL      string  `json:"url"`
	Title    string  `json:"title"`
	Uploader string  `json:"uploader,omitempty"`
	Duration string  `json:"duration,omitempty"`
	Seconds  float64 `json:"seconds,omitempty"`
	Source   string  `json:"source,omitempty"`
	Heights  []int   `json:"heights"`
}

func (s *server) handleProbe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := web.Read(w, r, &body); err != nil {
		web.Fail(w, http.StatusBadRequest, err)
		return
	}

	target, err := cleanURL(body.URL)
	if err != nil {
		web.Fail(w, http.StatusBadRequest, err)
		return
	}

	meta, err := s.probes.get(r.Context(), s.cfg, target)
	if err != nil {
		web.Fail(w, http.StatusBadGateway, err)
		return
	}

	web.JSON(w, http.StatusOK, probeResponse{
		URL:      target,
		Title:    meta.Title,
		Uploader: meta.author(),
		Duration: durationLabel(meta.Duration),
		Seconds:  meta.Duration,
		Source:   meta.Extractor,
		Heights:  meta.heights(),
	})
}

var (
	allowedBitrates = map[int]bool{128: true, 192: true, 256: true, 320: true}
	allowedModes    = map[string]bool{"cache": true, "library": true, "both": true}
)

func (s *server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req jobRequest
	if err := web.Read(w, r, &req); err != nil {
		web.Fail(w, http.StatusBadRequest, err)
		return
	}

	target, err := cleanURL(req.URL)
	if err != nil {
		web.Fail(w, http.StatusBadRequest, err)
		return
	}
	req.URL = target

	if req.Format != "mp4" && req.Format != "mp3" {
		web.Fail(w, http.StatusBadRequest, errors.New("format has to be mp4 or mp3"))
		return
	}
	// the picker offers whatever the video actually has, which is rarely a round
	// number, so this only rules out nonsense
	if req.Format == "mp4" && (req.Height < 0 || req.Height > 4320) {
		web.Fail(w, http.StatusBadRequest, errors.New("that is not a sane resolution"))
		return
	}
	if req.Format == "mp3" {
		if !allowedBitrates[req.Bitrate] {
			web.Fail(w, http.StatusBadRequest, errors.New("that is not a bitrate on offer"))
			return
		}
		req.Height = 0
	}
	if req.Mode == "" {
		req.Mode = "cache"
	}
	if !allowedModes[req.Mode] {
		web.Fail(w, http.StatusBadRequest, errors.New("that is not a destination on offer"))
		return
	}

	job, err := s.runner.submit(req)
	if err != nil {
		web.Fail(w, http.StatusServiceUnavailable, err)
		return
	}

	web.JSON(w, http.StatusAccepted, map[string]string{"id": job.id})
}

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	job, ok := s.runner.job(r.PathValue("id"))
	if !ok {
		web.Fail(w, http.StatusNotFound, errors.New("no such download"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flush := http.NewResponseController(w)
	sentFinal := false
	send := func(ev event) error {
		raw, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		if ev.Stage == "done" || ev.Stage == "error" {
			sentFinal = true
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		return flush.Flush()
	}

	snapshot, finished := job.snapshot()
	if err := send(snapshot); err != nil || finished {
		return
	}

	updates, live := job.subscribe()
	if !live {
		final, _ := job.snapshot()
		if err := send(final); err != nil {
			return
		}
		return
	}
	defer job.unsubscribe(updates)

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			if flush.Flush() != nil {
				return
			}
		case ev, open := <-updates:
			if !open {
				// the job ended; the terminal event may have been dropped if this
				// client was slow, so fall back to the stored snapshot
				if !sentFinal {
					final, _ := job.snapshot()
					if err := send(final); err != nil {
						return
					}
				}
				return
			}
			if err := send(ev); err != nil {
				return
			}
		}
	}
}

func (s *server) handleFiles(w http.ResponseWriter, r *http.Request) {
	used, limit := s.store.usage()
	web.JSON(w, http.StatusOK, map[string]any{
		"files": s.store.list(),
		"cache": map[string]int64{"used": used, "limit": limit},
	})
}

func (s *server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !ids.Valid(id, 6) {
		web.Fail(w, http.StatusBadRequest, errors.New("that is not a file id"))
		return
	}

	e, ok := s.store.get(id)
	if !ok {
		web.Fail(w, http.StatusNotFound, errors.New("no such file"))
		return
	}

	path, err := e.path()
	if err != nil {
		web.Fail(w, http.StatusGone, err)
		return
	}

	w.Header().Set("Content-Type", contentType(e.Format))
	w.Header().Set("Content-Disposition", web.Disposition(e.Name))
	http.ServeFile(w, r, path)
}

func (s *server) handleDrop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !ids.Valid(id, 6) {
		web.Fail(w, http.StatusBadRequest, errors.New("that is not a file id"))
		return
	}
	if err := s.store.drop(id); err != nil {
		web.Fail(w, http.StatusNotFound, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func cleanURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("paste a link first")
	}
	if len(raw) > 2048 {
		return "", errors.New("that link is absurdly long")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("that does not parse as a link")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("only http and https links work")
	}
	if parsed.Host == "" {
		return "", errors.New("that link has no host")
	}
	return parsed.String(), nil
}

func contentType(format string) string {
	if format == "mp3" {
		return "audio/mpeg"
	}
	return "video/mp4"
}

// command wires yt-dlp up so a service restart stops it politely instead of
// leaving half a file and a stuck ffmpeg behind
func command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 10 * time.Second
	cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	return cmd
}
