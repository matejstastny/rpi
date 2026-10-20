package main

import (
	"os"
	"strings"
)

type config struct {
	listen          string
	transmissionURL string
}

func loadConfig() config {
	return config{
		listen:          env("TORRENT_LISTEN", "127.0.0.1:8093"),
		transmissionURL: env("TORRENT_TRANSMISSION_URL", "http://127.0.0.1:9091/transmission/rpc"),
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
