package memory

import (
	"context"
	"testing"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/relay/types"
)

func TestServiceAddDocumentAddsRelayIdentityToEmbeddingContext(t *testing.T) {
	embedder := &identityRecordingEmbedder{
		batchEmbeddings: [][]float32{{0.1, 0.2}},
	}
	service := NewService(
		&fakeMemoryStore{},
		embedder,
		fixedChunker{chunks: []string{"commercial relay context"}},
		"text-embedding-3-small",
	)

	_, err := service.AddDocument(context.Background(), testMemorySession(), &AddDocumentRequest{
		Content: "commercial relay context",
	})
	if err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}

	if embedder.batchUserID != "user_1" || embedder.batchOrganizationID != "org_1" {
		t.Fatalf("expected relay identity user_1/org_1, got user=%q org=%q", embedder.batchUserID, embedder.batchOrganizationID)
	}
}

func TestServiceSearchAddsRelayIdentityToEmbeddingContext(t *testing.T) {
	embedder := &identityRecordingEmbedder{
		embedding: []float32{0.1, 0.2},
	}
	service := NewService(&fakeMemoryStore{}, embedder, fixedChunker{}, "text-embedding-3-small")

	_, err := service.Search(context.Background(), testMemorySession(), &SearchRequest{Query: "billing"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if embedder.userID != "user_1" || embedder.organizationID != "org_1" {
		t.Fatalf("expected relay identity user_1/org_1, got user=%q org=%q", embedder.userID, embedder.organizationID)
	}
}

func testMemorySession() auth.Session {
	return auth.Session{
		ID:             "session_1",
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		User: auth.User{
			ID:    "user_1",
			Email: "user@example.com",
		},
	}
}

type identityRecordingEmbedder struct {
	embedding           []float32
	batchEmbeddings     [][]float32
	userID              string
	organizationID      string
	batchUserID         string
	batchOrganizationID string
}

func (e *identityRecordingEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.userID, _ = types.TrustedUserIDFromContext(ctx)
	e.organizationID, _ = types.TrustedOrganizationIDFromContext(ctx)
	return append([]float32(nil), e.embedding...), nil
}

func (e *identityRecordingEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e.batchUserID, _ = types.TrustedUserIDFromContext(ctx)
	e.batchOrganizationID, _ = types.TrustedOrganizationIDFromContext(ctx)
	result := make([][]float32, len(e.batchEmbeddings))
	for i := range e.batchEmbeddings {
		result[i] = append([]float32(nil), e.batchEmbeddings[i]...)
	}
	return result, nil
}

type fixedChunker struct {
	chunks []string
}

func (c fixedChunker) Chunk(text string) []string {
	return append([]string(nil), c.chunks...)
}

type fakeMemoryStore struct {
	document *Document
}

func (s *fakeMemoryStore) CreateDocument(ctx context.Context, doc *Document) (*Document, error) {
	copyDoc := *doc
	s.document = &copyDoc
	return &copyDoc, nil
}

func (s *fakeMemoryStore) GetDocument(ctx context.Context, id, organizationID string) (*Document, error) {
	if s.document == nil {
		return &Document{ID: id, OrganizationID: organizationID, UserID: "user_1", Content: "old"}, nil
	}
	copyDoc := *s.document
	return &copyDoc, nil
}

func (s *fakeMemoryStore) ListDocuments(ctx context.Context, userID, organizationID string, limit, offset int) ([]*Document, error) {
	return nil, nil
}

func (s *fakeMemoryStore) UpdateDocument(ctx context.Context, id, organizationID string, title, content string) (*Document, error) {
	return &Document{ID: id, OrganizationID: organizationID, UserID: "user_1", Title: title, Content: content}, nil
}

func (s *fakeMemoryStore) DeleteDocument(ctx context.Context, id, organizationID string) error {
	return nil
}

func (s *fakeMemoryStore) CreateChunk(ctx context.Context, chunk *Chunk) (*Chunk, error) {
	return chunk, nil
}

func (s *fakeMemoryStore) CreateChunks(ctx context.Context, chunks []*Chunk) error {
	return nil
}

func (s *fakeMemoryStore) ListChunks(ctx context.Context, documentID, organizationID string) ([]*Chunk, error) {
	return nil, nil
}

func (s *fakeMemoryStore) DeleteChunks(ctx context.Context, documentID, organizationID string) error {
	return nil
}

func (s *fakeMemoryStore) SearchSimilar(ctx context.Context, userID, organizationID string, embedding []float32, topK int, minScore float64) ([]*SearchResult, error) {
	return []*SearchResult{{DocumentID: "doc_1", ChunkContent: "billing"}}, nil
}
