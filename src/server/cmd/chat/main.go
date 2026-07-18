package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/config"
	serverhttp "oblivious/server/internal/http"
	"oblivious/server/internal/releasecontract"

	_ "github.com/lib/pq"
)

func main() {
	if err := config.RunEntrypoint(context.Background(), releasecontract.EntrypointID("chat"), config.EntrypointPreflightOptions{
		RepoRoot: "/app", ContractPath: "config/release/contract.v1.json", SchemaPath: "config/release/contract.schema.json",
		ProfileID: os.Getenv("OBLIVIOUS_DEPLOYMENT_PROFILE"), Contracts: config.FileContractLoader{},
		Identities: buildinfo.NewEmbeddedProvider(), Profiles: releasecontract.NewFileProfileResolver(),
	}, func(context.Context, config.ResolvedEntrypointInputs) error {
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

		handler := serverhttp.NewChatRouter(cfg, db)
		addr := fmt.Sprintf(":%d", cfg.Port)
		srv := &http.Server{Addr: addr, Handler: handler}
		go func() {
			log.Printf("Chat service listening on %s", addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %s\n", err)
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down chat service...")
		_ = srv.Shutdown(context.Background())
		return nil
	}); err != nil {
		log.Fatalf("chat preflight failed: %v", err)
	}
}
