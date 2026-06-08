package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
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

func TestLoadMigrationFilesRejectsMalformedNames(t *testing.T) {
	dir := t.TempDir()
	writeMigrationFile(t, dir, "0001_create_probe.sql", `
CREATE TABLE IF NOT EXISTS migration_probe (
	id TEXT PRIMARY KEY
);
`)
	writeMigrationFile(t, dir, "next_probe.sql", `
CREATE TABLE IF NOT EXISTS migration_probe_next (
	id TEXT PRIMARY KEY
);
`)

	_, err := loadMigrationFiles(dir)
	if err == nil || !strings.Contains(err.Error(), "invalid migration filename") {
		t.Fatalf("expected invalid migration filename error, got %v", err)
	}
}

func TestApplyMigrationsBackfillsLegacyTenantScopeData(t *testing.T) {
	database := testMigrationDatabase(t)
	resetPublicSchema(t, database)

	migrationsDir := filepath.Join("..", "..", "migrations")
	legacyDir := copyMigrationsThrough(t, migrationsDir, "0026_membership_auth_security.sql")
	if _, err := applyMigrations(context.Background(), database, legacyDir); err != nil {
		t.Fatalf("apply legacy migrations through 0026: %v", err)
	}

	seedLegacyTenantScopeFixture(t, database)

	full, err := applyMigrations(context.Background(), database, migrationsDir)
	if err != nil {
		t.Fatalf("apply full migrations after legacy fixture: %v", err)
	}
	if full.Applied == 0 {
		t.Fatalf("expected remaining migrations to apply after legacy fixture, got %+v", full)
	}

	assertNoNullOrganizationIDs(t, database, []string{
		"workspaces",
		"sessions",
		"conversations",
		"messages",
		"conversation_configs",
		"conversation_knowledge_bindings",
		"knowledge_bases",
		"knowledge_documents",
		"knowledge_document_chunks",
		"agents",
		"agent_conversations",
		"agent_messages",
		"memory_documents",
		"memory_chunks",
		"mcp_servers",
		"usage_records",
		"quotas",
		"billing_sessions",
		"subscriptions",
		"topup_orders",
		"published_agents",
		"agent_versions",
		"agent_installs",
		"agent_reviews",
		"audit_logs",
	})

	var ownerMemberships int
	if err := database.QueryRow(`
SELECT COUNT(*)
FROM organization_memberships
WHERE user_id = 'legacy_user' AND role = 'owner' AND removed_at IS NULL
`).Scan(&ownerMemberships); err != nil {
		t.Fatalf("count owner memberships: %v", err)
	}
	if ownerMemberships == 0 {
		t.Fatalf("expected legacy user to receive an owner membership")
	}

	second, err := applyMigrations(context.Background(), database, migrationsDir)
	if err != nil {
		t.Fatalf("replay full migrations after legacy backfill: %v", err)
	}
	if second.Applied != 0 {
		t.Fatalf("expected second full run to apply 0 migrations, got %+v", second)
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

func resetPublicSchema(t *testing.T, database *sql.DB) {
	t.Helper()

	if _, err := database.Exec(`
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
`); err != nil {
		t.Fatalf("reset public schema: %v", err)
	}
}

func copyMigrationsThrough(t *testing.T, migrationsDir, throughName string) string {
	t.Helper()

	paths, err := loadMigrationFiles(migrationsDir)
	if err != nil {
		t.Fatalf("load migrations for legacy copy: %v", err)
	}
	dir := t.TempDir()
	for _, path := range paths {
		name := filepath.Base(path)
		if name > throughName {
			break
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatalf("copy migration %s: %v", name, err)
		}
	}
	return dir
}

func seedLegacyTenantScopeFixture(t *testing.T, database *sql.DB) {
	t.Helper()

	for _, statement := range []string{
		`INSERT INTO users (id, email, password_hash, name, role, status)
		 VALUES ('legacy_user', 'legacy@example.test', 'hash', 'Legacy User', 'admin', 'active')`,
		`INSERT INTO workspaces (id, user_id, name)
		 VALUES ('legacy_workspace', 'legacy_user', 'Legacy Workspace')`,
		`INSERT INTO sessions (id, user_id, workspace_id, expires_at)
		 VALUES ('legacy_session', 'legacy_user', 'legacy_workspace', NOW() + INTERVAL '1 day')`,
		`INSERT INTO conversations (id, workspace_id, title, created_at, updated_at)
		 VALUES ('legacy_conversation', 'legacy_workspace', 'Legacy Conversation', NOW(), NOW())`,
		`INSERT INTO messages (id, conversation_id, role, content, created_at)
		 VALUES ('legacy_message', 'legacy_conversation', 'user', 'hello', NOW())`,
		`INSERT INTO conversation_configs (conversation_id, model_id)
		 VALUES ('legacy_conversation', 'demo-reply')`,
		`INSERT INTO knowledge_bases (id, workspace_id, name)
		 VALUES ('legacy_kb', 'legacy_workspace', 'Legacy KB')`,
		`INSERT INTO knowledge_documents (id, knowledge_base_id, title, content)
		 VALUES ('legacy_doc', 'legacy_kb', 'Legacy Doc', 'content')`,
		`INSERT INTO knowledge_document_chunks (id, document_id, chunk_index, content)
		 VALUES ('legacy_chunk', 'legacy_doc', 0, 'chunk')`,
		`INSERT INTO conversation_knowledge_bindings (id, conversation_id, knowledge_base_id)
		 VALUES ('legacy_binding', 'legacy_conversation', 'legacy_kb')`,
		`INSERT INTO agents (id, user_id, name, description)
		 VALUES ('legacy_agent', 'legacy_user', 'Legacy Agent', 'desc')`,
		`INSERT INTO agent_conversations (id, agent_id, user_id, title)
		 VALUES ('legacy_agent_conversation', 'legacy_agent', 'legacy_user', 'Agent Conversation')`,
		`INSERT INTO agent_messages (id, conversation_id, role, content)
		 VALUES ('legacy_agent_message', 'legacy_agent_conversation', 'user', 'hi')`,
		`INSERT INTO memory_documents (id, user_id, title, content)
		 VALUES ('legacy_memory_doc', 'legacy_user', 'Memory Doc', 'memory')`,
		`INSERT INTO memory_chunks (id, document_id, user_id, content, chunk_index)
		 VALUES ('legacy_memory_chunk', 'legacy_memory_doc', 'legacy_user', 'memory chunk', 0)`,
		`INSERT INTO mcp_servers (id, user_id, name, url)
		 VALUES ('legacy_mcp', 'legacy_user', 'Legacy MCP', 'http://mcp.example.test')`,
		`INSERT INTO usage_records (id, user_id, workspace_id, conversation_id, model_id, input_tokens, output_tokens)
		 VALUES ('legacy_usage', 'legacy_user', 'legacy_workspace', 'legacy_conversation', 'demo-reply', 1, 2)`,
		`INSERT INTO quotas (id, user_id, balance, used)
		 VALUES ('legacy_quota', 'legacy_user', 10, 1)`,
		`INSERT INTO billing_sessions (id, user_id, channel_id, model, api_type, idempotency_key, pre_authorized_amt, settled_amt)
		 VALUES ('legacy_billing', 'legacy_user', 'channel', 'demo-reply', 'chat', 'legacy_idempotency', 1, 1)`,
		`INSERT INTO packages (id, name, description, quota_amount, price, duration_days, is_active, sort_order)
		 VALUES ('legacy_package', 'Legacy Package', 'desc', 100, 9.99, 30, true, 1)`,
		`INSERT INTO subscriptions (id, user_id, package_id, status)
		 VALUES ('legacy_subscription', 'legacy_user', 'legacy_package', 'active')`,
		`INSERT INTO topup_orders (id, user_id, amount, money, status)
		 VALUES ('legacy_topup', 'legacy_user', 10, 1, 'paid')`,
		`INSERT INTO published_agents (id, owner_id, name, description, tags, visibility, status, pricing_type)
		 VALUES ('legacy_published_agent', 'legacy_user', 'Published Agent', 'desc', ARRAY['legacy'], 'public', 'approved', 'free')`,
		`INSERT INTO agent_versions (id, agent_id, version, status)
		 VALUES ('legacy_agent_version', 'legacy_published_agent', '1.0.0', 'approved')`,
		`INSERT INTO agent_installs (id, agent_id, user_id, version_id)
		 VALUES ('legacy_agent_install', 'legacy_published_agent', 'legacy_user', 'legacy_agent_version')`,
		`INSERT INTO agent_reviews (id, agent_id, user_id, rating, body)
		 VALUES ('legacy_agent_review', 'legacy_published_agent', 'legacy_user', 5, 'great')`,
		`INSERT INTO audit_logs (id, actor_id, actor_email, action, resource_type, resource_id)
		 VALUES ('legacy_audit', 'legacy_user', 'legacy@example.test', 'created', 'organization', 'org_user_' || md5('legacy_user'))`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("seed legacy fixture statement %q: %v", statement, err)
		}
	}
}

func assertNoNullOrganizationIDs(t *testing.T, database *sql.DB, tables []string) {
	t.Helper()

	sortedTables := append([]string(nil), tables...)
	sort.Strings(sortedTables)
	for _, table := range sortedTables {
		var nullCount int
		query := "SELECT COUNT(*) FROM " + table + " WHERE organization_id IS NULL"
		if err := database.QueryRow(query).Scan(&nullCount); err != nil {
			t.Fatalf("count null organization_id in %s: %v", table, err)
		}
		if nullCount != 0 {
			t.Fatalf("expected %s organization_id to be backfilled, found %d NULL rows", table, nullCount)
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
