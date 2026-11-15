package main

import "rpi/internal/conf"

type config struct {
	listen          string
	transmissionURL string
}

func loadConfig() config {
	return config{
		listen:          conf.String("TORRENT_LISTEN", "127.0.0.1:8093"),
		transmissionURL: conf.String("TORRENT_TRANSMISSION_URL", "http://127.0.0.1:9091/transmission/rpc"),
	}
}
