package console

import (
	"context"
	"database/sql"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/userprefs"
)

type UsageSummary struct {
	Period   string `json:"period"`
	Requests int    `json:"requests"`
}

type ModelSummary struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Requests int    `json:"requests"`
}

type BillingSummary struct {
	Period           string  `json:"period"`
	Requests         int     `json:"requests"`
	InputTokens      int     `json:"inputTokens"`
	OutputTokens     int     `json:"outputTokens"`
	EstimatedCostUSD float64 `json:"estimatedCostUsd"`
}

type AccessSummary struct {
	DefaultMode         string `json:"defaultMode"`
	ModelStrategy       string `json:"modelStrategy"`
	NetworkEnabledHint  bool   `json:"networkEnabledHint"`
	OnboardingCompleted bool   `json:"onboardingCompleted"`
	SessionExpiresAt    string `json:"sessionExpiresAt"`
	SessionID           string `json:"sessionId"`
	UserEmail           string `json:"userEmail"`
	UserID              string `json:"userId"`
	WorkspaceID         string `json:"workspaceId"`
}

type Store interface {
	GetBillingSummary(ctx context.Context, workspaceID string) (BillingSummary, error)
	GetModelSummaries(ctx context.Context, workspaceID string) ([]ModelSummary, error)
	GetUsageSummary(ctx context.Context, workspaceID string) (UsageSummary, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) GetUsage(ctx context.Context, session auth.Session) (UsageSummary, error) {
	return s.store.GetUsageSummary(ctx, session.WorkspaceID)
}

func (s *Service) GetModels(ctx context.Context, session auth.Session) ([]ModelSummary, error) {
	return s.store.GetModelSummaries(ctx, session.WorkspaceID)
}

func (s *Service) GetBilling(ctx context.Context, session auth.Session) (BillingSummary, error) {
	return s.store.GetBillingSummary(ctx, session.WorkspaceID)
}

func (s *Service) GetAccess(session auth.Session, preferences userprefs.Preferences) AccessSummary {
	return AccessSummary{
		DefaultMode:         preferences.DefaultMode,
		ModelStrategy:       preferences.ModelStrategy,
		NetworkEnabledHint:  preferences.NetworkEnabledHint,
		OnboardingCompleted: preferences.OnboardingCompleted,
		SessionExpiresAt:    session.ExpiresAt.UTC().Format(time.RFC3339),
		SessionID:           session.ID,
		UserEmail:           session.User.Email,
		UserID:              session.User.ID,
		WorkspaceID:         session.WorkspaceID,
	}
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
