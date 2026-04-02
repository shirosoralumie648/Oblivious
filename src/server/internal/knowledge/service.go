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

type KnowledgeDocument struct {
	Content   string    `json:"content"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Store interface {
	CreateKnowledgeBase(ctx context.Context, workspaceID, name string) (KnowledgeBase, error)
	CreateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, title, content string) (KnowledgeDocument, error)
	GetKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) (KnowledgeBase, error)
	ListKnowledgeDocuments(ctx context.Context, workspaceID, knowledgeBaseID string) ([]KnowledgeDocument, error)
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

func (s *Service) Get(ctx context.Context, session auth.Session, knowledgeBaseID string) (KnowledgeBase, error) {
	return s.store.GetKnowledgeBase(ctx, session.WorkspaceID, knowledgeBaseID)
}

func (s *Service) ListDocuments(ctx context.Context, session auth.Session, knowledgeBaseID string) ([]KnowledgeDocument, error) {
	return s.store.ListKnowledgeDocuments(ctx, session.WorkspaceID, knowledgeBaseID)
}

func (s *Service) CreateDocument(ctx context.Context, session auth.Session, knowledgeBaseID, title, content string) (KnowledgeDocument, error) {
	return s.store.CreateKnowledgeDocument(ctx, session.WorkspaceID, knowledgeBaseID, title, content)
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
