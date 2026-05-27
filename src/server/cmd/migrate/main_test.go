package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

func TestApplyMigrationsRecordsLedgerAndSkipsAppliedFiles(t *testing.T) {
	database := testMigrationDatabase(t)
	resetMigrationTestTables(t, database)

	dir := t.TempDir()
	writeMigrationFile(t, dir, "0001_create_probe.sql", `
CREATE TABLE IF NOT EXISTS migration_probe (
	id TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`)
	writeMigrationFile(t, dir, "0002_insert_probe.sql", `
INSERT INTO migration_probe (id, value)
VALUES ('probe_1', 'first')
ON CONFLICT (id) DO UPDATE SET value = EXCLUDED.value;
`)

	first, err := applyMigrations(context.Background(), database, dir)
	if err != nil {
		t.Fatalf("first apply migrations: %v", err)
	}
	if first.Applied != 2 || first.Skipped != 0 {
		t.Fatalf("first run expected 2 applied and 0 skipped, got %+v", first)
	}

	var ledgerCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&ledgerCount); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if ledgerCount != 2 {
		t.Fatalf("expected 2 ledger rows, got %d", ledgerCount)
	}

	second, err := applyMigrations(context.Background(), database, dir)
	if err != nil {
		t.Fatalf("second apply migrations: %v", err)
	}
	if second.Applied != 0 || second.Skipped != 2 {
		t.Fatalf("second run expected 0 applied and 2 skipped, got %+v", second)
	}
}

func TestApplyMigrationsRejectsChecksumMismatch(t *testing.T) {
	database := testMigrationDatabase(t)
	resetMigrationTestTables(t, database)

	dir := t.TempDir()
	path := writeMigrationFile(t, dir, "0001_create_probe.sql", `
CREATE TABLE IF NOT EXISTS migration_probe (
	id TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`)

	if _, err := applyMigrations(context.Background(), database, dir); err != nil {
		t.Fatalf("first apply migrations: %v", err)
	}

	if err := os.WriteFile(path, []byte(`
CREATE TABLE IF NOT EXISTS migration_probe (
	id TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	changed_at TIMESTAMPTZ
);
`), 0o644); err != nil {
		t.Fatalf("rewrite migration: %v", err)
	}

	_, err := applyMigrations(context.Background(), database, dir)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func testMigrationDatabase(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for migration integration tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	lockIntegrationTestDatabase(t, database)

	return database
}

func lockIntegrationTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	if _, err := database.Exec(`SELECT pg_advisory_lock(104210)`); err != nil {
		t.Fatalf("lock integration test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104210)`); err != nil {
			t.Fatalf("unlock integration test database: %v", err)
		}
	})
}

func resetMigrationTestTables(t *testing.T, database *sql.DB) {
	t.Helper()

	for _, statement := range []string{
		`DROP TABLE IF EXISTS migration_probe`,
		`DROP TABLE IF EXISTS schema_migrations`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("reset migration test table: %v", err)
		}
	}
}

func writeMigrationFile(t *testing.T, dir, name, sql string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.TrimSpace(sql)+"\n"), 0o644); err != nil {
		t.Fatalf("write migration %s: %v", name, err)
	}
	return path
}
