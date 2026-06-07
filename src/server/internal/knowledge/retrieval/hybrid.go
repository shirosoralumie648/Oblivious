package retrieval

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"oblivious/server/internal/knowledge"
)

const (
	defaultRetrievalLimit = 5
	defaultVectorWeight   = 0.7
	defaultKeywordWeight  = 0.3
	rrfConstant           = 60
	maxRetrievalLimit     = 50
	snippetSize           = 220
)

// VectorSearcher executes vector similarity search.
type VectorSearcher interface {
	SearchByVector(ctx context.Context, organizationID, knowledgeBaseID string, embedding []float32, limit int, minScore float64) ([]knowledge.HybridEngineRetrievalResult, error)
}

// KeywordSearcher executes full-text search.
type KeywordSearcher interface {
	SearchByKeyword(ctx context.Context, organizationID, knowledgeBaseID, query string, limit int) ([]knowledge.HybridEngineRetrievalResult, error)
}

// HybridEngine performs Reciprocal Rank Fusion over vector and keyword results.
type HybridEngine struct {
	db              *sql.DB
	vectorSearcher  VectorSearcher
	keywordSearcher KeywordSearcher
}

// NewHybridEngine creates a HybridEngine backed by the given database and optional
// external searchers. If searchers are nil, falls back to direct PostgreSQL queries.
func NewHybridEngine(db *sql.DB, vs VectorSearcher, ks KeywordSearcher) *HybridEngine {
	return &HybridEngine{
		db:              db,
		vectorSearcher:  vs,
		keywordSearcher: ks,
	}
}

// Retrieve executes the hybrid retrieval pipeline.
func (e *HybridEngine) Retrieve(ctx context.Context, organizationID, knowledgeBaseID, query string, queryEmbedding []float32, opts knowledge.HybridEngineRetrievalOptions) ([]knowledge.HybridEngineRetrievalResult, error) {
	opts = normalizeOptions(opts)

	switch opts.Mode {
	case knowledge.RetrievalModeVector:
		return e.vectorOnly(ctx, organizationID, knowledgeBaseID, queryEmbedding, opts)
	case knowledge.RetrievalModeKeyword:
		return e.keywordOnly(ctx, organizationID, knowledgeBaseID, query, opts)
	case knowledge.RetrievalModeHybrid, knowledge.RetrievalModeHybridRerank:
		return e.hybrid(ctx, organizationID, knowledgeBaseID, query, queryEmbedding, opts)
	default:
		return e.hybrid(ctx, organizationID, knowledgeBaseID, query, queryEmbedding, opts)
	}
}

func (e *HybridEngine) vectorOnly(ctx context.Context, orgID, kbID string, embedding []float32, opts knowledge.HybridEngineRetrievalOptions) ([]knowledge.HybridEngineRetrievalResult, error) {
	if len(embedding) == 0 {
		return []knowledge.HybridEngineRetrievalResult{}, nil
	}
	if e.vectorSearcher != nil {
		return e.vectorSearcher.SearchByVector(ctx, orgID, kbID, embedding, opts.Limit, opts.MinScore)
	}
	return e.pgVectorSearch(ctx, orgID, kbID, embedding, opts)
}

func (e *HybridEngine) keywordOnly(ctx context.Context, orgID, kbID, query string, opts knowledge.HybridEngineRetrievalOptions) ([]knowledge.HybridEngineRetrievalResult, error) {
	if strings.TrimSpace(query) == "" {
		return []knowledge.HybridEngineRetrievalResult{}, nil
	}
	if e.keywordSearcher != nil {
		return e.keywordSearcher.SearchByKeyword(ctx, orgID, kbID, query, opts.Limit)
	}
	return e.pgKeywordSearch(ctx, orgID, kbID, query, opts)
}

func (e *HybridEngine) hybrid(ctx context.Context, orgID, kbID, query string, embedding []float32, opts knowledge.HybridEngineRetrievalOptions) ([]knowledge.HybridEngineRetrievalResult, error) {
	vectorResults, err := e.vectorOnly(ctx, orgID, kbID, embedding, opts)
	if err != nil {
		return nil, fmt.Errorf("hybrid vector search: %w", err)
	}
	keywordResults, err := e.keywordOnly(ctx, orgID, kbID, query, opts)
	if err != nil {
		return nil, fmt.Errorf("hybrid keyword search: %w", err)
	}
	fused := fuseResults(vectorResults, keywordResults, opts.VectorWeight, opts.KeywordWeight, opts.Limit)
	for i := range fused {
		fused[i].RetrievalMethod = knowledge.RetrievalModeHybrid
	}
	return fused, nil
}

// ---------------------------------------------------------------------------
// Reciprocal Rank Fusion
// ---------------------------------------------------------------------------

type fusedEntry struct {
	result   knowledge.HybridEngineRetrievalResult
	score    float64
	bestRank int
	bestSrc  int
}

func fuseResults(vectorResults, keywordResults []knowledge.HybridEngineRetrievalResult, vectorWeight, keywordWeight float64, limit int) []knowledge.HybridEngineRetrievalResult {
	if limit <= 0 {
		limit = defaultRetrievalLimit
	}

	entries := map[string]*fusedEntry{}
	addBatch := func(results []knowledge.HybridEngineRetrievalResult, weight float64, srcOrder int) {
		for rank, r := range results {
			key := r.ChunkID
			if key == "" {
				key = r.DocumentID + ":" + fmt.Sprintf("%d", r.ChunkIndex)
			}
			entry, ok := entries[key]
			if !ok {
				entry = &fusedEntry{result: r, bestRank: rank, bestSrc: srcOrder}
				entries[key] = entry
			}
			entry.score += weight / float64(rrfConstant+rank+1)
			if rank < entry.bestRank || (rank == entry.bestRank && srcOrder < entry.bestSrc) {
				entry.bestRank = rank
				entry.bestSrc = srcOrder
				entry.result = r
			}
		}
	}

	addBatch(vectorResults, vectorWeight, 0)
	addBatch(keywordResults, keywordWeight, 1)

	flat := make([]*fusedEntry, 0, len(entries))
	for _, e := range entries {
		e.result.Score = e.score
		flat = append(flat, e)
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

	if len(flat) > limit {
		flat = flat[:limit]
	}
	results := make([]knowledge.HybridEngineRetrievalResult, len(flat))
	for i, e := range flat {
		results[i] = e.result
	}
	return results
}

// ---------------------------------------------------------------------------
// PostgreSQL fallback queries
// ---------------------------------------------------------------------------

func (e *HybridEngine) pgVectorSearch(ctx context.Context, orgID, kbID string, embedding []float32, opts knowledge.HybridEngineRetrievalOptions) ([]knowledge.HybridEngineRetrievalResult, error) {
	if e.db == nil {
		return []knowledge.HybridEngineRetrievalResult{}, nil
	}
	embeddingVector := formatVector(embedding)
	rows, err := e.db.QueryContext(ctx, `
		SELECT
			d.id, d.title, d.document_version,
			c.id, c.chunk_index,
			COALESCE(NULLIF(c.document_version, ''), d.document_version, ''),
			c.content, c.metadata,
			1 - (c.embedding <=> $3::vector) AS similarity
		FROM knowledge_document_chunks c
		JOIN knowledge_documents d ON d.id = c.document_id
		JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
		WHERE kb.organization_id = $1
		  AND d.organization_id = $1
		  AND c.organization_id = $1
		  AND d.knowledge_base_id = $2
		  AND c.embedding IS NOT NULL
		  AND (1 - (c.embedding <=> $3::vector)) >= $5
		ORDER BY c.embedding <=> $3::vector, d.updated_at DESC, d.title ASC, c.chunk_index ASC
		LIMIT $4
	`, orgID, kbID, embeddingVector, opts.Limit, opts.MinScore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRetrievalRows(rows)
}

func (e *HybridEngine) pgKeywordSearch(ctx context.Context, orgID, kbID, query string, opts knowledge.HybridEngineRetrievalOptions) ([]knowledge.HybridEngineRetrievalResult, error) {
	if e.db == nil {
		return []knowledge.HybridEngineRetrievalResult{}, nil
	}
	normalized := strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
	if normalized == "" {
		return []knowledge.HybridEngineRetrievalResult{}, nil
	}
	rows, err := e.db.QueryContext(ctx, `
		WITH keyword_query AS (
			SELECT websearch_to_tsquery('simple', $3) AS query
		)
		SELECT
			d.id, d.title, d.document_version,
			c.id, c.chunk_index,
			COALESCE(NULLIF(c.document_version, ''), d.document_version, ''),
			c.content, c.metadata,
			ts_rank_cd(
				setweight(to_tsvector('simple', COALESCE(d.title, '')), 'A') ||
				setweight(to_tsvector('simple', COALESCE(c.content, '')), 'B'),
				keyword_query.query
			) AS score
		FROM knowledge_document_chunks c
		JOIN knowledge_documents d ON d.id = c.document_id
		JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
		CROSS JOIN keyword_query
		WHERE kb.organization_id = $1
		  AND d.organization_id = $1
		  AND c.organization_id = $1
		  AND d.knowledge_base_id = $2
		  AND keyword_query.query @@ (
			setweight(to_tsvector('simple', COALESCE(d.title, '')), 'A') ||
			setweight(to_tsvector('simple', COALESCE(c.content, '')), 'B')
		  )
		ORDER BY score DESC, d.updated_at DESC, d.title ASC, c.chunk_index ASC
		LIMIT $4
	`, orgID, kbID, normalized, opts.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRetrievalRows(rows)
}

func scanRetrievalRows(rows *sql.Rows) ([]knowledge.HybridEngineRetrievalResult, error) {
	var results []knowledge.HybridEngineRetrievalResult
	for rows.Next() {
		var (
			docID, docTitle, docVer string
			chunkID                 string
			chunkIndex              int
			chunkVer                string
			content                 string
			metadataRaw             []byte
			score                   float64
		)
		if err := rows.Scan(&docID, &docTitle, &docVer, &chunkID, &chunkIndex, &chunkVer, &content, &metadataRaw, &score); err != nil {
			return nil, err
		}
		if chunkVer != "" {
			docVer = chunkVer
		}
		var meta knowledge.KnowledgeChunkMetadata
		if len(metadataRaw) > 0 {
			_ = json.Unmarshal(metadataRaw, &meta)
		}
		if meta.DocumentVersion != "" {
			docVer = meta.DocumentVersion
		}

		results = append(results, knowledge.HybridEngineRetrievalResult{
			DocumentID:      docID,
			DocumentTitle:   docTitle,
			DocumentVersion: docVer,
			ChunkID:         chunkID,
			ChunkIndex:      chunkIndex,
			RetrievalMethod: knowledge.RetrievalModeVector,
			Score:           score,
			Snippet:         buildSnippet(content, ""),
			Citation: knowledge.CitationTraceV2{
				DocumentID:      docID,
				DocumentTitle:   docTitle,
				DocumentVersion: docVer,
				ChunkID:         chunkID,
				ChunkIndex:      chunkIndex,
				PageNumber:      meta.PageNumber,
				SourceURL:       meta.SourceURL,
				OriginalText:    content,
				MatchedSnippet:  buildSnippet(content, ""),
			},
		})
	}
	return results, rows.Err()
}

func normalizeOptions(opts knowledge.HybridEngineRetrievalOptions) knowledge.HybridEngineRetrievalOptions {
	switch strings.TrimSpace(opts.Mode) {
	case knowledge.RetrievalModeVector, knowledge.RetrievalModeKeyword,
		knowledge.RetrievalModeHybrid, knowledge.RetrievalModeHybridRerank:
		// valid
	default:
		opts.Mode = knowledge.RetrievalModeHybrid
	}
	if opts.Limit <= 0 {
		opts.Limit = defaultRetrievalLimit
	}
	if opts.Limit > maxRetrievalLimit {
		opts.Limit = maxRetrievalLimit
	}
	if opts.MinScore < 0 {
		opts.MinScore = 0
	}
	if opts.VectorWeight <= 0 {
		opts.VectorWeight = defaultVectorWeight
	}
	if opts.KeywordWeight <= 0 {
		opts.KeywordWeight = defaultKeywordWeight
	}
	return opts
}

func formatVector(embedding []float32) string {
	if len(embedding) == 0 {
		return "[]"
	}
	var buf []byte
	buf = append(buf, '[')
	for i, v := range embedding {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, fmt.Sprintf("%f", v)...)
	}
	buf = append(buf, ']')
	return string(buf)
}

func buildSnippet(content, query string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if normalized == "" {
		return ""
	}
	runes := []rune(normalized)
	lowerContent := strings.ToLower(normalized)
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	matchIndex := strings.Index(lowerContent, lowerQuery)
	if matchIndex == -1 {
		if len(runes) <= snippetSize {
			return normalized
		}
		return strings.TrimSpace(string(runes[:snippetSize])) + "..."
	}
	matchRunes := []rune(normalized[:matchIndex])
	start := len(matchRunes) - snippetSize/3
	if start < 0 {
		start = 0
	}
	end := start + snippetSize
	if end > len(runes) {
		end = len(runes)
	}
	snippet := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet += "..."
	}
	return snippet
}
