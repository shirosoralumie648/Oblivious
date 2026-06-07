package observability

import (
	"context"
	"testing"
	"time"
)

func TestAlertRouterReusesStateStoreForThrottleAndEscalation(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)

	firstNotifications := &captureAlertSink{}
	firstRouter := NewAlertRouter(AlertRouterOptions{
		NotifySink: firstNotifications,
		StateStore: store,
	})
	if err := firstRouter.Route(ctx, AlertEvent{
		Key:        "queue-backlog",
		Severity:   AlertSeverityWarning,
		Title:      "Queue backlog",
		OccurredAt: now,
	}); err != nil {
		t.Fatalf("route first alert: %v", err)
	}

	secondNotifications := &captureAlertSink{}
	secondRouter := NewAlertRouter(AlertRouterOptions{
		NotifySink: secondNotifications,
		StateStore: store,
	})
	if err := secondRouter.Route(ctx, AlertEvent{
		Key:        "queue-backlog",
		Severity:   AlertSeverityWarning,
		Title:      "Queue backlog",
		OccurredAt: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("route second alert: %v", err)
	}
	if len(secondNotifications.events) != 0 {
		t.Fatalf("expected shared state store to throttle second warning notification, got %d", len(secondNotifications.events))
	}

	if err := secondRouter.Route(ctx, AlertEvent{
		Key:        "queue-backlog",
		Severity:   AlertSeverityWarning,
		Title:      "Queue backlog",
		OccurredAt: now.Add(4 * time.Minute),
	}); err != nil {
		t.Fatalf("route third alert: %v", err)
	}

	state, ok, err := store.GetAlertState(ctx, "queue-backlog")
	if err != nil {
		t.Fatalf("get alert state: %v", err)
	}
	if !ok {
		t.Fatal("expected opened alert state to be stored")
	}
	if state.Status != AlertStatusOpen {
		t.Fatalf("expected alert to stay open, got %s", state.Status)
	}
	if state.Severity != AlertSeverityCritical || !state.Escalated {
		t.Fatalf("expected repeated warning to persist escalated critical state, got %+v", state)
	}
	if state.OccurrenceCount != 3 {
		t.Fatalf("expected three occurrences to survive router recreation, got %d", state.OccurrenceCount)
	}
}

func TestAlertStateStoreEscalatesWarningOpenForThirtyMinutes(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	openedAt := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)

	first, err := store.RecordAlertOpen(ctx, AlertEvent{
		Key:        "relay-latency",
		Severity:   AlertSeverityWarning,
		Title:      "Relay latency",
		Component:  "relay",
		OccurredAt: openedAt,
	})
	if err != nil {
		t.Fatalf("record first warning: %v", err)
	}
	if first.Severity != AlertSeverityWarning || first.Escalated {
		t.Fatalf("expected first warning to stay warning, got %+v", first)
	}

	escalated, err := store.RecordAlertOpen(ctx, AlertEvent{
		Key:        "relay-latency",
		Severity:   AlertSeverityWarning,
		Title:      "Relay latency",
		Component:  "relay",
		OccurredAt: openedAt.Add(31 * time.Minute),
	})
	if err != nil {
		t.Fatalf("record sustained warning: %v", err)
	}
	if escalated.Severity != AlertSeverityCritical || !escalated.Escalated || escalated.OriginalSeverity != AlertSeverityWarning {
		t.Fatalf("expected warning open for 30 minutes to escalate critical, got %+v", escalated)
	}
}

func TestAlertStateStoreTracksAcknowledgeAndResolve(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	openedAt := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)

	if _, err := store.RecordAlertOpen(ctx, AlertEvent{
		Key:        "database-down",
		Severity:   AlertSeverityCritical,
		Title:      "Database down",
		OccurredAt: openedAt,
	}); err != nil {
		t.Fatalf("record open: %v", err)
	}
	acked, err := store.AcknowledgeAlert(ctx, "database-down", openedAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("acknowledge alert: %v", err)
	}
	if acked.Status != AlertStatusAcknowledged || acked.AcknowledgedAt.IsZero() {
		t.Fatalf("expected acknowledged state with timestamp, got %+v", acked)
	}

	resolved, err := store.ResolveAlert(ctx, "database-down", openedAt.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("resolve alert: %v", err)
	}
	if resolved.Status != AlertStatusResolved || resolved.ResolvedAt.IsZero() {
		t.Fatalf("expected resolved state with timestamp, got %+v", resolved)
	}

	state, ok, err := store.GetAlertState(ctx, "database-down")
	if err != nil {
		t.Fatalf("get alert state: %v", err)
	}
	if !ok || state.Status != AlertStatusResolved {
		t.Fatalf("expected resolved state to persist, ok=%v state=%+v", ok, state)
	}
}

func TestAlertStateStoreListsAlertStatesWithStableFilters(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	base := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)

	events := []AlertEvent{
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
	}
	for _, event := range events {
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
		t.Fatalf("list alert states: %v", err)
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
