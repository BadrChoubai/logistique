//go:build ignore

// This example demonstrates wiring logistique into an existing application as
// a drop-in API gateway with rate limiting middleware.
//
// It is modelled after a real integration (the "resonance" project) where
// logistique replaces hand-rolled reverse-proxy setup. The key change is
// that route registration, proxying, and cross-cutting middleware all move
// into the Gateway, and the application only needs to supply service config
// and call g.Handler().
//
// To run (from the repo root):
//
//	go run ./examples/gateway/main.go
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BadrChoubai/logistique"
	"github.com/BadrChoubai/logistique/middleware"
)

// AppConfig represents the host application's own configuration type.
// In a real project this would be loaded from JSON/YAML/env; here we
// keep it inline so the example is self-contained.
type AppConfig struct {
	Port string

	// Embed logistique's config so gateway settings live in one place and
	// flow through cleanly without duplication.
	Gateway logistique.Config

	// Upstream service base URLs.
	ServiceOneURL string
	ServiceTwoURL string
}

func defaultConfig() AppConfig {
	return AppConfig{
		Port: ":8080",
		Gateway: logistique.Config{
			RateLimit: logistique.RateLimitConfig{
				RequestsPerSecond: 10,
				Burst:             20,
			},
		},
		ServiceOneURL: envOr("SERVICE_ONE_URL", "http://localhost:8081"),
		ServiceTwoURL: envOr("SERVICE_TWO_URL", "http://localhost:8082"),
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Stdout); err != nil {
		log.Fatalln(err)
	}
}

func run(ctx context.Context, _ io.Writer) error {
	cfg := defaultConfig()

	// --- Build the gateway ---------------------------------------------------
	//
	// Each ServiceConfig declares one upstream: the path prefix the gateway
	// will match on, the upstream base URL to proxy to, and an optional
	// Rewrite prefix to prepend to the outbound path after stripping Prefix.
	//
	// Before logistique this was:
	//
	//   rtr.AddRoute("", "/api/service-one/",   newServiceProxy(cfg.JournalServiceURL))
	//   rtr.AddRoute("", "/api/service-two/", newServiceProxy(cfg.ListeningServiceURL))
	//
	gw, err := logistique.New(
		cfg.Gateway,
		logistique.ServiceConfig{
			Prefix: "/api/service-one/",
			Target: cfg.ServiceOneURL,
			// Rewrite: "/v1", // uncomment to prepend a version prefix upstream
		},
		logistique.ServiceConfig{
			Prefix: "/api/service-two/",
			Target: cfg.ServiceTwoURL,
		},
	)
	if err != nil {
		return err
	}

	// --- Attach middleware ----------------------------------------------------
	//
	// Middleware is applied in registration order (outermost first).
	// RateLimit uses the same RateLimitConfig embedded in cfg.Gateway, so
	// there is a single source of truth for the token-bucket parameters.
	//
	// Upstream 429 responses (e.g. from X-Rate-Limit-Remaining headers) are
	// passed through to the caller unchanged; the gateway does not attempt to
	// sync its own buckets from upstream signals.
	gw.Use(middleware.RateLimit(cfg.Gateway.RateLimit))

	// --- Wire into http.Server -----------------------------------------------
	//
	// gw.Handler() returns the fully composed handler: routes + middleware
	// chain. Drop it wherever the application previously used rtr.Handler()
	// or http.DefaultServeMux.
	srv := &http.Server{
		Addr:         cfg.Port,
		Handler:      gw.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		log.Printf("gateway listening on %s", cfg.Port)
		errChan <- srv.ListenAndServe()
	}()

	select {
	case err := <-errChan:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil

	case <-ctx.Done():
		log.Println("shutting down gateway")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
