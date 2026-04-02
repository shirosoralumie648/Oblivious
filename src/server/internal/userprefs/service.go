package userprefs

import (
	"context"
	"database/sql"
)

type Preferences struct {
	DefaultMode         string `json:"defaultMode"`
	ModelStrategy       string `json:"modelStrategy"`
	NetworkEnabledHint  bool   `json:"networkEnabledHint"`
	OnboardingCompleted bool   `json:"onboardingCompleted"`
}

type Store interface {
	GetByUserID(ctx context.Context, userID string) (Preferences, error)
	UpsertByUserID(ctx context.Context, userID string, preferences Preferences) (Preferences, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Get(ctx context.Context, userID string) (Preferences, error) {
	return s.store.GetByUserID(ctx, userID)
}

func (s *Service) Update(ctx context.Context, userID string, preferences Preferences) (Preferences, error) {
	if preferences.DefaultMode == "" {
		preferences.DefaultMode = "chat"
	}
	if preferences.ModelStrategy == "" {
		preferences.ModelStrategy = "balanced"
	}

	return s.store.UpsertByUserID(ctx, userID, preferences)
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
