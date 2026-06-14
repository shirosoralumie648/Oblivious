package observability

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRecoveryControllerRecordsCriticalPolicyActionWithCooldown(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	controller := NewRecoveryController(RecoveryControllerOptions{
		StateStore: store,
		Now:        func() time.Time { return now },
		Policies: []RecoveryPolicy{
			{
				Name:       "restart-unhealthy-service",
				Severity:   AlertSeverityCritical,
				Component:  "relay",
				ActionType: RecoveryActionRestart,
				Cooldown:   5 * time.Minute,
			},
		},
	})

	first, err := controller.HandleAlert(ctx, AlertEvent{
		Key:       "relay-down",
		Severity:  AlertSeverityCritical,
		Component: "relay",
		Title:     "Relay health check failed",
	})
	if err != nil {
		t.Fatalf("handle first alert: %v", err)
	}
	if !first.Created {
		t.Fatalf("expected first critical alert to create a recovery action, got %+v", first)
	}
	if first.Action.Type != RecoveryActionRestart || first.Action.Status != RecoveryActionRecorded {
		t.Fatalf("expected recorded restart action, got %+v", first.Action)
	}

	second, err := controller.HandleAlert(ctx, AlertEvent{
		Key:       "relay-down",
		Severity:  AlertSeverityCritical,
		Component: "relay",
		Title:     "Relay health check failed",
	})
	if err != nil {
		t.Fatalf("handle second alert: %v", err)
	}
	if second.Created {
		t.Fatalf("expected cooldown to suppress duplicate recovery action, got %+v", second)
	}

	actions := store.RecoveryActions()
	if len(actions) != 1 {
		t.Fatalf("expected one stored recovery action inside cooldown, got %d", len(actions))
	}
}

func TestRecoveryControllerSchedulesRestartBackoffAndExhaustsAfterFiveAttempts(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	controller := NewRecoveryController(RecoveryControllerOptions{
		StateStore: store,
		Policies: []RecoveryPolicy{
			{
				Name:       "restart-unhealthy-service",
				Severity:   AlertSeverityCritical,
				Component:  "server",
				ActionType: RecoveryActionRestart,
			},
		},
	})
	startedAt := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	wantBackoff := []time.Duration{10 * time.Second, 30 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute}

	for attempt := 1; attempt <= 5; attempt++ {
		occurredAt := startedAt.Add(time.Duration(attempt-1) * time.Minute)
		decision, err := controller.HandleAlert(ctx, AlertEvent{
			Key:        "server-unhealthy",
			Severity:   AlertSeverityCritical,
			Component:  "server",
			Title:      "Health check failed",
			OccurredAt: occurredAt,
		})
		if err != nil {
			t.Fatalf("handle restart attempt %d: %v", attempt, err)
		}
		if !decision.Created || decision.Action.Status != RecoveryActionRecorded || decision.Action.Attempt != attempt {
			t.Fatalf("expected recorded restart attempt %d, got %+v", attempt, decision)
		}
		if !decision.Action.NextAttemptAt.Equal(occurredAt.Add(wantBackoff[attempt-1])) {
			t.Fatalf("expected attempt %d next retry at %s, got %+v", attempt, occurredAt.Add(wantBackoff[attempt-1]), decision.Action)
		}
	}

	exhausted, err := controller.HandleAlert(ctx, AlertEvent{
		Key:        "server-unhealthy",
		Severity:   AlertSeverityCritical,
		Component:  "server",
		Title:      "Health check failed",
		OccurredAt: startedAt.Add(6 * time.Minute),
	})
	if err != nil {
		t.Fatalf("handle exhausted restart attempt: %v", err)
	}
	if !exhausted.Created || exhausted.Action.Status != RecoveryActionExhausted || exhausted.Action.Attempt != 6 {
		t.Fatalf("expected sixth restart attempt to be exhausted, got %+v", exhausted)
	}
	if !exhausted.Action.NextAttemptAt.IsZero() || !strings.Contains(exhausted.Action.Reason, "manual intervention") {
		t.Fatalf("expected exhausted restart to stop retries and require manual intervention, got %+v", exhausted.Action)
	}
}

func TestRecoveryControllerMatchesPanicAndOOMRecoverySignals(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	controller := NewRecoveryController(RecoveryControllerOptions{
		StateStore: store,
		Policies: []RecoveryPolicy{
			{
				Name:         "record-http-panic",
				Severity:     AlertSeverityCritical,
				Component:    ComponentHTTP,
				FieldMatches: map[string]string{"failure_kind": "panic"},
				ActionType:   RecoveryActionRestart,
			},
			{
				Name:         "record-runtime-oom",
				Severity:     AlertSeverityCritical,
				Component:    ComponentHTTP,
				FieldMatches: map[string]string{"failure_kind": "oom"},
				ActionType:   RecoveryActionRestart,
			},
		},
	})

	panicDecision, err := controller.HandleAlert(ctx, AlertEvent{
		Key:       "http:/api/v1/agents:panic",
		Severity:  AlertSeverityCritical,
		Component: ComponentHTTP,
		Title:     "HTTP panic recovered",
		Fields: map[string]any{
			"failure_kind": "panic",
		},
	})
	if err != nil {
		t.Fatalf("handle panic recovery signal: %v", err)
	}
	if !panicDecision.Created || panicDecision.Action.PolicyName != "record-http-panic" || panicDecision.Action.Type != RecoveryActionRestart {
		t.Fatalf("expected panic signal to create restart recovery action, got %+v", panicDecision)
	}

	oomDecision, err := controller.HandleAlert(ctx, AlertEvent{
		Key:       "runtime:server:oom",
		Severity:  AlertSeverityCritical,
		Component: ComponentHTTP,
		Title:     "Container OOM kill observed",
		Fields: map[string]any{
			"failure_kind": "oom",
		},
	})
	if err != nil {
		t.Fatalf("handle OOM recovery signal: %v", err)
	}
	if !oomDecision.Created || oomDecision.Action.PolicyName != "record-runtime-oom" || oomDecision.Action.Type != RecoveryActionRestart {
		t.Fatalf("expected OOM signal to create restart recovery action, got %+v", oomDecision)
	}

	ignored, err := controller.HandleAlert(ctx, AlertEvent{
		Key:       "http:/api/v1/agents:critical",
		Severity:  AlertSeverityCritical,
		Component: ComponentHTTP,
		Title:     "Critical HTTP without runtime signal",
		Fields: map[string]any{
			"failure_kind": "http_5xx",
		},
	})
	if err != nil {
		t.Fatalf("handle non-matching recovery signal: %v", err)
	}
	if ignored.Created {
		t.Fatalf("expected non-matching failure_kind to be ignored by signal-specific policies, got %+v", ignored)
	}
}

func TestAlertStateStoreListsRecoveryActionsWithFilters(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	base := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)

	for _, action := range []RecoveryAction{
		{
			ID:         "restart-relay:relay-backlog:1",
			PolicyName: "restart-relay",
			AlertKey:   "relay-backlog",
			Severity:   AlertSeverityCritical,
			Component:  "relay",
			Type:       RecoveryActionRestart,
			Status:     RecoveryActionRecorded,
			Reason:     "Relay backlog",
			CreatedAt:  base,
		},
		{
			ID:         "scale-workflow:workflow-failures:1",
			PolicyName: "scale-workflow",
			AlertKey:   "workflow-failures",
			Severity:   AlertSeverityWarning,
			Component:  "workflow",
			Type:       RecoveryActionScaleOut,
			Status:     RecoveryActionRecorded,
			Reason:     "Workflow failures",
			CreatedAt:  base.Add(2 * time.Minute),
		},
	} {
		if _, _, err := store.RecordRecoveryAction(ctx, action, 0); err != nil {
			t.Fatalf("record recovery action %s: %v", action.ID, err)
		}
	}

	relayActions, err := store.ListRecoveryActions(ctx, RecoveryActionFilter{AlertKey: "relay-backlog"})
	if err != nil {
		t.Fatalf("list relay recovery actions: %v", err)
	}
	if len(relayActions) != 1 || relayActions[0].PolicyName != "restart-relay" || relayActions[0].Type != RecoveryActionRestart {
		t.Fatalf("expected relay restart action, got %+v", relayActions)
	}

	allActions, err := store.ListRecoveryActions(ctx, RecoveryActionFilter{})
	if err != nil {
		t.Fatalf("list all recovery actions: %v", err)
	}
	if len(allActions) != 2 || allActions[0].ID != "scale-workflow:workflow-failures:1" || allActions[1].ID != "restart-relay:relay-backlog:1" {
		t.Fatalf("expected recovery actions ordered by created_at desc, got %+v", allActions)
	}
}
