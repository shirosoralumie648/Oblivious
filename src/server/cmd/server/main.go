package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"
	"time"

	stdhttp "net/http"

	"oblivious/server/internal/config"
	"oblivious/server/internal/db"
	serverhttp "oblivious/server/internal/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	server := serverhttp.NewServer(cfg, database)
	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("server listening on %s env=%s", server.Addr, cfg.Env)
		serverErrors <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("server shutdown: %v", err)
		}
	}
}
