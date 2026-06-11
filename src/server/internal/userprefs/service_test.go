package userprefs

import (
	"context"
	"testing"
)

type fakePreferenceStore struct {
	saved Preferences
}

func (s *fakePreferenceStore) GetByUserID(ctx context.Context, userID string) (Preferences, error) {
	return s.saved, nil
}

func (s *fakePreferenceStore) UpsertByUserID(ctx context.Context, userID string, preferences Preferences) (Preferences, error) {
	s.saved = preferences
	return preferences, nil
}

func TestServiceUpdateAppliesDefaultsForPartialPreferences(t *testing.T) {
	store := &fakePreferenceStore{}
	service := NewService(store)

	preferences, err := service.Update(context.Background(), "user_1", Preferences{})
	if err != nil {
		t.Fatalf("update preferences: %v", err)
	}

	if preferences.DefaultMode != "chat" {
		t.Fatalf("expected defaultMode chat, got %q", preferences.DefaultMode)
	}
	if preferences.ModelStrategy != "balanced" {
		t.Fatalf("expected modelStrategy balanced, got %q", preferences.ModelStrategy)
	}
	if preferences.DefaultAgentModel != "gpt-4o-mini" {
		t.Fatalf("expected defaultAgentModel gpt-4o-mini, got %q", preferences.DefaultAgentModel)
	}
	if preferences.SidebarCollapsed {
		t.Fatal("expected sidebarCollapsed default false")
	}
	if preferences.Notifications == nil || len(preferences.Notifications) != 0 {
		t.Fatalf("expected empty notification settings map, got %#v", preferences.Notifications)
	}
}

func TestServiceGetReturnsStoredPreferences(t *testing.T) {
	stored := Preferences{
		DefaultMode:         "planning",
		ModelStrategy:       "cost",
		NetworkEnabledHint:  true,
		OnboardingCompleted: true,
		DefaultAgentModel:   "claude-sonnet-4-6",
		SidebarCollapsed:    true,
		Notifications:       map[string]any{"email": true},
	}
	store := &fakePreferenceStore{saved: stored}
	service := NewService(store)

	result, err := service.Get(context.Background(), "user_1")
	if err != nil {
		t.Fatalf("get preferences: %v", err)
	}
	if result.DefaultMode != "planning" {
		t.Fatalf("expected defaultMode planning, got %q", result.DefaultMode)
	}
	if result.ModelStrategy != "cost" {
		t.Fatalf("expected modelStrategy cost, got %q", result.ModelStrategy)
	}
	if !result.OnboardingCompleted {
		t.Fatal("expected onboardingCompleted true")
	}
	if result.DefaultAgentModel != "claude-sonnet-4-6" {
		t.Fatalf("expected defaultAgentModel claude-sonnet-4-6, got %q", result.DefaultAgentModel)
	}
	if !result.SidebarCollapsed {
		t.Fatal("expected sidebarCollapsed true")
	}
}

func TestServiceUpdatePreservesExplicitValues(t *testing.T) {
	store := &fakePreferenceStore{}
	service := NewService(store)

	input := Preferences{
		DefaultMode:       "planning",
		ModelStrategy:     "quality",
		DefaultAgentModel: "gpt-4o",
		SidebarCollapsed:  true,
		Notifications:     map[string]any{"slack": true},
	}
	result, err := service.Update(context.Background(), "user_1", input)
	if err != nil {
		t.Fatalf("update preferences: %v", err)
	}
	if result.DefaultMode != "planning" {
		t.Fatalf("expected defaultMode planning, got %q", result.DefaultMode)
	}
	if result.ModelStrategy != "quality" {
		t.Fatalf("expected modelStrategy quality, got %q", result.ModelStrategy)
	}
	if result.DefaultAgentModel != "gpt-4o" {
		t.Fatalf("expected defaultAgentModel gpt-4o, got %q", result.DefaultAgentModel)
	}
	if !result.SidebarCollapsed {
		t.Fatal("expected sidebarCollapsed true")
	}
	if result.Notifications == nil || result.Notifications["slack"] != true {
		t.Fatalf("expected slack notification setting preserved, got %#v", result.Notifications)
	}
}
