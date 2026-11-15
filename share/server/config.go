package main

import "rpi/internal/conf"

type config struct {
	listen      string
	dir         string
	maxFiles    int
	maxFileSize int64
}

func loadConfig() config {
	return config{
		listen:      conf.String("SHARE_LISTEN", "127.0.0.1:8091"),
		dir:         conf.String("SHARE_DIR", "/var/lib/share/files"),
		maxFiles:    conf.Int("SHARE_MAX_FILES", 10, 1),
		maxFileSize: conf.Bytes("SHARE_MAX_FILESIZE", 5_000_000_000),
	}
}
