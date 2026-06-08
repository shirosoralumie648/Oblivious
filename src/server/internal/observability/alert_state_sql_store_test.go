package observability

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func testSQLAlertStateStore(t *testing.T) (*SQLAlertStateStore, context.Context) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if strings.EqualFold(os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE"), "true") {
			t.Fatal("TEST_DATABASE_URL is required for SQL alert state store tests")
		}
		t.Skip("TEST_DATABASE_URL is required for SQL alert state store tests")
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

	if _, err := database.Exec(`SELECT pg_advisory_lock(104249)`); err != nil {
		t.Fatalf("lock observability alert state test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104249)`); err != nil {
			t.Fatalf("unlock observability alert state test database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS observability_alert_routing_rules CASCADE`,
		`DROP TABLE IF EXISTS observability_recovery_actions CASCADE`,
		`DROP TABLE IF EXISTS observability_alert_delivery_attempts CASCADE`,
		`DROP TABLE IF EXISTS observability_notification_states CASCADE`,
		`DROP TABLE IF EXISTS observability_alert_occurrences CASCADE`,
		`DROP TABLE IF EXISTS observability_alert_states CASCADE`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare observability alert state database: %v\nstatement: %s", err, statement)
		}
	}

	migration, err := os.ReadFile("../../migrations/0049_observability_alert_state.sql")
	if err != nil {
		t.Fatalf("read observability alert state migration: %v", err)
	}
	if _, err := database.Exec(string(migration)); err != nil {
		t.Fatalf("apply observability alert state migration: %v", err)
	}

	return NewSQLAlertStateStore(database), context.Background()
}

func TestSQLAlertRoutingRuleStorePersistsRoutingRules(t *testing.T) {
	stateStore, ctx := testSQLAlertStateStore(t)
	store := NewSQLAlertRoutingRuleStore(stateStore.db)

	defaults, err := store.GetRoutingRules(ctx)
	if err != nil {
		t.Fatalf("get default routing rules: %v", err)
	}
	if !sameObservabilityRoutingRules(defaults, DefaultAlertRoutingRules()) {
		t.Fatalf("expected default routing rules, got %+v", defaults)
	}

	updated, err := store.UpdateRoutingRules(ctx, AlertRoutingRules{
		AlertSeverityDebug:    {},
		AlertSeverityInfo:     {AlertDeliveryChannelInApp},
		AlertSeverityWarning:  {AlertDeliveryChannelIM, AlertDeliveryChannelIM, AlertDeliveryChannelEmail},
		AlertSeverityCritical: {AlertDeliveryChannelSMS, AlertDeliveryChannelPhone},
	})
	if err != nil {
		t.Fatalf("update routing rules: %v", err)
	}
	expected := AlertRoutingRules{
		AlertSeverityDebug:    {},
		AlertSeverityInfo:     {AlertDeliveryChannelInApp},
		AlertSeverityWarning:  {AlertDeliveryChannelIM, AlertDeliveryChannelEmail},
		AlertSeverityCritical: {AlertDeliveryChannelSMS, AlertDeliveryChannelPhone},
	}
	if !sameObservabilityRoutingRules(updated, expected) {
		t.Fatalf("expected normalized routing rules %+v, got %+v", expected, updated)
	}

	reloaded := NewSQLAlertRoutingRuleStore(stateStore.db)
	persisted, err := reloaded.GetRoutingRules(ctx)
	if err != nil {
		t.Fatalf("reload routing rules: %v", err)
	}
	if !sameObservabilityRoutingRules(persisted, expected) {
		t.Fatalf("expected persisted routing rules %+v, got %+v", expected, persisted)
	}
}

func TestSQLAlertStateStorePersistsAlertLifecycleAndEscalation(t *testing.T) {
	store, ctx := testSQLAlertStateStore(t)
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		state, err := store.RecordAlertOpen(ctx, AlertEvent{
			Key:        "queue-backlog",
			Severity:   AlertSeverityWarning,
			Title:      "Queue backlog",
			Component:  "relay",
			OccurredAt: now.Add(time.Duration(i) * 2 * time.Minute),
		})
		if err != nil {
			t.Fatalf("record alert open %d: %v", i+1, err)
		}
		if i == 2 && (state.Severity != AlertSeverityCritical || !state.Escalated || state.OriginalSeverity != AlertSeverityWarning) {
			t.Fatalf("expected third warning to escalate to critical, got %+v", state)
		}
	}

	ackedAt := now.Add(7 * time.Minute)
	acked, err := store.AcknowledgeAlert(ctx, "queue-backlog", ackedAt)
	if err != nil {
		t.Fatalf("acknowledge alert: %v", err)
	}
	if acked.Status != AlertStatusAcknowledged || !acked.AcknowledgedAt.Equal(ackedAt) {
		t.Fatalf("expected acknowledged state, got %+v", acked)
	}

	resolvedAt := now.Add(12 * time.Minute)
	resolved, err := store.ResolveAlert(ctx, "queue-backlog", resolvedAt)
	if err != nil {
		t.Fatalf("resolve alert: %v", err)
	}
	if resolved.Status != AlertStatusResolved || !resolved.ResolvedAt.Equal(resolvedAt) {
		t.Fatalf("expected resolved state, got %+v", resolved)
	}

	got, ok, err := store.GetAlertState(ctx, "queue-backlog")
	if err != nil {
		t.Fatalf("get alert state: %v", err)
	}
	if !ok || got.OccurrenceCount != 3 || got.Component != "relay" || got.Title != "Queue backlog" {
		t.Fatalf("expected persisted alert state to round trip, ok=%v state=%+v", ok, got)
	}
}

func TestSQLAlertStateStoreListsAlertStatesWithFilters(t *testing.T) {
	store, ctx := testSQLAlertStateStore(t)
	base := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)

	for _, event := range []AlertEvent{
		{
			Key:        "relay-latency",
			Severity:   AlertSeverityWarning,
			Title:      "Relay latency",
			Component:  "relay",
			OccurredAt: base.Add(2 * time.Minute),
		},
		{
			Key:        "workflow-errors",
			Severity:   AlertSeverityCritical,
			Title:      "Workflow errors",
			Component:  "workflow",
			OccurredAt: base.Add(5 * time.Minute),
		},
		{
			Key:        "relay-backlog",
			Severity:   AlertSeverityCritical,
			Title:      "Relay backlog",
			Component:  "relay",
			OccurredAt: base.Add(5 * time.Minute),
		},
	} {
		if _, err := store.RecordAlertOpen(ctx, event); err != nil {
			t.Fatalf("record %s: %v", event.Key, err)
		}
	}
	if _, err := store.ResolveAlert(ctx, "relay-latency", base.Add(7*time.Minute)); err != nil {
		t.Fatalf("resolve relay latency: %v", err)
	}

	states, err := store.ListAlertStates(ctx, AlertStateFilter{
		Status:    AlertStatusOpen,
		Component: "relay",
	})
	if err != nil {
		t.Fatalf("list filtered alert states: %v", err)
	}
	if len(states) != 1 || states[0].Key != "relay-backlog" {
		t.Fatalf("expected only open relay backlog alert, got %+v", states)
	}

	states, err = store.ListAlertStates(ctx, AlertStateFilter{})
	if err != nil {
		t.Fatalf("list all alert states: %v", err)
	}
	gotKeys := make([]string, 0, len(states))
	for _, state := range states {
		gotKeys = append(gotKeys, state.Key)
	}
	wantKeys := []string{"relay-backlog", "workflow-errors", "relay-latency"}
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("expected keys %v, got %v", wantKeys, gotKeys)
	}
	for i := range wantKeys {
		if gotKeys[i] != wantKeys[i] {
			t.Fatalf("expected stable list order %v, got %v", wantKeys, gotKeys)
		}
	}
}

func TestSQLAlertStateStorePersistsNotificationThrottleAndRecoveryCooldown(t *testing.T) {
	store, ctx := testSQLAlertStateStore(t)
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)

	firstNotify, err := store.RecordNotification(ctx, AlertEvent{
		Key:        "queue-backlog",
		Severity:   AlertSeverityWarning,
		OccurredAt: now,
	}, 15*time.Minute)
	if err != nil {
		t.Fatalf("record first notification: %v", err)
	}
	if !firstNotify {
		t.Fatal("expected first notification to be allowed")
	}

	secondNotify, err := store.RecordNotification(ctx, AlertEvent{
		Key:        "queue-backlog",
		Severity:   AlertSeverityWarning,
		OccurredAt: now.Add(5 * time.Minute),
	}, 15*time.Minute)
	if err != nil {
		t.Fatalf("record second notification: %v", err)
	}
	if secondNotify {
		t.Fatal("expected notification inside throttle window to be suppressed")
	}

	action := RecoveryAction{
		ID:         "restart-unhealthy-service:queue-backlog:1",
		PolicyName: "restart-unhealthy-service",
		AlertKey:   "queue-backlog",
		Severity:   AlertSeverityCritical,
		Component:  "relay",
		Type:       RecoveryActionRestart,
		Status:     RecoveryActionRecorded,
		Reason:     "Queue backlog",
		CreatedAt:  now,
	}
	firstAction, stored, err := store.RecordRecoveryAction(ctx, action, 5*time.Minute)
	if err != nil {
		t.Fatalf("record first recovery action: %v", err)
	}
	if !firstAction || stored.ID != action.ID {
		t.Fatalf("expected first recovery action to be stored, created=%v action=%+v", firstAction, stored)
	}

	action.ID = "restart-unhealthy-service:queue-backlog:2"
	action.CreatedAt = now.Add(2 * time.Minute)
	secondAction, stored, err := store.RecordRecoveryAction(ctx, action, 5*time.Minute)
	if err != nil {
		t.Fatalf("record second recovery action: %v", err)
	}
	if secondAction || stored.ID != "restart-unhealthy-service:queue-backlog:1" {
		t.Fatalf("expected cooldown to return previous action, created=%v action=%+v", secondAction, stored)
	}
}

func TestSQLAlertStateStoreRecordsRepeatedDeliveryBatchesForSameAlert(t *testing.T) {
	store, ctx := testSQLAlertStateStore(t)
	event := AlertEvent{
		Key:        "queue-backlog",
		Severity:   AlertSeverityWarning,
		Component:  "relay",
		OccurredAt: time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC),
	}
	results := []AlertDeliveryResult{
		{Channel: AlertDeliveryChannelEmail, Delivered: true},
		{Channel: AlertDeliveryChannelInApp, Delivered: true},
	}

	if err := store.RecordDeliveryAttempts(ctx, event, results); err != nil {
		t.Fatalf("record first delivery batch: %v", err)
	}
	event.OccurredAt = event.OccurredAt.Add(5 * time.Minute)
	if err := store.RecordDeliveryAttempts(ctx, event, results); err != nil {
		t.Fatalf("record second delivery batch: %v", err)
	}

	attempts, err := store.ListDeliveryAttempts(ctx, AlertDeliveryHistoryFilter{AlertKey: "queue-backlog"})
	if err != nil {
		t.Fatalf("list delivery attempts: %v", err)
	}
	if len(attempts) != 4 {
		t.Fatalf("expected both delivery batches to be stored, got %+v", attempts)
	}
	seenIDs := make(map[string]bool, len(attempts))
	for _, attempt := range attempts {
		if seenIDs[attempt.ID] {
			t.Fatalf("expected unique delivery attempt IDs, got duplicate %q in %+v", attempt.ID, attempts)
		}
		seenIDs[attempt.ID] = true
	}
}

func sameObservabilityRoutingRules(a, b AlertRoutingRules) bool {
	if len(a) != len(b) {
		return false
	}
	for severity, aChannels := range a {
		bChannels, ok := b[severity]
		if !ok || len(aChannels) != len(bChannels) {
			return false
		}
		for index := range aChannels {
			if aChannels[index] != bChannels[index] {
				return false
			}
		}
	}
	return true
}
