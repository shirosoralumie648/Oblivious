package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"oblivious/server/internal/config"
	"oblivious/server/internal/db"
	"oblivious/server/internal/migrations"
)

func main() {
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
