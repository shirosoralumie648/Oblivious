package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"oblivious/server/internal/billing"
	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/pkg/config"

	"github.com/gin-gonic/gin"
)

func main() {
	if err := config.RunEntrypoint(context.Background(), releasecontract.EntrypointID("billing"), config.EntrypointPreflightOptions{
		RepoRoot: "/app", ContractPath: "config/release/contract.v1.json", SchemaPath: "config/release/contract.schema.json",
		ProfileID: os.Getenv("OBLIVIOUS_DEPLOYMENT_PROFILE"), Contracts: config.FileContractLoader{},
		Identities: buildinfo.NewEmbeddedProvider(), Profiles: releasecontract.NewFileProfileResolver(),
	}, func(context.Context, config.ResolvedEntrypointInputs) error {
		cfg := config.LoadBillingConfig()
		router := gin.Default()
		svc := billing.New(cfg)
		svc.RegisterRoutes(router)
		srv := &http.Server{Addr: ":" + cfg.Port, Handler: router}

		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %s\n", err)
			}
		}()
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		log.Println("Shutting down billing service...")
		_ = srv.Shutdown(context.Background())
		return nil
	}); err != nil {
		log.Fatalf("billing preflight failed: %v", err)
	}
}
