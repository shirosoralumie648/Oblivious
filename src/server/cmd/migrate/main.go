package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"oblivious/server/internal/config"
	"oblivious/server/internal/db"
	"oblivious/server/internal/metrics"
	"oblivious/server/internal/observability"
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

	result, err := applyMigrations(context.Background(), database, "migrations")
	if err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	fmt.Printf("migrations applied: %d, skipped: %d\n", result.Applied, result.Skipped)
}

type migrationResult struct {
	Applied int
	Skipped int
}

var migrationFilePattern = regexp.MustCompile(`^[0-9]{4}_[a-z0-9][a-z0-9_]*\.sql$`)

func applyMigrations(ctx context.Context, database *sql.DB, migrationsDir string) (result migrationResult, err error) {
	ctx, span := observability.StartSpan(ctx, "migration.apply")
	defer span.End()
	defer func() {
		if err != nil {
			metrics.RecordMigrationRun("failure")
			return
		}
		metrics.RecordMigrationRun("success")
	}()

	if err := ensureMigrationLedger(ctx, database); err != nil {
		return migrationResult{}, err
	}

	migrationPaths, err := loadMigrationFiles(migrationsDir)
	if err != nil {
		return migrationResult{}, err
	}

	for _, migrationPath := range migrationPaths {
		version := filepath.Base(migrationPath)
		statement, err := os.ReadFile(migrationPath)
		if err != nil {
			return result, fmt.Errorf("read migration %s: %w", migrationPath, err)
		}
		checksum := migrationChecksum(statement)

		var existingChecksum string
		err = database.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE version = $1`, version).Scan(&existingChecksum)
		switch {
		case err == nil:
			if existingChecksum != checksum {
				return result, fmt.Errorf("migration %s checksum mismatch", version)
			}
			result.Skipped++
			continue
		case err != sql.ErrNoRows:
			return result, fmt.Errorf("check migration %s: %w", version, err)
		}

		if err := applyMigration(ctx, database, version, checksum, string(statement)); err != nil {
			return result, err
		}
		result.Applied++
	}

	return result, nil
}

func ensureMigrationLedger(ctx context.Context, database *sql.DB) error {
	_, err := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version TEXT PRIMARY KEY,
	checksum TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	return nil
}

func loadMigrationFiles(migrationsDir string) ([]string, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	migrationPaths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		if !migrationFilePattern.MatchString(entry.Name()) {
			return nil, fmt.Errorf("invalid migration filename %q: expected NNNN_description.sql", entry.Name())
		}
		migrationPaths = append(migrationPaths, filepath.Join(migrationsDir, entry.Name()))
	}
	sort.Strings(migrationPaths)

	return migrationPaths, nil
}

func migrationChecksum(statement []byte) string {
	sum := sha256.Sum256(statement)
	return hex.EncodeToString(sum[:])
}

func applyMigration(ctx context.Context, database *sql.DB, version, checksum, statement string) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", version, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO schema_migrations (version, checksum)
VALUES ($1, $2)
`, version, checksum); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}
	return nil
}
