package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/config"
	"oblivious/server/internal/db"
	"oblivious/server/internal/migrations"
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
	exitCode := runMain(context.Background(), os.Args[1:], inspectionDependencies{
		provider: buildinfo.NewEmbeddedProvider(), stdout: os.Stdout, stderr: os.Stderr,
		repoRoot: packagedRepoRoot, contract: packagedContractPath, schema: packagedSchemaPath,
	}, runMigrations)
	if exitCode != 0 {
		os.Exit(exitCode)
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

func runMigrations() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	result, err := migrations.Apply(context.Background(), database, "migrations")
	if err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	fmt.Printf("migrations applied: %d, skipped: %d\n", result.Applied, result.Skipped)
}

type migrationResult = migrations.Result

func applyMigrations(ctx context.Context, database *sql.DB, migrationsDir string) (migrationResult, error) {
	return migrations.Apply(ctx, database, migrationsDir)
}

func loadMigrationFiles(migrationsDir string) ([]string, error) {
	return migrations.LoadFiles(migrationsDir)
}

func migrationChecksum(statement []byte) string {
	return migrations.Checksum(statement)
}
