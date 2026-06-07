package knowledge

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/metrics"
	relaytypes "oblivious/server/internal/relay/types"
)

type fakeStore struct {
	createdName      string
	createdBase      KnowledgeBase
	createdChunks    []KnowledgeDocumentChunk
	createdDoc       KnowledgeDocument
	diffChunks       []KnowledgeDocumentChunk
	diffOptions      KnowledgeDocumentOptions
	deletedDocID     string
	deletedID        string
	detailBase       KnowledgeBase
	documentChunks   []KnowledgeDocumentChunkView
	documentVersions []KnowledgeDocumentVersion
	documents        []KnowledgeDocument
	listBases        []KnowledgeBase
	retrievalQuery   string
	retrievalOptions KnowledgeRetrievalOptions
	retrievalResults []KnowledgeRetrievalResult
	requestedDoc     KnowledgeDocument
	requestedID      string
	mergedChunks     []KnowledgeDocumentChunkView
	mergeDirection   string
	splitAt          int
	splitChunks      []KnowledgeDocumentChunkView
	updatedChunk     KnowledgeDocumentChunkView
	updatedChunkID   string
	updatedBase      KnowledgeBase
	updatedDoc       KnowledgeDocument
	workspaceID      string
}

type recordingKnowledgeVectorStore struct {
	chunks            []KnowledgeDocumentChunk
	documentID        string
	deletedCollection string
	ensuredCollection string
	ensuredVectorSize int
	organizationID    string
	knowledgeBaseID   string
	searchEmbedding   []float32
	searchOptions     KnowledgeRetrievalOptions
	searchQuery       string
	searchResults     []KnowledgeRetrievalResult
	deletedDocumentID string
}

func (f *fakeStore) CreateKnowledgeBase(ctx context.Context, workspaceID, name string) (KnowledgeBase, error) {
	f.workspaceID = workspaceID
	f.createdName = name
	return f.createdBase, nil
}

func (f *fakeStore) ListKnowledgeBases(ctx context.Context, workspaceID string) ([]KnowledgeBase, error) {
	f.workspaceID = workspaceID
	return f.listBases, nil
}

func (f *fakeStore) GetKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) (KnowledgeBase, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	return f.detailBase, nil
}

func (f *fakeStore) ListKnowledgeDocuments(ctx context.Context, workspaceID, knowledgeBaseID string) ([]KnowledgeDocument, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	return f.documents, nil
}

func (f *fakeStore) CreateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, title, content string) (KnowledgeDocument, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.requestedDoc = KnowledgeDocument{
		Title:   title,
		Content: content,
	}
	return f.createdDoc, nil
}

func (f *fakeStore) CreateKnowledgeDocumentWithOptions(ctx context.Context, workspaceID, knowledgeBaseID, title, content string, chunks []KnowledgeDocumentChunk, options KnowledgeDocumentOptions) (KnowledgeDocument, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.requestedDoc = KnowledgeDocument{
		Title:           title,
		Content:         content,
		DocumentVersion: options.DocumentVersion,
		UpdateStrategy:  options.UpdateStrategy,
	}
	f.createdChunks = append([]KnowledgeDocumentChunk(nil), chunks...)
	return f.createdDoc, nil
}

func (f *fakeStore) UpdateKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID, name string) (KnowledgeBase, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.createdName = name
	return f.updatedBase, nil
}

func (f *fakeStore) DeleteKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) error {
	f.workspaceID = workspaceID
	f.deletedID = knowledgeBaseID
	return nil
}

func (f *fakeStore) UpdateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, documentID, title, content string) (KnowledgeDocument, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.requestedDoc = KnowledgeDocument{
		Title:   title,
		Content: content,
	}
	return f.updatedDoc, nil
}

func (f *fakeStore) UpdateKnowledgeDocumentWithOptions(ctx context.Context, workspaceID, knowledgeBaseID, documentID, title, content string, chunks []KnowledgeDocumentChunk, options KnowledgeDocumentOptions) (KnowledgeDocument, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.requestedDoc = KnowledgeDocument{
		Title:           title,
		Content:         content,
		DocumentVersion: options.DocumentVersion,
		UpdateStrategy:  options.UpdateStrategy,
	}
	f.createdChunks = append([]KnowledgeDocumentChunk(nil), chunks...)
	return f.updatedDoc, nil
}

func (f *fakeStore) DiffKnowledgeDocumentChunks(ctx context.Context, workspaceID, knowledgeBaseID, documentID string, chunks []KnowledgeDocumentChunk, options KnowledgeDocumentOptions) ([]KnowledgeDocumentChunk, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.diffOptions = options
	if f.diffChunks != nil {
		return append([]KnowledgeDocumentChunk(nil), f.diffChunks...), nil
	}
	return append([]KnowledgeDocumentChunk(nil), chunks...), nil
}

func (f *fakeStore) ListKnowledgeDocumentChunks(ctx context.Context, workspaceID, knowledgeBaseID, documentID string) ([]KnowledgeDocumentChunkView, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	return append([]KnowledgeDocumentChunkView(nil), f.documentChunks...), nil
}

func (f *fakeStore) ListKnowledgeDocumentVersions(ctx context.Context, workspaceID, knowledgeBaseID, documentID string) ([]KnowledgeDocumentVersion, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	return append([]KnowledgeDocumentVersion(nil), f.documentVersions...), nil
}

func (f *fakeStore) UpdateKnowledgeDocumentChunk(ctx context.Context, workspaceID, knowledgeBaseID, documentID, chunkID, content string) (KnowledgeDocumentChunkView, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.updatedChunkID = chunkID
	f.requestedDoc = KnowledgeDocument{Content: content}
	return f.updatedChunk, nil
}

func (f *fakeStore) SplitKnowledgeDocumentChunk(ctx context.Context, workspaceID, knowledgeBaseID, documentID, chunkID string, splitAt int) ([]KnowledgeDocumentChunkView, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.updatedChunkID = chunkID
	f.splitAt = splitAt
	return append([]KnowledgeDocumentChunkView(nil), f.splitChunks...), nil
}

func (f *fakeStore) MergeKnowledgeDocumentChunks(ctx context.Context, workspaceID, knowledgeBaseID, documentID, chunkID, direction string) ([]KnowledgeDocumentChunkView, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.updatedChunkID = chunkID
	f.mergeDirection = direction
	return append([]KnowledgeDocumentChunkView(nil), f.mergedChunks...), nil
}

func (f *fakeStore) DeleteKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, documentID string) error {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	return nil
}

func (f *fakeStore) RetrieveKnowledge(ctx context.Context, workspaceID, knowledgeBaseID, query string) ([]KnowledgeRetrievalResult, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.retrievalQuery = query
	return f.retrievalResults, nil
}

func (f *fakeStore) RetrieveKnowledgeWithOptions(ctx context.Context, workspaceID, knowledgeBaseID, query string, queryEmbedding []float32, options KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.retrievalQuery = query
	f.retrievalOptions = options
	return f.retrievalResults, nil
}

func (s *recordingKnowledgeVectorStore) EnsureKnowledgeBaseCollection(ctx context.Context, organizationID, knowledgeBaseID string, vectorSize int) error {
	s.organizationID = organizationID
	s.knowledgeBaseID = knowledgeBaseID
	s.ensuredVectorSize = vectorSize
	s.ensuredCollection = KnowledgeVectorCollectionName(organizationID, knowledgeBaseID)
	return nil
}

func (s *recordingKnowledgeVectorStore) DeleteKnowledgeBaseCollection(ctx context.Context, organizationID, knowledgeBaseID string) error {
	s.organizationID = organizationID
	s.knowledgeBaseID = knowledgeBaseID
	s.deletedCollection = KnowledgeVectorCollectionName(organizationID, knowledgeBaseID)
	return nil
}

func (s *recordingKnowledgeVectorStore) DeleteKnowledgeDocumentChunks(ctx context.Context, organizationID, knowledgeBaseID, documentID string) error {
	s.organizationID = organizationID
	s.knowledgeBaseID = knowledgeBaseID
	s.deletedDocumentID = documentID
	return nil
}

func (s *recordingKnowledgeVectorStore) UpsertKnowledgeDocumentChunks(ctx context.Context, organizationID, knowledgeBaseID, documentID string, chunks []KnowledgeDocumentChunk) error {
	s.organizationID = organizationID
	s.knowledgeBaseID = knowledgeBaseID
	s.documentID = documentID
	s.chunks = append([]KnowledgeDocumentChunk(nil), chunks...)
	return nil
}

func (s *recordingKnowledgeVectorStore) SearchKnowledgeChunks(ctx context.Context, organizationID, knowledgeBaseID, query string, queryEmbedding []float32, options KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, error) {
	s.organizationID = organizationID
	s.knowledgeBaseID = knowledgeBaseID
	s.searchQuery = query
	s.searchEmbedding = append([]float32(nil), queryEmbedding...)
	s.searchOptions = options
	return append([]KnowledgeRetrievalResult(nil), s.searchResults...), nil
}

type recordingKnowledgeEmbedder struct {
	batchOrganizationID string
	batchUserID         string
	organizationID      string
	userID              string
}

type recordingKnowledgeReranker struct {
	err     error
	limit   int
	query   string
	results []KnowledgeRetrievalResult
}

func (r *recordingKnowledgeReranker) Rerank(ctx context.Context, query string, results []KnowledgeRetrievalResult, limit int) ([]KnowledgeRetrievalResult, error) {
	r.query = query
	r.limit = limit
	r.results = append([]KnowledgeRetrievalResult(nil), results...)
	if r.err != nil {
		return nil, r.err
	}
	reranked := append([]KnowledgeRetrievalResult(nil), results...)
	if len(reranked) >= 2 {
		reranked[0], reranked[1] = reranked[1], reranked[0]
		reranked[0].Score = 0.99
		reranked[1].Score = 0.42
	}
	for index := range reranked {
		reranked[index].RetrievalMethod = KnowledgeRetrievalModeHybridRerank
	}
	if limit > 0 && len(reranked) > limit {
		reranked = reranked[:limit]
	}
	return reranked, nil
}

func (e *recordingKnowledgeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.userID, _ = relaytypes.TrustedUserIDFromContext(ctx)
	e.organizationID, _ = relaytypes.TrustedOrganizationIDFromContext(ctx)
	return []float32{0.1, 0.2, 0.3}, nil
}

func (e *recordingKnowledgeEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e.batchUserID, _ = relaytypes.TrustedUserIDFromContext(ctx)
	e.batchOrganizationID, _ = relaytypes.TrustedOrganizationIDFromContext(ctx)
	embeddings := make([][]float32, len(texts))
	for index := range texts {
		embeddings[index] = []float32{0.1, 0.2, 0.3}
	}
	return embeddings, nil
}

func TestListReturnsWorkspaceKnowledgeBases(t *testing.T) {
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
	service := NewService(store)

	bases, err := service.List(context.Background(), auth.Session{WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("list knowledge bases: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if len(bases) != 1 {
		t.Fatalf("expected 1 knowledge base, got %d", len(bases))
	}
	if bases[0].Name != "Product Docs" {
		t.Fatalf("expected Product Docs, got %s", bases[0].Name)
	}
}

func TestCreateCreatesKnowledgeBaseInWorkspace(t *testing.T) {
	store := &fakeStore{
		createdBase: KnowledgeBase{
			DocumentCount: 0,
			ID:            "kb_1",
			Name:          "Research Vault",
			UpdatedAt:     time.Date(2026, time.April, 3, 8, 0, 0, 0, time.UTC),
		},
	}
	service := NewService(store)

	base, err := service.Create(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "Research Vault")
	if err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.createdName != "Research Vault" {
		t.Fatalf("expected created name Research Vault, got %s", store.createdName)
	}
	if base.ID != "kb_1" {
		t.Fatalf("expected base id kb_1, got %s", base.ID)
	}
}

func TestCreateWithConfigEnsuresTenantScopedQdrantCollection(t *testing.T) {
	store := &fakeStore{
		createdBase: KnowledgeBase{
			ID:   "kb_product_docs",
			Name: "Product Docs",
		},
	}
	vectorStore := &recordingKnowledgeVectorStore{}
	service := NewServiceWithVectorStore(store, vectorStore, 3072)

	base, err := service.CreateWithConfig(context.Background(), auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_acme"}, "Product Docs", KnowledgeBaseConfig{})
	if err != nil {
		t.Fatalf("create knowledge base with vector store: %v", err)
	}

	if base.ID != "kb_product_docs" {
		t.Fatalf("expected created base id, got %q", base.ID)
	}
	if vectorStore.organizationID != "org_acme" || vectorStore.knowledgeBaseID != "kb_product_docs" {
		t.Fatalf("unexpected vector store scope org=%q kb=%q", vectorStore.organizationID, vectorStore.knowledgeBaseID)
	}
	if vectorStore.ensuredCollection != "kb_org_acme_kb_product_docs" {
		t.Fatalf("expected tenant-scoped qdrant collection, got %q", vectorStore.ensuredCollection)
	}
	if vectorStore.ensuredVectorSize != 3072 {
		t.Fatalf("expected vector size 3072, got %d", vectorStore.ensuredVectorSize)
	}
}

func TestKnowledgeVectorCollectionNameSanitizesScope(t *testing.T) {
	collection := KnowledgeVectorCollectionName("Org 123/Prod", "KB:Main")
	if collection != "kb_org_123_prod_kb_main" {
		t.Fatalf("unexpected collection name %q", collection)
	}
}

func TestGetReturnsKnowledgeBaseFromWorkspace(t *testing.T) {
	store := &fakeStore{
		detailBase: KnowledgeBase{
			DocumentCount: 7,
			ID:            "kb_7",
			Name:          "Customer Notes",
			UpdatedAt:     time.Date(2026, time.April, 3, 11, 0, 0, 0, time.UTC),
		},
	}
	service := NewService(store)

	base, err := service.Get(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7")
	if err != nil {
		t.Fatalf("get knowledge base: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
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

	documents, err := service.ListDocuments(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7")
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.requestedID != "kb_7" {
		t.Fatalf("expected requested id kb_7, got %s", store.requestedID)
	}
	if len(documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(documents))
	}
}

func TestListDocumentVersionsReturnsOrganizationScopedHistory(t *testing.T) {
	store := &fakeStore{
		documentVersions: []KnowledgeDocumentVersion{
			{
				ChunkCount:      2,
				Content:         "Current version content.",
				DocumentID:      "doc_1",
				DocumentVersion: "v3",
				KnowledgeBaseID: "kb_7",
				Title:           "Runbook",
				UpdateStrategy:  KnowledgeUpdateStrategyVersioned,
				UpdatedAt:       time.Date(2026, time.June, 7, 10, 30, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(store)

	versions, err := service.ListDocumentVersions(context.Background(), auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}, "kb_7", "doc_1")
	if err != nil {
		t.Fatalf("list document versions: %v", err)
	}

	if store.workspaceID != "org_1" {
		t.Fatalf("expected organization scope org_1, got %s", store.workspaceID)
	}
	if store.requestedID != "kb_7" || store.deletedDocID != "doc_1" {
		t.Fatalf("expected kb_7/doc_1, got %s/%s", store.requestedID, store.deletedDocID)
	}
	if len(versions) != 1 || versions[0].DocumentVersion != "v3" || versions[0].ChunkCount != 2 {
		t.Fatalf("expected version history response, got %+v", versions)
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
	service := NewService(store)

	document, err := service.CreateDocument(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7", "Architecture Draft", "Initial architecture outline")
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.requestedID != "kb_7" {
		t.Fatalf("expected requested id kb_7, got %s", store.requestedID)
	}
	if store.requestedDoc.Title != "Architecture Draft" {
		t.Fatalf("expected title Architecture Draft, got %s", store.requestedDoc.Title)
	}
	if document.ID != "doc_9" {
		t.Fatalf("expected doc id doc_9, got %s", document.ID)
	}
}

func TestCreateDocumentRecordsRAGDocumentProcessingMetrics(t *testing.T) {
	store := &fakeStore{
		createdDoc: KnowledgeDocument{
			Content:   "alpha beta gamma",
			ID:        "doc_metrics",
			Title:     "Metrics Doc",
			UpdatedAt: time.Date(2026, time.June, 6, 10, 0, 0, 0, time.UTC),
		},
	}
	service := NewService(store)
	beforeProcessing := histogramSampleCount(t, metrics.RAGDocumentProcessingDurationSeconds.WithLabelValues(KnowledgeChunkStrategyTemplateBased))

	document, err := service.CreateDocumentWithOptions(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"},
		"kb_metrics",
		"Metrics Doc",
		"alpha beta gamma\n\nsecond chunk source text",
		KnowledgeDocumentOptions{},
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	if document.ID != "doc_metrics" {
		t.Fatalf("expected doc id doc_metrics, got %s", document.ID)
	}
	if len(store.createdChunks) == 0 {
		t.Fatal("expected generated chunks to be passed to store")
	}
	if count := histogramSampleCount(t, metrics.RAGDocumentProcessingDurationSeconds.WithLabelValues(KnowledgeChunkStrategyTemplateBased)); count <= beforeProcessing {
		t.Fatalf("expected RAG document processing histogram sample, before=%v after=%v", beforeProcessing, count)
	}
	if got := testutil.ToFloat64(metrics.RAGChunkCount); got != float64(len(store.createdChunks)) {
		t.Fatalf("expected RAG chunk count %d, got %v", len(store.createdChunks), got)
	}
}

func TestCreateDocumentUsesKnowledgeBaseChunkingConfig(t *testing.T) {
	store := &fakeStore{
		createdDoc: KnowledgeDocument{ID: "doc_chunk_config", Title: "Chunk Config Doc"},
		detailBase: KnowledgeBase{
			ChunkOverlap:  20,
			ChunkSize:     120,
			ChunkStrategy: KnowledgeChunkStrategyFixedSize,
			ID:            "kb_chunk_config",
			Name:          "Chunk Config KB",
		},
	}
	service := NewService(store)
	content := strings.Repeat("a", 260)

	if _, err := service.CreateDocumentWithOptions(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"},
		"kb_chunk_config",
		"Chunk Config Doc",
		content,
		KnowledgeDocumentOptions{},
	); err != nil {
		t.Fatalf("create document with configured chunks: %v", err)
	}

	if store.workspaceID != "org_1" || store.requestedID != "kb_chunk_config" {
		t.Fatalf("expected knowledge base config lookup and write to use org scope, scope=%q kb=%q", store.workspaceID, store.requestedID)
	}
	if len(store.createdChunks) != 3 {
		t.Fatalf("expected fixed-size chunk config to create 3 chunks, got %d: %+v", len(store.createdChunks), store.createdChunks)
	}
	if got := len([]rune(store.createdChunks[0].Content)); got != 120 {
		t.Fatalf("expected first chunk size 120 from KB config, got %d", got)
	}
	if store.createdChunks[1].Metadata.StartRune != 100 || store.createdChunks[1].Metadata.EndRune != 220 {
		t.Fatalf("expected overlap metadata start/end 100/220, got %+v", store.createdChunks[1].Metadata)
	}
}

func TestCreateDocumentUsesSemanticChunkingConfig(t *testing.T) {
	store := &fakeStore{
		createdDoc: KnowledgeDocument{ID: "doc_semantic_chunk_config", Title: "Semantic Chunk Config Doc"},
		detailBase: KnowledgeBase{
			ChunkSize:     120,
			ChunkStrategy: KnowledgeChunkStrategySemantic,
			ID:            "kb_semantic_chunk_config",
			Name:          "Semantic Chunk Config KB",
		},
	}
	service := NewService(store)
	content := strings.Repeat("semantic phrase ", 12)

	if _, err := service.CreateDocumentWithOptions(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"},
		"kb_semantic_chunk_config",
		"Semantic Chunk Config Doc",
		content,
		KnowledgeDocumentOptions{},
	); err != nil {
		t.Fatalf("create document with semantic chunks: %v", err)
	}

	if len(store.createdChunks) < 2 {
		t.Fatalf("expected semantic chunking to honor configured chunk size and split the paragraph, got %d chunks: %+v", len(store.createdChunks), store.createdChunks)
	}
	if got := len([]rune(store.createdChunks[0].Content)); got > 120 {
		t.Fatalf("expected semantic chunk to be capped by KB chunk size 120, got %d: %q", got, store.createdChunks[0].Content)
	}
	if store.createdChunks[0].Metadata.EndRune == 0 {
		t.Fatalf("expected semantic chunk metadata to include end rune, got %+v", store.createdChunks[0].Metadata)
	}
}

func TestCreateDocumentUsesQAChunkingConfig(t *testing.T) {
	store := &fakeStore{
		createdDoc: KnowledgeDocument{ID: "doc_qa_chunk_config", Title: "QA Chunk Config Doc"},
		detailBase: KnowledgeBase{
			ChunkSize:     200,
			ChunkStrategy: KnowledgeChunkStrategyQASplit,
			ID:            "kb_qa_chunk_config",
			Name:          "QA Chunk Config KB",
		},
	}
	service := NewService(store)
	content := strings.Join([]string{
		"Q: What does the workspace store?",
		"A: It stores tenant-scoped knowledge documents.",
		"Q: How are answers traced?",
		"A: Citations point back to source chunks.",
	}, "\n")

	if _, err := service.CreateDocumentWithOptions(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"},
		"kb_qa_chunk_config",
		"QA Chunk Config Doc",
		content,
		KnowledgeDocumentOptions{},
	); err != nil {
		t.Fatalf("create document with QA chunks: %v", err)
	}

	if len(store.createdChunks) != 2 {
		t.Fatalf("expected QA chunking to group each question and answer pair into 2 chunks, got %d: %+v", len(store.createdChunks), store.createdChunks)
	}
	if first := store.createdChunks[0].Content; !strings.Contains(first, "Q: What does the workspace store?") || !strings.Contains(first, "A: It stores tenant-scoped knowledge documents.") {
		t.Fatalf("expected first QA chunk to contain the first question and answer, got %q", first)
	}
	if second := store.createdChunks[1].Content; !strings.Contains(second, "Q: How are answers traced?") || !strings.Contains(second, "A: Citations point back to source chunks.") {
		t.Fatalf("expected second QA chunk to contain the second question and answer, got %q", second)
	}
}

func TestCreateDocumentUsesTemplateBasedChunkingConfig(t *testing.T) {
	store := &fakeStore{
		createdDoc: KnowledgeDocument{ID: "doc_template_chunk_config", Title: "Template Chunk Config Doc"},
		detailBase: KnowledgeBase{
			ChunkSize:     200,
			ChunkStrategy: KnowledgeChunkStrategyTemplateBased,
			ID:            "kb_template_chunk_config",
			Name:          "Template Chunk Config KB",
		},
	}
	service := NewService(store)
	content := strings.Join([]string{
		"# Overview",
		"Overview content stays attached to its heading.",
		"## Procedure",
		"Procedure content stays attached to the next heading.",
	}, "\n")

	if _, err := service.CreateDocumentWithOptions(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"},
		"kb_template_chunk_config",
		"Template Chunk Config Doc",
		content,
		KnowledgeDocumentOptions{},
	); err != nil {
		t.Fatalf("create document with template chunks: %v", err)
	}

	if len(store.createdChunks) != 2 {
		t.Fatalf("expected template chunking to split by headings into 2 sections, got %d: %+v", len(store.createdChunks), store.createdChunks)
	}
	if first := store.createdChunks[0].Content; !strings.Contains(first, "# Overview") || !strings.Contains(first, "Overview content stays attached") {
		t.Fatalf("expected first template chunk to keep overview heading with body, got %q", first)
	}
	if second := store.createdChunks[1].Content; !strings.Contains(second, "## Procedure") || !strings.Contains(second, "Procedure content stays attached") {
		t.Fatalf("expected second template chunk to keep procedure heading with body, got %q", second)
	}
}

func TestUpdateDocumentUsesKnowledgeBaseChunkingConfig(t *testing.T) {
	store := &fakeStore{
		detailBase: KnowledgeBase{
			ChunkOverlap:  20,
			ChunkSize:     120,
			ChunkStrategy: KnowledgeChunkStrategyFixedSize,
			ID:            "kb_chunk_config",
			Name:          "Chunk Config KB",
		},
		updatedDoc: KnowledgeDocument{ID: "doc_chunk_config", Title: "Chunk Config Doc"},
	}
	service := NewService(store)
	content := strings.Repeat("b", 260)

	if _, err := service.UpdateDocumentWithOptions(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"},
		"kb_chunk_config",
		"doc_chunk_config",
		"Chunk Config Doc",
		content,
		KnowledgeDocumentOptions{},
	); err != nil {
		t.Fatalf("update document with configured chunks: %v", err)
	}

	if store.deletedDocID != "doc_chunk_config" {
		t.Fatalf("expected update to target doc_chunk_config, got %q", store.deletedDocID)
	}
	if len(store.createdChunks) != 3 {
		t.Fatalf("expected fixed-size chunk config to create 3 update chunks, got %d: %+v", len(store.createdChunks), store.createdChunks)
	}
	if got := len([]rune(store.createdChunks[0].Content)); got != 120 {
		t.Fatalf("expected first update chunk size 120 from KB config, got %d", got)
	}
	if store.createdChunks[1].Metadata.StartRune != 100 || store.createdChunks[1].Metadata.EndRune != 220 {
		t.Fatalf("expected update overlap metadata start/end 100/220, got %+v", store.createdChunks[1].Metadata)
	}
}

func TestUpdateDocumentUsesTemplateBasedChunkingConfig(t *testing.T) {
	store := &fakeStore{
		detailBase: KnowledgeBase{
			ChunkSize:     200,
			ChunkStrategy: KnowledgeChunkStrategyTemplateBased,
			ID:            "kb_template_chunk_config",
			Name:          "Template Chunk Config KB",
		},
		updatedDoc: KnowledgeDocument{ID: "doc_template_chunk_config", Title: "Template Chunk Config Doc"},
	}
	service := NewService(store)
	content := strings.Join([]string{
		"# Existing Behavior",
		"Existing behavior stays with the first heading.",
		"## Updated Behavior",
		"Updated behavior stays with the next heading.",
	}, "\n")

	if _, err := service.UpdateDocumentWithOptions(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"},
		"kb_template_chunk_config",
		"doc_template_chunk_config",
		"Template Chunk Config Doc",
		content,
		KnowledgeDocumentOptions{},
	); err != nil {
		t.Fatalf("update document with template chunks: %v", err)
	}

	if store.deletedDocID != "doc_template_chunk_config" {
		t.Fatalf("expected update to target doc_template_chunk_config, got %q", store.deletedDocID)
	}
	if len(store.createdChunks) != 2 {
		t.Fatalf("expected template chunking to split updated content into 2 sections, got %d: %+v", len(store.createdChunks), store.createdChunks)
	}
	if first := store.createdChunks[0].Content; !strings.Contains(first, "# Existing Behavior") || !strings.Contains(first, "Existing behavior stays") {
		t.Fatalf("expected first updated template chunk to keep heading with body, got %q", first)
	}
	if second := store.createdChunks[1].Content; !strings.Contains(second, "## Updated Behavior") || !strings.Contains(second, "Updated behavior stays") {
		t.Fatalf("expected second updated template chunk to keep heading with body, got %q", second)
	}
}

func TestUpdateDocumentIncrementalUpsertsOnlyChangedChunksToQdrantVectorStore(t *testing.T) {
	store := &fakeStore{
		detailBase: KnowledgeBase{
			ChunkSize:     12,
			ChunkStrategy: KnowledgeChunkStrategyFixedSize,
			ID:            "kb_incremental_qdrant",
			Name:          "Incremental Qdrant KB",
		},
		diffChunks: []KnowledgeDocumentChunk{
			{ChunkIndex: 1, Content: "changed chunk", DocumentVersion: "v1", Embedding: []float32{0.4, 0.5, 0.6}},
		},
		updatedDoc: KnowledgeDocument{ID: "doc_incremental", Title: "Incremental Doc", UpdateStrategy: KnowledgeUpdateStrategyIncremental},
	}
	embedder := &recordingKnowledgeEmbedder{}
	vectorStore := &recordingKnowledgeVectorStore{}
	service := NewServiceWithEmbedderAndVectorStore(store, embedder, "text-embedding-3-small", vectorStore, 3)

	if _, err := service.UpdateDocumentWithOptions(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"},
		"kb_incremental_qdrant",
		"doc_incremental",
		"Incremental Doc",
		"unchanged chunk changed chunk",
		KnowledgeDocumentOptions{DocumentVersion: "v1", UpdateStrategy: KnowledgeUpdateStrategyIncremental},
	); err != nil {
		t.Fatalf("update document with incremental qdrant chunks: %v", err)
	}

	if store.diffOptions.UpdateStrategy != KnowledgeUpdateStrategyIncremental {
		t.Fatalf("expected diff options to use incremental strategy, got %+v", store.diffOptions)
	}
	if len(vectorStore.chunks) != 1 {
		t.Fatalf("expected only changed chunks to be upserted to qdrant, got %+v", vectorStore.chunks)
	}
	if vectorStore.chunks[0].ChunkIndex != 1 || vectorStore.chunks[0].Content != "changed chunk" {
		t.Fatalf("expected qdrant upsert to receive changed chunk index 1, got %+v", vectorStore.chunks[0])
	}
}

func TestCreateDocumentPassesTrustedRelayIdentityToEmbedder(t *testing.T) {
	store := &fakeStore{createdDoc: KnowledgeDocument{ID: "doc_identity", Title: "Identity Doc"}}
	embedder := &recordingKnowledgeEmbedder{}
	service := NewServiceWithEmbedder(store, embedder, "text-embedding-3-small")

	_, err := service.CreateDocumentWithOptions(
		context.Background(),
		auth.Session{
			OrganizationID: "org_knowledge",
			User:           auth.User{ID: "user_knowledge"},
			WorkspaceID:    "workspace_knowledge",
		},
		"kb_identity",
		"Identity Doc",
		"identity bearing document content",
		KnowledgeDocumentOptions{},
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	if embedder.batchUserID != "user_knowledge" {
		t.Fatalf("expected trusted user user_knowledge, got %q", embedder.batchUserID)
	}
	if embedder.batchOrganizationID != "org_knowledge" {
		t.Fatalf("expected trusted organization org_knowledge, got %q", embedder.batchOrganizationID)
	}
}

func TestCreateDocumentUpsertsEmbeddedChunksToQdrantVectorStore(t *testing.T) {
	store := &fakeStore{createdDoc: KnowledgeDocument{ID: "doc_qdrant", Title: "Qdrant Doc"}}
	embedder := &recordingKnowledgeEmbedder{}
	vectorStore := &recordingKnowledgeVectorStore{}
	service := NewServiceWithEmbedderAndVectorStore(store, embedder, "text-embedding-3-small", vectorStore, 3)

	document, err := service.CreateDocumentWithOptions(
		context.Background(),
		auth.Session{OrganizationID: "org_knowledge", WorkspaceID: "workspace_knowledge"},
		"kb_qdrant",
		"Qdrant Doc",
		"qdrant indexed document content",
		KnowledgeDocumentOptions{DocumentVersion: "v2", SourceURL: "https://docs.example/qdrant.md", PageNumber: 4},
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	if document.ID != "doc_qdrant" {
		t.Fatalf("expected created document id doc_qdrant, got %q", document.ID)
	}
	if vectorStore.organizationID != "org_knowledge" || vectorStore.knowledgeBaseID != "kb_qdrant" || vectorStore.documentID != "doc_qdrant" {
		t.Fatalf("expected tenant-scoped qdrant upsert, got org=%q kb=%q doc=%q", vectorStore.organizationID, vectorStore.knowledgeBaseID, vectorStore.documentID)
	}
	if len(vectorStore.chunks) == 0 {
		t.Fatal("expected qdrant upsert to receive generated chunks")
	}
	if len(vectorStore.chunks[0].Embedding) != 3 {
		t.Fatalf("expected qdrant chunk to include embedding, got %+v", vectorStore.chunks[0])
	}
	if vectorStore.chunks[0].Metadata.SourceURL != "https://docs.example/qdrant.md" || vectorStore.chunks[0].Metadata.PageNumber != 4 {
		t.Fatalf("expected qdrant chunk metadata to preserve source/page, got %+v", vectorStore.chunks[0].Metadata)
	}
}

func TestUpdateDocumentChunkReindexesEditedChunkInQdrantVectorStore(t *testing.T) {
	store := &fakeStore{
		updatedChunk: KnowledgeDocumentChunkView{
			ChunkID:         "kdc_1",
			ChunkIndex:      2,
			Content:         "Updated chunk content.",
			DocumentVersion: "v2",
			Metadata: KnowledgeChunkMetadata{
				DocumentVersion: "v2",
				PageNumber:      7,
				SourceURL:       "https://docs.example/runbook.md",
			},
		},
	}
	embedder := &recordingKnowledgeEmbedder{}
	vectorStore := &recordingKnowledgeVectorStore{}
	service := NewServiceWithEmbedderAndVectorStore(store, embedder, "text-embedding-3-small", vectorStore, 3)

	chunk, err := service.UpdateDocumentChunk(
		context.Background(),
		auth.Session{
			OrganizationID: "org_knowledge",
			User:           auth.User{ID: "user_knowledge"},
			WorkspaceID:    "workspace_knowledge",
		},
		"kb_qdrant",
		"doc_qdrant",
		"kdc_1",
		" Updated chunk content. ",
	)
	if err != nil {
		t.Fatalf("update document chunk: %v", err)
	}

	if chunk.ChunkID != "kdc_1" || chunk.Content != "Updated chunk content." {
		t.Fatalf("expected updated chunk response, got %+v", chunk)
	}
	if store.workspaceID != "org_knowledge" || store.requestedID != "kb_qdrant" || store.deletedDocID != "doc_qdrant" || store.updatedChunkID != "kdc_1" {
		t.Fatalf("expected tenant-scoped chunk update, scope=%q kb=%q doc=%q chunk=%q", store.workspaceID, store.requestedID, store.deletedDocID, store.updatedChunkID)
	}
	if store.requestedDoc.Content != "Updated chunk content." {
		t.Fatalf("expected store to receive trimmed chunk content, got %q", store.requestedDoc.Content)
	}
	if embedder.organizationID != "org_knowledge" || embedder.userID != "user_knowledge" {
		t.Fatalf("expected trusted relay identity for edited chunk embedding, org=%q user=%q", embedder.organizationID, embedder.userID)
	}
	if vectorStore.organizationID != "org_knowledge" || vectorStore.knowledgeBaseID != "kb_qdrant" || vectorStore.documentID != "doc_qdrant" {
		t.Fatalf("expected tenant-scoped qdrant chunk upsert, org=%q kb=%q doc=%q", vectorStore.organizationID, vectorStore.knowledgeBaseID, vectorStore.documentID)
	}
	if len(vectorStore.chunks) != 1 {
		t.Fatalf("expected one edited chunk to be upserted, got %+v", vectorStore.chunks)
	}
	upserted := vectorStore.chunks[0]
	if upserted.ChunkIndex != 2 || upserted.Content != "Updated chunk content." || upserted.DocumentVersion != "v2" {
		t.Fatalf("expected edited chunk index/content/version in qdrant upsert, got %+v", upserted)
	}
	if len(upserted.Embedding) != 3 {
		t.Fatalf("expected edited chunk embedding, got %+v", upserted)
	}
	if upserted.Metadata.SourceURL != "https://docs.example/runbook.md" || upserted.Metadata.PageNumber != 7 || upserted.Metadata.DocumentVersion != "v2" {
		t.Fatalf("expected edited chunk metadata to be preserved, got %+v", upserted.Metadata)
	}
}

func TestSplitDocumentChunkReindexesDocumentChunksInQdrantVectorStore(t *testing.T) {
	store := &fakeStore{
		splitChunks: []KnowledgeDocumentChunkView{
			{
				ChunkID:         "kdc_left",
				ChunkIndex:      0,
				Content:         "First half.",
				DocumentVersion: "v2",
				Metadata:        KnowledgeChunkMetadata{DocumentVersion: "v2", SourceURL: "https://docs.example/runbook.md"},
			},
			{
				ChunkID:         "kdc_right",
				ChunkIndex:      1,
				Content:         "Second half.",
				DocumentVersion: "v2",
				Metadata:        KnowledgeChunkMetadata{DocumentVersion: "v2", SourceURL: "https://docs.example/runbook.md"},
			},
		},
	}
	embedder := &recordingKnowledgeEmbedder{}
	vectorStore := &recordingKnowledgeVectorStore{}
	service := NewServiceWithEmbedderAndVectorStore(store, embedder, "text-embedding-3-small", vectorStore, 3)

	chunks, err := service.SplitDocumentChunk(
		context.Background(),
		auth.Session{OrganizationID: "org_knowledge", User: auth.User{ID: "user_knowledge"}, WorkspaceID: "workspace_knowledge"},
		"kb_qdrant",
		"doc_qdrant",
		"kdc_original",
		11,
	)
	if err != nil {
		t.Fatalf("split document chunk: %v", err)
	}

	if len(chunks) != 2 || chunks[0].Content != "First half." || chunks[1].Content != "Second half." {
		t.Fatalf("expected split chunks in response, got %+v", chunks)
	}
	if store.workspaceID != "org_knowledge" || store.requestedID != "kb_qdrant" || store.deletedDocID != "doc_qdrant" || store.updatedChunkID != "kdc_original" || store.splitAt != 11 {
		t.Fatalf("expected tenant-scoped split, scope=%q kb=%q doc=%q chunk=%q splitAt=%d", store.workspaceID, store.requestedID, store.deletedDocID, store.updatedChunkID, store.splitAt)
	}
	if vectorStore.deletedDocumentID != "doc_qdrant" {
		t.Fatalf("expected qdrant document points to be deleted before reindex, got %q", vectorStore.deletedDocumentID)
	}
	if vectorStore.organizationID != "org_knowledge" || vectorStore.knowledgeBaseID != "kb_qdrant" || vectorStore.documentID != "doc_qdrant" {
		t.Fatalf("expected tenant-scoped qdrant reindex, org=%q kb=%q doc=%q", vectorStore.organizationID, vectorStore.knowledgeBaseID, vectorStore.documentID)
	}
	if len(vectorStore.chunks) != 2 {
		t.Fatalf("expected all split chunks to be upserted, got %+v", vectorStore.chunks)
	}
	if embedder.batchOrganizationID != "org_knowledge" || embedder.batchUserID != "user_knowledge" {
		t.Fatalf("expected trusted relay identity for split chunk embeddings, org=%q user=%q", embedder.batchOrganizationID, embedder.batchUserID)
	}
	if vectorStore.chunks[0].ChunkIndex != 0 || vectorStore.chunks[1].ChunkIndex != 1 || len(vectorStore.chunks[0].Embedding) != 3 || len(vectorStore.chunks[1].Embedding) != 3 {
		t.Fatalf("expected indexed embedded split chunks, got %+v", vectorStore.chunks)
	}
}

func TestRetrieveReturnsRelevantDocumentSnippets(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{
			{
				DocumentID:    "doc_9",
				DocumentTitle: "Architecture Draft",
				Snippet:       "Initial architecture draft covers deployment boundaries.",
			},
		},
	}
	service := NewService(store)

	results, err := service.Retrieve(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7", "deployment")
	if err != nil {
		t.Fatalf("retrieve knowledge: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.requestedID != "kb_7" {
		t.Fatalf("expected requested id kb_7, got %s", store.requestedID)
	}
	if store.retrievalQuery != "deployment" {
		t.Fatalf("expected retrieval query deployment, got %s", store.retrievalQuery)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Snippet != "Initial architecture draft covers deployment boundaries." {
		t.Fatalf("unexpected snippet %q", results[0].Snippet)
	}
}

func TestRetrieveNormalizesKnowledgeQueryBeforeCallingStore(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{},
	}
	service := NewService(store)

	if _, err := service.Retrieve(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7", "  deployment   rollback  "); err != nil {
		t.Fatalf("retrieve knowledge: %v", err)
	}

	if store.retrievalQuery != "deployment rollback" {
		t.Fatalf("expected normalized query %q, got %q", "deployment rollback", store.retrievalQuery)
	}
}

func TestRetrieveBackfillsLegacyRAGRetrievalMethod(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{
			{
				DocumentID:    "doc_1",
				DocumentTitle: "Plan",
				Snippet:       "Deployment rollback plan.",
				Source: KnowledgeCitation{
					DocumentID:    "doc_1",
					DocumentTitle: "Plan",
					ChunkID:       "chunk_1",
					ChunkIndex:    2,
				},
			},
		},
	}
	service := NewService(store)

	results, err := service.Retrieve(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_1", "deployment")
	if err != nil {
		t.Fatalf("retrieve knowledge: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].RetrievalMethod != KnowledgeRetrievalMethodEmbeddingRAG {
		t.Fatalf("expected legacy RAG retrieval method %q, got %q", KnowledgeRetrievalMethodEmbeddingRAG, results[0].RetrievalMethod)
	}
	if results[0].Source.ChunkID != "chunk_1" || results[0].Source.ChunkIndex != 2 {
		t.Fatalf("expected Source citation to be preserved, got %+v", results[0].Source)
	}
}

func TestKnowledgeRetrievalMethodEmbeddingRAGContract(t *testing.T) {
	if KnowledgeRetrievalMethodEmbeddingRAG != "embedding_rag" {
		t.Fatalf("expected embedding_rag retrieval method contract, got %q", KnowledgeRetrievalMethodEmbeddingRAG)
	}
}

func TestRetrieveWithOptionsRecordsRAGRetrievalLatencyMetric(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{
			{DocumentID: "doc_1", DocumentTitle: "Doc", Snippet: "hybrid answer"},
		},
	}
	service := NewService(store)
	mode := KnowledgeRetrievalModeHybridRerank
	before := histogramSampleCount(t, metrics.RAGRetrievalLatencySeconds.WithLabelValues(mode))

	results, err := service.RetrieveWithOptions(context.Background(), auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"}, "kb_1", "  hybrid   answer  ", KnowledgeRetrievalOptions{Mode: mode})
	if err != nil {
		t.Fatalf("retrieve with options: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if store.retrievalQuery != "hybrid answer" {
		t.Fatalf("expected normalized query %q, got %q", "hybrid answer", store.retrievalQuery)
	}
	if count := histogramSampleCount(t, metrics.RAGRetrievalLatencySeconds.WithLabelValues(mode)); count <= before {
		t.Fatalf("expected RAG retrieval latency histogram sample, before=%v after=%v", before, count)
	}
}

func TestRetrieveWithOptionsUsesQdrantSearchForVectorOnlyMode(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{
			{ChunkID: "sql_chunk", DocumentID: "doc_sql", DocumentTitle: "SQL Doc", Snippet: "sql fallback"},
		},
	}
	embedder := &recordingKnowledgeEmbedder{}
	vectorStore := &recordingKnowledgeVectorStore{
		searchResults: []KnowledgeRetrievalResult{
			{ChunkID: "qdrant_chunk", DocumentID: "doc_qdrant", DocumentTitle: "Qdrant Doc", Snippet: "qdrant answer"},
		},
	}
	service := NewServiceWithEmbedderAndVectorStore(store, embedder, "text-embedding-3-small", vectorStore, 3)

	results, err := service.RetrieveWithOptions(
		context.Background(),
		auth.Session{OrganizationID: "org_knowledge", WorkspaceID: "workspace_knowledge"},
		"kb_qdrant",
		"  qdrant   answer  ",
		KnowledgeRetrievalOptions{Mode: KnowledgeRetrievalModeVector, Limit: 3, MinScore: 0.42},
	)
	if err != nil {
		t.Fatalf("retrieve with qdrant vector search: %v", err)
	}

	if vectorStore.organizationID != "org_knowledge" || vectorStore.knowledgeBaseID != "kb_qdrant" {
		t.Fatalf("expected tenant-scoped qdrant search, got org=%q kb=%q", vectorStore.organizationID, vectorStore.knowledgeBaseID)
	}
	if vectorStore.searchQuery != "qdrant answer" {
		t.Fatalf("expected normalized qdrant query, got %q", vectorStore.searchQuery)
	}
	if len(vectorStore.searchEmbedding) != 3 {
		t.Fatalf("expected qdrant search embedding, got %+v", vectorStore.searchEmbedding)
	}
	if vectorStore.searchOptions.Mode != KnowledgeRetrievalModeVector || vectorStore.searchOptions.Limit != 3 || vectorStore.searchOptions.MinScore != 0.42 {
		t.Fatalf("expected qdrant search options to match vector request, got %+v", vectorStore.searchOptions)
	}
	if store.retrievalQuery != "" {
		t.Fatalf("expected vector_only qdrant search to bypass SQL retrieval, got store query %q", store.retrievalQuery)
	}
	if len(results) != 1 || results[0].ChunkID != "qdrant_chunk" || results[0].RetrievalMode != KnowledgeRetrievalModeVector {
		t.Fatalf("expected qdrant vector result with vector mode, got %+v", results)
	}
}

func TestRetrieveWithOptionsFusesQdrantVectorAndKeywordResultsForHybridMode(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{
			{ChunkID: "keyword_chunk", DocumentID: "doc_keyword", DocumentTitle: "Keyword Doc", Snippet: "keyword match"},
			{ChunkID: "shared_chunk", DocumentID: "doc_shared", DocumentTitle: "Shared Doc", Snippet: "keyword duplicate"},
		},
	}
	embedder := &recordingKnowledgeEmbedder{}
	vectorStore := &recordingKnowledgeVectorStore{
		searchResults: []KnowledgeRetrievalResult{
			{ChunkID: "vector_chunk", DocumentID: "doc_vector", DocumentTitle: "Vector Doc", Snippet: "semantic match"},
			{ChunkID: "shared_chunk", DocumentID: "doc_shared", DocumentTitle: "Shared Doc", Snippet: "vector duplicate"},
		},
	}
	service := NewServiceWithEmbedderAndVectorStore(store, embedder, "text-embedding-3-small", vectorStore, 3)

	results, err := service.RetrieveWithOptions(
		context.Background(),
		auth.Session{OrganizationID: "org_knowledge", WorkspaceID: "workspace_knowledge"},
		"kb_qdrant",
		"hybrid answer",
		KnowledgeRetrievalOptions{Mode: KnowledgeRetrievalModeHybrid, Limit: 3, MinScore: 0.31, VectorWeight: 0.8, KeywordWeight: 0.2},
	)
	if err != nil {
		t.Fatalf("retrieve hybrid with qdrant vector candidates: %v", err)
	}

	if vectorStore.searchQuery != "hybrid answer" || len(vectorStore.searchEmbedding) != 3 {
		t.Fatalf("expected qdrant vector candidate search, query=%q embedding=%+v", vectorStore.searchQuery, vectorStore.searchEmbedding)
	}
	if vectorStore.searchOptions.Mode != KnowledgeRetrievalModeVector || vectorStore.searchOptions.MinScore != 0.31 {
		t.Fatalf("expected qdrant vector candidate options, got %+v", vectorStore.searchOptions)
	}
	if store.retrievalQuery != "hybrid answer" || store.retrievalOptions.Mode != KnowledgeRetrievalModeKeyword {
		t.Fatalf("expected keyword-only SQL candidate retrieval, query=%q options=%+v", store.retrievalQuery, store.retrievalOptions)
	}
	if len(results) != 3 {
		t.Fatalf("expected fused vector and keyword candidates with duplicate chunk collapsed, got %+v", results)
	}
	if results[0].ChunkID != "shared_chunk" || results[1].ChunkID != "vector_chunk" || results[2].ChunkID != "keyword_chunk" {
		t.Fatalf("expected weighted RRF order shared/vector/keyword, got %+v", results)
	}
	for _, result := range results {
		if result.RetrievalMode != KnowledgeRetrievalModeHybrid {
			t.Fatalf("expected fused result to keep hybrid mode, got %+v", result)
		}
	}
}

func TestRetrieveWithOptionsReranksQdrantHybridCandidatesForHybridRerankMode(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{
			{ChunkID: "keyword_chunk", DocumentID: "doc_keyword", DocumentTitle: "Keyword Doc", Snippet: "keyword match"},
		},
	}
	embedder := &recordingKnowledgeEmbedder{}
	vectorStore := &recordingKnowledgeVectorStore{
		searchResults: []KnowledgeRetrievalResult{
			{ChunkID: "vector_chunk", DocumentID: "doc_vector", DocumentTitle: "Vector Doc", Snippet: "semantic match"},
		},
	}
	reranker := &recordingKnowledgeReranker{}
	service := NewServiceWithEmbedderAndVectorStore(store, embedder, "text-embedding-3-small", vectorStore, 3).WithReranker(reranker)

	results, err := service.RetrieveWithOptions(
		context.Background(),
		auth.Session{OrganizationID: "org_knowledge", WorkspaceID: "workspace_knowledge"},
		"kb_qdrant",
		"hybrid rerank answer",
		KnowledgeRetrievalOptions{Mode: KnowledgeRetrievalModeHybridRerank, Limit: 1, RerankTopK: 2},
	)
	if err != nil {
		t.Fatalf("retrieve hybrid_rerank with qdrant candidates: %v", err)
	}

	if vectorStore.searchOptions.Mode != KnowledgeRetrievalModeVector || vectorStore.searchOptions.Limit != 2 {
		t.Fatalf("expected qdrant vector candidate search to use expanded candidate limit, got %+v", vectorStore.searchOptions)
	}
	if store.retrievalOptions.Mode != KnowledgeRetrievalModeKeyword || store.retrievalOptions.Limit != 2 {
		t.Fatalf("expected keyword candidate retrieval to use expanded candidate limit, got %+v", store.retrievalOptions)
	}
	if len(reranker.results) != 2 {
		t.Fatalf("expected reranker to receive fused qdrant+keyword candidates, got %+v", reranker.results)
	}
	if reranker.results[0].ChunkID != "vector_chunk" || reranker.results[1].ChunkID != "keyword_chunk" {
		t.Fatalf("expected reranker input order vector/keyword from fused candidates, got %+v", reranker.results)
	}
	if len(results) != 1 || results[0].RetrievalMode != KnowledgeRetrievalModeHybridRerank {
		t.Fatalf("expected final hybrid_rerank result, got %+v", results)
	}
}

func TestRetrieveWithOptionsReranksHybridRerankResults(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{
			{ChunkID: "chunk_a", DocumentID: "doc_a", DocumentTitle: "Alpha", Snippet: "less relevant", Score: 0.7},
			{ChunkID: "chunk_b", DocumentID: "doc_b", DocumentTitle: "Beta", Snippet: "best answer", Score: 0.6},
		},
	}
	reranker := &recordingKnowledgeReranker{}
	service := NewServiceWithReranker(store, reranker)

	results, err := service.RetrieveWithOptions(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"},
		"kb_1",
		"  best   answer  ",
		KnowledgeRetrievalOptions{Mode: KnowledgeRetrievalModeHybridRerank, Limit: 1},
	)
	if err != nil {
		t.Fatalf("retrieve with rerank: %v", err)
	}

	if reranker.query != "best answer" {
		t.Fatalf("expected reranker query %q, got %q", "best answer", reranker.query)
	}
	if reranker.limit != 1 {
		t.Fatalf("expected reranker limit 1, got %d", reranker.limit)
	}
	if len(reranker.results) != 2 {
		t.Fatalf("expected reranker to receive two candidates, got %+v", reranker.results)
	}
	if len(results) != 1 || results[0].ChunkID != "chunk_b" {
		t.Fatalf("expected reranked top chunk_b, got %+v", results)
	}
	if results[0].RetrievalMethod != KnowledgeRetrievalModeHybridRerank || results[0].RetrievalMode != KnowledgeRetrievalModeHybridRerank {
		t.Fatalf("expected hybrid_rerank method/mode, got %+v", results[0])
	}
}

func TestRetrieveWithOptionsExpandsHybridRerankCandidatePool(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{
			{ChunkID: "chunk_a", DocumentID: "doc_a", DocumentTitle: "Alpha", Snippet: "less relevant", Score: 0.7},
			{ChunkID: "chunk_b", DocumentID: "doc_b", DocumentTitle: "Beta", Snippet: "best answer", Score: 0.6},
			{ChunkID: "chunk_c", DocumentID: "doc_c", DocumentTitle: "Gamma", Snippet: "fallback answer", Score: 0.5},
		},
	}
	reranker := &recordingKnowledgeReranker{}
	service := NewServiceWithReranker(store, reranker)

	results, err := service.RetrieveWithOptions(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"},
		"kb_1",
		"best answer",
		KnowledgeRetrievalOptions{Mode: KnowledgeRetrievalModeHybridRerank, Limit: 1, RerankTopK: 3},
	)
	if err != nil {
		t.Fatalf("retrieve with rerank: %v", err)
	}

	if store.retrievalOptions.Limit != 3 {
		t.Fatalf("expected store candidate limit 3, got %+v", store.retrievalOptions)
	}
	if reranker.limit != 1 {
		t.Fatalf("expected final reranker limit 1, got %d", reranker.limit)
	}
	if len(reranker.results) != 3 {
		t.Fatalf("expected reranker to receive expanded candidate pool, got %+v", reranker.results)
	}
	if len(results) != 1 {
		t.Fatalf("expected final result limit 1, got %+v", results)
	}
}

func TestRetrieveWithOptionsFallsBackWhenHybridRerankerFails(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{
			{ChunkID: "chunk_a", DocumentID: "doc_a", DocumentTitle: "Alpha", Snippet: "original first", Score: 0.7},
			{ChunkID: "chunk_b", DocumentID: "doc_b", DocumentTitle: "Beta", Snippet: "original second", Score: 0.6},
		},
	}
	reranker := &recordingKnowledgeReranker{err: errors.New("reranker unavailable")}
	service := NewServiceWithReranker(store, reranker)
	beforeFallbacks := testutil.ToFloat64(metrics.RAGRerankerFallbackTotal.WithLabelValues(KnowledgeRetrievalModeHybridRerank))

	results, err := service.RetrieveWithOptions(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_1"},
		"kb_1",
		"fallback answer",
		KnowledgeRetrievalOptions{Mode: KnowledgeRetrievalModeHybridRerank, Limit: 1, RerankTopK: 2},
	)
	if err != nil {
		t.Fatalf("expected reranker outage to fall back to original retrieval results, got error: %v", err)
	}

	if len(reranker.results) != 2 {
		t.Fatalf("expected reranker to receive candidate pool before fallback, got %+v", reranker.results)
	}
	if len(results) != 1 || results[0].ChunkID != "chunk_a" {
		t.Fatalf("expected fallback to preserve original order with final limit, got %+v", results)
	}
	if results[0].RetrievalMode != KnowledgeRetrievalModeHybridRerank || results[0].RetrievalMethod != KnowledgeRetrievalMethodEmbeddingRAG {
		t.Fatalf("expected fallback result to keep requested mode while preserving original retrieval method, got %+v", results[0])
	}
	afterFallbacks := testutil.ToFloat64(metrics.RAGRerankerFallbackTotal.WithLabelValues(KnowledgeRetrievalModeHybridRerank))
	if afterFallbacks != beforeFallbacks+1 {
		t.Fatalf("expected reranker fallback counter increment, before=%v after=%v", beforeFallbacks, afterFallbacks)
	}
}

func TestRetrievePassesTrustedRelayIdentityToEmbedder(t *testing.T) {
	store := &fakeStore{
		retrievalResults: []KnowledgeRetrievalResult{
			{DocumentID: "doc_1", DocumentTitle: "Doc", Snippet: "answer"},
		},
	}
	embedder := &recordingKnowledgeEmbedder{}
	service := NewServiceWithEmbedder(store, embedder, "text-embedding-3-small")

	_, err := service.RetrieveWithOptions(
		context.Background(),
		auth.Session{
			OrganizationID: "org_knowledge",
			User:           auth.User{ID: "user_knowledge"},
			WorkspaceID:    "workspace_knowledge",
		},
		"kb_identity",
		"answer",
		KnowledgeRetrievalOptions{Mode: KnowledgeRetrievalModeHybrid},
	)
	if err != nil {
		t.Fatalf("retrieve with options: %v", err)
	}

	if embedder.userID != "user_knowledge" {
		t.Fatalf("expected trusted user user_knowledge, got %q", embedder.userID)
	}
	if embedder.organizationID != "org_knowledge" {
		t.Fatalf("expected trusted organization org_knowledge, got %q", embedder.organizationID)
	}
}

func histogramSampleCount(t *testing.T, observer prometheus.Observer) uint64 {
	t.Helper()
	metric, ok := observer.(prometheus.Metric)
	if !ok {
		t.Fatalf("observer %T does not expose prometheus metric samples", observer)
	}
	dtoMetric := &dto.Metric{}
	if err := metric.Write(dtoMetric); err != nil {
		t.Fatalf("write histogram metric: %v", err)
	}
	if dtoMetric.Histogram == nil {
		t.Fatalf("metric %T is not a histogram", observer)
	}
	return dtoMetric.Histogram.GetSampleCount()
}

func TestUpdateUpdatesKnowledgeBaseInWorkspace(t *testing.T) {
	store := &fakeStore{
		updatedBase: KnowledgeBase{
			DocumentCount: 1,
			ID:            "kb_7",
			Name:          "Architecture Decisions",
			UpdatedAt:     time.Date(2026, time.April, 3, 13, 0, 0, 0, time.UTC),
		},
	}
	service := NewService(store)

	base, err := service.Update(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7", "Architecture Decisions")
	if err != nil {
		t.Fatalf("update knowledge base: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
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

	if err := service.Delete(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7"); err != nil {
		t.Fatalf("delete knowledge base: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.deletedID != "kb_7" {
		t.Fatalf("expected deleted id kb_7, got %s", store.deletedID)
	}
}

func TestDeleteRemovesTenantScopedQdrantCollection(t *testing.T) {
	store := &fakeStore{}
	vectorStore := &recordingKnowledgeVectorStore{}
	service := NewServiceWithVectorStore(store, vectorStore, 1536)

	if err := service.Delete(context.Background(), auth.Session{WorkspaceID: "workspace_1", OrganizationID: "org_acme"}, "kb_product_docs"); err != nil {
		t.Fatalf("delete knowledge base with vector store: %v", err)
	}

	if store.deletedID != "kb_product_docs" {
		t.Fatalf("expected store delete for kb_product_docs, got %q", store.deletedID)
	}
	if vectorStore.deletedCollection != "kb_org_acme_kb_product_docs" {
		t.Fatalf("expected qdrant collection cleanup, got %q", vectorStore.deletedCollection)
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
	service := NewService(store)

	document, err := service.UpdateDocument(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1"},
		"kb_7",
		"doc_9",
		"Architecture Draft v2",
		"Updated outline",
	)
	if err != nil {
		t.Fatalf("update knowledge document: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.deletedDocID != "doc_9" {
		t.Fatalf("expected requested doc id doc_9, got %s", store.deletedDocID)
	}
	if document.Title != "Architecture Draft v2" {
		t.Fatalf("expected Architecture Draft v2, got %s", document.Title)
	}
}

func TestDeleteDocumentDeletesKnowledgeBaseDocument(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)

	if err := service.DeleteDocument(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7", "doc_9"); err != nil {
		t.Fatalf("delete knowledge document: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.deletedDocID != "doc_9" {
		t.Fatalf("expected deleted doc id doc_9, got %s", store.deletedDocID)
	}
}
