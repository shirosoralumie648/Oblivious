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
