package observability

import (
	"context"
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
