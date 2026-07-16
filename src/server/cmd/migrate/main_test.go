package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"oblivious/server/internal/buildinfo"
)

func TestIdentityInspectionPrecedesRuntimeSideEffects(t *testing.T) {
	identity := migrationTestIdentity()
	for _, test := range []struct {
		name     string
		args     []string
		provider buildinfo.IdentityProvider
		wantCode int
		wantRuns int
	}{
		{name: "inspection success", args: []string{buildinfo.InspectionFlag}, provider: migrationInspectionProvider{identity: identity}},
		{name: "inspection failure", args: []string{buildinfo.InspectionFlag}, provider: migrationInspectionProvider{err: &buildinfo.IdentityError{Code: buildinfo.ErrorContractDigestMismatch, Field: "contractDigest"}}, wantCode: 1},
		{name: "normal startup", args: []string{"--migrate"}, provider: migrationInspectionProvider{identity: identity}, wantRuns: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			configLoads, databaseOpens, migrationRuns := 0, 0, 0
			exitCode := runMain(context.Background(), test.args, inspectionDependencies{
				provider: test.provider, stdout: &stdout, stderr: &stderr,
				repoRoot: "/app", contract: packagedContractPath, schema: packagedSchemaPath,
			}, func() {
				configLoads++
				databaseOpens++
				migrationRuns++
			})
			if exitCode != test.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr=%s", exitCode, test.wantCode, stderr.String())
			}
			for name, calls := range map[string]int{"config": configLoads, "database": databaseOpens, "migration": migrationRuns} {
				if calls != test.wantRuns {
					t.Fatalf("%s calls = %d, want %d", name, calls, test.wantRuns)
				}
			}
			switch test.name {
			case "inspection success":
				var got buildinfo.BuildIdentityV1
				if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || got != identity {
					t.Fatalf("inspection output = %q, identity=%#v, error=%v", stdout.String(), got, err)
				}
			case "inspection failure":
				if !strings.Contains(stderr.String(), string(buildinfo.ErrorContractDigestMismatch)) {
					t.Fatalf("inspection error = %q", stderr.String())
				}
			}
		})
	}
}

type migrationInspectionProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p migrationInspectionProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	return p.identity, p.err
}

func migrationTestIdentity() buildinfo.BuildIdentityV1 {
	return buildinfo.BuildIdentityV1{
		SchemaVersion: buildinfo.BuildIdentitySchemaV1, ReleaseCommit: strings.Repeat("a", 40),
		SourceTree: strings.Repeat("b", 40), ContractDigest: "sha256:" + strings.Repeat("c", 64),
		Dirty: false, EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
}

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

func TestApplyMigrationsBackfillsMarketplaceCategoryIDs(t *testing.T) {
	database := testMigrationDatabase(t)
	resetPublicSchema(t, database)

	migrationsDir := filepath.Join("..", "..", "migrations")
	legacyDir := copyMigrationsThrough(t, migrationsDir, "0078_marketplace_order_failed_status.sql")
	if _, err := applyMigrations(context.Background(), database, legacyDir); err != nil {
		t.Fatalf("apply migrations through 0078: %v", err)
	}

	seedMarketplaceCategoryIDFixture(t, database)

	full, err := applyMigrations(context.Background(), database, migrationsDir)
	if err != nil {
		t.Fatalf("apply full migrations after marketplace category fixture: %v", err)
	}
	if full.Applied == 0 {
		t.Fatalf("expected marketplace category integrity migration to apply, got %+v", full)
	}

	assertMarketplaceCategoryIDsBackfilled(t, database)
	assertMarketplaceCategoryIDConstraints(t, database)

	second, err := applyMigrations(context.Background(), database, migrationsDir)
	if err != nil {
		t.Fatalf("replay full migrations after marketplace category backfill: %v", err)
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
	// Pin to a single connection so the advisory lock is held for the
	// lifetime of the test and cannot be bypassed by the connection pool.
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
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

func seedMarketplaceCategoryIDFixture(t *testing.T, database *sql.DB) {
	t.Helper()

	for _, statement := range []string{
		`INSERT INTO users (id, email, password_hash, name, role, status)
		 VALUES ('category_owner', 'category-owner@example.test', 'hash', 'Category Owner', 'user', 'active')`,
		`INSERT INTO organizations (id, slug, name, status, metadata, created_by_user_id)
		 VALUES ('org_category_owner', 'category-owner-org', 'Category Owner Org', 'active', '{}', 'category_owner')`,
		`INSERT INTO organization_memberships (id, organization_id, user_id, role, created_by_user_id)
		 VALUES ('membership_category_owner', 'org_category_owner', 'category_owner', 'owner', 'category_owner')`,
		`INSERT INTO published_agents (id, owner_id, organization_id, name, description, category_id, tags, visibility, status, pricing_type)
		 VALUES
		     ('agent_category_slug', 'category_owner', 'org_category_owner', 'Slug Category Agent', 'Uses a legacy category slug.', 'productivity', ARRAY['legacy'], 'public', 'approved', 'free'),
		     ('agent_category_blank', 'category_owner', 'org_category_owner', 'Blank Category Agent', 'Uses a blank category value.', '', ARRAY['legacy'], 'public', 'approved', 'free'),
		     ('agent_category_missing', 'category_owner', 'org_category_owner', 'Missing Category Agent', 'Uses an unknown category value.', 'missing-category', ARRAY['legacy'], 'public', 'approved', 'free'),
		     ('agent_category_valid', 'category_owner', 'org_category_owner', 'Valid Category Agent', 'Already uses a category ID.', 'cat_chat', ARRAY['legacy'], 'public', 'approved', 'free')`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("seed marketplace category fixture statement %q: %v", statement, err)
		}
	}
}

func assertMarketplaceCategoryIDsBackfilled(t *testing.T, database *sql.DB) {
	t.Helper()

	expected := map[string]string{
		"agent_category_slug":    "cat_productivity",
		"agent_category_blank":   "cat_productivity",
		"agent_category_missing": "cat_productivity",
		"agent_category_valid":   "cat_chat",
	}
	for agentID, expectedCategoryID := range expected {
		var categoryID string
		if err := database.QueryRow(`SELECT category_id FROM published_agents WHERE id = $1`, agentID).Scan(&categoryID); err != nil {
			t.Fatalf("query category_id for %s: %v", agentID, err)
		}
		if categoryID != expectedCategoryID {
			t.Fatalf("expected %s category_id %q, got %q", agentID, expectedCategoryID, categoryID)
		}
	}

	var invalidCount int
	if err := database.QueryRow(`
SELECT COUNT(*)
FROM published_agents pa
WHERE pa.category_id IS NULL
   OR btrim(pa.category_id) = ''
   OR NOT EXISTS (
       SELECT 1
       FROM categories c
       WHERE c.id = pa.category_id
   )
`).Scan(&invalidCount); err != nil {
		t.Fatalf("count invalid marketplace category IDs: %v", err)
	}
	if invalidCount != 0 {
		t.Fatalf("expected every published agent category_id to reference categories.id, found %d invalid rows", invalidCount)
	}
}

func assertMarketplaceCategoryIDConstraints(t *testing.T, database *sql.DB) {
	t.Helper()

	var constraintCount int
	if err := database.QueryRow(`
SELECT COUNT(*)
FROM pg_constraint
WHERE conname = 'published_agents_category_id_fkey'
  AND conrelid = 'published_agents'::regclass
`).Scan(&constraintCount); err != nil {
		t.Fatalf("query marketplace category foreign key: %v", err)
	}
	if constraintCount != 1 {
		t.Fatalf("expected published_agents_category_id_fkey to exist, found %d", constraintCount)
	}

	var isNullable string
	if err := database.QueryRow(`
SELECT is_nullable
FROM information_schema.columns
WHERE table_name = 'published_agents'
  AND column_name = 'category_id'
`).Scan(&isNullable); err != nil {
		t.Fatalf("query published_agents.category_id nullability: %v", err)
	}
	if isNullable != "NO" {
		t.Fatalf("expected published_agents.category_id to be NOT NULL, got is_nullable=%q", isNullable)
	}

	_, err := database.Exec(`
INSERT INTO published_agents (id, owner_id, organization_id, name, description, category_id, tags, visibility, status, pricing_type)
VALUES ('agent_category_invalid_after_fk', 'category_owner', 'org_category_owner', 'Invalid Category Agent', 'Should fail after FK.', 'missing-category', ARRAY['legacy'], 'public', 'approved', 'free')
`)
	if err == nil {
		t.Fatal("expected invalid marketplace category ID insert to fail after FK")
	}

	_, err = database.Exec(`
INSERT INTO published_agents (id, owner_id, organization_id, name, description, category_id, tags, visibility, status, pricing_type)
VALUES ('agent_category_null_after_not_null', 'category_owner', 'org_category_owner', 'Null Category Agent', 'Should fail after NOT NULL.', NULL, ARRAY['legacy'], 'public', 'approved', 'free')
`)
	if err == nil {
		t.Fatal("expected null marketplace category ID insert to fail after NOT NULL")
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
