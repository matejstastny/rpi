package main

import (
	"time"

	"rpi/internal/conf"
)

type config struct {
	listen  string
	vmURL   string
	catalog string

	lookback time.Duration
	step     time.Duration

	metricsEvery time.Duration
	probeEvery   time.Duration
	serviceEvery time.Duration

	probeTimeout time.Duration

	adguardURL  string
	adguardUser string
	adguardPass string

	transmissionURL string
	jellyfinURL     string
	ytdlURL         string
	shareURL        string

	tailscaleBin string
}

func loadConfig() config {
	c := config{
		listen:  conf.String("DASH_LISTEN", "127.0.0.1:8092"),
		vmURL:   conf.String("DASH_VM_URL", "http://127.0.0.1:8428"),
		catalog: conf.String("DASH_CATALOG", "/etc/dash/catalog.json"),

		adguardURL:  conf.String("DASH_ADGUARD_URL", ""),
		adguardUser: conf.String("DASH_ADGUARD_USER", ""),
		adguardPass: conf.String("DASH_ADGUARD_PASS", ""),

		transmissionURL: conf.String("DASH_TRANSMISSION_URL", "http://127.0.0.1:9091/transmission/rpc"),
		jellyfinURL:     conf.String("DASH_JELLYFIN_URL", "http://127.0.0.1:8096"),
		ytdlURL:         conf.String("DASH_YTDL_URL", "http://127.0.0.1:8090"),
		shareURL:        conf.String("DASH_SHARE_URL", "http://127.0.0.1:8091"),

		tailscaleBin: conf.String("DASH_TAILSCALE_BIN", "/usr/bin/tailscale"),
	}

	c.lookback = conf.Duration("DASH_LOOKBACK", 3*time.Hour)
	c.step = conf.Duration("DASH_STEP", 2*time.Minute)

	// node-exporter is scraped every 15s, so refreshing the metric cache any
	// faster than that only re-reads the same points
	c.metricsEvery = conf.Duration("DASH_METRICS_EVERY", 15*time.Second)
	c.probeEvery = conf.Duration("DASH_PROBE_EVERY", 20*time.Second)
	c.serviceEvery = conf.Duration("DASH_SERVICE_EVERY", 20*time.Second)
	c.probeTimeout = conf.Duration("DASH_PROBE_TIMEOUT", 2*time.Second)

	return c
}
