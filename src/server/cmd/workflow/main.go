package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"oblivious/server/internal/workflow"
	"oblivious/server/pkg/config"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "github.com/lib/pq"
)

func main() {
	cfg, err := config.LoadCommon()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db open failed: %v", err)
	}
	defer db.Close()

	store := workflow.NewSQLStore(db)
	svc := workflow.NewService(store)

	kafkaBrokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(kafkaBrokers) == 0 || kafkaBrokers[0] == "" {
		kafkaBrokers = []string{"localhost:9092"}
	}
	eventPublisher := workflow.NewEventPublisher(kafkaBrokers, "workflow.events")
	defer eventPublisher.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	srv := &http.Server{Addr: ":" + port, Handler: mux}

	go func() {
		log.Printf("Workflow service listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down workflow service...")
	srv.Shutdown(context.Background())
	_ = svc
	_ = eventPublisher
}
