package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	taskv1 "oblivious/server/internal/grpc/taskv1"
	internaltask "oblivious/server/internal/task"
	"oblivious/server/internal/task/scheduler"
	"oblivious/server/pkg/config"
	pkgtask "oblivious/server/pkg/task"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.LoadTaskConfig()

	kafkaBrokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(kafkaBrokers) == 0 || kafkaBrokers[0] == "" {
		kafkaBrokers = []string{"localhost:9092"}
	}

	workflowHandler := internaltask.NewWorkflowEventHandler()
	workflowConsumer := internaltask.NewEventConsumer(kafkaBrokers, "workflow.events", "task-group", workflowHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := workflowConsumer.Start(ctx); err != nil {
			log.Printf("workflow consumer error: %v", err)
		}
	}()

	router := gin.Default()
	svc := internaltask.New(cfg)
	svc.RegisterRoutes(router)

	taskScheduler := scheduler.NewCronScheduler(scheduler.CronSchedulerConfig{})
	if err := taskScheduler.Start(); err != nil {
		log.Fatalf("task scheduler start failed: %v", err)
	}
	grpcServer := grpc.NewServer()
	registerTaskGRPCService(grpcServer, taskScheduler)

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("task grpc listen failed: %v", err)
	}

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router, ReadHeaderTimeout: 5 * time.Second}

	serverErrors := make(chan error, 2)
	go func() {
		log.Printf("Task HTTP service listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- fmt.Errorf("task http server: %w", err)
		}
	}()
	go func() {
		log.Printf("Task gRPC service listening on :%s", cfg.GRPCPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			serverErrors <- fmt.Errorf("task grpc server: %w", err)
		}
	}()

	log.Println("Task service listening, consumer ready for workflow events")

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		log.Fatalf("task service failed: %v", err)
	case <-signalCtx.Done():
	}

	log.Println("Shutting down task service...")
	cancel()
	workflowConsumer.Close()
	taskScheduler.Stop()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("task HTTP shutdown error: %v", err)
	}
	gracefulStopGRPC(grpcServer, 10*time.Second)
}

func registerTaskGRPCService(grpcServer *grpc.Server, scheduler *scheduler.CronScheduler) {
	taskv1.RegisterTaskServiceServer(grpcServer, pkgtask.NewServer(scheduler, nil))
}

func gracefulStopGRPC(grpcServer *grpc.Server, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		grpcServer.Stop()
	}
}
