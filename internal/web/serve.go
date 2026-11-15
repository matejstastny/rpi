// Package web is the http boilerplate every service on the fleet repeats: an
// embedded astro bundle, json request and response helpers, and a listener
// that shuts down on a signal instead of being killed mid-write.
package web

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// Signals returns a context cancelled by SIGINT or SIGTERM, which is how
// openrc's supervise-daemon stops these services.
func Signals() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

// Listen binds addr up front, for a service that has to know the port is
// really open before it starts its own background work.
func Listen(addr string) net.Listener {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	return l
}

// Serve runs h on addr until ctx is cancelled.
func Serve(ctx context.Context, addr string, h http.Handler, banner string) {
	ServeOn(ctx, Listen(addr), h, banner)
}

// ServeOn runs h on an already-open listener until ctx is cancelled, then
// drains in-flight requests for ten seconds. banner is the one startup line
// the service logs, which is also the first thing you read in
// /var/log/<service>/.
func ServeOn(ctx context.Context, l net.Listener, h http.Handler, banner string) {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		log.Print(banner)
		if err := srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Print("shutting down")

	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
