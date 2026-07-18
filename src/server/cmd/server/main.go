package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	stdhttp "net/http"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/config"
	"oblivious/server/internal/db"
	serverhttp "oblivious/server/internal/http"
	"oblivious/server/internal/migrations"
	"oblivious/server/internal/releasecontract"
)

const (
	packagedRepoRoot     = "/app"
	packagedContractPath = "config/release/contract.v1.json"
	packagedSchemaPath   = "config/release/contract.schema.json"
)

type inspectionDependencies struct {
	provider buildinfo.IdentityProvider
	stdout   io.Writer
	stderr   io.Writer
	repoRoot string
	contract string
	schema   string
}

func main() {
	if err := config.RunEntrypoint(context.Background(), releasecontract.EntrypointID("server"), config.EntrypointPreflightOptions{
		RepoRoot: packagedRepoRoot, ContractPath: packagedContractPath, SchemaPath: packagedSchemaPath,
		ProfileID: os.Getenv("OBLIVIOUS_DEPLOYMENT_PROFILE"), Contracts: config.FileContractLoader{},
		Identities: buildinfo.NewEmbeddedProvider(), Profiles: releasecontract.NewFileProfileResolver(),
	}, func(ctx context.Context, _ config.ResolvedEntrypointInputs) error {
		exitCode := runMain(ctx, os.Args[1:], inspectionDependencies{
			provider: buildinfo.NewEmbeddedProvider(), stdout: os.Stdout, stderr: os.Stderr,
			repoRoot: packagedRepoRoot, contract: packagedContractPath, schema: packagedSchemaPath,
		}, runServer)
		if exitCode != 0 {
			return fmt.Errorf("server exited with code %d", exitCode)
		}
		return nil
	}); err != nil {
		log.Fatalf("server preflight failed: %v", err)
	}
}

func runMain(ctx context.Context, args []string, deps inspectionDependencies, normalStartup func()) int {
	handled, exitCode := buildinfo.HandleInspection(ctx, args, deps.stdout, deps.stderr, deps.provider, deps.repoRoot, deps.contract, deps.schema)
	if handled {
		return exitCode
	}
	if normalStartup != nil {
		normalStartup()
	}
	return 0
}

func runServer() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	migrationCtx, cancelMigrations := context.WithTimeout(context.Background(), 10*time.Minute)
	migrationResult, err := migrations.Apply(migrationCtx, database, "migrations")
	cancelMigrations()
	if err != nil {
		log.Fatalf("apply migrations: %v", err)
	}
	log.Printf("migrations ready: applied=%d skipped=%d", migrationResult.Applied, migrationResult.Skipped)

	server := serverhttp.NewServer(cfg, database)
	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("server listening on %s env=%s", server.Addr, cfg.Env)
		serverErrors <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("server shutdown: %v", err)
		}
	}
}
