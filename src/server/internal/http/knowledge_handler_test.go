package http

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	stdhttp "net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/knowledge"
)

type knowledgeFakeStore struct {
	createdName       string
	createdBase       knowledge.KnowledgeBase
	createdBaseConfig knowledge.KnowledgeBaseConfig
	createdDoc        knowledge.KnowledgeDocument
	deletedDocID      string
	deletedID         string
	detailBase        knowledge.KnowledgeBase
	documentChunks    []knowledge.KnowledgeDocumentChunkView
	documentVersions  []knowledge.KnowledgeDocumentVersion
	documents         []knowledge.KnowledgeDocument
	listBases         []knowledge.KnowledgeBase
	organizationID    string
	persistedChunks   []knowledge.KnowledgeDocumentChunk
	queryEmbedding    []float32
	retrievalOptions  knowledge.KnowledgeRetrievalOptions
	retrievalQuery    string
	retrievalResults  []knowledge.KnowledgeRetrievalResult
	createdTestCase   knowledge.KnowledgeRetrievalTestCase
	listTestCases     []knowledge.KnowledgeRetrievalTestCase
	testCaseRequest   knowledge.CreateKnowledgeRetrievalTestCaseRequest
	requestedDoc      knowledge.KnowledgeDocument
	requestedID       string
	mergedChunks      []knowledge.KnowledgeDocumentChunkView
	mergeDirection    string
	splitAt           int
	splitChunks       []knowledge.KnowledgeDocumentChunkView
	updatedBase       knowledge.KnowledgeBase
	updatedBaseConfig knowledge.KnowledgeBaseConfig
	updatedChunk      knowledge.KnowledgeDocumentChunkView
	updatedChunkID    string
	updatedContent    string
	updatedDoc        knowledge.KnowledgeDocument
}

type knowledgeFakeEmbedder struct {
	batchInputs [][]string
	embedInputs []string
}

func (e *knowledgeFakeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.embedInputs = append(e.embedInputs, text)
	return []float32{0.7, 0.2, 0.1}, nil
}

func (e *knowledgeFakeEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e.batchInputs = append(e.batchInputs, append([]string(nil), texts...))
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = []float32{float32(i + 1), 0, 0}
	}
	return embeddings, nil
}

func newKnowledgeTestHandler(store *knowledgeFakeStore) knowledgeHandler {
	return newKnowledgeHandler(knowledge.NewServiceWithEmbedder(store, &knowledgeFakeEmbedder{}, "text-embedding-3-small"))
}

func (f *knowledgeFakeStore) CreateKnowledgeBase(ctx context.Context, workspaceID, organizationID, name string) (knowledge.KnowledgeBase, error) {
	f.organizationID = organizationID
	f.createdName = name
	return f.createdBase, nil
}

func (f *knowledgeFakeStore) CreateKnowledgeBaseWithConfig(ctx context.Context, workspaceID, organizationID, name string, config knowledge.KnowledgeBaseConfig) (knowledge.KnowledgeBase, error) {
	f.organizationID = organizationID
	f.createdName = name
	f.createdBaseConfig = config
	return f.createdBase, nil
}

func (f *knowledgeFakeStore) ListKnowledgeBases(ctx context.Context, organizationID string) ([]knowledge.KnowledgeBase, error) {
	f.organizationID = organizationID
	return f.listBases, nil
}

func (f *knowledgeFakeStore) GetKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID string) (knowledge.KnowledgeBase, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	return f.detailBase, nil
}

func (f *knowledgeFakeStore) ListKnowledgeDocuments(ctx context.Context, organizationID, knowledgeBaseID string) ([]knowledge.KnowledgeDocument, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	return f.documents, nil
}

func (f *knowledgeFakeStore) ListKnowledgeDocumentChunks(ctx context.Context, organizationID, knowledgeBaseID, documentID string) ([]knowledge.KnowledgeDocumentChunkView, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	return f.documentChunks, nil
}

func (f *knowledgeFakeStore) ListKnowledgeDocumentVersions(ctx context.Context, organizationID, knowledgeBaseID, documentID string) ([]knowledge.KnowledgeDocumentVersion, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	return f.documentVersions, nil
}

func (f *knowledgeFakeStore) UpdateKnowledgeDocumentChunk(ctx context.Context, organizationID, knowledgeBaseID, documentID, chunkID, content string) (knowledge.KnowledgeDocumentChunkView, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.updatedChunkID = chunkID
	f.updatedContent = content
	return f.updatedChunk, nil
}

func (f *knowledgeFakeStore) SplitKnowledgeDocumentChunk(ctx context.Context, organizationID, knowledgeBaseID, documentID, chunkID string, splitAt int) ([]knowledge.KnowledgeDocumentChunkView, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.updatedChunkID = chunkID
	f.splitAt = splitAt
	return append([]knowledge.KnowledgeDocumentChunkView(nil), f.splitChunks...), nil
}

func (f *knowledgeFakeStore) MergeKnowledgeDocumentChunks(ctx context.Context, organizationID, knowledgeBaseID, documentID, chunkID, direction string) ([]knowledge.KnowledgeDocumentChunkView, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.updatedChunkID = chunkID
	f.mergeDirection = direction
	return append([]knowledge.KnowledgeDocumentChunkView(nil), f.mergedChunks...), nil
}

func (f *knowledgeFakeStore) CreateKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, title, content string, chunks []knowledge.KnowledgeDocumentChunk) (knowledge.KnowledgeDocument, error) {
	return f.CreateKnowledgeDocumentWithOptions(ctx, organizationID, knowledgeBaseID, title, content, chunks, knowledge.KnowledgeDocumentOptions{})
}

func (f *knowledgeFakeStore) CreateKnowledgeDocumentWithOptions(ctx context.Context, organizationID, knowledgeBaseID, title, content string, chunks []knowledge.KnowledgeDocumentChunk, options knowledge.KnowledgeDocumentOptions) (knowledge.KnowledgeDocument, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.requestedDoc = knowledge.KnowledgeDocument{
		Title:           title,
		Content:         content,
		DocumentVersion: options.DocumentVersion,
		UpdateStrategy:  options.UpdateStrategy,
	}
	f.persistedChunks = append([]knowledge.KnowledgeDocumentChunk(nil), chunks...)
	return f.createdDoc, nil
}

func (f *knowledgeFakeStore) UpdateKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID, name string) (knowledge.KnowledgeBase, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.createdName = name
	return f.updatedBase, nil
}

func (f *knowledgeFakeStore) UpdateKnowledgeBaseWithConfig(ctx context.Context, organizationID, knowledgeBaseID, name string, config knowledge.KnowledgeBaseConfig) (knowledge.KnowledgeBase, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.createdName = name
	f.updatedBaseConfig = config
	return f.updatedBase, nil
}

func (f *knowledgeFakeStore) DeleteKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID string) error {
	f.organizationID = organizationID
	f.deletedID = knowledgeBaseID
	return nil
}

func (f *knowledgeFakeStore) UpdateKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, documentID, title, content string, chunks []knowledge.KnowledgeDocumentChunk) (knowledge.KnowledgeDocument, error) {
	return f.UpdateKnowledgeDocumentWithOptions(ctx, organizationID, knowledgeBaseID, documentID, title, content, chunks, knowledge.KnowledgeDocumentOptions{})
}

func (f *knowledgeFakeStore) UpdateKnowledgeDocumentWithOptions(ctx context.Context, organizationID, knowledgeBaseID, documentID, title, content string, chunks []knowledge.KnowledgeDocumentChunk, options knowledge.KnowledgeDocumentOptions) (knowledge.KnowledgeDocument, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.requestedDoc = knowledge.KnowledgeDocument{
		Title:           title,
		Content:         content,
		DocumentVersion: options.DocumentVersion,
		UpdateStrategy:  options.UpdateStrategy,
	}
	return f.updatedDoc, nil
}

func (f *knowledgeFakeStore) DeleteKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, documentID string) error {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	return nil
}

func (f *knowledgeFakeStore) DeleteKnowledgeDocumentByID(ctx context.Context, organizationID, documentID string) error {
	f.organizationID = organizationID
	f.deletedDocID = documentID
	return nil
}

func (f *knowledgeFakeStore) RetrieveKnowledge(ctx context.Context, organizationID, knowledgeBaseID, query string, queryEmbedding []float32, options knowledge.KnowledgeRetrievalOptions) ([]knowledge.KnowledgeRetrievalResult, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.retrievalQuery = query
	f.queryEmbedding = append([]float32(nil), queryEmbedding...)
	f.retrievalOptions = options
	return f.retrievalResults, nil
}

func (f *knowledgeFakeStore) CreateRetrievalTestCase(ctx context.Context, organizationID, knowledgeBaseID string, req knowledge.CreateKnowledgeRetrievalTestCaseRequest) (knowledge.KnowledgeRetrievalTestCase, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	f.testCaseRequest = req
	return f.createdTestCase, nil
}

func (f *knowledgeFakeStore) ListRetrievalTestCases(ctx context.Context, organizationID, knowledgeBaseID string) ([]knowledge.KnowledgeRetrievalTestCase, error) {
	f.organizationID = organizationID
	f.requestedID = knowledgeBaseID
	return f.listTestCases, nil
}

func TestKnowledgeHandlerListReturnsWorkspaceBases(t *testing.T) {
	store := &knowledgeFakeStore{
		listBases: []knowledge.KnowledgeBase{
			{
				DocumentCount: 2,
				ID:            "kb_1",
				Name:          "Ops Runbooks",
				UpdatedAt:     time.Date(2026, time.April, 3, 9, 0, 0, 0, time.UTC),
			},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/knowledge-bases", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.listKnowledgeBases(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}

	var response struct {
		Data []knowledge.KnowledgeBase `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 {
		t.Fatalf("expected 1 knowledge base, got %d", len(response.Data))
	}
}

func TestKnowledgeHandlerCreateCreatesKnowledgeBase(t *testing.T) {
	store := &knowledgeFakeStore{
		createdBase: knowledge.KnowledgeBase{
			DocumentCount: 0,
			ID:            "kb_1",
			Name:          "Roadmap Notes",
			UpdatedAt:     time.Date(2026, time.April, 3, 9, 30, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases", strings.NewReader(`{"name":"Roadmap Notes"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createKnowledgeBase(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.createdName != "Roadmap Notes" {
		t.Fatalf("expected created name Roadmap Notes, got %s", store.createdName)
	}
}

func TestKnowledgeHandlerCreateKnowledgeBaseAcceptsRAGConfiguration(t *testing.T) {
	store := &knowledgeFakeStore{
		createdBase: knowledge.KnowledgeBase{
			DocumentCount:  0,
			ID:             "kb_1",
			Name:           "Roadmap Notes",
			RetrievalMode:  knowledge.KnowledgeRetrievalModeHybrid,
			ChunkStrategy:  knowledge.KnowledgeChunkStrategySemantic,
			ChunkSize:      900,
			ChunkOverlap:   80,
			EmbeddingModel: "text-embedding-3-small",
			UpdatedAt:      time.Date(2026, time.April, 3, 9, 30, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases", strings.NewReader(`{
		"name":"Roadmap Notes",
		"retrievalMode":"hybrid",
		"chunkStrategy":"semantic",
		"chunkSize":900,
		"chunkOverlap":80,
		"embeddingModel":"text-embedding-3-small"
	}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createKnowledgeBase(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.createdBaseConfig.RetrievalMode != knowledge.KnowledgeRetrievalModeHybrid {
		t.Fatalf("expected hybrid retrieval mode, got %+v", store.createdBaseConfig)
	}
	if store.createdBaseConfig.ChunkStrategy != knowledge.KnowledgeChunkStrategySemantic {
		t.Fatalf("expected semantic chunk strategy, got %+v", store.createdBaseConfig)
	}
	if store.createdBaseConfig.ChunkSize != 900 || store.createdBaseConfig.ChunkOverlap != 80 {
		t.Fatalf("expected chunk sizing 900/80, got %+v", store.createdBaseConfig)
	}
	if !strings.Contains(recorder.Body.String(), `"retrievalMode":"hybrid"`) {
		t.Fatalf("expected response to include RAG config, got %s", recorder.Body.String())
	}
}

func TestKnowledgeHandlerGetReturnsKnowledgeBase(t *testing.T) {
	store := &knowledgeFakeStore{
		detailBase: knowledge.KnowledgeBase{
			DocumentCount: 5,
			ID:            "kb_2",
			Name:          "Architecture Notes",
			UpdatedAt:     time.Date(2026, time.April, 3, 11, 30, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_2", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.getKnowledgeBase(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected requested id kb_2, got %s", store.requestedID)
	}
}

func TestKnowledgeHandlerListDocumentsReturnsKnowledgeBaseDocuments(t *testing.T) {
	store := &knowledgeFakeStore{
		documents: []knowledge.KnowledgeDocument{
			{
				Content:   "Deploy checklist",
				ID:        "doc_1",
				Title:     "Runbook",
				UpdatedAt: time.Date(2026, time.April, 3, 12, 45, 0, 0, time.UTC),
			},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_2/documents", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.listKnowledgeDocuments(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected requested id kb_2, got %s", store.requestedID)
	}
}

func TestKnowledgeHandlerListDocumentChunksReturnsTenantScopedChunks(t *testing.T) {
	store := &knowledgeFakeStore{
		documentChunks: []knowledge.KnowledgeDocumentChunkView{
			{
				ChunkID:             "kdc_1",
				ChunkIndex:          1,
				Content:             "Full chunk content.",
				DocumentVersion:     "v2",
				CharCount:           19,
				EstimatedTokenCount: 5,
				Metadata:            knowledge.KnowledgeChunkMetadata{DocumentVersion: "v2"},
			},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_2/documents/doc_1/chunks", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.listKnowledgeDocumentChunks(recorder, request, "kb_2", "doc_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.requestedID != "kb_2" || store.deletedDocID != "doc_1" {
		t.Fatalf("expected kb_2/doc_1, got %s/%s", store.requestedID, store.deletedDocID)
	}

	var response struct {
		Data []knowledge.KnowledgeDocumentChunkView `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ChunkID != "kdc_1" {
		t.Fatalf("expected chunk response, got %+v", response.Data)
	}
}

func TestKnowledgeHandlerListDocumentVersionsReturnsHistory(t *testing.T) {
	store := &knowledgeFakeStore{
		documentVersions: []knowledge.KnowledgeDocumentVersion{
			{
				ChunkCount:      2,
				Content:         "Current version content.",
				DocumentID:      "doc_1",
				DocumentVersion: "v3",
				KnowledgeBaseID: "kb_2",
				Title:           "Runbook",
				UpdateStrategy:  knowledge.KnowledgeUpdateStrategyVersioned,
				UpdatedAt:       time.Date(2026, time.June, 7, 10, 30, 0, 0, time.UTC),
			},
			{
				ChunkCount:      1,
				Content:         "Previous version content.",
				DocumentID:      "doc_1",
				DocumentVersion: "v2",
				KnowledgeBaseID: "kb_2",
				Title:           "Runbook",
				UpdateStrategy:  knowledge.KnowledgeUpdateStrategyVersioned,
				UpdatedAt:       time.Date(2026, time.June, 7, 9, 30, 0, 0, time.UTC),
			},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_2/documents/doc_1/versions", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.listKnowledgeDocumentVersions(recorder, request, "kb_2", "doc_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.organizationID != "org_1" || store.requestedID != "kb_2" || store.deletedDocID != "doc_1" {
		t.Fatalf("expected tenant-scoped version list, org=%q kb=%q doc=%q", store.organizationID, store.requestedID, store.deletedDocID)
	}
	var response struct {
		Data []knowledge.KnowledgeDocumentVersion `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 2 || response.Data[0].DocumentVersion != "v3" || response.Data[0].ChunkCount != 2 || response.Data[1].DocumentVersion != "v2" {
		t.Fatalf("expected version history response, got %+v", response.Data)
	}
}

func TestKnowledgeHandlerUpdateDocumentChunkReturnsUpdatedChunk(t *testing.T) {
	store := &knowledgeFakeStore{
		updatedChunk: knowledge.KnowledgeDocumentChunkView{
			ChunkID:             "kdc_1",
			ChunkIndex:          1,
			Content:             "Updated chunk content.",
			DocumentVersion:     "v2",
			CharCount:           22,
			EstimatedTokenCount: 6,
			Metadata:            knowledge.KnowledgeChunkMetadata{DocumentVersion: "v2"},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_2/documents/doc_1/chunks/kdc_1", strings.NewReader(`{"content":"  Updated chunk content.  "}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.updateKnowledgeDocumentChunk(recorder, request, "kb_2", "doc_1", "kdc_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.requestedID != "kb_2" || store.deletedDocID != "doc_1" || store.updatedChunkID != "kdc_1" {
		t.Fatalf("expected kb_2/doc_1/kdc_1, got %s/%s/%s", store.requestedID, store.deletedDocID, store.updatedChunkID)
	}
	if store.updatedContent != "Updated chunk content." {
		t.Fatalf("expected trimmed content, got %q", store.updatedContent)
	}

	var response struct {
		Data knowledge.KnowledgeDocumentChunkView `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ChunkID != "kdc_1" || response.Data.Content != "Updated chunk content." {
		t.Fatalf("expected updated chunk response, got %+v", response.Data)
	}
}

func TestKnowledgeHandlerUpdateDocumentChunkRejectsEmptyContent(t *testing.T) {
	store := &knowledgeFakeStore{}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_2/documents/doc_1/chunks/kdc_1", strings.NewReader(`{"content":"   "}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.updateKnowledgeDocumentChunk(recorder, request, "kb_2", "doc_1", "kdc_1")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "empty_chunk_content") {
		t.Fatalf("expected empty chunk content error, got %s", recorder.Body.String())
	}
	if store.updatedChunkID != "" {
		t.Fatalf("empty content must not call store, got chunk %q", store.updatedChunkID)
	}
}

func TestKnowledgeAliasRoutesUpdateDocumentChunkUsesPutWithoutBreakingList(t *testing.T) {
	store := &knowledgeFakeStore{
		documentChunks: []knowledge.KnowledgeDocumentChunkView{{ChunkID: "kdc_list", Content: "Existing chunk."}},
		updatedChunk:   knowledge.KnowledgeDocumentChunkView{ChunkID: "kdc_1", Content: "Updated chunk content."},
	}
	mux := stdhttp.NewServeMux()
	registerKnowledgeAliasRoutes(mux, &recordingSessionMiddleware{}, newKnowledgeTestHandler(store))

	updateRequest := knowledgeAliasRequest(stdhttp.MethodPut, "/api/v1/knowledge-bases/kb_2/documents/doc_1/chunks/kdc_1", `{"content":"Updated chunk content."}`)
	updateRecorder := httptest.NewRecorder()
	mux.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected PUT route 200, got %d with body %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	listRequest := knowledgeAliasRequest(stdhttp.MethodGet, "/api/v1/knowledge-bases/kb_2/documents/doc_1/chunks", "")
	listRecorder := httptest.NewRecorder()
	mux.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected GET chunks route 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}
}

func TestKnowledgeAliasRoutesDispatchDocumentVersions(t *testing.T) {
	store := &knowledgeFakeStore{
		documentVersions: []knowledge.KnowledgeDocumentVersion{{DocumentID: "doc_1", DocumentVersion: "v2", KnowledgeBaseID: "kb_2", Title: "Runbook"}},
	}
	mux := stdhttp.NewServeMux()
	registerKnowledgeAliasRoutes(mux, &recordingSessionMiddleware{}, newKnowledgeTestHandler(store))

	request := knowledgeAliasRequest(stdhttp.MethodGet, "/api/v1/knowledge-bases/kb_2/documents/doc_1/versions", "")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected document versions route 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.organizationID != "org_1" || store.requestedID != "kb_2" || store.deletedDocID != "doc_1" {
		t.Fatalf("expected version route to request org_1/kb_2/doc_1, got org=%q kb=%q doc=%q", store.organizationID, store.requestedID, store.deletedDocID)
	}
}

func TestKnowledgeHandlerSplitDocumentChunkReturnsReindexedChunks(t *testing.T) {
	store := &knowledgeFakeStore{
		splitChunks: []knowledge.KnowledgeDocumentChunkView{
			{ChunkID: "kdc_left", ChunkIndex: 0, Content: "First half."},
			{ChunkID: "kdc_right", ChunkIndex: 1, Content: "Second half."},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/documents/doc_1/chunks/kdc_1/split", strings.NewReader(`{"splitAt":11}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.splitKnowledgeDocumentChunk(recorder, request, "kb_2", "doc_1", "kdc_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.organizationID != "org_1" || store.requestedID != "kb_2" || store.deletedDocID != "doc_1" || store.updatedChunkID != "kdc_1" || store.splitAt != 11 {
		t.Fatalf("expected tenant-scoped split, org=%q kb=%q doc=%q chunk=%q splitAt=%d", store.organizationID, store.requestedID, store.deletedDocID, store.updatedChunkID, store.splitAt)
	}
	var response struct {
		Data []knowledge.KnowledgeDocumentChunkView `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 2 || response.Data[0].ChunkID != "kdc_left" || response.Data[1].ChunkID != "kdc_right" {
		t.Fatalf("expected split chunk response, got %+v", response.Data)
	}
}

func TestKnowledgeHandlerMergeDocumentChunksReturnsReindexedChunks(t *testing.T) {
	store := &knowledgeFakeStore{
		mergedChunks: []knowledge.KnowledgeDocumentChunkView{
			{ChunkID: "kdc_merged", ChunkIndex: 0, Content: "First half.\n\nSecond half."},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/documents/doc_1/chunks/kdc_1/merge", strings.NewReader(`{"direction":"next"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.mergeKnowledgeDocumentChunks(recorder, request, "kb_2", "doc_1", "kdc_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.organizationID != "org_1" || store.requestedID != "kb_2" || store.deletedDocID != "doc_1" || store.updatedChunkID != "kdc_1" || store.mergeDirection != "next" {
		t.Fatalf("expected tenant-scoped merge, org=%q kb=%q doc=%q chunk=%q direction=%q", store.organizationID, store.requestedID, store.deletedDocID, store.updatedChunkID, store.mergeDirection)
	}
	var response struct {
		Data []knowledge.KnowledgeDocumentChunkView `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ChunkID != "kdc_merged" {
		t.Fatalf("expected merged chunk response, got %+v", response.Data)
	}
}

func TestKnowledgeAliasRoutesDispatchChunkSplitAndMerge(t *testing.T) {
	store := &knowledgeFakeStore{
		mergedChunks: []knowledge.KnowledgeDocumentChunkView{{ChunkID: "kdc_merged", Content: "Merged."}},
		splitChunks:  []knowledge.KnowledgeDocumentChunkView{{ChunkID: "kdc_left", Content: "Left."}, {ChunkID: "kdc_right", Content: "Right."}},
	}
	mux := stdhttp.NewServeMux()
	registerKnowledgeAliasRoutes(mux, &recordingSessionMiddleware{}, newKnowledgeTestHandler(store))

	splitRequest := knowledgeAliasRequest(stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_2/documents/doc_1/chunks/kdc_1/split", `{"splitAt":5}`)
	splitRecorder := httptest.NewRecorder()
	mux.ServeHTTP(splitRecorder, splitRequest)
	if splitRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected split route 200, got %d with body %s", splitRecorder.Code, splitRecorder.Body.String())
	}

	mergeRequest := knowledgeAliasRequest(stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_2/documents/doc_1/chunks/kdc_1/merge", `{"direction":"next"}`)
	mergeRecorder := httptest.NewRecorder()
	mux.ServeHTTP(mergeRecorder, mergeRequest)
	if mergeRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected merge route 200, got %d with body %s", mergeRecorder.Code, mergeRecorder.Body.String())
	}
}

func TestKnowledgeHandlerCreateDocumentCreatesKnowledgeBaseDocument(t *testing.T) {
	store := &knowledgeFakeStore{
		createdDoc: knowledge.KnowledgeDocument{
			Content:   "Initial plan",
			ID:        "doc_2",
			Title:     "Plan",
			UpdatedAt: time.Date(2026, time.April, 3, 13, 0, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/documents", strings.NewReader(`{"title":"Plan","content":"Initial plan"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createKnowledgeDocument(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected requested id kb_2, got %s", store.requestedID)
	}
	if store.requestedDoc.Title != "Plan" {
		t.Fatalf("expected title Plan, got %s", store.requestedDoc.Title)
	}
}

func TestKnowledgeHandlerCreateDocumentAcceptsVersionOptions(t *testing.T) {
	store := &knowledgeFakeStore{
		createdDoc: knowledge.KnowledgeDocument{
			Content:         "Initial plan",
			DocumentVersion: "v3",
			ID:              "doc_2",
			Title:           "Plan",
			UpdateStrategy:  knowledge.KnowledgeUpdateStrategyVersioned,
			UpdatedAt:       time.Date(2026, time.April, 3, 13, 0, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/documents", strings.NewReader(`{
		"title":"Plan",
		"content":"Initial plan",
		"documentVersion":" v3 ",
		"updateStrategy":"versioned"
	}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createKnowledgeDocument(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.requestedDoc.DocumentVersion != "v3" {
		t.Fatalf("expected document version v3, got %q", store.requestedDoc.DocumentVersion)
	}
	if store.requestedDoc.UpdateStrategy != knowledge.KnowledgeUpdateStrategyVersioned {
		t.Fatalf("expected versioned update strategy, got %q", store.requestedDoc.UpdateStrategy)
	}
}

func TestKnowledgeHandlerUploadDocumentCreatesParsedKnowledgeDocument(t *testing.T) {
	store := &knowledgeFakeStore{
		createdDoc: knowledge.KnowledgeDocument{
			Content:         "deploy rollback steps",
			DocumentVersion: "v2",
			ID:              "doc_upload",
			Title:           "Runbook.md",
			UpdateStrategy:  knowledge.KnowledgeUpdateStrategyVersioned,
			UpdatedAt:       time.Date(2026, time.April, 3, 13, 0, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeTestHandler(store)
	body, contentType := knowledgeUploadMultipartBody(t, map[string]string{
		"documentVersion": " v2 ",
		"updateStrategy":  "versioned",
	}, "file", " Runbook.md ", "text/markdown", "\ufeffdeploy rollback steps")
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/documents/upload", body).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	handler.uploadKnowledgeDocument(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected kb_2, got %s", store.requestedID)
	}
	if store.requestedDoc.Title != "Runbook.md" {
		t.Fatalf("expected title from filename, got %q", store.requestedDoc.Title)
	}
	if store.requestedDoc.Content != "deploy rollback steps" {
		t.Fatalf("expected normalized uploaded content, got %q", store.requestedDoc.Content)
	}
	if store.requestedDoc.DocumentVersion != "v2" || store.requestedDoc.UpdateStrategy != knowledge.KnowledgeUpdateStrategyVersioned {
		t.Fatalf("expected versioned upload options, got %+v", store.requestedDoc)
	}
	if !strings.Contains(recorder.Body.String(), `"id":"doc_upload"`) {
		t.Fatalf("expected created document response, got %s", recorder.Body.String())
	}
}

func TestKnowledgeHandlerUploadDocumentPersistsSourceMetadataOnChunks(t *testing.T) {
	store := &knowledgeFakeStore{
		createdDoc: knowledge.KnowledgeDocument{
			Content: "deployment controls require approval",
			ID:      "doc_source",
			Title:   "Runbook.md",
		},
	}
	handler := newKnowledgeTestHandler(store)
	body, contentType := knowledgeUploadMultipartBody(t, map[string]string{
		"documentVersion": " v4 ",
		"pageNumber":      " 7 ",
		"sourceUrl":       " https://docs.example/runbook.md ",
	}, "file", "Runbook.md", "text/markdown", "deployment controls require approval")
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/documents/upload", body).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	handler.uploadKnowledgeDocument(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(store.persistedChunks) == 0 {
		t.Fatalf("expected indexed chunks to be passed to store")
	}
	metadata := store.persistedChunks[0].Metadata
	if metadata.DocumentVersion != "v4" || metadata.PageNumber != 7 || metadata.SourceURL != "https://docs.example/runbook.md" {
		t.Fatalf("expected upload source metadata on chunk, got %+v", metadata)
	}
}

func TestKnowledgeHandlerUploadDocumentCreatesParsedCSVKnowledgeDocument(t *testing.T) {
	store := &knowledgeFakeStore{
		createdDoc: knowledge.KnowledgeDocument{
			Content:   "title | owner\nDeploy | Ops",
			ID:        "doc_csv",
			Title:     "matrix.csv",
			UpdatedAt: time.Date(2026, time.April, 3, 13, 15, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeTestHandler(store)
	body, contentType := knowledgeUploadMultipartBody(t, nil, "file", " matrix.csv ", "text/csv", "title,owner\nDeploy,Ops")
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/documents/upload", body).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	handler.uploadKnowledgeDocument(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.requestedDoc.Title != "matrix.csv" {
		t.Fatalf("expected title from csv filename, got %q", store.requestedDoc.Title)
	}
	if store.requestedDoc.Content != "title | owner\nDeploy | Ops" {
		t.Fatalf("expected parsed csv rows, got %q", store.requestedDoc.Content)
	}
	if !strings.Contains(recorder.Body.String(), `"id":"doc_csv"`) {
		t.Fatalf("expected created csv document response, got %s", recorder.Body.String())
	}
}

func TestKnowledgeHandlerUploadDocumentCreatesParsedHTMLKnowledgeDocument(t *testing.T) {
	store := &knowledgeFakeStore{
		createdDoc: knowledge.KnowledgeDocument{
			Content:   "Deploy Plan\nRollback safely",
			ID:        "doc_html",
			Title:     "runbook.html",
			UpdatedAt: time.Date(2026, time.April, 3, 13, 30, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeTestHandler(store)
	body, contentType := knowledgeUploadMultipartBody(
		t,
		map[string]string{"title": " HTML Runbook "},
		"file",
		"runbook.html",
		"text/html",
		"<html><head><style>.hidden{display:none}</style><script>alert('x')</script></head><body><h1>Deploy Plan</h1><p>Rollback safely</p></body></html>",
	)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/documents/upload", body).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	handler.uploadKnowledgeDocument(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.requestedDoc.Title != "HTML Runbook" {
		t.Fatalf("expected explicit title, got %q", store.requestedDoc.Title)
	}
	if store.requestedDoc.Content != "Deploy Plan\nRollback safely" {
		t.Fatalf("expected parsed html text, got %q", store.requestedDoc.Content)
	}
	if strings.Contains(store.requestedDoc.Content, "alert") || strings.Contains(store.requestedDoc.Content, "hidden") {
		t.Fatalf("expected script/style content to be stripped, got %q", store.requestedDoc.Content)
	}
}

func TestKnowledgeHandlerUploadDocumentRejectsUnsupportedFormat(t *testing.T) {
	handler := newKnowledgeTestHandler(&knowledgeFakeStore{})
	body, contentType := knowledgeUploadMultipartBody(t, nil, "file", "manual.doc", "application/msword", "word document bytes")
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/documents/upload", body).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	handler.uploadKnowledgeDocument(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "unsupported_document_format") {
		t.Fatalf("expected unsupported document format response, got %s", recorder.Body.String())
	}
}

func TestKnowledgeHandlerRetrieveReturnsRelevantMatches(t *testing.T) {
	store := &knowledgeFakeStore{
		retrievalResults: []knowledge.KnowledgeRetrievalResult{
			{
				DocumentID:      "doc_2",
				DocumentTitle:   "Plan",
				ChunkID:         "kdc_2",
				ChunkIndex:      1,
				RetrievalMethod: "embedding_rag",
				Similarity:      0.92,
				Snippet:         "Initial plan mentions deployment boundaries.",
				Source: knowledge.KnowledgeCitation{
					DocumentID:    "doc_2",
					DocumentTitle: "Plan",
					ChunkID:       "kdc_2",
					ChunkIndex:    1,
				},
			},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieve", strings.NewReader(`{"query":"deployment"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.retrieveKnowledge(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected requested id kb_2, got %s", store.requestedID)
	}
	if len(store.queryEmbedding) == 0 {
		t.Fatalf("expected query embedding to reach store")
	}

	var response struct {
		Data []knowledge.KnowledgeRetrievalResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 {
		t.Fatalf("expected 1 retrieval result, got %d", len(response.Data))
	}
	rawResponse := recorder.Body.String()
	for _, field := range []string{`"chunkId"`, `"chunkIndex"`, `"retrievalMethod"`, `"similarity"`, `"source"`} {
		if !strings.Contains(rawResponse, field) {
			t.Fatalf("expected response to include %s, got %s", field, rawResponse)
		}
	}
	if response.Data[0].Snippet != "Initial plan mentions deployment boundaries." {
		t.Fatalf("unexpected snippet %q", response.Data[0].Snippet)
	}
	if response.Data[0].ChunkID != "kdc_2" || response.Data[0].ChunkIndex != 1 {
		t.Fatalf("expected citation chunk fields, got %+v", response.Data[0])
	}
	if response.Data[0].RetrievalMethod != "embedding_rag" || response.Data[0].Source.DocumentTitle != "Plan" {
		t.Fatalf("expected embedding RAG source citation, got %+v", response.Data[0])
	}
}

func TestKnowledgeHandlerCreateRetrievalTestCaseStoresResult(t *testing.T) {
	store := &knowledgeFakeStore{
		createdTestCase: knowledge.KnowledgeRetrievalTestCase{
			ID:                 "krtc_1",
			KnowledgeBaseID:    "kb_2",
			Query:              "deployment rollback",
			ExpectedDocumentID: "doc_2",
			ExpectedChunkID:    "kdc_2",
			ExpectedChunkIndex: 1,
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieval-test-cases", strings.NewReader(`{
		"query":" deployment   rollback ",
		"expectedResult":{
			"documentId":"doc_2",
			"documentTitle":"Plan",
			"chunkId":"kdc_2",
			"chunkIndex":1,
			"retrievalMethod":"hybrid",
			"similarity":0.92,
			"snippet":"Initial plan mentions deployment boundaries.",
			"source":{"documentId":"doc_2","documentTitle":"Plan","chunkId":"kdc_2","chunkIndex":1}
		}
	}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createRetrievalTestCase(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.organizationID != "org_1" || store.requestedID != "kb_2" {
		t.Fatalf("expected scoped retrieval test case store call, org=%q kb=%q", store.organizationID, store.requestedID)
	}
	if store.testCaseRequest.Query != "deployment rollback" || store.testCaseRequest.ExpectedResult.ChunkID != "kdc_2" {
		t.Fatalf("expected normalized test case request, got %+v", store.testCaseRequest)
	}
	if !strings.Contains(recorder.Body.String(), `"id":"krtc_1"`) {
		t.Fatalf("expected created test case response, got %s", recorder.Body.String())
	}
}

func TestKnowledgeHandlerListsRetrievalTestCases(t *testing.T) {
	store := &knowledgeFakeStore{
		listTestCases: []knowledge.KnowledgeRetrievalTestCase{
			{
				ID:                 "krtc_1",
				KnowledgeBaseID:    "kb_2",
				Query:              "deployment rollback",
				ExpectedDocumentID: "doc_2",
				ExpectedChunkID:    "kdc_2",
			},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_2/retrieval-test-cases", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.listRetrievalTestCases(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.organizationID != "org_1" || store.requestedID != "kb_2" {
		t.Fatalf("expected scoped retrieval test case list call, org=%q kb=%q", store.organizationID, store.requestedID)
	}
	if !strings.Contains(recorder.Body.String(), `"id":"krtc_1"`) {
		t.Fatalf("expected retrieval test case response, got %s", recorder.Body.String())
	}
}

func TestKnowledgeHandlerRunsRetrievalTestCases(t *testing.T) {
	store := &knowledgeFakeStore{
		detailBase: knowledge.KnowledgeBase{
			ID:             "kb_2",
			RetrievalLimit: 3,
			RetrievalMode:  knowledge.KnowledgeRetrievalModeHybrid,
		},
		listTestCases: []knowledge.KnowledgeRetrievalTestCase{
			{
				ID:                 "krtc_1",
				KnowledgeBaseID:    "kb_2",
				Query:              "deployment rollback",
				ExpectedDocumentID: "doc_2",
				ExpectedChunkID:    "kdc_2",
				ExpectedChunkIndex: 1,
			},
		},
		retrievalResults: []knowledge.KnowledgeRetrievalResult{
			{
				DocumentID: "doc_2",
				ChunkID:    "kdc_2",
				ChunkIndex: 1,
				Snippet:    "Initial plan mentions deployment boundaries.",
			},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieval-test-cases/run", strings.NewReader(`{
		"mode":"hybrid",
		"limit":3
	}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.runRetrievalTestCases(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.retrievalOptions.Mode != knowledge.KnowledgeRetrievalModeHybrid || store.retrievalOptions.Limit != 3 {
		t.Fatalf("expected hybrid run options, got %+v", store.retrievalOptions)
	}

	var response struct {
		Data knowledge.KnowledgeRetrievalTestRunReport `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Total != 1 || response.Data.Passed != 1 || response.Data.Failed != 0 {
		t.Fatalf("expected passing run report, got %+v", response.Data)
	}
	if len(response.Data.Results) != 1 || !response.Data.Results[0].Passed || response.Data.Results[0].Rank != 1 {
		t.Fatalf("expected passing case result, got %+v", response.Data.Results)
	}
	if !strings.Contains(recorder.Body.String(), `"expectedResult"`) || !strings.Contains(recorder.Body.String(), `"actualResult"`) {
		t.Fatalf("expected run report to use frontend-compatible result field names, got %s", recorder.Body.String())
	}
}

func TestKnowledgeHandlerRunsCuratedRetrievalBenchmarksAcrossModes(t *testing.T) {
	store := &knowledgeFakeStore{
		detailBase: knowledge.KnowledgeBase{
			ID:             "kb_2",
			RetrievalLimit: 3,
			RetrievalMode:  knowledge.KnowledgeRetrievalModeHybrid,
		},
		listTestCases: []knowledge.KnowledgeRetrievalTestCase{
			{
				ID:                 "krtc_1",
				KnowledgeBaseID:    "kb_2",
				Query:              "deployment rollback",
				ExpectedDocumentID: "doc_2",
				ExpectedChunkID:    "kdc_2",
			},
		},
		retrievalResults: []knowledge.KnowledgeRetrievalResult{
			{
				DocumentID: "doc_2",
				ChunkID:    "kdc_2",
				Snippet:    "Deployment rollback.",
			},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieval-test-cases/run", strings.NewReader(`{
		"benchmarkModes":["vector_only","hybrid","hybrid_rerank"],
		"limit":3
	}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.runRetrievalTestCases(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data knowledge.KnowledgeRetrievalTestRunReport `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.KnowledgeBaseID != "kb_2" {
		t.Fatalf("expected report knowledge base id kb_2, got %q", response.Data.KnowledgeBaseID)
	}
	if len(response.Data.Benchmarks) != 3 {
		t.Fatalf("expected three benchmark reports, got %+v", response.Data.Benchmarks)
	}
	if response.Data.Benchmarks[0].Mode != knowledge.KnowledgeRetrievalModeVector || response.Data.Benchmarks[1].Mode != knowledge.KnowledgeRetrievalModeHybrid || response.Data.Benchmarks[2].Mode != knowledge.KnowledgeRetrievalModeHybridRerank {
		t.Fatalf("unexpected benchmark mode order: %+v", response.Data.Benchmarks)
	}
}

func knowledgeUploadMultipartBody(t *testing.T, fields map[string]string, fieldName, filename, contentType, content string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write multipart field %s: %v", key, err)
		}
	}
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="` + fieldName + `"; filename="` + filename + `"`},
		"Content-Type":        {contentType},
	})
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, writer.FormDataContentType()
}

func TestKnowledgeHandlerRetrieveAcceptsDocumentVersionOptions(t *testing.T) {
	store := &knowledgeFakeStore{
		retrievalResults: []knowledge.KnowledgeRetrievalResult{},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieve", strings.NewReader(`{
		"query":"deployment",
		"mode":"hybrid",
		"documentVersion":" v2 "
	}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.retrieveKnowledge(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.retrievalOptions.DocumentVersion != "v2" {
		t.Fatalf("expected document version v2, got %q", store.retrievalOptions.DocumentVersion)
	}
	if store.retrievalOptions.AllVersions {
		t.Fatalf("expected allVersions false when documentVersion is selected")
	}
}

func TestKnowledgeHandlerRetrieveAllVersionsClearsVersionFilter(t *testing.T) {
	store := &knowledgeFakeStore{
		retrievalResults: []knowledge.KnowledgeRetrievalResult{},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieve", strings.NewReader(`{
		"query":"deployment",
		"allVersions":true,
		"documentVersion":"v2"
	}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.retrieveKnowledge(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !store.retrievalOptions.AllVersions {
		t.Fatalf("expected allVersions true")
	}
	if store.retrievalOptions.DocumentVersion != "" {
		t.Fatalf("expected allVersions to clear document version, got %q", store.retrievalOptions.DocumentVersion)
	}
}

func TestKnowledgeHandlerRetrieveTrimsAndNormalizesQuery(t *testing.T) {
	store := &knowledgeFakeStore{
		retrievalResults: []knowledge.KnowledgeRetrievalResult{},
	}
	embedder := &knowledgeFakeEmbedder{}
	handler := newKnowledgeHandler(knowledge.NewServiceWithEmbedder(store, embedder, "text-embedding-3-small"))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieve", strings.NewReader(`{"query":"  deployment   rollback  "}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.retrieveKnowledge(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if len(embedder.embedInputs) != 1 || embedder.embedInputs[0] != "deployment rollback" {
		t.Fatalf("expected normalized retrieval query, got %+v", embedder.embedInputs)
	}
}

func TestKnowledgeHandlerRetrieveAcceptsHybridOptions(t *testing.T) {
	store := &knowledgeFakeStore{
		retrievalResults: []knowledge.KnowledgeRetrievalResult{},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieve", strings.NewReader(`{"query":"deployment","mode":"hybrid","limit":3,"vectorWeight":0.6,"keywordWeight":0.4}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.retrieveKnowledge(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.retrievalOptions.Mode != knowledge.KnowledgeRetrievalModeHybrid {
		t.Fatalf("expected hybrid retrieval mode, got %q", store.retrievalOptions.Mode)
	}
	if store.retrievalOptions.Limit != 3 {
		t.Fatalf("expected limit 3, got %d", store.retrievalOptions.Limit)
	}
	if store.retrievalOptions.VectorWeight != 0.6 || store.retrievalOptions.KeywordWeight != 0.4 {
		t.Fatalf("expected weights 0.6/0.4, got %.2f/%.2f", store.retrievalOptions.VectorWeight, store.retrievalOptions.KeywordWeight)
	}
}

func TestKnowledgeHandlerRetrieveAcceptsVectorOnlyMode(t *testing.T) {
	store := &knowledgeFakeStore{
		retrievalResults: []knowledge.KnowledgeRetrievalResult{},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieve", strings.NewReader(`{"query":"deployment","mode":"vector_only","limit":2}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.retrieveKnowledge(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", recorder.Code, recorder.Body.String())
	}
	if store.retrievalOptions.Mode != knowledge.KnowledgeRetrievalModeVector {
		t.Fatalf("expected vector_only retrieval mode, got %q", store.retrievalOptions.Mode)
	}
}

func TestKnowledgeHandlerRetrieveUsesKnowledgeBaseRerankTopK(t *testing.T) {
	store := &knowledgeFakeStore{
		detailBase: knowledge.KnowledgeBase{
			ID:         "kb_2",
			RerankTopK: 4,
		},
		retrievalResults: []knowledge.KnowledgeRetrievalResult{
			{DocumentID: "doc_1", ChunkID: "chunk_1", Snippet: "deployment rollback"},
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieve", strings.NewReader(`{"query":"deployment","mode":"hybrid_rerank","limit":1}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.retrieveKnowledge(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.retrievalOptions.RerankTopK != 4 {
		t.Fatalf("expected rerank topK 4 from knowledge base config, got %+v", store.retrievalOptions)
	}
}

func TestKnowledgeHandlerRetrieveRejectsInvalidMode(t *testing.T) {
	store := &knowledgeFakeStore{
		retrievalResults: []knowledge.KnowledgeRetrievalResult{},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieve", strings.NewReader(`{"query":"deployment","mode":"hybird"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.retrieveKnowledge(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "invalid_retrieval_options") {
		t.Fatalf("expected invalid retrieval options response, got %s", recorder.Body.String())
	}
	if store.retrievalQuery != "" {
		t.Fatalf("invalid mode must not reach store, got query %q", store.retrievalQuery)
	}
}

func TestKnowledgeHandlerRetrieveReturnsEmptyListWhenNoMatchExists(t *testing.T) {
	store := &knowledgeFakeStore{
		retrievalResults: []knowledge.KnowledgeRetrievalResult{},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/retrieve", strings.NewReader(`{"query":"deployment"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.retrieveKnowledge(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var response struct {
		Data []knowledge.KnowledgeRetrievalResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 0 {
		t.Fatalf("expected empty retrieval results, got %d", len(response.Data))
	}
}

func TestKnowledgeHandlerUpdateKnowledgeBaseUpdatesKnowledgeBase(t *testing.T) {
	store := &knowledgeFakeStore{
		updatedBase: knowledge.KnowledgeBase{
			DocumentCount: 2,
			ID:            "kb_2",
			Name:          "Updated Notes",
			UpdatedAt:     time.Date(2026, time.April, 3, 13, 30, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_2", strings.NewReader(`{"name":"Updated Notes"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.updateKnowledgeBase(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected requested id kb_2, got %s", store.requestedID)
	}
	if store.createdName != "Updated Notes" {
		t.Fatalf("expected updated name Updated Notes, got %s", store.createdName)
	}
}

func TestKnowledgeHandlerUpdateKnowledgeBaseAcceptsRAGConfiguration(t *testing.T) {
	store := &knowledgeFakeStore{
		updatedBase: knowledge.KnowledgeBase{
			DocumentCount:  2,
			ID:             "kb_2",
			Name:           "Updated Notes",
			RetrievalMode:  knowledge.KnowledgeRetrievalModeHybridRerank,
			ChunkStrategy:  knowledge.KnowledgeChunkStrategyQASplit,
			ChunkSize:      1200,
			ChunkOverlap:   120,
			EmbeddingModel: "text-embedding-3-large",
			UpdatedAt:      time.Date(2026, time.April, 3, 13, 30, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_2", strings.NewReader(`{
		"name":"Updated Notes",
		"retrievalMode":"hybrid_rerank",
		"chunkStrategy":"qa_split",
		"chunkSize":1200,
		"chunkOverlap":120,
		"embeddingModel":"text-embedding-3-large"
	}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.updateKnowledgeBase(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.updatedBaseConfig.RetrievalMode != knowledge.KnowledgeRetrievalModeHybridRerank {
		t.Fatalf("expected hybrid_rerank retrieval mode, got %+v", store.updatedBaseConfig)
	}
	if store.updatedBaseConfig.ChunkStrategy != knowledge.KnowledgeChunkStrategyQASplit {
		t.Fatalf("expected qa_split chunk strategy, got %+v", store.updatedBaseConfig)
	}
	if store.updatedBaseConfig.EmbeddingModel != "text-embedding-3-large" {
		t.Fatalf("expected embedding model to be forwarded, got %+v", store.updatedBaseConfig)
	}
	if !strings.Contains(recorder.Body.String(), `"chunkStrategy":"qa_split"`) {
		t.Fatalf("expected response to include normalized chunk strategy, got %s", recorder.Body.String())
	}
}

func TestKnowledgeHandlerDeleteKnowledgeBaseDeletesKnowledgeBase(t *testing.T) {
	store := &knowledgeFakeStore{}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_2", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.deleteKnowledgeBase(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if store.deletedID != "kb_2" {
		t.Fatalf("expected deleted id kb_2, got %s", store.deletedID)
	}
}

func TestKnowledgeHandlerUpdateDocumentUpdatesKnowledgeBaseDocument(t *testing.T) {
	store := &knowledgeFakeStore{
		updatedDoc: knowledge.KnowledgeDocument{
			Content:   "Updated plan",
			ID:        "doc_2",
			Title:     "Plan v2",
			UpdatedAt: time.Date(2026, time.April, 3, 13, 45, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_2/documents/doc_2", strings.NewReader(`{"title":"Plan v2","content":"Updated plan"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.updateKnowledgeDocument(recorder, request, "kb_2", "doc_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected knowledge base id kb_2, got %s", store.requestedID)
	}
	if store.deletedDocID != "doc_2" {
		t.Fatalf("expected document id doc_2, got %s", store.deletedDocID)
	}
	if store.requestedDoc.Title != "Plan v2" {
		t.Fatalf("expected title Plan v2, got %s", store.requestedDoc.Title)
	}
}

func TestKnowledgeHandlerDeleteDocumentDeletesKnowledgeBaseDocument(t *testing.T) {
	store := &knowledgeFakeStore{}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_2/documents/doc_2", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.deleteKnowledgeDocument(recorder, request, "kb_2", "doc_2")

	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected knowledge base id kb_2, got %s", store.requestedID)
	}
	if store.deletedDocID != "doc_2" {
		t.Fatalf("expected document id doc_2, got %s", store.deletedDocID)
	}
}

func TestKnowledgeHandlerDeleteDocumentByIDDeletesOrganizationDocument(t *testing.T) {
	store := &knowledgeFakeStore{}
	handler := newKnowledgeTestHandler(store)
	request := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/documents/doc_3", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.deleteKnowledgeDocumentByID(recorder, request, "doc_3")

	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %s", store.organizationID)
	}
	if store.requestedID != "" {
		t.Fatalf("expected no knowledge base id, got %s", store.requestedID)
	}
	if store.deletedDocID != "doc_3" {
		t.Fatalf("expected document id doc_3, got %s", store.deletedDocID)
	}
}
