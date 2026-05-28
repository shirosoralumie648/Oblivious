package knowledge

import (
	"context"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
)

type fakeStore struct {
	createdName       string
	createdBase       KnowledgeBase
	createdDoc        KnowledgeDocument
	deletedDocID      string
	deletedID         string
	detailBase        KnowledgeBase
	documents         []KnowledgeDocument
	listBases         []KnowledgeBase
	persistedChunks   []KnowledgeDocumentChunk
	queryEmbedding    []float32
	retrievalLimit    int
	retrievalQuery    string
	retrievalResults  []KnowledgeRetrievalResult
	requestedDoc      KnowledgeDocument
	requestedID       string
	retrievalMinScore float64
	organizationID    string
	updatedBase       KnowledgeBase
	updatedDoc        KnowledgeDocument
}

type fakeEmbedder struct {
	batchInputs [][]string
	batches     [][][]float32
	embedInputs []string
	embeddings  [][]float32
}

func (e *fakeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.embedInputs = append(e.embedInputs, text)
	if len(e.embeddings) == 0 {
		return []float32{0.1, 0.2, 0.3}, nil
	}
	embedding := e.embeddings[0]
	e.embeddings = e.embeddings[1:]
	return embedding, nil
}

func (e *fakeEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	copiedTexts := append([]string(nil), texts...)
	e.batchInputs = append(e.batchInputs, copiedTexts)
	if len(e.batches) == 0 {
		embeddings := make([][]float32, len(texts))
		for i := range texts {
			embeddings[i] = []float32{float32(i + 1), 0, 0}
		}
		return embeddings, nil
	}
	batch := e.batches[0]
	e.batches = e.batches[1:]
	return batch, nil
}

func (f *fakeStore) CreateKnowledgeBase(ctx context.Context, workspaceID, organizationID, name string) (KnowledgeBase, error) {
	f.organizationID = organizationID
	f.createdName = name
	return f.createdBase, nil
}

func (f *fakeStore) ListKnowledgeBases(ctx context.Context, organizationID string) ([]KnowledgeBase, error) {
	f.organizationID = organizationID
	return f.listBases, nil
}

func (f *fakeStore) GetKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID string) (KnowledgeBase, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	return f.detailBase, nil
}

func (f *fakeStore) ListKnowledgeDocuments(ctx context.Context, organizationID, knowledgeBaseID string) ([]KnowledgeDocument, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	return f.documents, nil
}

func (f *fakeStore) CreateKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, title, content string, chunks []KnowledgeDocumentChunk) (KnowledgeDocument, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.requestedDoc = KnowledgeDocument{
		Title:   title,
		Content: content,
	}
	f.persistedChunks = append([]KnowledgeDocumentChunk(nil), chunks...)
	return f.createdDoc, nil
}

func (f *fakeStore) UpdateKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID, name string) (KnowledgeBase, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.createdName = name
	return f.updatedBase, nil
}

func (f *fakeStore) DeleteKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID string) error {
	f.organizationID = organizationID
	f.deletedID = knowledgeBaseID
	return nil
}

func (f *fakeStore) UpdateKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, documentID, title, content string, chunks []KnowledgeDocumentChunk) (KnowledgeDocument, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.requestedDoc = KnowledgeDocument{
		Title:   title,
		Content: content,
	}
	f.persistedChunks = append([]KnowledgeDocumentChunk(nil), chunks...)
	return f.updatedDoc, nil
}

func (f *fakeStore) DeleteKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, documentID string) error {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	return nil
}

func (f *fakeStore) RetrieveKnowledge(ctx context.Context, organizationID, knowledgeBaseID string, queryEmbedding []float32, limit int, minScore float64) ([]KnowledgeRetrievalResult, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.queryEmbedding = append([]float32(nil), queryEmbedding...)
	f.retrievalLimit = limit
	f.retrievalMinScore = minScore
	return f.retrievalResults, nil
}

func TestListReturnsOrganizationKnowledgeBases(t *testing.T) {
	store := &fakeStore{
		listBases: []KnowledgeBase{
			{
				DocumentCount: 3,
				ID:            "kb_1",
				Name:          "Product Docs",
				UpdatedAt:     time.Date(2026, time.April, 2, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	embedder := &fakeEmbedder{}
	service := NewServiceWithEmbedder(store, embedder, "text-embedding-3-small")

	bases, err := service.List(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("list knowledge bases: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if len(bases) != 1 {
		t.Fatalf("expected 1 knowledge base, got %d", len(bases))
	}
	if bases[0].Name != "Product Docs" {
		t.Fatalf("expected Product Docs, got %s", bases[0].Name)
	}
}

func TestCreateCreatesKnowledgeBaseInOrganization(t *testing.T) {
	store := &fakeStore{
		createdBase: KnowledgeBase{
			DocumentCount: 0,
			ID:            "kb_1",
			Name:          "Research Vault",
			UpdatedAt:     time.Date(2026, time.April, 3, 8, 0, 0, 0, time.UTC),
		},
	}
	embedder := &fakeEmbedder{}
	service := NewServiceWithEmbedder(store, embedder, "text-embedding-3-small")

	base, err := service.Create(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"}, "Research Vault")
	if err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.createdName != "Research Vault" {
		t.Fatalf("expected created name Research Vault, got %s", store.createdName)
	}
	if base.ID != "kb_1" {
		t.Fatalf("expected base id kb_1, got %s", base.ID)
	}
}

func TestGetReturnsKnowledgeBaseFromOrganization(t *testing.T) {
	store := &fakeStore{
		detailBase: KnowledgeBase{
			DocumentCount: 7,
			ID:            "kb_7",
			Name:          "Customer Notes",
			UpdatedAt:     time.Date(2026, time.April, 3, 11, 0, 0, 0, time.UTC),
		},
	}
	embedder := &fakeEmbedder{}
	service := NewServiceWithEmbedder(store, embedder, "text-embedding-3-small")

	base, err := service.Get(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"}, "kb_7")
	if err != nil {
		t.Fatalf("get knowledge base: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.requestedID != "kb_7" {
		t.Fatalf("expected requested id kb_7, got %s", store.requestedID)
	}
	if base.Name != "Customer Notes" {
		t.Fatalf("expected Customer Notes, got %s", base.Name)
	}
}

func TestListDocumentsReturnsKnowledgeBaseDocuments(t *testing.T) {
	store := &fakeStore{
		documents: []KnowledgeDocument{
			{
				Content:   "Deployment notes",
				ID:        "doc_1",
				Title:     "Runbook",
				UpdatedAt: time.Date(2026, time.April, 3, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(store)

	documents, err := service.ListDocuments(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"}, "kb_7")
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.requestedID != "kb_7" {
		t.Fatalf("expected requested id kb_7, got %s", store.requestedID)
	}
	if len(documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(documents))
	}
}

func TestCreateDocumentCreatesDocumentInKnowledgeBase(t *testing.T) {
	store := &fakeStore{
		createdDoc: KnowledgeDocument{
			Content:   "Initial architecture outline",
			ID:        "doc_9",
			Title:     "Architecture Draft",
			UpdatedAt: time.Date(2026, time.April, 3, 12, 30, 0, 0, time.UTC),
		},
	}
	embedder := &fakeEmbedder{}
	service := NewServiceWithEmbedder(store, embedder, "text-embedding-3-small")

	document, err := service.CreateDocument(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"}, "kb_7", "Architecture Draft", "Initial architecture outline")
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.requestedID != "kb_7" {
		t.Fatalf("expected requested id kb_7, got %s", store.requestedID)
	}
	if store.requestedDoc.Title != "Architecture Draft" {
		t.Fatalf("expected title Architecture Draft, got %s", store.requestedDoc.Title)
	}
	if len(embedder.batchInputs) != 1 {
		t.Fatalf("expected document chunks to be embedded once, got %d calls", len(embedder.batchInputs))
	}
	if len(store.persistedChunks) == 0 {
		t.Fatalf("expected indexed chunks to be passed to store")
	}
	if store.persistedChunks[0].EmbeddingModel != "text-embedding-3-small" {
		t.Fatalf("expected embedding model to persist, got %q", store.persistedChunks[0].EmbeddingModel)
	}
	if len(store.persistedChunks[0].Embedding) == 0 {
		t.Fatalf("expected chunk embedding to persist")
	}
	if document.ID != "doc_9" {
		t.Fatalf("expected doc id doc_9, got %s", document.ID)
	}
}

func TestRetrieveUsesQueryEmbeddingAndReturnsCitations(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{
			{
				DocumentID:      "doc_9",
				DocumentTitle:   "Architecture Draft",
				ChunkID:         "kdc_1",
				ChunkIndex:      2,
				RetrievalMethod: "embedding_rag",
				Similarity:      0.91,
				Source: KnowledgeCitation{
					DocumentID:    "doc_9",
					DocumentTitle: "Architecture Draft",
					ChunkID:       "kdc_1",
					ChunkIndex:    2,
				},
				Snippet: "Initial architecture draft covers deployment boundaries.",
			},
		},
	}
	embedder := &fakeEmbedder{embeddings: [][]float32{{0.4, 0.5, 0.6}}}
	service := NewServiceWithEmbedder(store, embedder, "text-embedding-3-small")

	results, err := service.Retrieve(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"}, "kb_7", "deployment")
	if err != nil {
		t.Fatalf("retrieve knowledge: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.requestedID != "kb_7" {
		t.Fatalf("expected requested id kb_7, got %s", store.requestedID)
	}
	if len(embedder.embedInputs) != 1 || embedder.embedInputs[0] != "deployment" {
		t.Fatalf("expected query embedding for deployment, got %+v", embedder.embedInputs)
	}
	if len(store.queryEmbedding) != 3 || store.queryEmbedding[0] != 0.4 {
		t.Fatalf("expected query embedding to reach store, got %+v", store.queryEmbedding)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].RetrievalMethod != "embedding_rag" {
		t.Fatalf("expected embedding_rag retrieval method, got %q", results[0].RetrievalMethod)
	}
	if results[0].Source.ChunkID != "kdc_1" || results[0].Source.ChunkIndex != 2 {
		t.Fatalf("expected source citation to round trip, got %+v", results[0].Source)
	}
	if results[0].Snippet != "Initial architecture draft covers deployment boundaries." {
		t.Fatalf("unexpected snippet %q", results[0].Snippet)
	}
}

func TestRetrieveNormalizesKnowledgeQueryBeforeCallingStore(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{},
	}
	embedder := &fakeEmbedder{}
	service := NewServiceWithEmbedder(store, embedder, "text-embedding-3-small")

	if _, err := service.Retrieve(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"}, "kb_7", "  deployment   rollback  "); err != nil {
		t.Fatalf("retrieve knowledge: %v", err)
	}

	if len(embedder.embedInputs) != 1 || embedder.embedInputs[0] != "deployment rollback" {
		t.Fatalf("expected normalized query %q, got %+v", "deployment rollback", embedder.embedInputs)
	}
}

func TestRetrieveReturnsConfigurationErrorWhenEmbedderMissing(t *testing.T) {
	service := NewService(&fakeStore{})

	_, err := service.Retrieve(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"}, "kb_7", "deployment")
	if err == nil {
		t.Fatal("expected missing embedder error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "embedding") {
		t.Fatalf("expected embedding configuration error, got %v", err)
	}
}

func TestUpdateUpdatesKnowledgeBaseInOrganization(t *testing.T) {
	store := &fakeStore{
		updatedBase: KnowledgeBase{
			DocumentCount: 1,
			ID:            "kb_7",
			Name:          "Architecture Decisions",
			UpdatedAt:     time.Date(2026, time.April, 3, 13, 0, 0, 0, time.UTC),
		},
	}
	embedder := &fakeEmbedder{}
	service := NewServiceWithEmbedder(store, embedder, "text-embedding-3-small")

	base, err := service.Update(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"}, "kb_7", "Architecture Decisions")
	if err != nil {
		t.Fatalf("update knowledge base: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.requestedID != "kb_7" {
		t.Fatalf("expected requested id kb_7, got %s", store.requestedID)
	}
	if base.Name != "Architecture Decisions" {
		t.Fatalf("expected Architecture Decisions, got %s", base.Name)
	}
}

func TestDeleteDeletesKnowledgeBaseInWorkspace(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)

	if err := service.Delete(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"}, "kb_7"); err != nil {
		t.Fatalf("delete knowledge base: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.deletedID != "kb_7" {
		t.Fatalf("expected deleted id kb_7, got %s", store.deletedID)
	}
}

func TestUpdateDocumentUpdatesKnowledgeBaseDocument(t *testing.T) {
	store := &fakeStore{
		updatedDoc: KnowledgeDocument{
			Content:   "Updated outline",
			ID:        "doc_9",
			Title:     "Architecture Draft v2",
			UpdatedAt: time.Date(2026, time.April, 3, 13, 30, 0, 0, time.UTC),
		},
	}
	embedder := &fakeEmbedder{}
	service := NewServiceWithEmbedder(store, embedder, "text-embedding-3-small")

	document, err := service.UpdateDocument(
		context.Background(),
		auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"},
		"kb_7",
		"doc_9",
		"Architecture Draft v2",
		"Updated outline",
	)
	if err != nil {
		t.Fatalf("update knowledge document: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.deletedDocID != "doc_9" {
		t.Fatalf("expected requested doc id doc_9, got %s", store.deletedDocID)
	}
	if len(embedder.batchInputs) != 1 {
		t.Fatalf("expected updated document chunks to be embedded once, got %d calls", len(embedder.batchInputs))
	}
	if len(store.persistedChunks) == 0 || store.persistedChunks[0].EmbeddingModel != "text-embedding-3-small" {
		t.Fatalf("expected reindexed chunks with embedding model, got %+v", store.persistedChunks)
	}
	if document.Title != "Architecture Draft v2" {
		t.Fatalf("expected Architecture Draft v2, got %s", document.Title)
	}
}

func TestDeleteDocumentDeletesKnowledgeBaseDocument(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)

	if err := service.DeleteDocument(context.Background(), auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1"}, "kb_7", "doc_9"); err != nil {
		t.Fatalf("delete knowledge document: %v", err)
	}

	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.deletedDocID != "doc_9" {
		t.Fatalf("expected deleted doc id doc_9, got %s", store.deletedDocID)
	}
}
