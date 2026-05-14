package userprefs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

func (s *SQLStore) GetByUserID(ctx context.Context, userID string) (Preferences, error) {
	var prefs Preferences
	var notificationsJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT default_mode, model_strategy, network_enabled_hint, onboarding_completed,
		       COALESCE(default_agent_model, 'gpt-4o-mini'),
		       COALESCE(sidebar_collapsed, false),
		       COALESCE(notifications, '{}')
		FROM user_preferences WHERE user_id = $1
	`, userID).Scan(&prefs.DefaultMode, &prefs.ModelStrategy, &prefs.NetworkEnabledHint,
		&prefs.OnboardingCompleted, &prefs.DefaultAgentModel, &prefs.SidebarCollapsed, &notificationsJSON)

	if err == sql.ErrNoRows {
		return Preferences{
			DefaultMode:       "chat",
			ModelStrategy:     "balanced",
			DefaultAgentModel: "gpt-4o-mini",
			SidebarCollapsed:  false,
			Notifications:     map[string]any{},
		}, nil
	}
	if err != nil {
		return prefs, fmt.Errorf("get preferences: %w", err)
	}

	if len(notificationsJSON) > 0 {
		json.Unmarshal(notificationsJSON, &prefs.Notifications)
	}
	if prefs.Notifications == nil {
		prefs.Notifications = map[string]any{}
	}

	return prefs, nil
}

func (s *SQLStore) UpsertByUserID(ctx context.Context, userID string, preferences Preferences) (Preferences, error) {
	if preferences.Notifications == nil {
		preferences.Notifications = map[string]any{}
	}
	notificationsJSON, _ := json.Marshal(preferences.Notifications)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, default_mode, model_strategy, network_enabled_hint, onboarding_completed, default_agent_model, sidebar_collapsed, notifications)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			default_mode = EXCLUDED.default_mode,
			model_strategy = EXCLUDED.model_strategy,
			network_enabled_hint = EXCLUDED.network_enabled_hint,
			onboarding_completed = EXCLUDED.onboarding_completed,
			default_agent_model = EXCLUDED.default_agent_model,
			sidebar_collapsed = EXCLUDED.sidebar_collapsed,
			notifications = EXCLUDED.notifications
	`, userID, preferences.DefaultMode, preferences.ModelStrategy, preferences.NetworkEnabledHint,
		preferences.OnboardingCompleted, preferences.DefaultAgentModel, preferences.SidebarCollapsed, notificationsJSON)

	if err != nil {
		return preferences, fmt.Errorf("upsert preferences: %w", err)
	}

	return preferences, nil
}
