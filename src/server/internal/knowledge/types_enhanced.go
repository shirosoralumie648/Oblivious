package knowledge

// New document format constants extending the existing set defined in
// document_parser.go (text, markdown, pdf, docx).
const (
	KnowledgeDocumentFormatCSV  = "csv"
	KnowledgeDocumentFormatHTML = "html"
	KnowledgeDocumentFormatXLSX = "xlsx"
	KnowledgeDocumentFormatPPTX = "pptx"
)

const (
	RetrievalModeVector       = "vector"
	RetrievalModeKeyword      = "keyword"
	RetrievalModeHybrid       = "hybrid"
	RetrievalModeHybridRerank = "hybrid_rerank"

	KnowledgeRetrievalModeVector       = RetrievalModeVector
	KnowledgeRetrievalModeKeyword      = RetrievalModeKeyword
	KnowledgeRetrievalModeHybrid       = RetrievalModeHybrid
	KnowledgeRetrievalModeHybridRerank = RetrievalModeHybridRerank
)

// DocumentPage represents one logical page extracted from a multi-page document.
type DocumentPage struct {
	PageNumber int    `json:"pageNumber"`
	Content    string `json:"content"`
}

// ParsedDocumentWithPages extends the basic parsed document with per-page
// content so that citation tracing can reference specific page numbers.
type ParsedDocumentWithPages struct {
	Content     string         `json:"content"`
	Format      string         `json:"format"`
	Title       string         `json:"title"`
	ContentType string         `json:"contentType"`
	SizeBytes   int64          `json:"sizeBytes"`
	Pages       []DocumentPage `json:"pages,omitempty"`
}

// ChunkingEngineStrategy enumerates the available chunking algorithms.
const (
	ChunkingEngineStrategyFixedSize = "fixed_size"
	ChunkingEngineStrategySemantic  = "semantic"
	ChunkingEngineStrategyQASplit   = "qa_split"
	ChunkingEngineStrategyTemplate  = "template"

	KnowledgeChunkStrategyFixedSize     = ChunkingEngineStrategyFixedSize
	KnowledgeChunkStrategySemantic      = ChunkingEngineStrategySemantic
	KnowledgeChunkStrategyQASplit       = ChunkingEngineStrategyQASplit
	KnowledgeChunkStrategyTemplate      = ChunkingEngineStrategyTemplate
	KnowledgeChunkStrategyTemplateBased = "template_based"
)

const (
	KnowledgeUpdateStrategyFullReplace = "full_replace"
	KnowledgeUpdateStrategyVersioned   = "versioned"
)

// ChunkingEngineConfig holds parameters for the chunking engine.
type ChunkingEngineConfig struct {
	Strategy  string `json:"strategy"`
	ChunkSize int    `json:"chunkSize"`
	Overlap   int    `json:"overlap"`
}

// EngineDocumentChunk is a single chunk produced by the chunking engine with
// positional metadata preserved for citation tracing.
type EngineDocumentChunk struct {
	Index      int    `json:"index"`
	Content    string `json:"content"`
	PageNumber int    `json:"pageNumber,omitempty"`
	StartRune  int    `json:"startRune,omitempty"`
	EndRune    int    `json:"endRune,omitempty"`
}

type KnowledgeChunkMetadata struct {
	DocumentVersion string         `json:"documentVersion,omitempty"`
	PageNumber      int            `json:"pageNumber,omitempty"`
	SourceURL       string         `json:"sourceUrl,omitempty"`
	StartRune       int            `json:"startRune,omitempty"`
	EndRune         int            `json:"endRune,omitempty"`
	Extra           map[string]any `json:"extra,omitempty"`
}

type KnowledgeDocumentChunk struct {
	ChunkIndex          int                    `json:"chunkIndex"`
	Content             string                 `json:"content"`
	DocumentVersion     string                 `json:"documentVersion,omitempty"`
	Embedding           []float32              `json:"-"`
	EstimatedTokenCount int                    `json:"estimatedTokenCount,omitempty"`
	Metadata            KnowledgeChunkMetadata `json:"metadata,omitempty"`
}

type KnowledgeDocumentChunkView struct {
	ChunkID             string                 `json:"chunkId"`
	ChunkIndex          int                    `json:"chunkIndex"`
	Content             string                 `json:"content"`
	DocumentVersion     string                 `json:"documentVersion,omitempty"`
	CharCount           int                    `json:"charCount,omitempty"`
	EstimatedTokenCount int                    `json:"estimatedTokenCount,omitempty"`
	Metadata            KnowledgeChunkMetadata `json:"metadata,omitempty"`
}

// EmbeddingServiceConfig holds configuration for the embedding service.
type EmbeddingServiceConfig struct {
	Model   string `json:"model"`
	BaseURL string `json:"baseUrl,omitempty"`
	APIKey  string `json:"-"`
}

// HybridEngineRetrievalOptions controls the hybrid retrieval behaviour.
type HybridEngineRetrievalOptions struct {
	Mode          string  `json:"mode"`
	Limit         int     `json:"limit"`
	MinScore      float64 `json:"minScore"`
	VectorWeight  float64 `json:"vectorWeight"`
	KeywordWeight float64 `json:"keywordWeight"`
}

type KnowledgeRetrievalOptions struct {
	AllVersions     bool    `json:"allVersions,omitempty"`
	DocumentVersion string  `json:"documentVersion,omitempty"`
	Mode            string  `json:"mode,omitempty"`
	Limit           int     `json:"limit,omitempty"`
	MinScore        float64 `json:"minScore,omitempty"`
	VectorWeight    float64 `json:"vectorWeight,omitempty"`
	KeywordWeight   float64 `json:"keywordWeight,omitempty"`
}

// HybridEngineRetrievalResult is a single retrieval result from the hybrid engine.
type HybridEngineRetrievalResult struct {
	DocumentID      string          `json:"documentId"`
	DocumentTitle   string          `json:"documentTitle"`
	DocumentVersion string          `json:"documentVersion"`
	ChunkID         string          `json:"chunkId"`
	ChunkIndex      int             `json:"chunkIndex"`
	RetrievalMethod string          `json:"retrievalMethod"`
	Score           float64         `json:"score"`
	Snippet         string          `json:"snippet"`
	Citation        CitationTraceV2 `json:"citation"`
}

type HybridRetrievalResult = HybridEngineRetrievalResult

// RerankerServiceConfig holds configuration for the reranker service.
type RerankerServiceConfig struct {
	Model   string `json:"model"`
	BaseURL string `json:"baseUrl,omitempty"`
	APIKey  string `json:"-"`
	TopK    int    `json:"topK"`
}

type RerankerConfig = RerankerServiceConfig

// CitationTraceV2 provides full provenance information for a retrieved chunk.
type CitationTrace struct {
	DocumentID         string              `json:"documentId"`
	DocumentTitle      string              `json:"documentTitle"`
	DocumentVersion    string              `json:"documentVersion"`
	ChunkID            string              `json:"chunkId"`
	ChunkIndex         int                 `json:"chunkIndex"`
	PageNumber         int                 `json:"pageNumber,omitempty"`
	SourceURL          string              `json:"sourceUrl,omitempty"`
	OriginalText       string              `json:"originalText,omitempty"`
	MatchedSnippet     string              `json:"matchedSnippet,omitempty"`
	ConfidenceScore    float64             `json:"confidenceScore,omitempty"`
	HighlightPositions []HighlightPosition `json:"highlightPositions,omitempty"`
}

type CitationTraceV2 = CitationTrace
type KnowledgeCitation = CitationTrace

// HighlightPosition marks a highlighted region within the original text.
type HighlightPosition struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type HighlightPosV2 = HighlightPosition
type KnowledgeHighlightPosition = HighlightPosition

type KnowledgeBaseConfig struct {
	ChunkOverlap   int     `json:"chunkOverlap,omitempty"`
	ChunkSize      int     `json:"chunkSize,omitempty"`
	ChunkStrategy  string  `json:"chunkStrategy,omitempty"`
	EmbeddingModel string  `json:"embeddingModel,omitempty"`
	KeywordWeight  float64 `json:"keywordWeight,omitempty"`
	MinScore       float64 `json:"minScore,omitempty"`
	RerankTopK     int     `json:"rerankTopK,omitempty"`
	RerankerModel  string  `json:"rerankerModel,omitempty"`
	RetrievalLimit int     `json:"retrievalLimit,omitempty"`
	RetrievalMode  string  `json:"retrievalMode,omitempty"`
	UpdateStrategy string  `json:"updateStrategy,omitempty"`
	VectorWeight   float64 `json:"vectorWeight,omitempty"`
}

type KnowledgeDocumentOptions struct {
	DocumentVersion string `json:"documentVersion,omitempty"`
	PageNumber      int    `json:"pageNumber,omitempty"`
	SourceURL       string `json:"sourceUrl,omitempty"`
	UpdateStrategy  string `json:"updateStrategy,omitempty"`
}

type CreateKnowledgeRetrievalTestCaseRequest struct {
	ExpectedResult KnowledgeRetrievalResult `json:"expectedResult"`
	Query          string                   `json:"query"`
}

type KnowledgeRetrievalTestCase struct {
	ID                      string                   `json:"id"`
	KnowledgeBaseID         string                   `json:"knowledgeBaseId"`
	Query                   string                   `json:"query"`
	ExpectedDocumentID      string                   `json:"expectedDocumentId"`
	ExpectedDocumentTitle   string                   `json:"expectedDocumentTitle,omitempty"`
	ExpectedDocumentVersion string                   `json:"expectedDocumentVersion,omitempty"`
	ExpectedChunkID         string                   `json:"expectedChunkId"`
	ExpectedChunkIndex      int                      `json:"expectedChunkIndex"`
	ExpectedSnippet         string                   `json:"expectedSnippet,omitempty"`
	ExpectedResult          KnowledgeRetrievalResult `json:"expectedResult"`
}

type KnowledgeRetrievalTestRunResult struct {
	TestCaseID string                   `json:"testCaseId"`
	Query      string                   `json:"query"`
	Passed     bool                     `json:"passed"`
	Rank       int                      `json:"rank"`
	Expected   KnowledgeRetrievalResult `json:"expected"`
	Actual     KnowledgeRetrievalResult `json:"actual,omitempty"`
}

type KnowledgeRetrievalTestRunReport struct {
	Total   int                               `json:"total"`
	Passed  int                               `json:"passed"`
	Failed  int                               `json:"failed"`
	Results []KnowledgeRetrievalTestRunResult `json:"results"`
}
