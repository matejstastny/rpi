package main

import (
	"os"

	"rpi/internal/conf"
)

type config struct {
	listen      string
	cacheDir    string
	videoDir    string
	audioDir    string
	cacheLimit  int64
	maxFileSize string
	ytdlp       string
	ffmpeg      string
	cookies     string
	workers     int
}

func loadConfig() config {
	return config{
		listen:      conf.String("YTDL_LISTEN", "127.0.0.1:8090"),
		cacheDir:    conf.String("YTDL_CACHE_DIR", "/var/lib/ytdl/cache"),
		videoDir:    conf.String("YTDL_VIDEO_DIR", "/srv/media/video"),
		audioDir:    conf.String("YTDL_AUDIO_DIR", "/srv/media/music"),
		maxFileSize: conf.String("YTDL_MAX_FILESIZE", "10G"),
		ytdlp:       conf.String("YTDL_YTDLP", "/usr/bin/yt-dlp"),
		ffmpeg:      conf.String("YTDL_FFMPEG", "/usr/bin/ffmpeg"),
		cookies:     conf.String("YTDL_COOKIES", "/var/lib/ytdl/cookies.txt"),
		cacheLimit:  conf.Bytes("YTDL_CACHE_LIMIT", 20_000_000_000),
		workers:     conf.Int("YTDL_WORKERS", 2, 1),
	}
}

// cookieArgs is skipped whenever the file is absent, which is the default and
// keeps ordinary videos working with nothing extra to configure. YouTube's age
// and bot gates are the reason it exists at all: yt-dlp needs a real logged-in
// session to get past them, and a headless box has no browser to pull one from.
func cookieArgs(cfg config) []string {
	if cfg.cookies == "" {
		return nil
	}
	if _, err := os.Stat(cfg.cookies); err != nil {
		return nil
	}
	return []string{"--cookies", cfg.cookies}
}
