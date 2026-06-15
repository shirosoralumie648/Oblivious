package main

import (
	"context"
	"database/sql"
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

	workflowv1 "oblivious/server/internal/grpc/workflowv1"
	internalworkflow "oblivious/server/internal/workflow"
	"oblivious/server/pkg/config"
	pkgworkflow "oblivious/server/pkg/workflow"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.LoadWorkflowConfig()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db open failed: %v", err)
	}
	defer db.Close()

	store := internalworkflow.NewSQLStore(db)
	svc := internalworkflow.NewService(store)

	kafkaBrokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(kafkaBrokers) == 0 || kafkaBrokers[0] == "" {
		kafkaBrokers = []string{"localhost:9092"}
	}
	eventPublisher := internalworkflow.NewEventPublisher(kafkaBrokers, "workflow.events")
	defer eventPublisher.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	grpcServer := grpc.NewServer()
	registerWorkflowGRPCService(grpcServer, svc)

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("workflow grpc listen failed: %v", err)
	}

	serverErrors := make(chan error, 2)
	go func() {
		log.Printf("Workflow HTTP health listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- fmt.Errorf("workflow http server: %w", err)
		}
	}()
	go func() {
		log.Printf("Workflow gRPC service listening on :%s", cfg.GRPCPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			serverErrors <- fmt.Errorf("workflow grpc server: %w", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		log.Fatalf("workflow service failed: %v", err)
	case <-ctx.Done():
	}

	log.Println("Shutting down workflow service...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("workflow HTTP shutdown error: %v", err)
	}
	gracefulStopGRPC(grpcServer, 10*time.Second)
}

func registerWorkflowGRPCService(grpcServer *grpc.Server, service *internalworkflow.Service) {
	workflowv1.RegisterWorkflowServiceServer(grpcServer, pkgworkflow.NewServer(service))
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
