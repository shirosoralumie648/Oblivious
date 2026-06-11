package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/config"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbURL := cfg.DatabaseURL
	if cfg.DBMode != "monolith" && cfg.DBURLChat != "" {
		dbURL = cfg.DBURLChat
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	store := chat.NewSQLStore(db)
	replyGenerator := &demoReplyGenerator{}
	usageRecorder := &noopUsageRecorder{}
	service := chat.NewService(store, replyGenerator, cfg.ModelDefaultName, usageRecorder)

	router := gin.Default()
	registerRoutes(router, service)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		log.Println("Chat service listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down chat service...")
	srv.Shutdown(context.Background())
}

func registerRoutes(r *gin.Engine, service *chat.Service) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

type demoReplyGenerator struct{}

func (d *demoReplyGenerator) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	return "Demo reply", nil
}

type noopUsageRecorder struct{}

func (n *noopUsageRecorder) RecordChatUsage(ctx context.Context, record chat.UsageRecord) error {
	return nil
}
