package main

import (
	"context"
	"log"
	"net/http"
	"oblivious/server/internal/observability"
	"oblivious/server/pkg/config"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadObservabilityConfig()

	reporter := observability.NewMemoryReporter()
	consumer := observability.NewChannelConsumer(reporter)

	kafkaBrokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(kafkaBrokers) == 0 || kafkaBrokers[0] == "" {
		kafkaBrokers = []string{"localhost:9092"}
	}

	workflowHandler := observability.NewWorkflowEventHandler()
	workflowConsumer := observability.NewEventConsumer(kafkaBrokers, "workflow.events", "observability-group", workflowHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := workflowConsumer.Start(ctx); err != nil {
			log.Printf("workflow consumer error: %v", err)
		}
	}()

	router := gin.Default()
	registerRoutes(router, cfg, reporter)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	log.Printf("Observability service listening on :%s db_mode=%s", cfg.Port, cfg.DBMode)
	log.Println("Consumer ready to receive channel and workflow events")
	_ = consumer

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down observability service...")
	cancel()
	workflowConsumer.Close()
	srv.Shutdown(context.Background())
}

func registerRoutes(router *gin.Engine, cfg *config.ObservabilityConfig, reporter *observability.MemoryReporter) {
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "observability", "db_mode": cfg.DBMode})
	})

	router.GET("/metrics", func(c *gin.Context) {
		events := reporter.Events()
		c.JSON(200, gin.H{"events": events})
	})

	router.POST("/subscribe", func(c *gin.Context) {
		var req struct {
			ChannelServiceAddr string `json:"channel_service_addr"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "subscription endpoint ready"})
	})
}
