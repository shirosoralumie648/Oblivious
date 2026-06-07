package relay

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"oblivious/server/internal/relay/handler"
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
	affinityMigration, err := os.ReadFile("../../migrations/0062_relay_conversation_affinity.sql")
	if err != nil {
		t.Fatalf("read relay conversation affinity migration: %v", err)
	}
	if _, err := database.Exec(string(affinityMigration)); err != nil {
		t.Fatalf("apply relay conversation affinity migration: %v", err)
	}

	return NewRelayStore(database), database, context.Background()
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
