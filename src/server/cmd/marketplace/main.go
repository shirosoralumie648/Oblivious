package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/config"
	"oblivious/server/internal/marketplace"
	"oblivious/server/internal/releasecontract"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	if err := config.RunEntrypoint(context.Background(), releasecontract.EntrypointID("marketplace"), config.EntrypointPreflightOptions{
		RepoRoot: "/app", ContractPath: "config/release/contract.v1.json", SchemaPath: "config/release/contract.schema.json",
		ProfileID: os.Getenv("OBLIVIOUS_DEPLOYMENT_PROFILE"), Contracts: config.FileContractLoader{},
		Identities: buildinfo.NewEmbeddedProvider(), Profiles: releasecontract.NewFileProfileResolver(),
	}, func(context.Context, config.ResolvedEntrypointInputs) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		dbURL := cfg.DatabaseURL
		if cfg.DBMode != "monolith" && cfg.DBURLMarketplace != "" {
			dbURL = cfg.DBURLMarketplace
		}

		db, err := sql.Open("postgres", dbURL)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			return fmt.Errorf("ping database: %w", err)
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
		_ = srv.Shutdown(context.Background())
		return nil
	}); err != nil {
		log.Fatalf("marketplace preflight failed: %v", err)
	}
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
