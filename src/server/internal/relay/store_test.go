package relay

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/lib/pq"

	"oblivious/server/internal/relay/handler"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/secretbox"
)

func testRelaySQLStore(t *testing.T) (*RelayStore, *sql.DB, context.Context) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if strings.EqualFold(os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE"), "true") {
			t.Fatal("TEST_DATABASE_URL is required for DB-backed relay store tests")
		}
		t.Skip("TEST_DATABASE_URL is required for DB-backed relay store tests")
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

	if _, err := database.Exec(`SELECT pg_advisory_lock(104261)`); err != nil {
		t.Fatalf("lock relay store test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104261)`); err != nil {
			t.Fatalf("unlock relay store test database: %v", err)
		}
	})

	if _, err := database.Exec(`DROP TABLE IF EXISTS relay_file_mappings CASCADE`); err != nil {
		t.Fatalf("drop relay file mappings table: %v", err)
	}
	if _, err := database.Exec(`DROP TABLE IF EXISTS relay_batch_polling_jobs CASCADE`); err != nil {
		t.Fatalf("drop relay batch polling jobs table: %v", err)
	}
	if _, err := database.Exec(`DROP TABLE IF EXISTS relay_conversation_affinity CASCADE`); err != nil {
		t.Fatalf("drop relay conversation affinity table: %v", err)
	}
	migration, err := os.ReadFile("../../migrations/0061_relay_file_mappings.sql")
	if err != nil {
		t.Fatalf("read relay file mappings migration: %v", err)
	}
	if _, err := database.Exec(string(migration)); err != nil {
		t.Fatalf("apply relay file mappings migration: %v", err)
	}
	tombstoneMigration, err := os.ReadFile("../../migrations/0095_relay_file_mapping_tombstones.sql")
	if err != nil {
		t.Fatalf("read relay file mappings tombstone migration: %v", err)
	}
	if _, err := database.Exec(string(tombstoneMigration)); err != nil {
		t.Fatalf("apply relay file mappings tombstone migration: %v", err)
	}
	batchPollingMigration, err := os.ReadFile("../../migrations/0097_relay_batch_polling_jobs.sql")
	if err != nil {
		t.Fatalf("read relay batch polling jobs migration: %v", err)
	}
	if _, err := database.Exec(string(batchPollingMigration)); err != nil {
		t.Fatalf("apply relay batch polling jobs migration: %v", err)
	}
	affinityMigration, err := os.ReadFile("../../migrations/0062_relay_conversation_affinity.sql")
	if err != nil {
		t.Fatalf("read relay conversation affinity migration: %v", err)
	}
	if _, err := database.Exec(string(affinityMigration)); err != nil {
		t.Fatalf("apply relay conversation affinity migration: %v", err)
	}

	return NewRelayStore(database), database, context.Background()
}

func testRelayChannelSQLStore(t *testing.T) (*RelayStore, *sql.DB, context.Context) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if strings.EqualFold(os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE"), "true") {
			t.Fatal("TEST_DATABASE_URL is required for DB-backed relay channel tests")
		}
		t.Skip("TEST_DATABASE_URL is required for DB-backed relay channel tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	if _, err := database.Exec(`SELECT pg_advisory_lock(104262)`); err != nil {
		t.Fatalf("lock relay channel test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104262)`); err != nil {
			t.Fatalf("unlock relay channel test database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS model_channel_weights CASCADE`,
		`DROP TABLE IF EXISTS model_routes CASCADE`,
		`DROP TABLE IF EXISTS channels CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare relay channel test database: %v\nstatement: %s", err, statement)
		}
	}

	migrationPaths := []string{
		"../../migrations/0013_channels.sql",
		"../../migrations/0035_channel_groups.sql",
		"../../migrations/0036_channel_costs.sql",
		"../../migrations/0077_channel_default_weight.sql",
		"../../migrations/0081_admin_relay_channel_organization_scope.sql",
	}
	for _, path := range migrationPaths {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read relay channel migration %s: %v", path, err)
		}
		if _, err := database.Exec(string(migration)); err != nil {
			t.Fatalf("apply relay channel migration %s: %v", path, err)
		}
	}

	return NewRelayStore(database), database, context.Background()
}

func TestRelayStoreLoadPoolPreservesChannelOrganizationScope(t *testing.T) {
	store, database, _ := testRelayChannelSQLStore(t)

	if _, err := database.Exec(`
		INSERT INTO organizations (id, slug, name, created_at, updated_at)
		VALUES
			('org_a', 'relay-runtime-a', 'Relay Runtime A', NOW(), NOW()),
			('org_b', 'relay-runtime-b', 'Relay Runtime B', NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert relay runtime organizations: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO channels (
			id, organization_id, name, provider, base_url, api_key_encrypted, models, groups,
			rpm_limit, tpm_limit, cb_threshold, cb_timeout, strategy, priority, weight,
			estimated_cost_per_1k, cost_multiplier, enabled, created_at, updated_at
		)
		VALUES
			('relay_runtime_org_a', 'org_a', 'Runtime Org A', 'openai', 'https://org-a.example.test', 'sk-org-a', ARRAY['gpt-4o-mini']::text[], ARRAY['default']::text[], 100, 100000, 5, 30, 'weighted', 10, 1, 0.02, 1.1, true, NOW(), NOW()),
			('relay_runtime_org_b', 'org_b', 'Runtime Org B', 'openai', 'https://org-b.example.test', 'sk-org-b', ARRAY['gpt-4o-mini']::text[], ARRAY['default']::text[], 100, 100000, 5, 30, 'weighted', 0, 100, 0.02, 1.1, true, NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert relay runtime channels: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO model_routes (id, model, strategy, created_at)
		VALUES ('route_runtime_gpt4o_mini', 'gpt-4o-mini', 'weighted', NOW());
		INSERT INTO model_channel_weights (id, route_id, channel_id, weight, priority, enabled)
		VALUES
			('weight_runtime_org_a', 'route_runtime_gpt4o_mini', 'relay_runtime_org_a', 1, 10, true),
			('weight_runtime_org_b', 'route_runtime_gpt4o_mini', 'relay_runtime_org_b', 100, 0, true)
	`); err != nil {
		t.Fatalf("insert relay runtime model route: %v", err)
	}

	pool := NewChannelPool()
	if err := store.ReloadPoolFromStore(pool); err != nil {
		t.Fatalf("ReloadPoolFromStore returned error: %v", err)
	}
	if ch, ok := pool.GetChannel("relay_runtime_org_a"); !ok || ch.OrganizationID != "org_a" {
		t.Fatalf("expected org_a channel scope after reload, ok=%v channel=%+v", ok, ch)
	}
	if ch, ok := pool.GetChannel("relay_runtime_org_b"); !ok || ch.OrganizationID != "org_b" {
		t.Fatalf("expected org_b channel scope after reload, ok=%v channel=%+v", ok, ch)
	}

	lb := NewLoadBalancer(pool, "weighted")
	ch := lb.SelectModelForOrganization("chat", "gpt-4o-mini", "org_a")
	if ch == nil || ch.Channel == nil || ch.Channel.ID != "relay_runtime_org_a" {
		t.Fatalf("expected DB-loaded route to select org_a channel, got %+v", ch)
	}
	ch = lb.SelectModelForOrganization("chat", "gpt-4o-mini", "org_b")
	if ch == nil || ch.Channel == nil || ch.Channel.ID != "relay_runtime_org_b" {
		t.Fatalf("expected DB-loaded route to select org_b channel, got %+v", ch)
	}
	if ch = lb.SelectModelForOrganization("chat", "gpt-4o-mini", "org_missing"); ch != nil {
		t.Fatalf("expected DB-loaded route to fail closed for missing org, got %+v", ch)
	}
}

func TestRelayStoreProtectsChannelAPIKeyAtRestAndHydratesRuntimeKey(t *testing.T) {
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", "test-relay-runtime-channel-secret")
	store, database, _ := testRelayChannelSQLStore(t)

	if _, err := database.Exec(`
		INSERT INTO organizations (id, slug, name, created_at, updated_at)
		VALUES ('org_secret', 'relay-secret-runtime', 'Relay Secret Runtime', NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert relay runtime organization: %v", err)
	}

	channel := &types.Channel{
		ID:                 "relay_secret_channel",
		OrganizationID:     "org_secret",
		Name:               "Runtime Secret Channel",
		Provider:           "openai",
		BaseURL:            "https://runtime-secret.example.test",
		APIKey:             "sk-runtime-secret",
		Models:             []string{"gpt-4o-mini"},
		Groups:             []string{"default"},
		RPMLimit:           100,
		TPMLimit:           100000,
		CBThreshold:        5,
		CBTimeout:          30,
		Strategy:           "weighted",
		Priority:           10,
		Weight:             100,
		EstimatedCostPer1K: 0.02,
		CostMultiplier:     1.1,
		Enabled:            true,
	}
	if err := store.CreateChannel(channel); err != nil {
		t.Fatalf("CreateChannel returned error: %v", err)
	}
	assertRelayStoreProtectedAPIKey(t, database, channel.ID, "sk-runtime-secret")

	got, err := store.GetChannel(channel.ID)
	if err != nil {
		t.Fatalf("GetChannel returned error: %v", err)
	}
	if got == nil || got.APIKey != "sk-runtime-secret" {
		t.Fatalf("expected GetChannel to hydrate raw runtime API key, got %+v", got)
	}

	channel.APIKey = "sk-runtime-rotated"
	channel.BaseURL = "https://runtime-secret-rotated.example.test"
	if err := store.UpdateChannel(channel); err != nil {
		t.Fatalf("UpdateChannel returned error: %v", err)
	}
	assertRelayStoreProtectedAPIKey(t, database, channel.ID, "sk-runtime-rotated")

	channels, err := store.ListChannels()
	if err != nil {
		t.Fatalf("ListChannels returned error: %v", err)
	}
	found := false
	for _, ch := range channels {
		if ch.ID == channel.ID {
			found = true
			if ch.APIKey != "sk-runtime-rotated" {
				t.Fatalf("expected ListChannels to hydrate raw runtime API key, got %+v", ch)
			}
		}
	}
	if !found {
		t.Fatalf("expected ListChannels to include %s", channel.ID)
	}

	if _, err := database.Exec(`
		INSERT INTO channels (
			id, organization_id, name, provider, base_url, api_key_encrypted, models, groups,
			rpm_limit, tpm_limit, cb_threshold, cb_timeout, strategy, priority, weight,
			estimated_cost_per_1k, cost_multiplier, enabled, created_at, updated_at
		)
		VALUES (
			'relay_legacy_plaintext_channel', 'org_secret', 'Legacy Plaintext Channel', 'openai',
			'https://legacy-secret.example.test', 'sk-legacy-plaintext',
			ARRAY['gpt-4o-mini']::text[], ARRAY['default']::text[],
			100, 100000, 5, 30, 'weighted', 5, 100, 0.02, 1.1, true, NOW(), NOW()
		)
	`); err != nil {
		t.Fatalf("insert legacy plaintext relay channel: %v", err)
	}
	legacy, err := store.GetChannel("relay_legacy_plaintext_channel")
	if err != nil {
		t.Fatalf("GetChannel legacy returned error: %v", err)
	}
	if legacy == nil || legacy.APIKey != "sk-legacy-plaintext" {
		t.Fatalf("expected legacy plaintext channel to remain readable, got %+v", legacy)
	}
}

func assertRelayStoreProtectedAPIKey(t *testing.T, database *sql.DB, channelID, want string) {
	t.Helper()

	var stored string
	if err := database.QueryRow(`SELECT api_key_encrypted FROM channels WHERE id = $1`, channelID).Scan(&stored); err != nil {
		t.Fatalf("query stored relay channel api key: %v", err)
	}
	if stored == "" || stored == want || strings.Contains(stored, want) {
		t.Fatalf("expected stored relay channel api key to be protected, got %q", stored)
	}
	if !secretbox.IsProtected(stored) {
		t.Fatalf("expected stored relay channel api key to use protected prefix, got %q", stored)
	}
	opened, err := secretbox.Open(secretbox.DomainRelayChannelAPIKey, stored)
	if err != nil {
		t.Fatalf("open stored relay channel api key: %v", err)
	}
	if opened != want {
		t.Fatalf("opened relay channel api key = %q, want %q", opened, want)
	}
}

func TestRelayStoreSaveFileMappingPersistsTenantOwnership(t *testing.T) {
	store, database, ctx := testRelaySQLStore(t)

	createdAt := time.Date(2026, 6, 5, 8, 30, 0, 0, time.UTC)
	record := handler.FileMappingRecord{
		LocalFileID:    "file_local_123",
		OpenAIFileID:   "file_openai_123",
		LocalPath:      "/tmp/oblivious-relay-files/files/file_local_123.jsonl",
		SizeBytes:      42,
		UserID:         "user_1",
		OrganizationID: "org_1",
		RequestID:      "req_file_123",
		CreatedAt:      createdAt,
	}

	if err := store.SaveFileMapping(ctx, record); err != nil {
		t.Fatalf("SaveFileMapping returned error: %v", err)
	}

	var got handler.FileMappingRecord
	if err := database.QueryRowContext(ctx, `
		SELECT local_file_id, openai_file_id, local_path, size_bytes,
		       user_id, organization_id, request_id, created_at
		FROM relay_file_mappings
		WHERE local_file_id = $1
	`, record.LocalFileID).Scan(
		&got.LocalFileID,
		&got.OpenAIFileID,
		&got.LocalPath,
		&got.SizeBytes,
		&got.UserID,
		&got.OrganizationID,
		&got.RequestID,
		&got.CreatedAt,
	); err != nil {
		t.Fatalf("query saved file mapping: %v", err)
	}

	if got.LocalFileID != record.LocalFileID ||
		got.OpenAIFileID != record.OpenAIFileID ||
		got.LocalPath != record.LocalPath ||
		got.SizeBytes != record.SizeBytes ||
		got.UserID != record.UserID ||
		got.OrganizationID != record.OrganizationID ||
		got.RequestID != record.RequestID ||
		!got.CreatedAt.Equal(createdAt) {
		t.Fatalf("saved file mapping mismatch:\n got: %+v\nwant: %+v", got, record)
	}

	if err := store.SaveFileMapping(ctx, record); err == nil {
		t.Fatal("duplicate local file mapping should be rejected")
	}
}

func TestRelayStoreGetFileMappingRequiresTenantOwnership(t *testing.T) {
	store, _, ctx := testRelaySQLStore(t)

	record := handler.FileMappingRecord{
		LocalFileID:    "file_local_123",
		OpenAIFileID:   "file_openai_123",
		LocalPath:      "/tmp/oblivious-relay-files/files/file_local_123.jsonl",
		SizeBytes:      42,
		UserID:         "user_1",
		OrganizationID: "org_1",
		RequestID:      "req_file_123",
		CreatedAt:      time.Date(2026, 6, 5, 8, 30, 0, 0, time.UTC),
	}
	if err := store.SaveFileMapping(ctx, record); err != nil {
		t.Fatalf("SaveFileMapping returned error: %v", err)
	}

	got, err := store.GetFileMapping(ctx, "file_local_123", "user_1", "org_1")
	if err != nil {
		t.Fatalf("GetFileMapping returned error: %v", err)
	}
	if got.LocalFileID != record.LocalFileID ||
		got.OpenAIFileID != record.OpenAIFileID ||
		got.UserID != record.UserID ||
		got.OrganizationID != record.OrganizationID {
		t.Fatalf("lookup mismatch:\n got: %+v\nwant: %+v", got, record)
	}

	if _, err := store.GetFileMapping(ctx, "file_local_123", "user_2", "org_1"); err == nil {
		t.Fatal("lookup with wrong user should fail closed")
	}
	if _, err := store.GetFileMapping(ctx, "file_local_123", "user_1", "org_2"); err == nil {
		t.Fatal("lookup with wrong organization should fail closed")
	}
}

func TestRelayStoreListFileMappingsRequiresTenantOwnership(t *testing.T) {
	store, _, ctx := testRelaySQLStore(t)

	records := []handler.FileMappingRecord{
		{
			LocalFileID:    "file_local_old",
			OpenAIFileID:   "file_openai_old",
			LocalPath:      "/tmp/oblivious-relay-files/files/file_local_old.jsonl",
			SizeBytes:      42,
			UserID:         "user_1",
			OrganizationID: "org_1",
			RequestID:      "req_file_old",
			CreatedAt:      time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC),
		},
		{
			LocalFileID:    "file_local_new",
			OpenAIFileID:   "file_openai_new",
			LocalPath:      "/tmp/oblivious-relay-files/files/file_local_new.jsonl",
			SizeBytes:      84,
			UserID:         "user_1",
			OrganizationID: "org_1",
			RequestID:      "req_file_new",
			CreatedAt:      time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC),
		},
		{
			LocalFileID:    "file_local_other_user",
			OpenAIFileID:   "file_openai_other_user",
			LocalPath:      "/tmp/oblivious-relay-files/files/file_local_other_user.jsonl",
			SizeBytes:      21,
			UserID:         "user_2",
			OrganizationID: "org_1",
			RequestID:      "req_file_other_user",
			CreatedAt:      time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC),
		},
		{
			LocalFileID:    "file_local_other_org",
			OpenAIFileID:   "file_openai_other_org",
			LocalPath:      "/tmp/oblivious-relay-files/files/file_local_other_org.jsonl",
			SizeBytes:      21,
			UserID:         "user_1",
			OrganizationID: "org_2",
			RequestID:      "req_file_other_org",
			CreatedAt:      time.Date(2026, 6, 5, 11, 0, 0, 0, time.UTC),
		},
	}
	for _, record := range records {
		if err := store.SaveFileMapping(ctx, record); err != nil {
			t.Fatalf("SaveFileMapping(%s) returned error: %v", record.LocalFileID, err)
		}
	}

	got, err := store.ListFileMappings(ctx, "user_1", "org_1")
	if err != nil {
		t.Fatalf("ListFileMappings returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("mapping count = %d, want 2: %+v", len(got), got)
	}
	if got[0].LocalFileID != "file_local_new" || got[0].OpenAIFileID != "file_openai_new" ||
		got[1].LocalFileID != "file_local_old" || got[1].OpenAIFileID != "file_openai_old" {
		t.Fatalf("unexpected tenant list order/content: %+v", got)
	}

	wrongUser, err := store.ListFileMappings(ctx, "user_2", "org_2")
	if err != nil {
		t.Fatalf("ListFileMappings wrong tenant returned error: %v", err)
	}
	if len(wrongUser) != 0 {
		t.Fatalf("wrong tenant list leaked mappings: %+v", wrongUser)
	}
}

func TestRelayStoreTombstoneFileMappingHidesTenantMapping(t *testing.T) {
	store, database, ctx := testRelaySQLStore(t)

	record := handler.FileMappingRecord{
		LocalFileID:    "file_local_delete",
		OpenAIFileID:   "file_openai_delete",
		LocalPath:      "/tmp/oblivious-relay-files/files/file_local_delete.jsonl",
		SizeBytes:      42,
		UserID:         "user_1",
		OrganizationID: "org_1",
		RequestID:      "req_file_delete",
		CreatedAt:      time.Date(2026, 6, 5, 8, 30, 0, 0, time.UTC),
	}
	if err := store.SaveFileMapping(ctx, record); err != nil {
		t.Fatalf("SaveFileMapping returned error: %v", err)
	}

	if err := store.TombstoneFileMapping(ctx, record.LocalFileID, "user_2", record.OrganizationID, time.Now().UTC()); err == nil {
		t.Fatal("wrong tenant tombstone should fail closed")
	}

	deletedAt := time.Date(2026, 7, 4, 13, 55, 0, 0, time.UTC)
	if err := store.TombstoneFileMapping(ctx, record.LocalFileID, record.UserID, record.OrganizationID, deletedAt); err != nil {
		t.Fatalf("TombstoneFileMapping returned error: %v", err)
	}

	var storedDeletedAt time.Time
	if err := database.QueryRowContext(ctx, `
		SELECT deleted_at
		FROM relay_file_mappings
		WHERE local_file_id = $1
	`, record.LocalFileID).Scan(&storedDeletedAt); err != nil {
		t.Fatalf("query deleted_at: %v", err)
	}
	if !storedDeletedAt.Equal(deletedAt) {
		t.Fatalf("deleted_at = %s, want %s", storedDeletedAt, deletedAt)
	}

	if _, err := store.GetFileMapping(ctx, record.LocalFileID, record.UserID, record.OrganizationID); !errors.Is(err, handler.ErrFileMappingNotFound) {
		t.Fatalf("GetFileMapping after tombstone error = %v, want ErrFileMappingNotFound", err)
	}

	got, err := store.ListFileMappings(ctx, record.UserID, record.OrganizationID)
	if err != nil {
		t.Fatalf("ListFileMappings after tombstone returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("tombstoned mapping should be hidden from list: %+v", got)
	}
}

func TestRelayStoreRegisterBatchPollingPersistsDurableTask(t *testing.T) {
	store, database, ctx := testRelaySQLStore(t)

	task := handler.BatchPollingRegistration{
		BatchID:                  "batch_123",
		RequestID:                "req_batch_123",
		Model:                    "gpt-4o",
		APIType:                  types.APITypeBatch,
		UserID:                   "user_batch",
		OrganizationID:           "org_batch",
		APITokenID:               "tok_batch",
		FeatureType:              "workflow",
		BillingSessionID:         "bill_batch_123",
		PreauthorizedAmount:      1.25,
		TokenPreauthorizedAmount: 1.5,
	}
	if err := store.RegisterBatchPolling(ctx, task); err != nil {
		t.Fatalf("RegisterBatchPolling returned error: %v", err)
	}

	var got struct {
		BatchID                  string
		RequestID                string
		UserID                   string
		OrgID                    string
		APITokenID               string
		FeatureType              string
		Model                    string
		APIType                  string
		Status                   string
		Attempts                 int
		MaxAttempts              int
		BillingSessionID         string
		PreauthorizedAmount      float64
		TokenPreauthorizedAmount float64
		AvailableAt              time.Time
		CreatedAt                time.Time
	}
	if err := database.QueryRowContext(ctx, `
		SELECT batch_id, request_id, user_id, organization_id, api_token_id, feature_type, model, api_type, status, attempts, max_attempts, billing_session_id, preauthorized_amount, token_preauthorized_amount, available_at, created_at
		FROM relay_batch_polling_jobs
		WHERE batch_id = $1
	`, task.BatchID).Scan(
		&got.BatchID,
		&got.RequestID,
		&got.UserID,
		&got.OrgID,
		&got.APITokenID,
		&got.FeatureType,
		&got.Model,
		&got.APIType,
		&got.Status,
		&got.Attempts,
		&got.MaxAttempts,
		&got.BillingSessionID,
		&got.PreauthorizedAmount,
		&got.TokenPreauthorizedAmount,
		&got.AvailableAt,
		&got.CreatedAt,
	); err != nil {
		t.Fatalf("query batch polling job: %v", err)
	}

	if got.BatchID != task.BatchID ||
		got.RequestID != task.RequestID ||
		got.UserID != task.UserID ||
		got.OrgID != task.OrganizationID ||
		got.APITokenID != task.APITokenID ||
		got.FeatureType != task.FeatureType ||
		got.Model != task.Model ||
		got.APIType != task.APIType.String() ||
		got.Status != "pending" ||
		got.Attempts != 0 ||
		got.MaxAttempts != 5 ||
		got.BillingSessionID != task.BillingSessionID ||
		got.PreauthorizedAmount != task.PreauthorizedAmount ||
		got.TokenPreauthorizedAmount != task.TokenPreauthorizedAmount ||
		got.AvailableAt.IsZero() ||
		got.CreatedAt.IsZero() {
		t.Fatalf("unexpected batch polling job: %+v", got)
	}
}

func TestRelayStoreRegisterBatchPollingRejectsMissingBatchID(t *testing.T) {
	store, _, ctx := testRelaySQLStore(t)

	if err := store.RegisterBatchPolling(ctx, handler.BatchPollingRegistration{
		RequestID: "req_missing_batch",
		Model:     "gpt-4o",
		APIType:   types.APITypeBatch,
	}); err == nil {
		t.Fatal("RegisterBatchPolling should reject missing batch id")
	}
}

func TestRelayStoreClaimBatchPollingJobsLeasesDueJobs(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer database.Close()

	store := NewRelayStore(database)
	now := time.Date(2026, 7, 5, 4, 30, 0, 0, time.UTC)
	leaseUntil := now.Add(defaultRelayBatchPollingJobClaimLease)
	lockedAt := now
	rows := sqlmock.NewRows([]string{
		"batch_id", "request_id", "user_id", "organization_id", "api_token_id", "feature_type", "model", "api_type", "status", "error", "attempts", "max_attempts",
		"billing_session_id", "preauthorized_amount", "token_preauthorized_amount", "locked_at", "locked_by", "available_at", "completed_at", "created_at", "updated_at",
	}).AddRow(
		"batch_123", "req_batch_123", "user_batch", "org_batch", "tok_batch", "workflow", "gpt-4o", types.APITypeBatch.String(), RelayBatchPollingJobStatusProcessing,
		"", 1, 5, "bill_batch_123", 1.25, 1.5, lockedAt, "worker_batch_1", leaseUntil, nil, now.Add(-time.Minute), now,
	)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE relay_batch_polling_jobs job")).
		WithArgs(now, RelayBatchPollingJobStatusPending, RelayBatchPollingJobStatusFailed, RelayBatchPollingJobStatusProcessing, 2, "worker_batch_1", leaseUntil).
		WillReturnRows(rows)
	mock.ExpectCommit()

	jobs, err := store.ClaimBatchPollingJobs(context.Background(), now, 2, "worker_batch_1")
	if err != nil {
		t.Fatalf("ClaimBatchPollingJobs returned error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("claimed jobs = %d, want 1", len(jobs))
	}
	job := jobs[0]
	if job.BatchID != "batch_123" ||
		job.RequestID != "req_batch_123" ||
		job.UserID != "user_batch" ||
		job.OrganizationID != "org_batch" ||
		job.APITokenID != "tok_batch" ||
		job.FeatureType != "workflow" ||
		job.Model != "gpt-4o" ||
		job.APIType != types.APITypeBatch.String() ||
		job.Status != RelayBatchPollingJobStatusProcessing ||
		job.Attempts != 1 ||
		job.MaxAttempts != 5 ||
		job.BillingSessionID != "bill_batch_123" ||
		job.PreauthorizedAmount != 1.25 ||
		job.TokenPreauthorizedAmount != 1.5 ||
		job.LockedBy != "worker_batch_1" ||
		job.LockedAt == nil ||
		!job.AvailableAt.Equal(leaseUntil) {
		t.Fatalf("unexpected claimed job: %+v", job)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestRelayStoreMarkBatchPollingJobTerminalStatesUseOwnerGuard(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer database.Close()

	store := NewRelayStore(database)
	completedAt := time.Date(2026, 7, 5, 4, 35, 0, 0, time.UTC)
	availableAt := time.Date(2026, 7, 5, 4, 40, 0, 0, time.UTC)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE relay_batch_polling_jobs")).
		WithArgs("batch_123", "worker_batch_1", RelayBatchPollingJobStatusSucceeded, completedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE relay_batch_polling_jobs")).
		WithArgs("batch_123", "worker_batch_1", RelayBatchPollingJobStatusFailed, "upstream status pending", availableAt, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE relay_batch_polling_jobs")).
		WithArgs("batch_123", "worker_batch_1", RelayBatchPollingJobStatusDeadLetter, "dead_letter: polling attempts exhausted", completedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.MarkBatchPollingJobSucceeded(context.Background(), "batch_123", "worker_batch_1", completedAt); err != nil {
		t.Fatalf("MarkBatchPollingJobSucceeded returned error: %v", err)
	}
	if err := store.MarkBatchPollingJobFailed(context.Background(), "batch_123", "worker_batch_1", "upstream status pending", availableAt); err != nil {
		t.Fatalf("MarkBatchPollingJobFailed returned error: %v", err)
	}
	if err := store.MarkBatchPollingJobDeadLetter(context.Background(), "batch_123", "worker_batch_1", "dead_letter: polling attempts exhausted", completedAt); err != nil {
		t.Fatalf("MarkBatchPollingJobDeadLetter returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestRelayStoreConversationAffinityPersistsAndUpdatesChannel(t *testing.T) {
	store, database, ctx := testRelaySQLStore(t)

	if err := store.SaveConversationAffinity(ctx, "conversation_1", "primary"); err != nil {
		t.Fatalf("SaveConversationAffinity returned error: %v", err)
	}

	got, err := store.GetConversationAffinity(ctx, "conversation_1")
	if err != nil {
		t.Fatalf("GetConversationAffinity returned error: %v", err)
	}
	if got != "primary" {
		t.Fatalf("initial affinity channel = %q, want primary", got)
	}

	if err := store.SaveConversationAffinity(ctx, "conversation_1", "secondary"); err != nil {
		t.Fatalf("SaveConversationAffinity update returned error: %v", err)
	}

	got, err = store.GetConversationAffinity(ctx, "conversation_1")
	if err != nil {
		t.Fatalf("GetConversationAffinity after update returned error: %v", err)
	}
	if got != "secondary" {
		t.Fatalf("updated affinity channel = %q, want secondary", got)
	}

	var count int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relay_conversation_affinity WHERE conversation_id = $1
	`, "conversation_1").Scan(&count); err != nil {
		t.Fatalf("count affinity rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("affinity row count = %d, want 1", count)
	}
}
