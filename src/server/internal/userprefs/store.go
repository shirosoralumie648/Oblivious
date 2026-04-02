package userprefs

import (
	"context"
	"database/sql"
	"errors"
)

func (s *SQLStore) GetByUserID(ctx context.Context, userID string) (Preferences, error) {
	preferences := Preferences{
		DefaultMode:         "chat",
		ModelStrategy:       "balanced",
		NetworkEnabledHint:  false,
		OnboardingCompleted: false,
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT onboarding_completed, default_mode, model_strategy, network_enabled_hint
		FROM user_preferences
		WHERE user_id = $1
	`, userID).Scan(
		&preferences.OnboardingCompleted,
		&preferences.DefaultMode,
		&preferences.ModelStrategy,
		&preferences.NetworkEnabledHint,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return preferences, nil
		}
		return Preferences{}, err
	}

	return preferences, nil
}

func (s *SQLStore) UpsertByUserID(ctx context.Context, userID string, preferences Preferences) (Preferences, error) {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, onboarding_completed, default_mode, model_strategy, network_enabled_hint)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			onboarding_completed = EXCLUDED.onboarding_completed,
			default_mode = EXCLUDED.default_mode,
			model_strategy = EXCLUDED.model_strategy,
			network_enabled_hint = EXCLUDED.network_enabled_hint,
			updated_at = NOW()
	`, userID, preferences.OnboardingCompleted, preferences.DefaultMode, preferences.ModelStrategy, preferences.NetworkEnabledHint); err != nil {
		return Preferences{}, err
	}

	return s.GetByUserID(ctx, userID)
}
