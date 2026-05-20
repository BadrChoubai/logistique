package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BadrChoubai/logistique"
)

// This example proxies requests to JSONPlaceholder, a free fake REST API.
//
// Run it:
//
//	go run main.go
//
// Then try:
//
//	curl http://localhost:8080/api/posts/
//	curl http://localhost:8080/api/posts/1
//	curl http://localhost:8080/api/users/
//	curl http://localhost:8080/api/users/1
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	gw, err := logistique.New(
		logistique.Config{},
		logistique.ServiceConfig{
			Prefix:  "/api/posts/",
			Target:  "https://jsonplaceholder.typicode.com",
			Rewrite: "/posts",
		},
	)

	if err != nil {
		log.Fatalln("failed to create gateway:", err)
	}

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      gw.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.ListenAndServe()
	}()

	select {
	case err := <-errChan:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalln("server error:", err)
		}
	case <-ctx.Done():
		log.Println("shutting down")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Fatalln("shutdown error:", err)
		}
	}

	os.Exit(0)
}
