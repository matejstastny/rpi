package main

import (
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	listen string
	dir    string
	// state.json lives here; art/ is the poster cache next to it
	statePath string
	artDir    string
	seedPath  string
	tmdbKey   string
}

func loadConfig() config {
	dir := env("WATCH_DIR", "/var/lib/watch")
	return config{
		listen:    env("WATCH_LISTEN", "127.0.0.1:8095"),
		dir:       dir,
		statePath: filepath.Join(dir, "state.json"),
		artDir:    filepath.Join(dir, "art"),
		// only read when state.json does not exist yet, so the pi comes up
		// with the markdown watchlist already in it
		seedPath: env("WATCH_SEED", "/etc/watch/seed.json"),
		tmdbKey:  strings.TrimSpace(os.Getenv("WATCH_TMDB_KEY")),
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
