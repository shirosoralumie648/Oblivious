package knowledge

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/metrics"
	relaytypes "oblivious/server/internal/relay/types"
)

var (
	ErrEmptyKnowledgeDocumentChunk       = errors.New("empty knowledge document chunk")
	ErrInvalidKnowledgeRetrievalOptions  = errors.New("invalid knowledge retrieval options")
	ErrInvalidKnowledgeRetrievalTestCase = errors.New("invalid knowledge retrieval test case")
)

const KnowledgeRetrievalMethodEmbeddingRAG = "embedding_rag"

type KnowledgeBase struct {
	ChunkOverlap   int       `json:"chunkOverlap,omitempty"`
	ChunkSize      int       `json:"chunkSize,omitempty"`
	ChunkStrategy  string    `json:"chunkStrategy,omitempty"`
	DocumentCount  int       `json:"documentCount"`
	EmbeddingModel string    `json:"embeddingModel,omitempty"`
	ID             string    `json:"id"`
	KeywordWeight  float64   `json:"keywordWeight,omitempty"`
	MinScore       float64   `json:"minScore,omitempty"`
	Name           string    `json:"name"`
	RerankTopK     int       `json:"rerankTopK,omitempty"`
	RerankerModel  string    `json:"rerankerModel,omitempty"`
	RetrievalLimit int       `json:"retrievalLimit,omitempty"`
	RetrievalMode  string    `json:"retrievalMode,omitempty"`
	UpdateStrategy string    `json:"updateStrategy,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
	VectorWeight   float64   `json:"vectorWeight,omitempty"`
}

type KnowledgeDocument struct {
	Content         string    `json:"content"`
	DocumentVersion string    `json:"documentVersion,omitempty"`
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	UpdateStrategy  string    `json:"updateStrategy,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type KnowledgeRetrievalResult struct {
	ChunkID         string            `json:"chunkId,omitempty"`
	ChunkIndex      int               `json:"chunkIndex,omitempty"`
	DocumentID      string            `json:"documentId"`
	DocumentTitle   string            `json:"documentTitle"`
	DocumentVersion string            `json:"documentVersion,omitempty"`
	RetrievalMethod string            `json:"retrievalMethod,omitempty"`
	RetrievalMode   string            `json:"retrievalMode,omitempty"`
	Score           float64           `json:"score,omitempty"`
	Similarity      float64           `json:"similarity,omitempty"`
	Snippet         string            `json:"snippet"`
	Source          KnowledgeCitation `json:"source,omitempty"`
}

type Store interface {
	CreateKnowledgeBase(ctx context.Context, workspaceID, name string) (KnowledgeBase, error)
	CreateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, title, content string) (KnowledgeDocument, error)
	DeleteKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) error
	DeleteKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, documentID string) error
	GetKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) (KnowledgeBase, error)
	ListKnowledgeDocuments(ctx context.Context, workspaceID, knowledgeBaseID string) ([]KnowledgeDocument, error)
	ListKnowledgeBases(ctx context.Context, workspaceID string) ([]KnowledgeBase, error)
	RetrieveKnowledge(ctx context.Context, workspaceID, knowledgeBaseID, query string) ([]KnowledgeRetrievalResult, error)
	UpdateKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID, name string) (KnowledgeBase, error)
	UpdateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, documentID, title, content string) (KnowledgeDocument, error)
}

type organizationBaseCreator interface {
	CreateKnowledgeBase(ctx context.Context, workspaceID, organizationID, name string) (KnowledgeBase, error)
}

type knowledgeBaseConfigCreator interface {
	CreateKnowledgeBaseWithConfig(ctx context.Context, workspaceID, organizationID, name string, config KnowledgeBaseConfig) (KnowledgeBase, error)
}

type knowledgeBaseConfigUpdater interface {
	UpdateKnowledgeBaseWithConfig(ctx context.Context, organizationID, knowledgeBaseID, name string, config KnowledgeBaseConfig) (KnowledgeBase, error)
}

type documentCreatorWithChunks interface {
	CreateKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, title, content string, chunks []KnowledgeDocumentChunk) (KnowledgeDocument, error)
}

type documentCreatorWithOptions interface {
	CreateKnowledgeDocumentWithOptions(ctx context.Context, organizationID, knowledgeBaseID, title, content string, chunks []KnowledgeDocumentChunk, options KnowledgeDocumentOptions) (KnowledgeDocument, error)
}

type documentUpdaterWithChunks interface {
	UpdateKnowledgeDocument(ctx context.Context, organizationID, knowledgeBaseID, documentID, title, content string, chunks []KnowledgeDocumentChunk) (KnowledgeDocument, error)
}

type documentUpdaterWithOptions interface {
	UpdateKnowledgeDocumentWithOptions(ctx context.Context, organizationID, knowledgeBaseID, documentID, title, content string, chunks []KnowledgeDocumentChunk, options KnowledgeDocumentOptions) (KnowledgeDocument, error)
}

type documentIDDeleter interface {
	DeleteKnowledgeDocumentByID(ctx context.Context, organizationID, documentID string) error
}

type documentChunkLister interface {
	ListKnowledgeDocumentChunks(ctx context.Context, organizationID, knowledgeBaseID, documentID string) ([]KnowledgeDocumentChunkView, error)
}

type documentChunkUpdater interface {
	UpdateKnowledgeDocumentChunk(ctx context.Context, organizationID, knowledgeBaseID, documentID, chunkID, content string) (KnowledgeDocumentChunkView, error)
}

type documentChunkDiffer interface {
	DiffKnowledgeDocumentChunks(ctx context.Context, organizationID, knowledgeBaseID, documentID string, chunks []KnowledgeDocumentChunk, options KnowledgeDocumentOptions) ([]KnowledgeDocumentChunk, error)
}

type knowledgeRetrieverWithOptions interface {
	RetrieveKnowledge(ctx context.Context, organizationID, knowledgeBaseID, query string, queryEmbedding []float32, options KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, error)
}

type knowledgeRetrieverNamedWithOptions interface {
	RetrieveKnowledgeWithOptions(ctx context.Context, organizationID, knowledgeBaseID, query string, queryEmbedding []float32, options KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, error)
}

type KnowledgeVectorStore interface {
	EnsureKnowledgeBaseCollection(ctx context.Context, organizationID, knowledgeBaseID string, vectorSize int) error
	DeleteKnowledgeBaseCollection(ctx context.Context, organizationID, knowledgeBaseID string) error
	SearchKnowledgeChunks(ctx context.Context, organizationID, knowledgeBaseID, query string, queryEmbedding []float32, options KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, error)
	UpsertKnowledgeDocumentChunks(ctx context.Context, organizationID, knowledgeBaseID, documentID string, chunks []KnowledgeDocumentChunk) error
}

type retrievalTestCaseCreator interface {
	CreateRetrievalTestCase(ctx context.Context, organizationID, knowledgeBaseID string, req CreateKnowledgeRetrievalTestCaseRequest) (KnowledgeRetrievalTestCase, error)
}

type retrievalTestCaseLister interface {
	ListRetrievalTestCases(ctx context.Context, organizationID, knowledgeBaseID string) ([]KnowledgeRetrievalTestCase, error)
}

type Service struct {
	store          any
	embedder       Embedder
	embeddingModel string
	reranker       KnowledgeReranker
	vectorSize     int
	vectorStore    KnowledgeVectorStore
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

type KnowledgeEmbedder = Embedder

type KnowledgeReranker interface {
	Rerank(ctx context.Context, query string, results []KnowledgeRetrievalResult, limit int) ([]KnowledgeRetrievalResult, error)
}

func NewService(store any) *Service {
	return &Service{store: store}
}

func NewServiceWithEmbedder(store any, embedder Embedder, embeddingModel string) *Service {
	return &Service{
		store:          store,
		embedder:       embedder,
		embeddingModel: strings.TrimSpace(embeddingModel),
	}
}

func NewServiceWithReranker(store any, reranker KnowledgeReranker) *Service {
	return NewService(store).WithReranker(reranker)
}

func NewServiceWithVectorStore(store any, vectorStore KnowledgeVectorStore, vectorSize int) *Service {
	return NewService(store).WithVectorStore(vectorStore, vectorSize)
}

func NewServiceWithEmbedderAndVectorStore(store any, embedder Embedder, embeddingModel string, vectorStore KnowledgeVectorStore, vectorSize int) *Service {
	return NewServiceWithEmbedder(store, embedder, embeddingModel).WithVectorStore(vectorStore, vectorSize)
}

func (s *Service) WithReranker(reranker KnowledgeReranker) *Service {
	if s == nil {
		return nil
	}
	s.reranker = reranker
	return s
}

func (s *Service) WithVectorStore(vectorStore KnowledgeVectorStore, vectorSize int) *Service {
	if s == nil {
		return nil
	}
	s.vectorStore = vectorStore
	if vectorSize > 0 {
		s.vectorSize = vectorSize
	}
	return s
}

func normalizeKnowledgeQuery(query string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
}

func knowledgeSessionScope(session auth.Session) string {
	if strings.TrimSpace(session.OrganizationID) != "" {
		return strings.TrimSpace(session.OrganizationID)
	}
	return strings.TrimSpace(session.WorkspaceID)
}

func KnowledgeVectorCollectionName(organizationID, knowledgeBaseID string) string {
	return "kb_" + sanitizeKnowledgeVectorScope(organizationID) + "_" + sanitizeKnowledgeVectorScope(knowledgeBaseID)
}

func sanitizeKnowledgeVectorScope(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	sanitized := strings.Trim(builder.String(), "_")
	if sanitized == "" {
		return "unknown"
	}
	return sanitized
}

func withKnowledgeRelayIdentity(ctx context.Context, session auth.Session) context.Context {
	if userID := strings.TrimSpace(session.User.ID); userID != "" {
		ctx = relaytypes.WithTrustedUserID(ctx, userID)
	}
	if organizationID := strings.TrimSpace(session.OrganizationID); organizationID != "" {
		ctx = relaytypes.WithTrustedOrganizationID(ctx, organizationID)
	}
	return ctx
}

func normalizeKnowledgeBaseConfig(config KnowledgeBaseConfig) KnowledgeBaseConfig {
	config.RetrievalMode = normalizeKnowledgeRetrievalMode(config.RetrievalMode)
	if config.RetrievalLimit <= 0 {
		config.RetrievalLimit = 5
	}
	if config.VectorWeight <= 0 {
		config.VectorWeight = 0.7
	}
	if config.KeywordWeight <= 0 {
		config.KeywordWeight = 0.3
	}
	if config.RerankTopK <= 0 {
		config.RerankTopK = 5
	}
	if config.ChunkStrategy == "" {
		config.ChunkStrategy = KnowledgeChunkStrategyTemplateBased
	}
	if config.ChunkSize <= 0 {
		config.ChunkSize = 500
	}
	if config.ChunkOverlap < 0 {
		config.ChunkOverlap = 0
	}
	if config.EmbeddingModel == "" {
		config.EmbeddingModel = "text-embedding-3-small"
	}
	if config.UpdateStrategy == "" {
		config.UpdateStrategy = KnowledgeUpdateStrategyFullReplace
	}
	return config
}

func normalizeKnowledgeDocumentOptions(options KnowledgeDocumentOptions) KnowledgeDocumentOptions {
	options.DocumentVersion = strings.TrimSpace(options.DocumentVersion)
	if options.DocumentVersion == "" {
		options.DocumentVersion = "v1"
	}
	options.SourceURL = strings.TrimSpace(options.SourceURL)
	options.UpdateStrategy = strings.TrimSpace(options.UpdateStrategy)
	if options.UpdateStrategy == "" {
		options.UpdateStrategy = KnowledgeUpdateStrategyFullReplace
	}
	if options.PageNumber < 0 {
		options.PageNumber = 0
	}
	return options
}

func normalizeKnowledgeRetrievalMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "", KnowledgeRetrievalModeHybrid:
		return KnowledgeRetrievalModeHybrid
	case KnowledgeRetrievalModeVector, "vector":
		return KnowledgeRetrievalModeVector
	case KnowledgeRetrievalModeKeyword:
		return KnowledgeRetrievalModeKeyword
	case KnowledgeRetrievalModeHybridRerank:
		return KnowledgeRetrievalModeHybridRerank
	default:
		return strings.TrimSpace(mode)
	}
}

func normalizeKnowledgeRetrievalOptions(options KnowledgeRetrievalOptions) (KnowledgeRetrievalOptions, error) {
	if strings.TrimSpace(options.Mode) == "" {
		options.Mode = KnowledgeRetrievalModeHybrid
	} else {
		options.Mode = normalizeKnowledgeRetrievalMode(options.Mode)
	}
	switch options.Mode {
	case KnowledgeRetrievalModeVector, KnowledgeRetrievalModeKeyword, KnowledgeRetrievalModeHybrid, KnowledgeRetrievalModeHybridRerank:
	default:
		return KnowledgeRetrievalOptions{}, ErrInvalidKnowledgeRetrievalOptions
	}
	if options.Limit < 0 {
		return KnowledgeRetrievalOptions{}, ErrInvalidKnowledgeRetrievalOptions
	}
	if options.Limit == 0 {
		options.Limit = knowledgeRetrievalLimit
	}
	if options.RerankTopK < 0 {
		return KnowledgeRetrievalOptions{}, ErrInvalidKnowledgeRetrievalOptions
	}
	if options.MinScore < 0 {
		return KnowledgeRetrievalOptions{}, ErrInvalidKnowledgeRetrievalOptions
	}
	if options.VectorWeight < 0 || options.KeywordWeight < 0 {
		return KnowledgeRetrievalOptions{}, ErrInvalidKnowledgeRetrievalOptions
	}
	if options.VectorWeight == 0 {
		options.VectorWeight = 0.7
	}
	if options.KeywordWeight == 0 {
		options.KeywordWeight = 0.3
	}
	options.DocumentVersion = strings.TrimSpace(options.DocumentVersion)
	if options.AllVersions {
		options.DocumentVersion = ""
	}
	return options, nil
}

func knowledgeRetrievalCandidateOptions(options KnowledgeRetrievalOptions, hasReranker bool) KnowledgeRetrievalOptions {
	if !hasReranker || options.Mode != KnowledgeRetrievalModeHybridRerank || options.RerankTopK <= options.Limit {
		return options
	}
	candidateOptions := options
	candidateOptions.Limit = options.RerankTopK
	return candidateOptions
}

func (s *Service) List(ctx context.Context, session auth.Session) ([]KnowledgeBase, error) {
	if s == nil || s.store == nil {
		return []KnowledgeBase{}, nil
	}
	store, ok := s.store.(interface {
		ListKnowledgeBases(ctx context.Context, scopeID string) ([]KnowledgeBase, error)
	})
	if !ok {
		return []KnowledgeBase{}, nil
	}
	return store.ListKnowledgeBases(ctx, knowledgeSessionScope(session))
}

func (s *Service) Create(ctx context.Context, session auth.Session, name string) (KnowledgeBase, error) {
	if s == nil || s.store == nil {
		return KnowledgeBase{}, sql.ErrNoRows
	}
	if store, ok := s.store.(organizationBaseCreator); ok {
		return store.CreateKnowledgeBase(ctx, session.WorkspaceID, knowledgeSessionScope(session), name)
	}
	store, ok := s.store.(interface {
		CreateKnowledgeBase(ctx context.Context, workspaceID, name string) (KnowledgeBase, error)
	})
	if !ok {
		return KnowledgeBase{}, sql.ErrNoRows
	}
	return store.CreateKnowledgeBase(ctx, session.WorkspaceID, name)
}

func (s *Service) CreateWithConfig(ctx context.Context, session auth.Session, name string, config KnowledgeBaseConfig) (KnowledgeBase, error) {
	config = normalizeKnowledgeBaseConfig(config)
	var base KnowledgeBase
	var err error
	if store, ok := s.store.(knowledgeBaseConfigCreator); ok {
		base, err = store.CreateKnowledgeBaseWithConfig(ctx, session.WorkspaceID, knowledgeSessionScope(session), name, config)
	} else {
		base, err = s.Create(ctx, session, name)
	}
	if err != nil {
		return KnowledgeBase{}, err
	}
	if err := s.ensureVectorCollection(ctx, session, base.ID); err != nil {
		return KnowledgeBase{}, err
	}
	return base, nil
}

func (s *Service) Get(ctx context.Context, session auth.Session, knowledgeBaseID string) (KnowledgeBase, error) {
	store, ok := s.store.(interface {
		GetKnowledgeBase(ctx context.Context, scopeID, knowledgeBaseID string) (KnowledgeBase, error)
	})
	if !ok {
		return KnowledgeBase{}, sql.ErrNoRows
	}
	return store.GetKnowledgeBase(ctx, knowledgeSessionScope(session), knowledgeBaseID)
}

func (s *Service) ListDocuments(ctx context.Context, session auth.Session, knowledgeBaseID string) ([]KnowledgeDocument, error) {
	store, ok := s.store.(interface {
		ListKnowledgeDocuments(ctx context.Context, scopeID, knowledgeBaseID string) ([]KnowledgeDocument, error)
	})
	if !ok {
		return []KnowledgeDocument{}, nil
	}
	return store.ListKnowledgeDocuments(ctx, knowledgeSessionScope(session), knowledgeBaseID)
}

func (s *Service) ListDocumentChunks(ctx context.Context, session auth.Session, knowledgeBaseID, documentID string) ([]KnowledgeDocumentChunkView, error) {
	store, ok := s.store.(documentChunkLister)
	if !ok {
		return []KnowledgeDocumentChunkView{}, nil
	}
	return store.ListKnowledgeDocumentChunks(ctx, knowledgeSessionScope(session), knowledgeBaseID, documentID)
}

func (s *Service) UpdateDocumentChunk(ctx context.Context, session auth.Session, knowledgeBaseID, documentID, chunkID, content string) (KnowledgeDocumentChunkView, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return KnowledgeDocumentChunkView{}, ErrEmptyKnowledgeDocumentChunk
	}
	store, ok := s.store.(documentChunkUpdater)
	if !ok {
		return KnowledgeDocumentChunkView{}, sql.ErrNoRows
	}
	scope := knowledgeSessionScope(session)
	chunk, err := store.UpdateKnowledgeDocumentChunk(ctx, scope, knowledgeBaseID, documentID, chunkID, content)
	if err != nil {
		return KnowledgeDocumentChunkView{}, err
	}
	if err := s.upsertEditedDocumentChunk(ctx, session, scope, knowledgeBaseID, documentID, chunk); err != nil {
		return KnowledgeDocumentChunkView{}, err
	}
	return chunk, nil
}

func (s *Service) CreateDocument(ctx context.Context, session auth.Session, knowledgeBaseID, title, content string) (KnowledgeDocument, error) {
	return s.CreateDocumentWithOptions(ctx, session, knowledgeBaseID, title, content, KnowledgeDocumentOptions{})
}

func (s *Service) CreateDocumentWithOptions(ctx context.Context, session auth.Session, knowledgeBaseID, title, content string, options KnowledgeDocumentOptions) (KnowledgeDocument, error) {
	options = normalizeKnowledgeDocumentOptions(options)
	startedAt := time.Now()
	baseConfig, err := s.knowledgeBaseConfigForDocument(ctx, session, knowledgeBaseID)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	chunks, err := s.buildDocumentChunks(withKnowledgeRelayIdentity(ctx, session), content, options, baseConfig)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	recordRAGDocumentProcessingMetrics(baseConfig.ChunkStrategy, len(chunks), startedAt)
	scope := knowledgeSessionScope(session)
	if store, ok := s.store.(documentCreatorWithOptions); ok {
		document, err := store.CreateKnowledgeDocumentWithOptions(ctx, scope, knowledgeBaseID, title, content, chunks, options)
		if err != nil {
			return KnowledgeDocument{}, err
		}
		if err := s.upsertDocumentChunks(ctx, scope, knowledgeBaseID, document.ID, chunks); err != nil {
			return KnowledgeDocument{}, err
		}
		return document, nil
	}
	if store, ok := s.store.(documentCreatorWithChunks); ok {
		document, err := store.CreateKnowledgeDocument(ctx, scope, knowledgeBaseID, title, content, chunks)
		if err != nil {
			return KnowledgeDocument{}, err
		}
		if err := s.upsertDocumentChunks(ctx, scope, knowledgeBaseID, document.ID, chunks); err != nil {
			return KnowledgeDocument{}, err
		}
		return document, nil
	}
	store, ok := s.store.(interface {
		CreateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, title, content string) (KnowledgeDocument, error)
	})
	if !ok {
		return KnowledgeDocument{}, sql.ErrNoRows
	}
	return store.CreateKnowledgeDocument(ctx, session.WorkspaceID, knowledgeBaseID, title, content)
}

func (s *Service) Retrieve(ctx context.Context, session auth.Session, knowledgeBaseID, query string) ([]KnowledgeRetrievalResult, error) {
	return s.RetrieveWithOptions(ctx, session, knowledgeBaseID, query, KnowledgeRetrievalOptions{})
}

func (s *Service) RetrieveWithOptions(ctx context.Context, session auth.Session, knowledgeBaseID, query string, options KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, error) {
	startedAt := time.Now()
	normalizedQuery := normalizeKnowledgeQuery(query)
	if normalizedQuery == "" {
		return []KnowledgeRetrievalResult{}, nil
	}
	options, err := normalizeKnowledgeRetrievalOptions(options)
	if err != nil {
		return nil, err
	}
	defer metrics.ObserveRAGRetrievalLatency(options.Mode, time.Since(startedAt).Seconds())
	queryEmbedding, err := s.embedQuery(withKnowledgeRelayIdentity(ctx, session), normalizedQuery)
	if err != nil {
		return nil, err
	}
	scope := knowledgeSessionScope(session)
	candidateOptions := knowledgeRetrievalCandidateOptions(options, s != nil && s.reranker != nil)
	if options.Mode == KnowledgeRetrievalModeVector && s != nil && s.vectorStore != nil && len(queryEmbedding) > 0 {
		results, err := s.vectorStore.SearchKnowledgeChunks(ctx, scope, knowledgeBaseID, normalizedQuery, queryEmbedding, candidateOptions)
		if err != nil {
			return nil, err
		}
		return s.rerankKnowledgeResults(ctx, normalizedQuery, normalizeKnowledgeRetrievalResults(results, options.Mode), options)
	}
	if isHybridKnowledgeRetrievalMode(options.Mode) && s != nil && s.vectorStore != nil && len(queryEmbedding) > 0 {
		if results, ok, err := s.retrieveHybridWithVectorStore(ctx, scope, knowledgeBaseID, normalizedQuery, queryEmbedding, candidateOptions); ok || err != nil {
			if err != nil {
				return nil, err
			}
			return s.rerankKnowledgeResults(ctx, normalizedQuery, normalizeKnowledgeRetrievalResults(results, options.Mode), options)
		}
	}
	if store, ok := s.store.(knowledgeRetrieverWithOptions); ok {
		results, err := store.RetrieveKnowledge(ctx, scope, knowledgeBaseID, normalizedQuery, queryEmbedding, candidateOptions)
		if err != nil {
			return nil, err
		}
		return s.rerankKnowledgeResults(ctx, normalizedQuery, normalizeKnowledgeRetrievalResults(results, options.Mode), options)
	}
	if store, ok := s.store.(knowledgeRetrieverNamedWithOptions); ok {
		results, err := store.RetrieveKnowledgeWithOptions(ctx, scope, knowledgeBaseID, normalizedQuery, queryEmbedding, candidateOptions)
		if err != nil {
			return nil, err
		}
		return s.rerankKnowledgeResults(ctx, normalizedQuery, normalizeKnowledgeRetrievalResults(results, options.Mode), options)
	}
	store, ok := s.store.(interface {
		RetrieveKnowledge(ctx context.Context, workspaceID, knowledgeBaseID, query string) ([]KnowledgeRetrievalResult, error)
	})
	if !ok {
		return []KnowledgeRetrievalResult{}, nil
	}
	results, err := store.RetrieveKnowledge(ctx, session.WorkspaceID, knowledgeBaseID, normalizedQuery)
	if err != nil {
		return nil, err
	}
	return s.rerankKnowledgeResults(ctx, normalizedQuery, normalizeKnowledgeRetrievalResults(results, KnowledgeRetrievalModeHybrid), options)
}

func isHybridKnowledgeRetrievalMode(mode string) bool {
	return mode == KnowledgeRetrievalModeHybrid || mode == KnowledgeRetrievalModeHybridRerank
}

func (s *Service) retrieveHybridWithVectorStore(ctx context.Context, organizationID, knowledgeBaseID, query string, queryEmbedding []float32, options KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, bool, error) {
	var keywordResults []KnowledgeRetrievalResult
	keywordOptions := options
	keywordOptions.Mode = KnowledgeRetrievalModeKeyword
	if store, ok := s.store.(knowledgeRetrieverWithOptions); ok {
		results, err := store.RetrieveKnowledge(ctx, organizationID, knowledgeBaseID, query, nil, keywordOptions)
		if err != nil {
			return nil, true, err
		}
		keywordResults = results
	} else if store, ok := s.store.(knowledgeRetrieverNamedWithOptions); ok {
		results, err := store.RetrieveKnowledgeWithOptions(ctx, organizationID, knowledgeBaseID, query, nil, keywordOptions)
		if err != nil {
			return nil, true, err
		}
		keywordResults = results
	} else {
		return nil, false, nil
	}

	vectorOptions := options
	vectorOptions.Mode = KnowledgeRetrievalModeVector
	vectorResults, err := s.vectorStore.SearchKnowledgeChunks(ctx, organizationID, knowledgeBaseID, query, queryEmbedding, vectorOptions)
	if err != nil {
		return nil, true, err
	}
	return fuseKnowledgeRetrievalResults(vectorResults, keywordResults, options), true, nil
}

func (s *Service) Update(ctx context.Context, session auth.Session, knowledgeBaseID, name string) (KnowledgeBase, error) {
	store, ok := s.store.(interface {
		UpdateKnowledgeBase(ctx context.Context, scopeID, knowledgeBaseID, name string) (KnowledgeBase, error)
	})
	if !ok {
		return KnowledgeBase{}, sql.ErrNoRows
	}
	return store.UpdateKnowledgeBase(ctx, knowledgeSessionScope(session), knowledgeBaseID, name)
}

func (s *Service) UpdateWithConfig(ctx context.Context, session auth.Session, knowledgeBaseID, name string, config KnowledgeBaseConfig) (KnowledgeBase, error) {
	config = normalizeKnowledgeBaseConfig(config)
	if store, ok := s.store.(knowledgeBaseConfigUpdater); ok {
		return store.UpdateKnowledgeBaseWithConfig(ctx, knowledgeSessionScope(session), knowledgeBaseID, name, config)
	}
	return s.Update(ctx, session, knowledgeBaseID, name)
}

func (s *Service) Delete(ctx context.Context, session auth.Session, knowledgeBaseID string) error {
	store, ok := s.store.(interface {
		DeleteKnowledgeBase(ctx context.Context, scopeID, knowledgeBaseID string) error
	})
	if !ok {
		return sql.ErrNoRows
	}
	if err := store.DeleteKnowledgeBase(ctx, knowledgeSessionScope(session), knowledgeBaseID); err != nil {
		return err
	}
	return s.deleteVectorCollection(ctx, session, knowledgeBaseID)
}

func (s *Service) UpdateDocument(ctx context.Context, session auth.Session, knowledgeBaseID, documentID, title, content string) (KnowledgeDocument, error) {
	return s.UpdateDocumentWithOptions(ctx, session, knowledgeBaseID, documentID, title, content, KnowledgeDocumentOptions{})
}

func (s *Service) UpdateDocumentWithOptions(ctx context.Context, session auth.Session, knowledgeBaseID, documentID, title, content string, options KnowledgeDocumentOptions) (KnowledgeDocument, error) {
	options = normalizeKnowledgeDocumentOptions(options)
	startedAt := time.Now()
	baseConfig, err := s.knowledgeBaseConfigForDocument(ctx, session, knowledgeBaseID)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	chunks, err := s.buildDocumentChunks(withKnowledgeRelayIdentity(ctx, session), content, options, baseConfig)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	recordRAGDocumentProcessingMetrics(baseConfig.ChunkStrategy, len(chunks), startedAt)
	scope := knowledgeSessionScope(session)
	if store, ok := s.store.(documentUpdaterWithOptions); ok {
		vectorChunks, err := s.vectorChunksForDocumentUpdate(ctx, scope, knowledgeBaseID, documentID, chunks, options)
		if err != nil {
			return KnowledgeDocument{}, err
		}
		document, err := store.UpdateKnowledgeDocumentWithOptions(ctx, scope, knowledgeBaseID, documentID, title, content, chunks, options)
		if err != nil {
			return KnowledgeDocument{}, err
		}
		if err := s.upsertDocumentChunks(ctx, scope, knowledgeBaseID, documentID, vectorChunks); err != nil {
			return KnowledgeDocument{}, err
		}
		return document, nil
	}
	if store, ok := s.store.(documentUpdaterWithChunks); ok {
		vectorChunks, err := s.vectorChunksForDocumentUpdate(ctx, scope, knowledgeBaseID, documentID, chunks, options)
		if err != nil {
			return KnowledgeDocument{}, err
		}
		document, err := store.UpdateKnowledgeDocument(ctx, scope, knowledgeBaseID, documentID, title, content, chunks)
		if err != nil {
			return KnowledgeDocument{}, err
		}
		if err := s.upsertDocumentChunks(ctx, scope, knowledgeBaseID, documentID, vectorChunks); err != nil {
			return KnowledgeDocument{}, err
		}
		return document, nil
	}
	store, ok := s.store.(interface {
		UpdateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, documentID, title, content string) (KnowledgeDocument, error)
	})
	if !ok {
		return KnowledgeDocument{}, sql.ErrNoRows
	}
	return store.UpdateKnowledgeDocument(ctx, session.WorkspaceID, knowledgeBaseID, documentID, title, content)
}

func (s *Service) vectorChunksForDocumentUpdate(ctx context.Context, organizationID, knowledgeBaseID, documentID string, chunks []KnowledgeDocumentChunk, options KnowledgeDocumentOptions) ([]KnowledgeDocumentChunk, error) {
	if options.UpdateStrategy != KnowledgeUpdateStrategyIncremental || s == nil || s.vectorStore == nil {
		return chunks, nil
	}
	store, ok := s.store.(documentChunkDiffer)
	if !ok {
		return chunks, nil
	}
	return store.DiffKnowledgeDocumentChunks(ctx, organizationID, knowledgeBaseID, documentID, chunks, options)
}

func (s *Service) upsertDocumentChunks(ctx context.Context, organizationID, knowledgeBaseID, documentID string, chunks []KnowledgeDocumentChunk) error {
	if s == nil || s.vectorStore == nil || len(chunks) == 0 {
		return nil
	}
	return s.vectorStore.UpsertKnowledgeDocumentChunks(ctx, organizationID, knowledgeBaseID, documentID, chunks)
}

func (s *Service) upsertEditedDocumentChunk(ctx context.Context, session auth.Session, organizationID, knowledgeBaseID, documentID string, view KnowledgeDocumentChunkView) error {
	if s == nil || s.vectorStore == nil || s.embedder == nil {
		return nil
	}
	content := strings.TrimSpace(view.Content)
	if content == "" {
		return nil
	}
	embedding, err := s.embedder.Embed(withKnowledgeRelayIdentity(ctx, session), content)
	if err != nil {
		return err
	}
	chunk := knowledgeDocumentChunkFromView(view)
	chunk.Content = content
	chunk.Embedding = append([]float32(nil), embedding...)
	if chunk.EstimatedTokenCount == 0 {
		chunk.EstimatedTokenCount = estimateKnowledgeTokens(content)
	}
	return s.upsertDocumentChunks(ctx, organizationID, knowledgeBaseID, documentID, []KnowledgeDocumentChunk{chunk})
}

func knowledgeDocumentChunkFromView(view KnowledgeDocumentChunkView) KnowledgeDocumentChunk {
	metadata := view.Metadata
	documentVersion := strings.TrimSpace(view.DocumentVersion)
	if documentVersion == "" {
		documentVersion = strings.TrimSpace(metadata.DocumentVersion)
	}
	if strings.TrimSpace(metadata.DocumentVersion) == "" {
		metadata.DocumentVersion = documentVersion
	}
	return KnowledgeDocumentChunk{
		ChunkIndex:          view.ChunkIndex,
		Content:             view.Content,
		DocumentVersion:     documentVersion,
		EstimatedTokenCount: view.EstimatedTokenCount,
		Metadata:            metadata,
	}
}

func (s *Service) DeleteDocument(ctx context.Context, session auth.Session, knowledgeBaseID, documentID string) error {
	store, ok := s.store.(interface {
		DeleteKnowledgeDocument(ctx context.Context, scopeID, knowledgeBaseID, documentID string) error
	})
	if !ok {
		return sql.ErrNoRows
	}
	return store.DeleteKnowledgeDocument(ctx, knowledgeSessionScope(session), knowledgeBaseID, documentID)
}

func (s *Service) DeleteDocumentByID(ctx context.Context, session auth.Session, documentID string) error {
	store, ok := s.store.(documentIDDeleter)
	if !ok {
		return sql.ErrNoRows
	}
	return store.DeleteKnowledgeDocumentByID(ctx, knowledgeSessionScope(session), documentID)
}

func (s *Service) CreateRetrievalTestCase(ctx context.Context, session auth.Session, knowledgeBaseID string, req CreateKnowledgeRetrievalTestCaseRequest) (KnowledgeRetrievalTestCase, error) {
	req.Query = normalizeKnowledgeQuery(req.Query)
	if req.Query == "" || (req.ExpectedResult.DocumentID == "" && req.ExpectedResult.ChunkID == "") {
		return KnowledgeRetrievalTestCase{}, ErrInvalidKnowledgeRetrievalTestCase
	}
	store, ok := s.store.(retrievalTestCaseCreator)
	if !ok {
		return KnowledgeRetrievalTestCase{}, sql.ErrNoRows
	}
	return store.CreateRetrievalTestCase(ctx, knowledgeSessionScope(session), knowledgeBaseID, req)
}

func (s *Service) ListRetrievalTestCases(ctx context.Context, session auth.Session, knowledgeBaseID string) ([]KnowledgeRetrievalTestCase, error) {
	store, ok := s.store.(retrievalTestCaseLister)
	if !ok {
		return []KnowledgeRetrievalTestCase{}, nil
	}
	return store.ListRetrievalTestCases(ctx, knowledgeSessionScope(session), knowledgeBaseID)
}

func (s *Service) RunRetrievalTestCases(ctx context.Context, session auth.Session, knowledgeBaseID string, options KnowledgeRetrievalOptions) (KnowledgeRetrievalTestRunReport, error) {
	options, err := normalizeKnowledgeRetrievalOptions(options)
	if err != nil {
		return KnowledgeRetrievalTestRunReport{}, err
	}
	cases, err := s.ListRetrievalTestCases(ctx, session, knowledgeBaseID)
	if err != nil {
		return KnowledgeRetrievalTestRunReport{}, err
	}
	report := KnowledgeRetrievalTestRunReport{
		Total:   len(cases),
		Results: make([]KnowledgeRetrievalTestRunResult, 0, len(cases)),
	}
	for _, testCase := range cases {
		results, err := s.RetrieveWithOptions(ctx, session, knowledgeBaseID, testCase.Query, options)
		if err != nil {
			return KnowledgeRetrievalTestRunReport{}, err
		}
		runResult := KnowledgeRetrievalTestRunResult{
			TestCaseID: testCase.ID,
			Query:      testCase.Query,
			Expected:   expectedResultFromTestCase(testCase),
		}
		for i, result := range results {
			if knowledgeResultMatchesTestCase(result, testCase) {
				runResult.Passed = true
				runResult.Rank = i + 1
				runResult.Actual = result
				break
			}
		}
		if runResult.Passed {
			report.Passed++
		} else {
			report.Failed++
			if len(results) > 0 {
				runResult.Actual = results[0]
			}
		}
		report.Results = append(report.Results, runResult)
	}
	return report, nil
}

func (s *Service) rerankKnowledgeResults(ctx context.Context, query string, results []KnowledgeRetrievalResult, options KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, error) {
	if options.Mode != KnowledgeRetrievalModeHybridRerank || s == nil || s.reranker == nil || len(results) == 0 {
		return results, nil
	}
	reranked, err := s.reranker.Rerank(ctx, query, results, options.Limit)
	if err != nil {
		metrics.RecordRAGRerankerFallback(options.Mode)
		return limitKnowledgeRetrievalResults(normalizeKnowledgeRetrievalResults(results, options.Mode), options.Limit), nil
	}
	return limitKnowledgeRetrievalResults(normalizeKnowledgeRetrievalResults(reranked, KnowledgeRetrievalModeHybridRerank), options.Limit), nil
}

func limitKnowledgeRetrievalResults(results []KnowledgeRetrievalResult, limit int) []KnowledgeRetrievalResult {
	if limit <= 0 || len(results) <= limit {
		return results
	}
	return results[:limit]
}

type knowledgeFusedRetrievalEntry struct {
	result   KnowledgeRetrievalResult
	score    float64
	bestRank int
	bestSrc  int
}

func fuseKnowledgeRetrievalResults(vectorResults, keywordResults []KnowledgeRetrievalResult, options KnowledgeRetrievalOptions) []KnowledgeRetrievalResult {
	entries := map[string]*knowledgeFusedRetrievalEntry{}
	addBatch := func(results []KnowledgeRetrievalResult, weight float64, sourceOrder int) {
		for rank, result := range results {
			key := knowledgeRetrievalResultKey(result)
			entry, ok := entries[key]
			if !ok {
				entry = &knowledgeFusedRetrievalEntry{result: result, bestRank: rank, bestSrc: sourceOrder}
				entries[key] = entry
			}
			entry.score += weight / float64(60+rank+1)
			if rank < entry.bestRank || (rank == entry.bestRank && sourceOrder < entry.bestSrc) {
				entry.bestRank = rank
				entry.bestSrc = sourceOrder
				entry.result = result
			}
		}
	}
	addBatch(vectorResults, options.VectorWeight, 0)
	addBatch(keywordResults, options.KeywordWeight, 1)

	flat := make([]*knowledgeFusedRetrievalEntry, 0, len(entries))
	for _, entry := range entries {
		entry.result.Score = entry.score
		entry.result.RetrievalMode = options.Mode
		if strings.TrimSpace(entry.result.RetrievalMethod) == "" {
			entry.result.RetrievalMethod = KnowledgeRetrievalMethodEmbeddingRAG
		}
		flat = append(flat, entry)
	}
	sort.SliceStable(flat, func(i, j int) bool {
		if flat[i].score != flat[j].score {
			return flat[i].score > flat[j].score
		}
		if flat[i].bestRank != flat[j].bestRank {
			return flat[i].bestRank < flat[j].bestRank
		}
		if flat[i].bestSrc != flat[j].bestSrc {
			return flat[i].bestSrc < flat[j].bestSrc
		}
		if flat[i].result.DocumentTitle != flat[j].result.DocumentTitle {
			return flat[i].result.DocumentTitle < flat[j].result.DocumentTitle
		}
		return flat[i].result.ChunkIndex < flat[j].result.ChunkIndex
	})

	results := make([]KnowledgeRetrievalResult, len(flat))
	for index, entry := range flat {
		results[index] = entry.result
	}
	return limitKnowledgeRetrievalResults(results, options.Limit)
}

func knowledgeRetrievalResultKey(result KnowledgeRetrievalResult) string {
	if strings.TrimSpace(result.ChunkID) != "" {
		return strings.TrimSpace(result.ChunkID)
	}
	if strings.TrimSpace(result.DocumentID) != "" {
		return strings.TrimSpace(result.DocumentID) + ":" + strings.TrimSpace(result.DocumentVersion) + ":" + strings.TrimSpace(result.Snippet)
	}
	return strings.TrimSpace(result.DocumentTitle) + ":" + strings.TrimSpace(result.Snippet)
}

func (s *Service) knowledgeBaseConfigForDocument(ctx context.Context, session auth.Session, knowledgeBaseID string) (KnowledgeBaseConfig, error) {
	base, err := s.Get(ctx, session, knowledgeBaseID)
	if err != nil {
		return normalizeKnowledgeBaseConfig(KnowledgeBaseConfig{}), err
	}
	return normalizeKnowledgeBaseConfig(KnowledgeBaseConfig{
		ChunkOverlap:  base.ChunkOverlap,
		ChunkSize:     base.ChunkSize,
		ChunkStrategy: base.ChunkStrategy,
	}), nil
}

func (s *Service) buildDocumentChunks(ctx context.Context, content string, options KnowledgeDocumentOptions, baseConfig KnowledgeBaseConfig) ([]KnowledgeDocumentChunk, error) {
	baseConfig = normalizeKnowledgeBaseConfig(baseConfig)
	rawChunks := buildKnowledgeDocumentChunksWithConfig(content, baseConfig)
	chunks := make([]KnowledgeDocumentChunk, 0, len(rawChunks))
	for index, chunk := range rawChunks {
		metadata := KnowledgeChunkMetadata{
			DocumentVersion: options.DocumentVersion,
			PageNumber:      options.PageNumber,
			SourceURL:       options.SourceURL,
			StartRune:       chunk.StartRune,
			EndRune:         chunk.EndRune,
		}
		chunks = append(chunks, KnowledgeDocumentChunk{
			ChunkIndex:          index,
			Content:             chunk.Content,
			DocumentVersion:     options.DocumentVersion,
			EstimatedTokenCount: estimateKnowledgeTokens(chunk.Content),
			Metadata:            metadata,
		})
	}
	if s != nil && s.embedder != nil && len(chunks) > 0 {
		texts := make([]string, len(chunks))
		for i := range chunks {
			texts[i] = chunks[i].Content
		}
		embeddings, err := s.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return nil, err
		}
		for i := range chunks {
			if i < len(embeddings) {
				chunks[i].Embedding = append([]float32(nil), embeddings[i]...)
			}
		}
	}
	return chunks, nil
}

func recordRAGDocumentProcessingMetrics(strategy string, chunkCount int, startedAt time.Time) {
	metrics.ObserveRAGDocumentProcessingDuration(strategy, time.Since(startedAt).Seconds())
	metrics.SetRAGChunkCount(chunkCount)
}

func normalizeKnowledgeRetrievalResults(results []KnowledgeRetrievalResult, mode string) []KnowledgeRetrievalResult {
	for index := range results {
		if strings.TrimSpace(results[index].RetrievalMode) == "" {
			results[index].RetrievalMode = normalizeKnowledgeRetrievalMode(mode)
		}
		if strings.TrimSpace(results[index].RetrievalMethod) == "" {
			results[index].RetrievalMethod = KnowledgeRetrievalMethodEmbeddingRAG
		}
	}
	return results
}

func (s *Service) embedQuery(ctx context.Context, query string) ([]float32, error) {
	if s == nil || s.embedder == nil {
		return nil, nil
	}
	embedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return append([]float32(nil), embedding...), nil
}

func (s *Service) ensureVectorCollection(ctx context.Context, session auth.Session, knowledgeBaseID string) error {
	if s == nil || s.vectorStore == nil {
		return nil
	}
	vectorSize := s.vectorSize
	if vectorSize <= 0 {
		vectorSize = 1536
	}
	return s.vectorStore.EnsureKnowledgeBaseCollection(ctx, knowledgeSessionScope(session), knowledgeBaseID, vectorSize)
}

func (s *Service) deleteVectorCollection(ctx context.Context, session auth.Session, knowledgeBaseID string) error {
	if s == nil || s.vectorStore == nil {
		return nil
	}
	return s.vectorStore.DeleteKnowledgeBaseCollection(ctx, knowledgeSessionScope(session), knowledgeBaseID)
}

func estimateKnowledgeTokens(content string) int {
	runes := len([]rune(strings.TrimSpace(content)))
	if runes == 0 {
		return 0
	}
	tokens := runes / 4
	if runes%4 != 0 {
		tokens++
	}
	if tokens == 0 {
		return 1
	}
	return tokens
}

func expectedResultFromTestCase(testCase KnowledgeRetrievalTestCase) KnowledgeRetrievalResult {
	if testCase.ExpectedResult.DocumentID != "" || testCase.ExpectedResult.ChunkID != "" {
		return testCase.ExpectedResult
	}
	return KnowledgeRetrievalResult{
		ChunkID:         testCase.ExpectedChunkID,
		ChunkIndex:      testCase.ExpectedChunkIndex,
		DocumentID:      testCase.ExpectedDocumentID,
		DocumentTitle:   testCase.ExpectedDocumentTitle,
		DocumentVersion: testCase.ExpectedDocumentVersion,
		Snippet:         testCase.ExpectedSnippet,
	}
}

func knowledgeResultMatchesTestCase(result KnowledgeRetrievalResult, testCase KnowledgeRetrievalTestCase) bool {
	expected := expectedResultFromTestCase(testCase)
	if expected.DocumentID != "" && result.DocumentID != expected.DocumentID {
		return false
	}
	if expected.ChunkID != "" && result.ChunkID != expected.ChunkID {
		return false
	}
	if expected.ChunkIndex != 0 && result.ChunkIndex != expected.ChunkIndex {
		return false
	}
	if expected.DocumentVersion != "" && result.DocumentVersion != expected.DocumentVersion {
		return false
	}
	return true
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
