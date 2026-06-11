package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"oblivious/server/internal/config"
	"oblivious/server/internal/marketplace"
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
	if cfg.DBMode != "monolith" && cfg.DBURLMarketplace != "" {
		dbURL = cfg.DBURLMarketplace
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	store := marketplace.NewSQLStore(db)
	audit := &noopAuditLogger{}
	service := marketplace.NewService(store, audit)

	router := gin.Default()
	registerRoutes(router, service)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		log.Println("Marketplace service listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down marketplace service...")
	srv.Shutdown(context.Background())
}

func registerRoutes(r *gin.Engine, service *marketplace.Service) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

type noopAuditLogger struct{}

func (n *noopAuditLogger) LogAction(ctx context.Context, actorID, actorEmail, action, resourceType, resourceID, changes, ipAddress string) error {
	return nil
}
