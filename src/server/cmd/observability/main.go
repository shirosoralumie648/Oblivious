package main

import (
	"context"
	"log"
	"net/http"
	"oblivious/server/internal/config"
	"oblivious/server/internal/observability"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	router := gin.Default()
	registerRoutes(router, cfg)

	port := os.Getenv("OBSERVABILITY_PORT")
	if port == "" {
		port = "8090"
	}

	srv := &http.Server{Addr: ":" + port, Handler: router}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down observability service...")
	srv.Shutdown(context.Background())
}

func registerRoutes(router *gin.Engine, cfg config.Config) {
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	router.GET("/metrics", func(c *gin.Context) {
		reporter := observability.NewMemoryReporter()
		events := reporter.Events()
		c.JSON(200, gin.H{"events": events})
	})
}
