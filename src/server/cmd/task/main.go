package main

import (
	"context"
	"log"
	"net/http"
	"oblivious/server/internal/task"
	"oblivious/server/pkg/config"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadTaskConfig()

	kafkaBrokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(kafkaBrokers) == 0 || kafkaBrokers[0] == "" {
		kafkaBrokers = []string{"localhost:9092"}
	}

	workflowHandler := task.NewWorkflowEventHandler()
	workflowConsumer := task.NewEventConsumer(kafkaBrokers, "workflow.events", "task-group", workflowHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := workflowConsumer.Start(ctx); err != nil {
			log.Printf("workflow consumer error: %v", err)
		}
	}()

	router := gin.Default()
	svc := task.New(cfg)
	svc.RegisterRoutes(router)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	log.Println("Task service listening, consumer ready for workflow events")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down task service...")
	cancel()
	workflowConsumer.Close()
	srv.Shutdown(context.Background())
}
