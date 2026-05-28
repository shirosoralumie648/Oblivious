package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"oblivious/server/internal/auth"
	relaytypes "oblivious/server/internal/relay/types"
)

const (
	defaultKnowledgeEmbeddingModel = "text-embedding-3-small"
	knowledgeRAGRetrievalMethod    = "embedding_rag"
	knowledgeRAGRetrievalLimit     = 5
	knowledgeRAGMinSimilarity      = 0.0
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
	DocumentID      string            `json:"documentId"`
	DocumentTitle   string            `json:"documentTitle"`
	ChunkID         string            `json:"chunkId"`
	ChunkIndex      int               `json:"chunkIndex"`
	RetrievalMethod string            `json:"retrievalMethod"`
	Similarity      float64           `json:"similarity"`
	Snippet         string            `json:"snippet"`
	Source          KnowledgeCitation `json:"source"`
}

type KnowledgeCitation struct {
	DocumentID    string `json:"documentId"`
	DocumentTitle string `json:"documentTitle"`
	ChunkID       string `json:"chunkId"`
	ChunkIndex    int    `json:"chunkIndex"`
}

type KnowledgeDocumentChunk struct {
	ID             string    `json:"id"`
	DocumentID     string    `json:"documentId"`
	OrganizationID string    `json:"organizationId"`
	ChunkIndex     int       `json:"chunkIndex"`
	Content        string    `json:"content"`
	Embedding      []float32 `json:"embedding,omitempty"`
	EmbeddingModel string    `json:"embeddingModel"`
	IndexedAt      time.Time `json:"indexedAt"`
}

type KnowledgeEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

type Store interface {
	CreateKnowledgeBase(ctx context.Context, workspaceID, organizationID, name string) (KnowledgeBase, error)
	CreateKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, title, content string, chunks []KnowledgeDocumentChunk) (KnowledgeDocument, error)
	DeleteKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID string) error
	DeleteKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, documentID string) error
	GetKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID string) (KnowledgeBase, error)
	ListKnowledgeDocuments(ctx context.Context, organizationID, knowledgeBaseID string) ([]KnowledgeDocument, error)
	ListKnowledgeBases(ctx context.Context, organizationID string) ([]KnowledgeBase, error)
	RetrieveKnowledge(ctx context.Context, organizationID, knowledgeBaseID string, queryEmbedding []float32, limit int, minScore float64) ([]KnowledgeRetrievalResult, error)
	UpdateKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID, name string) (KnowledgeBase, error)
	UpdateKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, documentID, title, content string, chunks []KnowledgeDocumentChunk) (KnowledgeDocument, error)
}

type Service struct {
	embeddingModel string
	embedder       KnowledgeEmbedder
	store          Store
}

func NewService(store Store) *Service {
	return NewServiceWithEmbedder(store, nil, defaultKnowledgeEmbeddingModel)
}

func NewServiceWithEmbedder(store Store, embedder KnowledgeEmbedder, embeddingModel string) *Service {
	if strings.TrimSpace(embeddingModel) == "" {
		embeddingModel = defaultKnowledgeEmbeddingModel
	}
	return &Service{
		embeddingModel: embeddingModel,
		embedder:       embedder,
		store:          store,
	}
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
	chunks, err := s.buildIndexedChunks(ctx, session, content)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	return s.store.CreateKnowledgeDocument(ctx, session.OrganizationID, knowledgeBaseID, title, content, chunks)
}

func (s *Service) Retrieve(ctx context.Context, session auth.Session, knowledgeBaseID, query string) ([]KnowledgeRetrievalResult, error) {
	normalizedQuery := normalizeKnowledgeQuery(query)
	if normalizedQuery == "" {
		return []KnowledgeRetrievalResult{}, nil
	}
	if s.embedder == nil {
		return nil, fmt.Errorf("knowledge embedding retrieval is not configured")
	}

	queryEmbedding, err := s.embedder.Embed(withKnowledgeRelayIdentity(ctx, session), normalizedQuery)
	if err != nil {
		return nil, fmt.Errorf("embed knowledge query: %w", err)
	}

	return s.store.RetrieveKnowledge(ctx, session.OrganizationID, knowledgeBaseID, queryEmbedding, knowledgeRAGRetrievalLimit, knowledgeRAGMinSimilarity)
}

func (s *Service) Update(ctx context.Context, session auth.Session, knowledgeBaseID, name string) (KnowledgeBase, error) {
	return s.store.UpdateKnowledgeBase(ctx, session.OrganizationID, knowledgeBaseID, name)
}

func (s *Service) Delete(ctx context.Context, session auth.Session, knowledgeBaseID string) error {
	return s.store.DeleteKnowledgeBase(ctx, session.OrganizationID, knowledgeBaseID)
}

func (s *Service) UpdateDocument(ctx context.Context, session auth.Session, knowledgeBaseID, documentID, title, content string) (KnowledgeDocument, error) {
	chunks, err := s.buildIndexedChunks(ctx, session, content)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	return s.store.UpdateKnowledgeDocument(ctx, session.OrganizationID, knowledgeBaseID, documentID, title, content, chunks)
}

func (s *Service) DeleteDocument(ctx context.Context, session auth.Session, knowledgeBaseID, documentID string) error {
	return s.store.DeleteKnowledgeDocument(ctx, session.OrganizationID, knowledgeBaseID, documentID)
}

func (s *Service) buildIndexedChunks(ctx context.Context, session auth.Session, content string) ([]KnowledgeDocumentChunk, error) {
	chunkContents := buildKnowledgeDocumentChunks(content)
	if len(chunkContents) == 0 {
		return nil, nil
	}
	if s.embedder == nil {
		return nil, fmt.Errorf("knowledge embedding indexing is not configured")
	}

	embeddings, err := s.embedder.EmbedBatch(withKnowledgeRelayIdentity(ctx, session), chunkContents)
	if err != nil {
		return nil, fmt.Errorf("embed knowledge chunks: %w", err)
	}

	now := time.Now().UTC()
	chunks := make([]KnowledgeDocumentChunk, len(chunkContents))
	for i, chunkContent := range chunkContents {
		var embedding []float32
		if i < len(embeddings) {
			embedding = append([]float32(nil), embeddings[i]...)
		}
		chunks[i] = KnowledgeDocumentChunk{
			OrganizationID: session.OrganizationID,
			ChunkIndex:     i,
			Content:        chunkContent,
			Embedding:      embedding,
			EmbeddingModel: s.embeddingModel,
			IndexedAt:      now,
		}
	}
	return chunks, nil
}

func withKnowledgeRelayIdentity(ctx context.Context, session auth.Session) context.Context {
	if session.User.ID != "" {
		ctx = relaytypes.WithTrustedUserID(ctx, session.User.ID)
	}
	if session.OrganizationID != "" {
		ctx = relaytypes.WithTrustedOrganizationID(ctx, session.OrganizationID)
	}
	return ctx
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
