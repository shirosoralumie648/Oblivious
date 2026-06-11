package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/config"
	"oblivious/server/internal/relay"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if !cfg.RelayEnabled {
		log.Fatal("Relay service requires RELAY_ENABLED=true")
	}

	gin.SetMode(gin.ReleaseMode)
	pool := relay.NewChannelPool()
	relayCfg := &relay.Config{
		Pool:       pool,
		Production: cfg.Env == "production",
	}

	r, err := relay.NewRelay(relayCfg)
	if err != nil {
		log.Fatalf("Failed to create relay: %v", err)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r.Engine(),
	}

	go func() {
		log.Printf("Relay service listening on port %d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Relay service stopped")
}
