package knowledge

import (
	"context"
	"database/sql"
	"strings"
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

type KnowledgeRetrievalResult struct {
	DocumentID    string `json:"documentId"`
	DocumentTitle string `json:"documentTitle"`
	Snippet       string `json:"snippet"`
}

type Store interface {
	CreateKnowledgeBase(ctx context.Context, workspaceID, organizationID, name string) (KnowledgeBase, error)
	CreateKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, title, content string) (KnowledgeDocument, error)
	DeleteKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID string) error
	DeleteKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, documentID string) error
	GetKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID string) (KnowledgeBase, error)
	ListKnowledgeDocuments(ctx context.Context, organizationID, knowledgeBaseID string) ([]KnowledgeDocument, error)
	ListKnowledgeBases(ctx context.Context, organizationID string) ([]KnowledgeBase, error)
	RetrieveKnowledge(ctx context.Context, organizationID, knowledgeBaseID, query string) ([]KnowledgeRetrievalResult, error)
	UpdateKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID, name string) (KnowledgeBase, error)
	UpdateKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, documentID, title, content string) (KnowledgeDocument, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func normalizeKnowledgeQuery(query string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
}

func (s *Service) List(ctx context.Context, session auth.Session) ([]KnowledgeBase, error) {
	return s.store.ListKnowledgeBases(ctx, session.OrganizationID)
}

func (s *Service) Create(ctx context.Context, session auth.Session, name string) (KnowledgeBase, error) {
	return s.store.CreateKnowledgeBase(ctx, session.WorkspaceID, session.OrganizationID, name)
}

func (s *Service) Get(ctx context.Context, session auth.Session, knowledgeBaseID string) (KnowledgeBase, error) {
	return s.store.GetKnowledgeBase(ctx, session.OrganizationID, knowledgeBaseID)
}

func (s *Service) ListDocuments(ctx context.Context, session auth.Session, knowledgeBaseID string) ([]KnowledgeDocument, error) {
	return s.store.ListKnowledgeDocuments(ctx, session.OrganizationID, knowledgeBaseID)
}

func (s *Service) CreateDocument(ctx context.Context, session auth.Session, knowledgeBaseID, title, content string) (KnowledgeDocument, error) {
	return s.store.CreateKnowledgeDocument(ctx, session.OrganizationID, knowledgeBaseID, title, content)
}

func (s *Service) Retrieve(ctx context.Context, session auth.Session, knowledgeBaseID, query string) ([]KnowledgeRetrievalResult, error) {
	return s.store.RetrieveKnowledge(ctx, session.OrganizationID, knowledgeBaseID, normalizeKnowledgeQuery(query))
}

func (s *Service) Update(ctx context.Context, session auth.Session, knowledgeBaseID, name string) (KnowledgeBase, error) {
	return s.store.UpdateKnowledgeBase(ctx, session.OrganizationID, knowledgeBaseID, name)
}

func (s *Service) Delete(ctx context.Context, session auth.Session, knowledgeBaseID string) error {
	return s.store.DeleteKnowledgeBase(ctx, session.OrganizationID, knowledgeBaseID)
}

func (s *Service) UpdateDocument(ctx context.Context, session auth.Session, knowledgeBaseID, documentID, title, content string) (KnowledgeDocument, error) {
	return s.store.UpdateKnowledgeDocument(ctx, session.OrganizationID, knowledgeBaseID, documentID, title, content)
}

func (s *Service) DeleteDocument(ctx context.Context, session auth.Session, knowledgeBaseID, documentID string) error {
	return s.store.DeleteKnowledgeDocument(ctx, session.OrganizationID, knowledgeBaseID, documentID)
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
