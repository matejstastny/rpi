package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"rpi/internal/web"
)

//go:embed all:dist
var embedded embed.FS

type server struct {
	cfg  config
	cat  catalog
	vm   *vmClient
	prb  *prober
	intg *collector

	mu           sync.RWMutex
	hosts        []hostStats
	services     []serviceStatus
	integrations integrations
	tailnet      *tailnet
	updated      time.Time
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	cfg := loadConfig()
	cat := loadCatalog(cfg.catalog)

	ctx, stop := web.Signals()
	defer stop()

	s := &server{
		cfg:  cfg,
		cat:  cat,
		vm:   newVMClient(cfg.vmURL),
		prb:  newProber(cfg.probeTimeout),
		intg: newCollector(cfg),
	}

	// warm the caches that do not depend on this process before the listener
	// opens, so the first page load is never the one waiting on
	// victoria-metrics
	var warm sync.WaitGroup
	warm.Add(2)
	go func() { defer warm.Done(); s.refreshMetrics(ctx) }()
	go func() { defer warm.Done(); s.refreshServices(ctx) }()
	warm.Wait()

	go loop(ctx, cfg.metricsEvery, s.refreshMetrics)
	go loop(ctx, cfg.serviceEvery, s.refreshServices)

	listener := web.Listen(cfg.listen)

	// the catalog probes dash itself, so the first sweep only makes sense once
	// this process is accepting connections; otherwise every restart paints a
	// red "dash is down" for one probe interval
	go func() {
		s.refreshProbes(ctx)
		loop(ctx, cfg.probeEvery, s.refreshProbes)
	}()

	web.ServeOn(ctx, listener, s.routes(), fmt.Sprintf("listening on %s, watching %s across %d services",
		cfg.listen, strings.Join(cat.hostNames(), ", "), len(cat.Services)))
}

func loop(ctx context.Context, every time.Duration, fn func(context.Context)) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn(ctx)
		}
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.Handle("GET /", web.Static(embedded))
	return mux
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	updated := s.updated
	s.mu.RUnlock()
	web.JSON(w, http.StatusOK, map[string]any{"ok": true, "updated": updated})
}

func (s *server) handleStats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	resp := statsResponse{
		GeneratedAt:  s.updated,
		Window:       window{Lookback: s.cfg.lookback.String(), Step: s.cfg.step.String()},
		Hosts:        s.hosts,
		Services:     s.services,
		Integrations: s.integrations,
		Tailnet:      s.tailnet,
	}
	s.mu.RUnlock()

	resp.Fleet = summarise(resp.Hosts, resp.Services)

	web.JSON(w, http.StatusOK, resp)
}

func (s *server) refreshMetrics(ctx context.Context) {
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	hosts := collectHosts(reqCtx, s.vm, s.cat, s.cfg)

	// the tailnet address is the one host fact that is neither in the catalog
	// nor in node-exporter, so it gets stitched in from tailscale here
	if net, err := collectTailnet(reqCtx, s.cfg.tailscaleBin); err == nil {
		byName := map[string]string{}
		for _, p := range net.Peers {
			byName[p.Name] = p.IP
		}
		for i := range hosts {
			hosts[i].Tailnet = byName[hosts[i].Name]
		}
		s.mu.Lock()
		s.tailnet = net
		s.mu.Unlock()
	} else {
		logOnce("tailscale", err)
	}

	s.mu.Lock()
	s.hosts = hosts
	s.updated = time.Now()
	s.mu.Unlock()
}

func (s *server) refreshProbes(ctx context.Context) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	services := s.prb.probeAll(reqCtx, s.cat.Services)

	s.mu.Lock()
	s.services = services
	s.mu.Unlock()
}

func (s *server) refreshServices(ctx context.Context) {
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	found := s.intg.collect(reqCtx)

	// a single integration timing out should not blank a panel that was fine
	// a moment ago, so each one only overwrites when it actually came back
	s.mu.Lock()
	defer s.mu.Unlock()
	if found.AdGuard != nil {
		s.integrations.AdGuard = found.AdGuard
	}
	if found.Transmission != nil {
		s.integrations.Transmission = found.Transmission
	}
	if found.Jellyfin != nil {
		s.integrations.Jellyfin = found.Jellyfin
	}
	if found.Ytdl != nil {
		s.integrations.Ytdl = found.Ytdl
	}
	if found.Share != nil {
		s.integrations.Share = found.Share
	}
	if found.Victoria != nil {
		s.integrations.Victoria = found.Victoria
	}
}
