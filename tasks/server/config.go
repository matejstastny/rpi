package main

import (
	"log"
	"time"

	"rpi/internal/conf"
)

type config struct {
	listen string
	dir    string
	// the week is drawn in one timezone: the planner's, not the browser's, so
	// a phone in another country still shows the same days
	zone     *time.Location
	interval time.Duration
}

func loadConfig() config {
	name := conf.String("TASKS_TZ", "Local")
	zone, err := time.LoadLocation(name)
	if err != nil {
		log.Printf("unknown timezone %q, falling back to the system one: %v", name, err)
		zone = time.Local
	}

	return config{
		listen:   conf.String("TASKS_LISTEN", "127.0.0.1:8094"),
		dir:      conf.String("TASKS_DIR", "/var/lib/tasks"),
		zone:     zone,
		interval: conf.Duration("TASKS_FEED_INTERVAL", 30*time.Minute),
	}
}
