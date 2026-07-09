package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"oblivious/server/internal/metrics"
	"oblivious/server/internal/observability"
)

const advisoryLockID int64 = 7592541001

type Result struct {
	Applied int
	Skipped int
}

type migrationDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

var filePattern = regexp.MustCompile(`^[0-9]{4}_[a-z0-9][a-z0-9_]*\.sql$`)

func Apply(ctx context.Context, database *sql.DB, migrationsDir string) (result Result, err error) {
	ctx, span := observability.StartSpan(ctx, "migration.apply")
	defer span.End()
	defer func() {
		if err != nil {
			metrics.RecordMigrationRun("failure")
			return
		}
		metrics.RecordMigrationRun("success")
	}()

	conn, err := database.Conn(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("open migration connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, advisoryLockID); err != nil {
		return Result{}, fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		if _, unlockErr := conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, advisoryLockID); unlockErr != nil && err == nil {
			err = fmt.Errorf("release migration advisory lock: %w", unlockErr)
		}
	}()

	return applyLocked(ctx, conn, migrationsDir)
}

func applyLocked(ctx context.Context, database migrationDB, migrationsDir string) (Result, error) {
	var result Result
	if err := ensureLedger(ctx, database); err != nil {
		return Result{}, err
	}

	migrationPaths, err := LoadFiles(migrationsDir)
	if err != nil {
		return Result{}, err
	}

	for _, migrationPath := range migrationPaths {
		version := filepath.Base(migrationPath)
		statement, err := os.ReadFile(migrationPath)
		if err != nil {
			return result, fmt.Errorf("read migration %s: %w", migrationPath, err)
		}
		checksum := Checksum(statement)

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

		if err := applyOne(ctx, database, version, checksum, string(statement)); err != nil {
			return result, err
		}
		result.Applied++
	}

	return result, nil
}

func ensureLedger(ctx context.Context, database migrationDB) error {
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

func LoadFiles(migrationsDir string) ([]string, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	migrationPaths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		if !filePattern.MatchString(entry.Name()) {
			return nil, fmt.Errorf("invalid migration filename %q: expected NNNN_description.sql", entry.Name())
		}
		migrationPaths = append(migrationPaths, filepath.Join(migrationsDir, entry.Name()))
	}
	sort.Strings(migrationPaths)

	return migrationPaths, nil
}

func Checksum(statement []byte) string {
	sum := sha256.Sum256(statement)
	return hex.EncodeToString(sum[:])
}

func applyOne(ctx context.Context, database migrationDB, version, checksum, statement string) error {
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
