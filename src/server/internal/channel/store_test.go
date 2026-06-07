package channel

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestUpdateRetryMessageLogSQLPersistsFallbackChannelID(t *testing.T) {
	for _, fragment := range []string{
		"channel_id = $2",
		"conversation_id = $3",
		"RETURNING id, channel_id",
	} {
		if !strings.Contains(updateRetryMessageLogSQL, fragment) {
			t.Fatalf("expected retry update SQL to include %q, got: %s", fragment, updateRetryMessageLogSQL)
		}
	}
}

func TestArchiveExpiredMessageLogsSQLPreservesRetryQueueAndBatchesDeletes(t *testing.T) {
	for _, fragment := range []string{
		"DELETE FROM channel_messages",
		"created_at < $1",
		"status IN ($2, $3)",
		"ORDER BY created_at ASC, id ASC",
		"LIMIT $4",
		"RETURNING id",
	} {
		if !strings.Contains(archiveExpiredMessageLogsSQL, fragment) {
			t.Fatalf("expected expired message archive SQL to include %q, got: %s", fragment, archiveExpiredMessageLogsSQL)
		}
	}
	if strings.Contains(archiveExpiredMessageLogsSQL, string(MessageStatusRetryPending)) || strings.Contains(archiveExpiredMessageLogsSQL, string(MessageStatusSending)) {
		t.Fatalf("expired message archive SQL must not delete retry queue statuses directly: %s", archiveExpiredMessageLogsSQL)
	}

	before, limit := normalizeArchiveExpiredMessageLogsInput(ArchiveExpiredMessageLogsInput{})
	if !before.IsZero() {
		t.Fatalf("expected zero cutoff to remain zero for caller validation, got %s", before)
	}
	if limit != defaultArchiveMessageLogLimit {
		t.Fatalf("expected default archive limit %d, got %d", defaultArchiveMessageLogLimit, limit)
	}
	cutoff := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	before, limit = normalizeArchiveExpiredMessageLogsInput(ArchiveExpiredMessageLogsInput{
		Before: cutoff,
		Limit:  defaultArchiveMessageLogLimit + 50,
	})
	if !before.Equal(cutoff) {
		t.Fatalf("expected cutoff to be preserved, got %s", before)
	}
	if limit != defaultArchiveMessageLogLimit {
		t.Fatalf("expected archive limit cap %d, got %d", defaultArchiveMessageLogLimit, limit)
	}
}

func TestMessageLogRetentionPolicyBuildsDefaultAndCustomArchiveCutoffs(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	defaultInput := MessageLogRetentionPolicy{
		Now: func() time.Time { return now },
	}.ArchiveInput()
	if !defaultInput.Before.Equal(now.Add(-30 * 24 * time.Hour)) {
		t.Fatalf("expected default 30 day retention cutoff, got %s", defaultInput.Before)
	}

	customInput := MessageLogRetentionPolicy{
		Retention: 7 * 24 * time.Hour,
		Limit:     25,
		Now:       func() time.Time { return now },
	}.ArchiveInput()
	if !customInput.Before.Equal(now.Add(-7*24*time.Hour)) || customInput.Limit != 25 {
		t.Fatalf("expected custom retention archive input, got %+v", customInput)
	}
}

func testChannelSQLStore(t *testing.T) (*SQLStore, *sql.DB, context.Context) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if strings.EqualFold(os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE"), "true") {
			t.Fatal("TEST_DATABASE_URL is required for DB-backed channel publishing tests")
		}
		t.Skip("TEST_DATABASE_URL is required for DB-backed channel publishing tests")
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

	if _, err := database.Exec(`SELECT pg_advisory_lock(104243)`); err != nil {
		t.Fatalf("lock channel publishing test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104243)`); err != nil {
			t.Fatalf("unlock channel publishing test database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS channel_messages CASCADE`,
		`DROP TABLE IF EXISTS channel_configs CASCADE`,
		`DROP TABLE IF EXISTS agent_conversations CASCADE`,
		`DROP TABLE IF EXISTS conversations CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, title TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`INSERT INTO organizations (id, slug, name) VALUES ('org_1', 'channel-org-1', 'Channel Org 1'), ('org_2', 'channel-org-2', 'Channel Org 2')`,
		`INSERT INTO conversations (id, organization_id, title) VALUES ('conversation_1', 'org_1', 'Publishing Conversation')`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare channel publishing database: %v\nstatement: %s", err, statement)
		}
	}

	for _, migrationPath := range []string{
		"../../migrations/0043_channel_publishing.sql",
		"../../migrations/0053_channel_retry_claim_status.sql",
	} {
		migration, err := os.ReadFile(migrationPath)
		if err != nil {
			if os.IsNotExist(err) && strings.Contains(migrationPath, "0053_") {
				continue
			}
			t.Fatalf("read channel publishing migration %s: %v", migrationPath, err)
		}
		if _, err := database.Exec(string(migration)); err != nil {
			t.Fatalf("apply channel publishing migration %s: %v", migrationPath, err)
		}
	}

	return NewSQLStore(database), database, context.Background()
}

func TestChannelSQLStorePersistsConfigsAndMessageLogs(t *testing.T) {
	store, database, ctx := testChannelSQLStore(t)

	created, err := store.CreateConfig(ctx, &ChannelConfig{
		ID:             "channel_config_1",
		OrganizationID: "org_1",
		Type:           ChannelTypeWebhook,
		Name:           "Ops Webhook",
		Config: map[string]any{
			"secret": "shared-secret",
			"events": []any{"message.created"},
		},
	})
	if err != nil {
		t.Fatalf("CreateConfig returned error: %v", err)
	}
	if created.Status != ChannelStatusActive {
		t.Fatalf("expected default active status, got %q", created.Status)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps on created config: %+v", created)
	}

	got, err := store.GetConfig(ctx, "org_1", created.ID)
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}
	if got == nil || got.ID != created.ID || got.Name != "Ops Webhook" {
		t.Fatalf("unexpected fetched config: %+v", got)
	}
	if got.Config["secret"] != "shared-secret" {
		t.Fatalf("expected config JSON to round trip, got %+v", got.Config)
	}

	other, err := store.CreateConfig(ctx, &ChannelConfig{
		ID:             "channel_config_2",
		OrganizationID: "org_2",
		Type:           ChannelTypeWebhook,
		Name:           "Other Org Webhook",
		Config:         map[string]any{"secret": "other"},
	})
	if err != nil {
		t.Fatalf("CreateConfig for other org returned error: %v", err)
	}
	configs, err := store.ListConfigs(ctx, "org_1")
	if err != nil {
		t.Fatalf("ListConfigs returned error: %v", err)
	}
	if len(configs) != 1 || configs[0].ID != created.ID {
		t.Fatalf("expected list scoped to org_1, got %+v; other=%+v", configs, other)
	}

	degraded, err := store.UpdateConfigStatus(ctx, "org_1", created.ID, ChannelStatusDegraded)
	if err != nil {
		t.Fatalf("UpdateConfigStatus returned error: %v", err)
	}
	if degraded.Status != ChannelStatusDegraded {
		t.Fatalf("expected degraded status, got %+v", degraded)
	}
	if !degraded.UpdatedAt.After(degraded.CreatedAt) && !degraded.UpdatedAt.Equal(degraded.CreatedAt) {
		t.Fatalf("expected updated_at to be maintained, got %+v", degraded)
	}

	updated, err := store.UpdateConfig(ctx, "org_1", created.ID, ConfigUpdate{
		Type:   ChannelTypeWebhook,
		Name:   "Ops Webhook Renamed",
		Config: map[string]any{"secret": "rotated-secret"},
		Status: ChannelStatusActive,
	})
	if err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}
	if updated.Name != "Ops Webhook Renamed" || updated.Status != ChannelStatusActive || updated.Config["secret"] != "rotated-secret" {
		t.Fatalf("expected full config update to round trip, got %+v", updated)
	}
	wrongOrg, err := store.UpdateConfig(ctx, "org_2", created.ID, ConfigUpdate{
		Type:   ChannelTypeWebhook,
		Name:   "Wrong Org",
		Config: map[string]any{},
		Status: ChannelStatusDisabled,
	})
	if err != nil {
		t.Fatalf("UpdateConfig wrong org returned error: %v", err)
	}
	if wrongOrg != nil {
		t.Fatalf("expected wrong organization update to return nil, got %+v", wrongOrg)
	}

	successLog, err := store.RecordMessageLog(ctx, &ChannelMessageLog{
		ID:             "channel_message_1",
		ChannelID:      created.ID,
		ConversationID: "conversation_1",
		Direction:      DirectionInbound,
		RawMessage:     json.RawMessage(`{"text":"hello from webhook"}`),
		TransformedMessage: InternalMessage{
			ID:             "msg_1",
			ConversationID: "conversation_1",
			Role:           RoleUser,
			Content:        []ContentPart{{Type: ContentTypeText, Text: "hello from webhook"}},
			Timestamp:      time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		},
		TransformSuccess: true,
	})
	if err != nil {
		t.Fatalf("RecordMessageLog success returned error: %v", err)
	}
	if successLog.ID != "channel_message_1" || !successLog.TransformSuccess {
		t.Fatalf("unexpected success log: %+v", successLog)
	}

	failureLog, err := store.RecordMessageLog(ctx, &ChannelMessageLog{
		ID:               "channel_message_2",
		ChannelID:        created.ID,
		Direction:        DirectionInbound,
		RawMessage:       json.RawMessage(`{"role":"invalid"}`),
		TransformSuccess: false,
		TransformError:   "invalid role",
	})
	if err != nil {
		t.Fatalf("RecordMessageLog failure returned error: %v", err)
	}
	if failureLog.TransformSuccess || failureLog.TransformError != "invalid role" {
		t.Fatalf("unexpected failure log: %+v", failureLog)
	}

	retryAt := time.Date(2026, 6, 4, 12, 31, 0, 0, time.UTC)
	retryLog, err := store.RecordMessageLog(ctx, &ChannelMessageLog{
		ID:                 "channel_message_retry_1",
		ChannelID:          created.ID,
		ConversationID:     "conversation_1",
		Direction:          DirectionOutbound,
		RawMessage:         json.RawMessage(`{"text":"send me later"}`),
		TransformedMessage: InternalMessage{ID: "msg_retry_1", ConversationID: "conversation_1", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "send me later"}}},
		TransformSuccess:   true,
		Status:             MessageStatusRetryPending,
		RetryCount:         1,
		FailureReason:      "upstream 500",
		NextRetryAt:        &retryAt,
	})
	if err != nil {
		t.Fatalf("RecordMessageLog retry returned error: %v", err)
	}
	if retryLog.Status != MessageStatusRetryPending || retryLog.RetryCount != 1 || retryLog.FailureReason != "upstream 500" || retryLog.NextRetryAt == nil {
		t.Fatalf("unexpected retry log: %+v", retryLog)
	}

	var rawMessage, transformedMessage []byte
	var transformSuccess bool
	var transformError sql.NullString
	if err := database.QueryRowContext(ctx, `
		SELECT raw_message, transformed_message, transform_success, transform_error
		FROM channel_messages
		WHERE id = 'channel_message_1'
	`).Scan(&rawMessage, &transformedMessage, &transformSuccess, &transformError); err != nil {
		t.Fatalf("query success message log: %v", err)
	}
	if string(rawMessage) != `{"text": "hello from webhook"}` {
		t.Fatalf("expected raw message JSONB to persist, got %s", rawMessage)
	}
	if !transformSuccess || transformError.Valid {
		t.Fatalf("expected successful transform fields, success=%v error=%+v", transformSuccess, transformError)
	}
	var transformed InternalMessage
	if err := json.Unmarshal(transformedMessage, &transformed); err != nil {
		t.Fatalf("unmarshal transformed message: %v", err)
	}
	if transformed.ID != "msg_1" || transformed.Content[0].Text != "hello from webhook" {
		t.Fatalf("unexpected transformed payload: %+v", transformed)
	}

	var failedTransformed sql.NullString
	if err := database.QueryRowContext(ctx, `
		SELECT transformed_message, transform_success, transform_error
		FROM channel_messages
		WHERE id = 'channel_message_2'
	`).Scan(&failedTransformed, &transformSuccess, &transformError); err != nil {
		t.Fatalf("query failed message log: %v", err)
	}
	if failedTransformed.Valid || transformSuccess || !transformError.Valid || transformError.String != "invalid role" {
		t.Fatalf("expected failed transform fields, transformed=%+v success=%v error=%+v", failedTransformed, transformSuccess, transformError)
	}

	var status string
	var retryCount int
	var failureReason sql.NullString
	var nextRetryAt sql.NullTime
	if err := database.QueryRowContext(ctx, `
		SELECT status, retry_count, failure_reason, next_retry_at
		FROM channel_messages
		WHERE id = 'channel_message_retry_1'
	`).Scan(&status, &retryCount, &failureReason, &nextRetryAt); err != nil {
		t.Fatalf("query retry message log: %v", err)
	}
	if status != string(MessageStatusRetryPending) || retryCount != 1 || !failureReason.Valid || failureReason.String != "upstream 500" {
		t.Fatalf("expected retry queue fields, status=%q retry=%d reason=%+v", status, retryCount, failureReason)
	}
	if !nextRetryAt.Valid || !nextRetryAt.Time.Equal(retryAt) {
		t.Fatalf("expected next retry timestamp %s, got %+v", retryAt, nextRetryAt)
	}
}

func TestChannelSQLStoreCountsConsecutiveSuccessfulOutboundDeliveries(t *testing.T) {
	store, _, ctx := testChannelSQLStore(t)

	created, err := store.CreateConfig(ctx, &ChannelConfig{
		ID:             "channel_config_successes",
		OrganizationID: "org_1",
		Type:           ChannelTypeWebhook,
		Name:           "Ops Webhook",
		Config:         map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateConfig returned error: %v", err)
	}

	baseTime := time.Date(2026, 6, 4, 15, 0, 0, 0, time.UTC)
	logs := []ChannelMessageLog{
		{
			ID:               "channel_message_success_1",
			ChannelID:        created.ID,
			Direction:        DirectionOutbound,
			RawMessage:       json.RawMessage(`{"text":"success before failure"}`),
			TransformSuccess: true,
			Status:           MessageStatusRecorded,
			CreatedAt:        baseTime,
		},
		{
			ID:               "channel_message_failure_1",
			ChannelID:        created.ID,
			Direction:        DirectionOutbound,
			RawMessage:       json.RawMessage(`{"text":"failed"}`),
			TransformSuccess: false,
			TransformError:   "missing content",
			Status:           MessageStatusRetryPending,
			CreatedAt:        baseTime.Add(time.Minute),
		},
		{
			ID:               "channel_message_success_2",
			ChannelID:        created.ID,
			Direction:        DirectionOutbound,
			RawMessage:       json.RawMessage(`{"text":"success two"}`),
			TransformSuccess: true,
			Status:           MessageStatusRecorded,
			CreatedAt:        baseTime.Add(2 * time.Minute),
		},
		{
			ID:               "channel_message_success_3",
			ChannelID:        created.ID,
			Direction:        DirectionOutbound,
			RawMessage:       json.RawMessage(`{"text":"success three"}`),
			TransformSuccess: true,
			Status:           MessageStatusRecorded,
			CreatedAt:        baseTime.Add(3 * time.Minute),
		},
	}
	for _, log := range logs {
		if _, err := store.RecordMessageLog(ctx, &log); err != nil {
			t.Fatalf("RecordMessageLog(%s) returned error: %v", log.ID, err)
		}
	}

	count, err := store.CountConsecutiveSuccessfulDeliveries(ctx, created.ID, 3)
	if err != nil {
		t.Fatalf("CountConsecutiveSuccessfulDeliveries returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected latest failure to break the success streak at 2, got %d", count)
	}

	if _, err := store.RecordMessageLog(ctx, &ChannelMessageLog{
		ID:               "channel_message_success_4",
		ChannelID:        created.ID,
		Direction:        DirectionOutbound,
		RawMessage:       json.RawMessage(`{"text":"success four"}`),
		TransformSuccess: true,
		Status:           MessageStatusRecorded,
		CreatedAt:        baseTime.Add(4 * time.Minute),
	}); err != nil {
		t.Fatalf("RecordMessageLog final success returned error: %v", err)
	}
	count, err = store.CountConsecutiveSuccessfulDeliveries(ctx, created.ID, 3)
	if err != nil {
		t.Fatalf("CountConsecutiveSuccessfulDeliveries final returned error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected three consecutive successes after final log, got %d", count)
	}
}

func TestChannelSQLStoreListsAndClaimsDueRetryMessages(t *testing.T) {
	store, database, ctx := testChannelSQLStore(t)

	created, err := store.CreateConfig(ctx, &ChannelConfig{
		ID:             "channel_config_retry_claim",
		OrganizationID: "org_1",
		Type:           ChannelTypeWebhook,
		Name:           "Retry Webhook",
		Config:         map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateConfig returned error: %v", err)
	}

	now := time.Date(2026, 6, 4, 16, 0, 0, 0, time.UTC)
	dueFirst := now.Add(-10 * time.Minute)
	dueSecond := now.Add(-5 * time.Minute)
	dueThird := dueSecond
	future := now.Add(time.Minute)

	logs := []ChannelMessageLog{
		{
			ID:                 "channel_message_due_second",
			ChannelID:          created.ID,
			ConversationID:     "conversation_1",
			Direction:          DirectionOutbound,
			RawMessage:         json.RawMessage(`{"text":"second"}`),
			TransformedMessage: InternalMessage{ID: "msg_due_second", ConversationID: "conversation_1", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "second"}}},
			TransformSuccess:   true,
			Status:             MessageStatusRetryPending,
			RetryCount:         1,
			FailureReason:      "temporary failure",
			NextRetryAt:        &dueSecond,
			CreatedAt:          now.Add(-8 * time.Minute),
		},
		{
			ID:                 "channel_message_due_first",
			ChannelID:          created.ID,
			ConversationID:     "conversation_1",
			Direction:          DirectionOutbound,
			RawMessage:         json.RawMessage(`{"text":"first"}`),
			TransformedMessage: InternalMessage{ID: "msg_due_first", ConversationID: "conversation_1", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "first"}}},
			TransformSuccess:   true,
			Status:             MessageStatusRetryPending,
			RetryCount:         2,
			FailureReason:      "temporary failure",
			NextRetryAt:        &dueFirst,
			CreatedAt:          now.Add(-2 * time.Minute),
		},
		{
			ID:                 "channel_message_due_third",
			ChannelID:          created.ID,
			ConversationID:     "conversation_1",
			Direction:          DirectionOutbound,
			RawMessage:         json.RawMessage(`{"text":"third"}`),
			TransformedMessage: InternalMessage{ID: "msg_due_third", ConversationID: "conversation_1", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "third"}}},
			TransformSuccess:   true,
			Status:             MessageStatusRetryPending,
			RetryCount:         1,
			FailureReason:      "temporary failure",
			NextRetryAt:        &dueThird,
			CreatedAt:          now.Add(-3 * time.Minute),
		},
		{
			ID:                 "channel_message_future",
			ChannelID:          created.ID,
			Direction:          DirectionOutbound,
			RawMessage:         json.RawMessage(`{"text":"future"}`),
			TransformedMessage: InternalMessage{ID: "msg_future", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "future"}}},
			TransformSuccess:   true,
			Status:             MessageStatusRetryPending,
			RetryCount:         1,
			NextRetryAt:        &future,
			CreatedAt:          now.Add(-20 * time.Minute),
		},
		{
			ID:               "channel_message_inbound_due",
			ChannelID:        created.ID,
			Direction:        DirectionInbound,
			RawMessage:       json.RawMessage(`{"text":"inbound"}`),
			TransformSuccess: true,
			Status:           MessageStatusRetryPending,
			RetryCount:       1,
			NextRetryAt:      &dueFirst,
			CreatedAt:        now.Add(-30 * time.Minute),
		},
		{
			ID:               "channel_message_recorded_due",
			ChannelID:        created.ID,
			Direction:        DirectionOutbound,
			RawMessage:       json.RawMessage(`{"text":"recorded"}`),
			TransformSuccess: true,
			Status:           MessageStatusRecorded,
			NextRetryAt:      &dueFirst,
			CreatedAt:        now.Add(-40 * time.Minute),
		},
	}
	for _, log := range logs {
		if _, err := store.RecordMessageLog(ctx, &log); err != nil {
			t.Fatalf("RecordMessageLog(%s) returned error: %v", log.ID, err)
		}
	}

	listed, err := store.ListDueRetryMessages(ctx, ClaimDueRetryMessagesInput{Now: now, Limit: 2})
	if err != nil {
		t.Fatalf("ListDueRetryMessages returned error: %v", err)
	}
	assertChannelMessageIDs(t, listed, []string{"channel_message_due_first", "channel_message_due_second"})
	if listed[0].Status != MessageStatusRetryPending || listed[0].TransformedMessage.ID != "msg_due_first" {
		t.Fatalf("expected listed message to preserve retry payload, got %+v", listed[0])
	}

	claimed, err := store.ClaimDueRetryMessages(ctx, ClaimDueRetryMessagesInput{Now: now, Limit: 10})
	if err != nil {
		t.Fatalf("ClaimDueRetryMessages returned error: %v", err)
	}
	assertChannelMessageIDs(t, claimed, []string{"channel_message_due_first", "channel_message_due_second", "channel_message_due_third"})
	for _, log := range claimed {
		if log.Status != MessageStatusSending {
			t.Fatalf("expected claimed message %s to be sending, got %q", log.ID, log.Status)
		}
	}

	claimedAgain, err := store.ClaimDueRetryMessages(ctx, ClaimDueRetryMessagesInput{Now: now, Limit: 10})
	if err != nil {
		t.Fatalf("ClaimDueRetryMessages second call returned error: %v", err)
	}
	if len(claimedAgain) != 0 {
		t.Fatalf("expected already claimed messages not to be claimed again, got %+v", claimedAgain)
	}

	var claimedStatus string
	if err := database.QueryRowContext(ctx, `
		SELECT status
		FROM channel_messages
		WHERE id = 'channel_message_due_first'
	`).Scan(&claimedStatus); err != nil {
		t.Fatalf("query claimed retry status: %v", err)
	}
	if claimedStatus != string(MessageStatusSending) {
		t.Fatalf("expected claimed status to persist as sending, got %q", claimedStatus)
	}

	stillListed, err := store.ListDueRetryMessages(ctx, ClaimDueRetryMessagesInput{Now: now, Limit: 10})
	if err != nil {
		t.Fatalf("ListDueRetryMessages after claim returned error: %v", err)
	}
	if len(stillListed) != 0 {
		t.Fatalf("expected claimed messages to leave retry_pending list, got %+v", stillListed)
	}
}

func TestChannelSQLStoreListsAndClaimsDueRetryMessagesForSpecificChannel(t *testing.T) {
	store, _, ctx := testChannelSQLStore(t)

	primary, err := store.CreateConfig(ctx, &ChannelConfig{
		ID:             "channel_config_retry_primary",
		OrganizationID: "org_1",
		Type:           ChannelTypeWebhook,
		Name:           "Primary Retry Webhook",
		Config:         map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateConfig primary returned error: %v", err)
	}
	other, err := store.CreateConfig(ctx, &ChannelConfig{
		ID:             "channel_config_retry_other",
		OrganizationID: "org_1",
		Type:           ChannelTypeWebhook,
		Name:           "Other Retry Webhook",
		Config:         map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateConfig other returned error: %v", err)
	}

	now := time.Date(2026, 6, 4, 17, 0, 0, 0, time.UTC)
	dueAt := now.Add(-time.Minute)
	for _, log := range []ChannelMessageLog{
		{
			ID:                 "channel_message_primary_due",
			ChannelID:          primary.ID,
			Direction:          DirectionOutbound,
			TransformedMessage: InternalMessage{ID: "msg_primary_due", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "primary due"}}},
			TransformSuccess:   true,
			Status:             MessageStatusRetryPending,
			RetryCount:         1,
			NextRetryAt:        &dueAt,
			CreatedAt:          now.Add(-3 * time.Minute),
		},
		{
			ID:                 "channel_message_other_due",
			ChannelID:          other.ID,
			Direction:          DirectionOutbound,
			TransformedMessage: InternalMessage{ID: "msg_other_due", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "other due"}}},
			TransformSuccess:   true,
			Status:             MessageStatusRetryPending,
			RetryCount:         1,
			NextRetryAt:        &dueAt,
			CreatedAt:          now.Add(-2 * time.Minute),
		},
	} {
		if _, err := store.RecordMessageLog(ctx, &log); err != nil {
			t.Fatalf("RecordMessageLog(%s) returned error: %v", log.ID, err)
		}
	}

	listed, err := store.ListDueRetryMessages(ctx, ClaimDueRetryMessagesInput{ChannelID: primary.ID, Now: now, Limit: 10})
	if err != nil {
		t.Fatalf("ListDueRetryMessages returned error: %v", err)
	}
	assertChannelMessageIDs(t, listed, []string{"channel_message_primary_due"})

	claimed, err := store.ClaimDueRetryMessages(ctx, ClaimDueRetryMessagesInput{ChannelID: primary.ID, Now: now, Limit: 10})
	if err != nil {
		t.Fatalf("ClaimDueRetryMessages returned error: %v", err)
	}
	assertChannelMessageIDs(t, claimed, []string{"channel_message_primary_due"})

	stillDue, err := store.ListDueRetryMessages(ctx, ClaimDueRetryMessagesInput{ChannelID: other.ID, Now: now, Limit: 10})
	if err != nil {
		t.Fatalf("ListDueRetryMessages other returned error: %v", err)
	}
	assertChannelMessageIDs(t, stillDue, []string{"channel_message_other_due"})
	if stillDue[0].Status != MessageStatusRetryPending {
		t.Fatalf("expected other channel due message to remain unclaimed, got %+v", stillDue[0])
	}
}

func TestChannelSQLStoreArchivesExpiredMessageLogsWithoutDeletingRetryQueue(t *testing.T) {
	store, database, ctx := testChannelSQLStore(t)

	created, err := store.CreateConfig(ctx, &ChannelConfig{
		ID:             "channel_config_archive",
		OrganizationID: "org_1",
		Type:           ChannelTypeWebhook,
		Name:           "Archive Webhook",
		Config:         map[string]any{},
	})
	if err != nil {
		t.Fatalf("CreateConfig returned error: %v", err)
	}

	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-30 * 24 * time.Hour)
	for _, log := range []ChannelMessageLog{
		{
			ID:               "channel_message_old_recorded",
			ChannelID:        created.ID,
			Direction:        DirectionInbound,
			RawMessage:       json.RawMessage(`{"text":"old recorded"}`),
			TransformSuccess: true,
			Status:           MessageStatusRecorded,
			CreatedAt:        cutoff.Add(-time.Hour),
		},
		{
			ID:               "channel_message_old_permanent",
			ChannelID:        created.ID,
			Direction:        DirectionOutbound,
			RawMessage:       json.RawMessage(`{"text":"old permanent"}`),
			TransformSuccess: false,
			TransformError:   "permanent",
			Status:           MessageStatusPermanentFailure,
			CreatedAt:        cutoff.Add(-2 * time.Hour),
		},
		{
			ID:                 "channel_message_old_retry_pending",
			ChannelID:          created.ID,
			Direction:          DirectionOutbound,
			RawMessage:         json.RawMessage(`{"text":"old retry"}`),
			TransformedMessage: InternalMessage{ID: "msg_old_retry", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "old retry"}}},
			TransformSuccess:   true,
			Status:             MessageStatusRetryPending,
			RetryCount:         2,
			CreatedAt:          cutoff.Add(-3 * time.Hour),
		},
		{
			ID:               "channel_message_recent_recorded",
			ChannelID:        created.ID,
			Direction:        DirectionInbound,
			RawMessage:       json.RawMessage(`{"text":"recent"}`),
			TransformSuccess: true,
			Status:           MessageStatusRecorded,
			CreatedAt:        cutoff.Add(time.Hour),
		},
	} {
		if _, err := store.RecordMessageLog(ctx, &log); err != nil {
			t.Fatalf("RecordMessageLog(%s) returned error: %v", log.ID, err)
		}
	}

	result, err := store.ArchiveExpiredMessageLogs(ctx, ArchiveExpiredMessageLogsInput{Before: cutoff, Limit: 10})
	if err != nil {
		t.Fatalf("ArchiveExpiredMessageLogs returned error: %v", err)
	}
	assertStringSet(t, result.ArchivedIDs, []string{"channel_message_old_recorded", "channel_message_old_permanent"})
	if result.Count != 2 || !result.Before.Equal(cutoff) {
		t.Fatalf("unexpected archive result: %+v", result)
	}

	var remaining []string
	rows, err := database.QueryContext(ctx, `SELECT id FROM channel_messages ORDER BY id ASC`)
	if err != nil {
		t.Fatalf("query remaining channel messages: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan remaining id: %v", err)
		}
		remaining = append(remaining, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("scan remaining ids: %v", err)
	}
	assertStringSet(t, remaining, []string{"channel_message_old_retry_pending", "channel_message_recent_recorded"})
}

func assertChannelMessageIDs(t *testing.T, logs []*ChannelMessageLog, want []string) {
	t.Helper()
	if len(logs) != len(want) {
		t.Fatalf("expected %d message logs, got %d: %+v", len(want), len(logs), logs)
	}
	for i, expectedID := range want {
		if logs[i].ID != expectedID {
			t.Fatalf("expected message id at index %d to be %q, got %q; logs=%+v", i, expectedID, logs[i].ID, logs)
		}
	}
}

func assertStringSet(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d strings, got %d: got=%+v want=%+v", len(want), len(got), got, want)
	}
	seen := map[string]int{}
	for _, value := range got {
		seen[value]++
	}
	for _, expected := range want {
		if seen[expected] != 1 {
			t.Fatalf("expected string %q exactly once, got=%+v want=%+v", expected, got, want)
		}
	}
}
