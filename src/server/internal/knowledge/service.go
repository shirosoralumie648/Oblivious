package knowledge

import (
	"context"
	"database/sql"
	"time"

	"oblivious/server/internal/auth"
)

type KnowledgeBase struct {
	DocumentCount int       `json:"documentCount"`
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Store interface {
	CreateKnowledgeBase(ctx context.Context, workspaceID, name string) (KnowledgeBase, error)
	ListKnowledgeBases(ctx context.Context, workspaceID string) ([]KnowledgeBase, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(ctx context.Context, session auth.Session) ([]KnowledgeBase, error) {
	return s.store.ListKnowledgeBases(ctx, session.WorkspaceID)
}

func (s *Service) Create(ctx context.Context, session auth.Session, name string) (KnowledgeBase, error) {
	return s.store.CreateKnowledgeBase(ctx, session.WorkspaceID, name)
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
