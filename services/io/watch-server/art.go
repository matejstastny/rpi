package main

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// posters are fetched once and kept on disk, so the browser never talks to
// anilist or tmdb itself and the library still renders with the pi offline
type artCache struct {
	dir   string
	group sync.Mutex
}

const maxArtBytes = 8 << 20

var artClient = &http.Client{Timeout: 20 * time.Second}

func newArtCache(dir string) *artCache { return &artCache{dir: dir} }

func (a *artCache) path(id, ext string) string {
	return filepath.Join(a.dir, id+ext)
}

// find returns the cached file for an entry, whatever extension it landed with
func (a *artCache) find(id string) (string, bool) {
	for _, ext := range []string{".jpg", ".png", ".webp", ".avif"} {
		p := a.path(id, ext)
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			return p, true
		}
	}
	return "", false
}

func (a *artCache) fetch(id, remote string) (string, error) {
	if remote == "" {
		return "", errors.New("no poster for that entry")
	}

	parsed, err := url.Parse(remote)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", errors.New("that poster url is not usable")
	}

	// one fetch at a time: a fresh library load would otherwise open a few
	// dozen connections at once on a pi
	a.group.Lock()
	defer a.group.Unlock()

	if p, ok := a.find(id); ok {
		return p, nil
	}

	req, err := http.NewRequest(http.MethodGet, remote, nil)
	if err != nil {
		return "", errors.New("that poster url is not usable")
	}
	req.Header.Set("User-Agent", userAgent)

	res, err := artClient.Do(req)
	if err != nil {
		return "", errors.New("could not fetch that poster")
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return "", errors.New("the poster host refused that request")
	}

	ext := extFor(res.Header.Get("Content-Type"), parsed.Path)
	if ext == "" {
		return "", errors.New("that url is not an image")
	}

	if err := os.MkdirAll(a.dir, 0o755); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(a.dir, ".art-*")
	if err != nil {
		return "", err
	}
	name := tmp.Name()

	_, err = io.Copy(tmp, io.LimitReader(res.Body, maxArtBytes))
	closeErr := tmp.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(name)
		return "", errors.New("the poster did not download fully")
	}

	final := a.path(id, ext)
	if err := os.Chmod(name, 0o644); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	if err := os.Rename(name, final); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return final, nil
}

func (a *artCache) drop(id string) {
	for _, ext := range []string{".jpg", ".png", ".webp", ".avif"} {
		_ = os.Remove(a.path(id, ext))
	}
}

func extFor(contentType, path string) string {
	switch {
	case strings.HasPrefix(contentType, "image/jpeg"):
		return ".jpg"
	case strings.HasPrefix(contentType, "image/png"):
		return ".png"
	case strings.HasPrefix(contentType, "image/webp"):
		return ".webp"
	case strings.HasPrefix(contentType, "image/avif"):
		return ".avif"
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return ".jpg"
	case ".png":
		return ".png"
	case ".webp":
		return ".webp"
	case ".avif":
		return ".avif"
	}
	return ""
}

// previewHosts is the allowlist for the search dialog's thumbnails. the ui
// never loads a provider url directly, so nothing in the browser talks to
// anilist or tmdb, and this stays a two-host proxy rather than an open one.
var previewHosts = map[string]bool{
	"s4.anilist.co":       true,
	"img.anili.st":        true,
	"image.tmdb.org":      true,
	"media.kitsu.io":      true,
	"cdn.myanimelist.net": true,
}

func previewAllowed(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" {
		return "", false
	}
	if !previewHosts[strings.ToLower(parsed.Hostname())] {
		return "", false
	}
	return parsed.String(), true
}
