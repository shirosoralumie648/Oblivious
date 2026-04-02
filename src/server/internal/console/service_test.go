package console

import (
	"context"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/userprefs"
)

type fakeStore struct {
	billing     BillingSummary
	models      []ModelSummary
	summary     UsageSummary
	workspaceID string
}

func (f *fakeStore) GetUsageSummary(ctx context.Context, workspaceID string) (UsageSummary, error) {
	f.workspaceID = workspaceID
	return f.summary, nil
}

func (f *fakeStore) GetModelSummaries(ctx context.Context, workspaceID string) ([]ModelSummary, error) {
	f.workspaceID = workspaceID
	return f.models, nil
}

func (f *fakeStore) GetBillingSummary(ctx context.Context, workspaceID string) (BillingSummary, error) {
	f.workspaceID = workspaceID
	return f.billing, nil
}

func TestGetUsageReturnsWorkspaceSummary(t *testing.T) {
	store := &fakeStore{
		summary: UsageSummary{
			Period:   "7d",
			Requests: 4,
		},
	}
	service := NewService(store)

	summary, err := service.GetUsage(context.Background(), auth.Session{WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("get usage: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace id workspace_1, got %s", store.workspaceID)
	}
	if summary.Period != "7d" {
		t.Fatalf("expected period 7d, got %s", summary.Period)
	}
	if summary.Requests != 4 {
		t.Fatalf("expected requests 4, got %d", summary.Requests)
	}
}

func TestGetModelsReturnsWorkspaceModelSummaries(t *testing.T) {
	store := &fakeStore{
		models: []ModelSummary{
			{ID: "balanced-chat", Label: "balanced-chat", Requests: 2},
			{ID: "quality-chat", Label: "quality-chat", Requests: 1},
		},
	}
	service := NewService(store)

	models, err := service.GetModels(context.Background(), auth.Session{WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("get models: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace id workspace_1, got %s", store.workspaceID)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 model summaries, got %d", len(models))
	}
	if models[0].ID != "balanced-chat" {
		t.Fatalf("expected first model balanced-chat, got %s", models[0].ID)
	}
}

func TestGetBillingReturnsWorkspaceSummary(t *testing.T) {
	store := &fakeStore{
		billing: BillingSummary{
			Period:           "30d",
			Requests:         5,
			InputTokens:      120,
			OutputTokens:     80,
			EstimatedCostUSD: 0.0004,
		},
	}
	service := NewService(store)

	summary, err := service.GetBilling(context.Background(), auth.Session{WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("get billing: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace id workspace_1, got %s", store.workspaceID)
	}
	if summary.Requests != 5 {
		t.Fatalf("expected requests 5, got %d", summary.Requests)
	}
	if summary.EstimatedCostUSD != 0.0004 {
		t.Fatalf("expected estimated cost 0.0004, got %f", summary.EstimatedCostUSD)
	}
}

func TestGetAccessReturnsSessionAndPreferenceSummary(t *testing.T) {
	service := NewService(&fakeStore{})

	summary := service.GetAccess(
		auth.Session{
			ExpiresAt:   mustTime(t, "2026-04-03T00:00:00Z"),
			ID:          "session_1",
			WorkspaceID: "workspace_1",
			User: auth.User{
				Email: "user@example.com",
				ID:    "user_1",
			},
		},
		userprefs.Preferences{
			DefaultMode:         "chat",
			ModelStrategy:       "balanced",
			NetworkEnabledHint:  true,
			OnboardingCompleted: true,
		},
	)

	if summary.UserEmail != "user@example.com" {
		t.Fatalf("expected user email user@example.com, got %s", summary.UserEmail)
	}
	if summary.WorkspaceID != "workspace_1" {
		t.Fatalf("expected workspace id workspace_1, got %s", summary.WorkspaceID)
	}
	if summary.DefaultMode != "chat" {
		t.Fatalf("expected default mode chat, got %s", summary.DefaultMode)
	}
	if summary.SessionExpiresAt != "2026-04-03T00:00:00Z" {
		t.Fatalf("expected session expiry 2026-04-03T00:00:00Z, got %s", summary.SessionExpiresAt)
	}
}

func mustTime(t *testing.T, raw string) (parsedTime time.Time) {
	t.Helper()

	parsedTime, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}

	return parsedTime
}
