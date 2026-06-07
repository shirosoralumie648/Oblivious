package knowledge

import (
	"context"
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
	deletedDocID     string
	deletedID        string
	detailBase       KnowledgeBase
	documents        []KnowledgeDocument
	listBases        []KnowledgeBase
	retrievalQuery   string
	retrievalOptions KnowledgeRetrievalOptions
	retrievalResults []KnowledgeRetrievalResult
	requestedDoc     KnowledgeDocument
	requestedID      string
	updatedBase      KnowledgeBase
	updatedDoc       KnowledgeDocument
	workspaceID      string
}

type recordingKnowledgeVectorStore struct {
	deletedCollection string
	ensuredCollection string
	ensuredVectorSize int
	organizationID    string
	knowledgeBaseID   string
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

type recordingKnowledgeEmbedder struct {
	batchOrganizationID string
	batchUserID         string
	organizationID      string
	userID              string
}

type recordingKnowledgeReranker struct {
	limit   int
	query   string
	results []KnowledgeRetrievalResult
}

func (r *recordingKnowledgeReranker) Rerank(ctx context.Context, query string, results []KnowledgeRetrievalResult, limit int) ([]KnowledgeRetrievalResult, error) {
	r.query = query
	r.limit = limit
	r.results = append([]KnowledgeRetrievalResult(nil), results...)
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
