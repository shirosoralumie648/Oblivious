package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"oblivious/server/internal/auth"
)

const (
	defaultKnowledgeStoreEmbeddingModel = "text-embedding-3-small"
	knowledgeDocumentChunkSize          = 280
	knowledgeRetrievalLimit             = 5
	knowledgeSnippetSize                = 220
)

type knowledgeRetrievalCandidate struct {
	documentID    string
	documentTitle string
	documentBody  string
	chunkContent  sql.NullString
	chunkIndex    int
	updatedAt     time.Time
}

func buildKnowledgeQueryTerms(query string) []string {
	normalized := normalizeKnowledgeQuery(query)
	if normalized == "" {
		return nil
	}

	return strings.Fields(strings.ToLower(normalized))
}

func countKnowledgeTermHits(content string, terms []string) int {
	lowerContent := strings.ToLower(content)
	hits := 0
	for _, term := range terms {
		if term != "" && strings.Contains(lowerContent, term) {
			hits++
		}
	}
	return hits
}

func scoreKnowledgeCandidate(title, body string, chunk sql.NullString, terms []string) int {
	titleHits := countKnowledgeTermHits(title, terms)
	bodyHits := countKnowledgeTermHits(body, terms)
	chunkHits := 0
	if chunk.Valid {
		chunkHits = countKnowledgeTermHits(chunk.String, terms)
	}

	score := titleHits*100 + chunkHits*25 + bodyHits*10
	if titleHits == len(terms) && len(terms) > 0 {
		score += 50
	}
	return score
}

func chooseKnowledgeSnippetSource(body string, chunk sql.NullString, terms []string) string {
	if chunk.Valid && countKnowledgeTermHits(chunk.String, terms) >= countKnowledgeTermHits(body, terms) {
		return chunk.String
	}
	return body
}

func (s *SQLStore) CreateKnowledgeBase(ctx context.Context, workspaceID, name string) (KnowledgeBase, error) {
	return s.CreateKnowledgeBaseWithConfig(ctx, workspaceID, workspaceID, name, KnowledgeBaseConfig{})
}

func (s *SQLStore) CreateKnowledgeBaseWithConfig(ctx context.Context, workspaceID, organizationID, name string, config KnowledgeBaseConfig) (KnowledgeBase, error) {
	config = normalizeKnowledgeBaseConfig(config)
	knowledgeBaseID, err := auth.NewID("kb")
	if err != nil {
		return KnowledgeBase{}, err
	}

	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO knowledge_bases (
			id,
			workspace_id,
			organization_id,
			name,
			document_count,
			retrieval_mode,
			retrieval_limit,
			min_score,
			vector_weight,
			keyword_weight,
			reranker_model,
			rerank_top_k,
			chunk_strategy,
			chunk_size,
			chunk_overlap,
			embedding_model,
			update_strategy,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, 0, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $17)
	`, knowledgeBaseID, workspaceID, organizationID, name, config.RetrievalMode, config.RetrievalLimit, config.MinScore, config.VectorWeight, config.KeywordWeight, config.RerankerModel, config.RerankTopK, config.ChunkStrategy, config.ChunkSize, config.ChunkOverlap, config.EmbeddingModel, config.UpdateStrategy, now); err != nil {
		return KnowledgeBase{}, err
	}

	return KnowledgeBase{
		ChunkOverlap:   config.ChunkOverlap,
		ChunkSize:      config.ChunkSize,
		ChunkStrategy:  config.ChunkStrategy,
		DocumentCount:  0,
		EmbeddingModel: config.EmbeddingModel,
		ID:             knowledgeBaseID,
		KeywordWeight:  config.KeywordWeight,
		MinScore:       config.MinScore,
		Name:           name,
		RerankTopK:     config.RerankTopK,
		RerankerModel:  config.RerankerModel,
		RetrievalLimit: config.RetrievalLimit,
		RetrievalMode:  config.RetrievalMode,
		UpdateStrategy: config.UpdateStrategy,
		UpdatedAt:      now,
		VectorWeight:   config.VectorWeight,
	}, nil
}

func (s *SQLStore) UpdateKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID, name string) (KnowledgeBase, error) {
	var base KnowledgeBase

	if err := s.db.QueryRowContext(ctx, `
		UPDATE knowledge_bases
		SET name = $3, updated_at = $4
		WHERE (organization_id = $1 OR workspace_id = $1) AND id = $2
		RETURNING id, name, document_count, updated_at
	`, workspaceID, knowledgeBaseID, name, time.Now().UTC()).Scan(&base.ID, &base.Name, &base.DocumentCount, &base.UpdatedAt); err != nil {
		return KnowledgeBase{}, err
	}

	return base, nil
}

func (s *SQLStore) UpdateKnowledgeBaseWithConfig(ctx context.Context, organizationID, knowledgeBaseID, name string, config KnowledgeBaseConfig) (KnowledgeBase, error) {
	config = normalizeKnowledgeBaseConfig(config)
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		UPDATE knowledge_bases
		SET name = $3,
			retrieval_mode = $4,
			retrieval_limit = $5,
			min_score = $6,
			vector_weight = $7,
			keyword_weight = $8,
			reranker_model = $9,
			rerank_top_k = $10,
			chunk_strategy = $11,
			chunk_size = $12,
			chunk_overlap = $13,
			embedding_model = $14,
			update_strategy = $15,
			updated_at = $16
		WHERE organization_id = $1 AND id = $2
	`, organizationID, knowledgeBaseID, name, config.RetrievalMode, config.RetrievalLimit, config.MinScore, config.VectorWeight, config.KeywordWeight, config.RerankerModel, config.RerankTopK, config.ChunkStrategy, config.ChunkSize, config.ChunkOverlap, config.EmbeddingModel, config.UpdateStrategy, now)
	if err != nil {
		return KnowledgeBase{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return KnowledgeBase{}, err
	}
	if rowsAffected == 0 {
		return KnowledgeBase{}, sql.ErrNoRows
	}
	return KnowledgeBase{
		ChunkOverlap:   config.ChunkOverlap,
		ChunkSize:      config.ChunkSize,
		ChunkStrategy:  config.ChunkStrategy,
		EmbeddingModel: config.EmbeddingModel,
		ID:             knowledgeBaseID,
		KeywordWeight:  config.KeywordWeight,
		MinScore:       config.MinScore,
		Name:           name,
		RerankTopK:     config.RerankTopK,
		RerankerModel:  config.RerankerModel,
		RetrievalLimit: config.RetrievalLimit,
		RetrievalMode:  config.RetrievalMode,
		UpdateStrategy: config.UpdateStrategy,
		UpdatedAt:      now,
		VectorWeight:   config.VectorWeight,
	}, nil
}

type knowledgeBaseScanner interface {
	Scan(dest ...any) error
}

func scanKnowledgeBase(scanner knowledgeBaseScanner, base *KnowledgeBase) error {
	if err := scanner.Scan(
		&base.ID,
		&base.Name,
		&base.DocumentCount,
		&base.RetrievalMode,
		&base.RetrievalLimit,
		&base.MinScore,
		&base.VectorWeight,
		&base.KeywordWeight,
		&base.RerankerModel,
		&base.RerankTopK,
		&base.ChunkStrategy,
		&base.ChunkSize,
		&base.ChunkOverlap,
		&base.EmbeddingModel,
		&base.UpdateStrategy,
		&base.UpdatedAt,
	); err != nil {
		return err
	}
	*base = knowledgeBaseWithNormalizedConfig(*base)
	return nil
}

func knowledgeBaseWithNormalizedConfig(base KnowledgeBase) KnowledgeBase {
	config := normalizeKnowledgeBaseConfig(KnowledgeBaseConfig{
		ChunkOverlap:   base.ChunkOverlap,
		ChunkSize:      base.ChunkSize,
		ChunkStrategy:  base.ChunkStrategy,
		EmbeddingModel: base.EmbeddingModel,
		KeywordWeight:  base.KeywordWeight,
		MinScore:       base.MinScore,
		RerankTopK:     base.RerankTopK,
		RerankerModel:  base.RerankerModel,
		RetrievalLimit: base.RetrievalLimit,
		RetrievalMode:  base.RetrievalMode,
		UpdateStrategy: base.UpdateStrategy,
		VectorWeight:   base.VectorWeight,
	})
	base.ChunkOverlap = config.ChunkOverlap
	base.ChunkSize = config.ChunkSize
	base.ChunkStrategy = config.ChunkStrategy
	base.EmbeddingModel = config.EmbeddingModel
	base.KeywordWeight = config.KeywordWeight
	base.MinScore = config.MinScore
	base.RerankTopK = config.RerankTopK
	base.RerankerModel = config.RerankerModel
	base.RetrievalLimit = config.RetrievalLimit
	base.RetrievalMode = config.RetrievalMode
	base.UpdateStrategy = config.UpdateStrategy
	base.VectorWeight = config.VectorWeight
	return base
}

func (s *SQLStore) DeleteKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM knowledge_bases
		WHERE (organization_id = $1 OR workspace_id = $1) AND id = $2
	`, workspaceID, knowledgeBaseID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *SQLStore) ListKnowledgeBases(ctx context.Context, workspaceID string) ([]KnowledgeBase, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			name,
			document_count,
			COALESCE(retrieval_mode, ''),
			retrieval_limit,
			min_score,
			vector_weight,
			keyword_weight,
			COALESCE(reranker_model, ''),
			rerank_top_k,
			COALESCE(chunk_strategy, ''),
			chunk_size,
			chunk_overlap,
			COALESCE(embedding_model, ''),
			COALESCE(update_strategy, ''),
			updated_at
		FROM knowledge_bases
		WHERE organization_id = $1 OR workspace_id = $1
		ORDER BY updated_at DESC, name ASC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bases := []KnowledgeBase{}
	for rows.Next() {
		var base KnowledgeBase
		if err := scanKnowledgeBase(rows, &base); err != nil {
			return nil, err
		}
		bases = append(bases, base)
	}

	return bases, rows.Err()
}

func (s *SQLStore) GetKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) (KnowledgeBase, error) {
	var base KnowledgeBase

	if err := scanKnowledgeBase(s.db.QueryRowContext(ctx, `
		SELECT
			id,
			name,
			document_count,
			COALESCE(retrieval_mode, ''),
			retrieval_limit,
			min_score,
			vector_weight,
			keyword_weight,
			COALESCE(reranker_model, ''),
			rerank_top_k,
			COALESCE(chunk_strategy, ''),
			chunk_size,
			chunk_overlap,
			COALESCE(embedding_model, ''),
			COALESCE(update_strategy, ''),
			updated_at
		FROM knowledge_bases
		WHERE (organization_id = $1 OR workspace_id = $1) AND id = $2
	`, workspaceID, knowledgeBaseID), &base); err != nil {
		return KnowledgeBase{}, err
	}

	return base, nil
}

func (s *SQLStore) ListKnowledgeDocuments(ctx context.Context, workspaceID, knowledgeBaseID string) ([]KnowledgeDocument, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.title, d.content, d.updated_at
		FROM knowledge_documents d
		JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
		WHERE (kb.organization_id = $1 OR kb.workspace_id = $1) AND d.knowledge_base_id = $2
		ORDER BY d.updated_at DESC, d.title ASC
	`, workspaceID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	documents := []KnowledgeDocument{}
	for rows.Next() {
		var document KnowledgeDocument
		if err := rows.Scan(&document.ID, &document.Title, &document.Content, &document.UpdatedAt); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}

	return documents, rows.Err()
}

func (s *SQLStore) CreateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, title, content string) (KnowledgeDocument, error) {
	return s.CreateKnowledgeDocumentWithOptions(ctx, workspaceID, knowledgeBaseID, title, content, chunksFromContent(content), KnowledgeDocumentOptions{})
}

func (s *SQLStore) CreateKnowledgeDocumentWithOptions(ctx context.Context, organizationID, knowledgeBaseID, title, content string, chunks []KnowledgeDocumentChunk, options KnowledgeDocumentOptions) (KnowledgeDocument, error) {
	options = normalizeKnowledgeDocumentOptions(options)
	documentID, err := auth.NewID("doc")
	if err != nil {
		return KnowledgeDocument{}, err
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO knowledge_documents (id, organization_id, knowledge_base_id, title, content, document_version, update_strategy, created_at, updated_at)
		SELECT $1, $2, kb.id, $4, $5, $6, $7, $8, $8
		FROM knowledge_bases kb
		WHERE kb.organization_id = $2 AND kb.id = $3
	`, documentID, organizationID, knowledgeBaseID, title, content, options.DocumentVersion, options.UpdateStrategy, now)
	if err != nil {
		return KnowledgeDocument{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return KnowledgeDocument{}, err
	}
	if rowsAffected == 0 {
		return KnowledgeDocument{}, sql.ErrNoRows
	}

	if err := replaceKnowledgeDocumentChunksWithOptions(ctx, tx, organizationID, documentID, chunks, options, defaultKnowledgeStoreEmbeddingModel, now); err != nil {
		return KnowledgeDocument{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE knowledge_bases
		SET document_count = document_count + 1, updated_at = $2
		WHERE organization_id = $1 AND id = $3
	`, organizationID, now, knowledgeBaseID); err != nil {
		return KnowledgeDocument{}, err
	}

	if err := tx.Commit(); err != nil {
		return KnowledgeDocument{}, err
	}

	return KnowledgeDocument{
		Content:         content,
		DocumentVersion: options.DocumentVersion,
		ID:              documentID,
		Title:           title,
		UpdateStrategy:  options.UpdateStrategy,
		UpdatedAt:       now,
	}, nil
}

func (s *SQLStore) UpdateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, documentID, title, content string) (KnowledgeDocument, error) {
	return s.UpdateKnowledgeDocumentWithOptions(ctx, workspaceID, knowledgeBaseID, documentID, title, content, chunksFromContent(content), KnowledgeDocumentOptions{})
}

func (s *SQLStore) UpdateKnowledgeDocumentWithOptions(ctx context.Context, organizationID, knowledgeBaseID, documentID, title, content string, chunks []KnowledgeDocumentChunk, options KnowledgeDocumentOptions) (KnowledgeDocument, error) {
	options = normalizeKnowledgeDocumentOptions(options)
	now := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE knowledge_documents d
		SET title = $4,
			content = $5,
			document_version = $6,
			update_strategy = $7,
			updated_at = $8
		FROM knowledge_bases kb
		WHERE d.knowledge_base_id = kb.id
		  AND kb.organization_id = $1
		  AND kb.id = $2
		  AND d.id = $3
	`, organizationID, knowledgeBaseID, documentID, title, content, options.DocumentVersion, options.UpdateStrategy, now)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return KnowledgeDocument{}, err
	}
	if rowsAffected == 0 {
		return KnowledgeDocument{}, sql.ErrNoRows
	}

	if err := replaceKnowledgeDocumentChunksWithOptions(ctx, tx, organizationID, documentID, chunks, options, defaultKnowledgeStoreEmbeddingModel, now); err != nil {
		return KnowledgeDocument{}, err
	}

	if err := tx.Commit(); err != nil {
		return KnowledgeDocument{}, err
	}

	return KnowledgeDocument{
		Content:         content,
		DocumentVersion: options.DocumentVersion,
		ID:              documentID,
		Title:           title,
		UpdateStrategy:  options.UpdateStrategy,
		UpdatedAt:       now,
	}, nil
}

func (s *SQLStore) DeleteKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, documentID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		DELETE FROM knowledge_documents d
		USING knowledge_bases kb
		WHERE d.knowledge_base_id = kb.id AND (kb.organization_id = $1 OR kb.workspace_id = $1) AND kb.id = $2 AND d.id = $3
	`, workspaceID, knowledgeBaseID, documentID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE knowledge_bases
		SET document_count = GREATEST(document_count - 1, 0), updated_at = $2
		WHERE (organization_id = $1 OR workspace_id = $1) AND id = $3
	`, workspaceID, time.Now().UTC(), knowledgeBaseID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLStore) RetrieveKnowledge(ctx context.Context, workspaceID, knowledgeBaseID, query string) ([]KnowledgeRetrievalResult, error) {
	normalizedQuery := normalizeKnowledgeQuery(query)
	if normalizedQuery == "" {
		return []KnowledgeRetrievalResult{}, nil
	}

	pattern := "%" + escapeLikePattern(normalizedQuery) + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.title, d.content, c.content, COALESCE(c.chunk_index, -1), d.updated_at
		FROM knowledge_documents d
		JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
		LEFT JOIN knowledge_document_chunks c ON c.document_id = d.id
		WHERE (kb.organization_id = $1 OR kb.workspace_id = $1) AND d.knowledge_base_id = $2 AND (
			d.title ILIKE $3 ESCAPE '\'
			OR d.content ILIKE $3 ESCAPE '\'
			OR c.content ILIKE $3 ESCAPE '\'
		)
		ORDER BY d.updated_at DESC, d.title ASC, COALESCE(c.chunk_index, -1) ASC
		LIMIT 20
	`, workspaceID, knowledgeBaseID, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	terms := buildKnowledgeQueryTerms(normalizedQuery)
	candidates := []knowledgeRetrievalCandidate{}
	for rows.Next() {
		var (
			documentID    string
			documentTitle string
			documentBody  string
			chunkContent  sql.NullString
			chunkIndex    int
			updatedAt     time.Time
		)

		if err := rows.Scan(&documentID, &documentTitle, &documentBody, &chunkContent, &chunkIndex, &updatedAt); err != nil {
			return nil, err
		}

		candidates = append(candidates, knowledgeRetrievalCandidate{
			documentID:    documentID,
			documentTitle: documentTitle,
			documentBody:  documentBody,
			chunkContent:  chunkContent,
			chunkIndex:    chunkIndex,
			updatedAt:     updatedAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]

		leftScore := scoreKnowledgeCandidate(left.documentTitle, left.documentBody, left.chunkContent, terms)
		rightScore := scoreKnowledgeCandidate(right.documentTitle, right.documentBody, right.chunkContent, terms)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if !left.updatedAt.Equal(right.updatedAt) {
			return left.updatedAt.After(right.updatedAt)
		}
		if left.documentTitle != right.documentTitle {
			return left.documentTitle < right.documentTitle
		}
		return left.chunkIndex < right.chunkIndex
	})

	results := make([]KnowledgeRetrievalResult, 0, knowledgeRetrievalLimit)
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		source := chooseKnowledgeSnippetSource(candidate.documentBody, candidate.chunkContent, terms)
		snippet := buildKnowledgeSnippet(source, normalizedQuery)
		if snippet == "" {
			continue
		}

		resultKey := candidate.documentID + "|" + snippet
		if _, exists := seen[resultKey]; exists {
			continue
		}
		seen[resultKey] = struct{}{}

		results = append(results, KnowledgeRetrievalResult{
			DocumentID:    candidate.documentID,
			DocumentTitle: candidate.documentTitle,
			Snippet:       snippet,
		})
		if len(results) == knowledgeRetrievalLimit {
			break
		}
	}

	return results, nil
}

func (s *SQLStore) RetrieveKnowledgeWithOptions(ctx context.Context, organizationID, knowledgeBaseID, query string, queryEmbedding []float32, options KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, error) {
	normalizedQuery := normalizeKnowledgeQuery(query)
	if normalizedQuery == "" {
		return []KnowledgeRetrievalResult{}, nil
	}
	options, err := normalizeKnowledgeRetrievalOptions(options)
	if err != nil {
		return nil, err
	}
	if options.Mode == KnowledgeRetrievalModeVector && len(queryEmbedding) == 0 {
		return []KnowledgeRetrievalResult{}, nil
	}
	if len(queryEmbedding) == 0 {
		options.Mode = KnowledgeRetrievalModeKeyword
		return s.retrieveKnowledgeByKeywordWithOptions(ctx, organizationID, knowledgeBaseID, normalizedQuery, options)
	}

	embeddingVector := formatKnowledgeVector(queryEmbedding)
	rows, err := s.db.QueryContext(ctx, `
		WITH vector_results AS (
			SELECT
				d.id AS document_id,
				d.title AS document_title,
				COALESCE(d.document_version, '') AS document_version,
				c.id AS chunk_id,
				c.chunk_index AS chunk_index,
				COALESCE(NULLIF(c.document_version, ''), d.document_version, '') AS chunk_version,
				c.content AS content,
				COALESCE(c.metadata, '{}'::jsonb) AS metadata,
				1 - (c.embedding <=> $3::vector) AS similarity,
				(1 - (c.embedding <=> $3::vector)) * $7 AS score,
				$9::text AS retrieval_method,
				c.embedding <=> $3::vector AS vector_distance
			FROM knowledge_document_chunks c
			JOIN knowledge_documents d ON d.id = c.document_id
			JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
			WHERE kb.organization_id = $1
			  AND d.organization_id = $1
			  AND c.organization_id = $1
			  AND d.knowledge_base_id = $2
			  AND c.embedding IS NOT NULL
			  AND ($10::text = '' OR COALESCE(NULLIF(c.document_version, ''), d.document_version, '') = $10)
			  AND $3::vector IS NOT NULL
			  AND (1 - (c.embedding <=> $3::vector)) >= $5
			  AND $8::text IN ('vector_only', 'hybrid', 'hybrid_rerank')
			ORDER BY c.embedding <=> $3::vector, d.updated_at DESC, d.title ASC, c.chunk_index ASC
			LIMIT $4
		),
		keyword_query AS (
			SELECT websearch_to_tsquery('simple', $6) AS query
		),
		keyword_results AS (
			SELECT
				d.id AS document_id,
				d.title AS document_title,
				COALESCE(d.document_version, '') AS document_version,
				c.id AS chunk_id,
				c.chunk_index AS chunk_index,
				COALESCE(NULLIF(c.document_version, ''), d.document_version, '') AS chunk_version,
				c.content AS content,
				COALESCE(c.metadata, '{}'::jsonb) AS metadata,
				0::double precision AS similarity,
				ts_rank_cd(
					setweight(to_tsvector('simple', COALESCE(d.title, '')), 'A') ||
					setweight(to_tsvector('simple', COALESCE(c.content, '')), 'B'),
					keyword_query.query
				) * $11 AS score,
				$9::text AS retrieval_method,
				NULL::double precision AS vector_distance
			FROM knowledge_document_chunks c
			JOIN knowledge_documents d ON d.id = c.document_id
			JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
			CROSS JOIN keyword_query
			WHERE kb.organization_id = $1
			  AND d.organization_id = $1
			  AND c.organization_id = $1
			  AND d.knowledge_base_id = $2
			  AND ($10::text = '' OR COALESCE(NULLIF(c.document_version, ''), d.document_version, '') = $10)
			  AND $8::text IN ('keyword', 'hybrid', 'hybrid_rerank')
			  AND keyword_query.query @@ (
				setweight(to_tsvector('simple', COALESCE(d.title, '')), 'A') ||
				setweight(to_tsvector('simple', COALESCE(c.content, '')), 'B')
			  )
			ORDER BY score DESC, d.updated_at DESC, d.title ASC, c.chunk_index ASC
			LIMIT $4
		),
		fused AS (
			SELECT DISTINCT ON (chunk_id)
				document_id,
				document_title,
				document_version,
				chunk_id,
				chunk_index,
				chunk_version,
				content,
				metadata,
				MAX(similarity) OVER (PARTITION BY chunk_id) AS "Similarity",
				SUM(score) OVER (PARTITION BY chunk_id) AS score,
				retrieval_method AS "RetrievalMethod",
				NULL::jsonb AS "KnowledgeCitation",
				vector_distance
			FROM (
				SELECT * FROM vector_results
				UNION ALL
				SELECT * FROM keyword_results
			) merged
			ORDER BY chunk_id, score DESC
		)
		SELECT
			document_id,
			document_title,
			document_version,
			chunk_id,
			chunk_index,
			chunk_version,
			content,
			metadata,
			"Similarity",
			score,
			"RetrievalMethod"
		FROM fused
		ORDER BY score DESC, vector_distance ASC NULLS LAST, document_title ASC, chunk_index ASC
		LIMIT $4
	`, organizationID, knowledgeBaseID, embeddingVector, options.Limit, options.MinScore, normalizedQuery, options.VectorWeight, options.Mode, KnowledgeRetrievalMethodEmbeddingRAG, options.DocumentVersion, options.KeywordWeight)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanKnowledgeRetrievalRows(rows, normalizedQuery)
}

func (s *SQLStore) retrieveKnowledgeByKeywordWithOptions(ctx context.Context, organizationID, knowledgeBaseID, query string, options KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH keyword_query AS (
			SELECT websearch_to_tsquery('simple', $3) AS query
		)
		SELECT
			d.id AS document_id,
			d.title AS document_title,
			COALESCE(d.document_version, '') AS document_version,
			c.id AS chunk_id,
			c.chunk_index AS chunk_index,
			COALESCE(NULLIF(c.document_version, ''), d.document_version, '') AS chunk_version,
			c.content AS content,
			COALESCE(c.metadata, '{}'::jsonb) AS metadata,
			0::double precision AS "Similarity",
			ts_rank_cd(
				setweight(to_tsvector('simple', COALESCE(d.title, '')), 'A') ||
				setweight(to_tsvector('simple', COALESCE(c.content, '')), 'B'),
				keyword_query.query
			) AS score,
			$6::text AS "RetrievalMethod"
		FROM knowledge_document_chunks c
		JOIN knowledge_documents d ON d.id = c.document_id
		JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
		CROSS JOIN keyword_query
		WHERE kb.organization_id = $1
		  AND d.organization_id = $1
		  AND c.organization_id = $1
		  AND d.knowledge_base_id = $2
		  AND ($5::text = '' OR COALESCE(NULLIF(c.document_version, ''), d.document_version, '') = $5)
		  AND keyword_query.query @@ (
			setweight(to_tsvector('simple', COALESCE(d.title, '')), 'A') ||
			setweight(to_tsvector('simple', COALESCE(c.content, '')), 'B')
		  )
		ORDER BY score DESC, d.updated_at DESC, d.title ASC, c.chunk_index ASC
		LIMIT $4
	`, organizationID, knowledgeBaseID, query, options.Limit, options.DocumentVersion, KnowledgeRetrievalMethodEmbeddingRAG)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanKnowledgeRetrievalRows(rows, query)
}

func (s *SQLStore) CreateRetrievalTestCase(ctx context.Context, organizationID, knowledgeBaseID string, req CreateKnowledgeRetrievalTestCaseRequest) (KnowledgeRetrievalTestCase, error) {
	testCaseID, err := auth.NewID("krtc")
	if err != nil {
		return KnowledgeRetrievalTestCase{}, err
	}
	resultJSON, err := json.Marshal(req.ExpectedResult)
	if err != nil {
		return KnowledgeRetrievalTestCase{}, err
	}
	expected := req.ExpectedResult
	var testCase KnowledgeRetrievalTestCase
	if err := scanKnowledgeRetrievalTestCase(s.db.QueryRowContext(ctx, `
		INSERT INTO knowledge_retrieval_test_cases (
			id,
			organization_id,
			knowledge_base_id,
			query,
			expected_document_id,
			expected_document_title,
			expected_document_version,
			expected_chunk_id,
			expected_chunk_index,
			expected_snippet,
			expected_result,
			created_at,
			updated_at
		)
		SELECT
			$3,
			$1,
			kb.id,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11::jsonb,
			$12,
			$12
		FROM knowledge_bases kb
		WHERE kb.organization_id = $1 AND kb.id = $2
		RETURNING
			id,
			knowledge_base_id,
			query,
			expected_document_id,
			expected_document_title,
			expected_document_version,
			expected_chunk_id,
			expected_chunk_index,
			expected_snippet,
			expected_result
	`, organizationID, knowledgeBaseID, testCaseID, req.Query, expected.DocumentID, expected.DocumentTitle, expected.DocumentVersion, expected.ChunkID, expected.ChunkIndex, expected.Snippet, string(resultJSON), time.Now().UTC()), &testCase); err != nil {
		return KnowledgeRetrievalTestCase{}, err
	}
	return testCase, nil
}

func (s *SQLStore) ListRetrievalTestCases(ctx context.Context, organizationID, knowledgeBaseID string) ([]KnowledgeRetrievalTestCase, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			knowledge_base_id,
			query,
			expected_document_id,
			expected_document_title,
			expected_document_version,
			expected_chunk_id,
			expected_chunk_index,
			expected_snippet,
			expected_result
		FROM knowledge_retrieval_test_cases
		WHERE organization_id = $1 AND knowledge_base_id = $2
		ORDER BY created_at DESC, id DESC
	`, organizationID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	testCases := []KnowledgeRetrievalTestCase{}
	for rows.Next() {
		var testCase KnowledgeRetrievalTestCase
		if err := scanKnowledgeRetrievalTestCase(rows, &testCase); err != nil {
			return nil, err
		}
		testCases = append(testCases, testCase)
	}
	return testCases, rows.Err()
}

func scanKnowledgeRetrievalTestCase(scanner interface{ Scan(dest ...any) error }, testCase *KnowledgeRetrievalTestCase) error {
	var expectedResultRaw []byte
	if err := scanner.Scan(
		&testCase.ID,
		&testCase.KnowledgeBaseID,
		&testCase.Query,
		&testCase.ExpectedDocumentID,
		&testCase.ExpectedDocumentTitle,
		&testCase.ExpectedDocumentVersion,
		&testCase.ExpectedChunkID,
		&testCase.ExpectedChunkIndex,
		&testCase.ExpectedSnippet,
		&expectedResultRaw,
	); err != nil {
		return err
	}
	if len(expectedResultRaw) > 0 {
		if err := json.Unmarshal(expectedResultRaw, &testCase.ExpectedResult); err != nil {
			return err
		}
	}
	return nil
}

func scanKnowledgeRetrievalRows(rows *sql.Rows, query string) ([]KnowledgeRetrievalResult, error) {
	results := []KnowledgeRetrievalResult{}
	for rows.Next() {
		var (
			documentID      string
			documentTitle   string
			documentVersion string
			chunkID         string
			chunkIndex      int
			chunkVersion    string
			content         string
			metadataRaw     []byte
			similarity      float64
			score           float64
			retrievalMethod string
		)
		if err := rows.Scan(
			&documentID,
			&documentTitle,
			&documentVersion,
			&chunkID,
			&chunkIndex,
			&chunkVersion,
			&content,
			&metadataRaw,
			&similarity,
			&score,
			&retrievalMethod,
		); err != nil {
			return nil, err
		}

		if strings.TrimSpace(chunkVersion) != "" {
			documentVersion = strings.TrimSpace(chunkVersion)
		}
		metadata := KnowledgeChunkMetadata{}
		if len(metadataRaw) > 0 {
			_ = json.Unmarshal(metadataRaw, &metadata)
		}
		if strings.TrimSpace(metadata.DocumentVersion) != "" {
			documentVersion = strings.TrimSpace(metadata.DocumentVersion)
		}

		snippet := buildKnowledgeSnippet(content, query)
		if snippet == "" {
			snippet = strings.Join(strings.Fields(content), " ")
		}
		source := KnowledgeCitation{
			DocumentID:         documentID,
			DocumentTitle:      documentTitle,
			DocumentVersion:    documentVersion,
			ChunkID:            chunkID,
			ChunkIndex:         chunkIndex,
			HighlightPositions: buildKnowledgeHighlightPositions(content, query),
			PageNumber:         metadata.PageNumber,
			SourceURL:          metadata.SourceURL,
			OriginalText:       content,
			MatchedSnippet:     snippet,
			ConfidenceScore:    similarity,
		}

		results = append(results, KnowledgeRetrievalResult{
			ChunkID:         chunkID,
			ChunkIndex:      chunkIndex,
			DocumentID:      documentID,
			DocumentTitle:   documentTitle,
			DocumentVersion: documentVersion,
			RetrievalMethod: retrievalMethod,
			Score:           score,
			Similarity:      similarity,
			Snippet:         snippet,
			Source:          source,
		})
	}
	return results, rows.Err()
}

func buildKnowledgeHighlightPositions(content, query string) []HighlightPosition {
	query = strings.TrimSpace(query)
	if content == "" || query == "" {
		return nil
	}

	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)
	positions := []HighlightPosition{}

	index := 0
	for {
		position := strings.Index(lowerContent[index:], lowerQuery)
		if position < 0 {
			break
		}
		startByte := index + position
		positions = append(positions, HighlightPosition{
			Start: knowledgeByteToRuneOffset(content, startByte),
			End:   knowledgeByteToRuneOffset(content, startByte+len(query)),
		})
		index = startByte + len(query)
	}
	return positions
}

func knowledgeByteToRuneOffset(value string, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset >= len(value) {
		return len([]rune(value))
	}
	return len([]rune(value[:byteOffset]))
}

func formatKnowledgeVector(embedding []float32) string {
	if len(embedding) == 0 {
		return ""
	}
	parts := make([]string, len(embedding))
	for index, value := range embedding {
		parts[index] = fmt.Sprintf("%f", value)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func replaceKnowledgeDocumentChunks(ctx context.Context, tx *sql.Tx, documentID, content string, now time.Time) error {
	return replaceKnowledgeDocumentChunksWithOptions(ctx, tx, "", documentID, chunksFromContent(content), KnowledgeDocumentOptions{}, defaultKnowledgeStoreEmbeddingModel, now)
}

func replaceKnowledgeDocumentChunksWithOptions(ctx context.Context, tx *sql.Tx, organizationID, documentID string, chunks []KnowledgeDocumentChunk, options KnowledgeDocumentOptions, embeddingModel string, now time.Time) error {
	options = normalizeKnowledgeDocumentOptions(options)
	if options.UpdateStrategy == KnowledgeUpdateStrategyVersioned {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM knowledge_document_chunks
			WHERE document_id = $1
			  AND document_version = $2
		`, documentID, options.DocumentVersion); err != nil {
			return err
		}
	} else if _, err := tx.ExecContext(ctx, `
		DELETE FROM knowledge_document_chunks
		WHERE document_id = $1
	`, documentID); err != nil {
		return err
	}

	if embeddingModel = strings.TrimSpace(embeddingModel); embeddingModel == "" {
		embeddingModel = defaultKnowledgeStoreEmbeddingModel
	}
	if len(chunks) == 0 {
		chunks = []KnowledgeDocumentChunk{}
	}
	for index, chunk := range chunks {
		chunkID, err := auth.NewID("kdc")
		if err != nil {
			return err
		}
		chunk.ChunkIndex = index
		if strings.TrimSpace(chunk.DocumentVersion) == "" {
			chunk.DocumentVersion = options.DocumentVersion
		}
		if strings.TrimSpace(chunk.Metadata.DocumentVersion) == "" {
			chunk.Metadata.DocumentVersion = chunk.DocumentVersion
		}
		if chunk.Metadata.PageNumber == 0 {
			chunk.Metadata.PageNumber = options.PageNumber
		}
		if strings.TrimSpace(chunk.Metadata.SourceURL) == "" {
			chunk.Metadata.SourceURL = options.SourceURL
		}
		metadataJSON, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO knowledge_document_chunks (id, document_id, organization_id, chunk_index, content, embedding, embedding_model, indexed_at, document_version, metadata, created_at)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::vector, $7, $8, $9, $10::jsonb, $11)
		`, chunkID, documentID, organizationID, chunk.ChunkIndex, chunk.Content, formatKnowledgeVector(chunk.Embedding), embeddingModel, now, chunk.DocumentVersion, string(metadataJSON), now); err != nil {
			return err
		}
	}

	return nil
}

func chunksFromContent(content string) []KnowledgeDocumentChunk {
	rawChunks := buildKnowledgeDocumentChunksWithConfig(content, normalizeKnowledgeBaseConfig(KnowledgeBaseConfig{}))
	chunks := make([]KnowledgeDocumentChunk, 0, len(rawChunks))
	for index, chunk := range rawChunks {
		chunks = append(chunks, KnowledgeDocumentChunk{
			ChunkIndex:          index,
			Content:             chunk.Content,
			DocumentVersion:     "v1",
			EstimatedTokenCount: estimateKnowledgeTokens(chunk.Content),
			Metadata:            KnowledgeChunkMetadata{DocumentVersion: "v1", StartRune: chunk.StartRune, EndRune: chunk.EndRune},
		})
	}
	return chunks
}

type configuredKnowledgeChunk struct {
	Content   string
	EndRune   int
	StartRune int
}

func buildKnowledgeDocumentChunks(content string) []configuredKnowledgeChunk {
	return buildKnowledgeDocumentChunksWithConfig(content, normalizeKnowledgeBaseConfig(KnowledgeBaseConfig{}))
}

func buildKnowledgeDocumentChunksWithConfig(content string, config KnowledgeBaseConfig) []configuredKnowledgeChunk {
	config = normalizeKnowledgeBaseConfig(config)
	switch normalizeKnowledgeChunkStrategy(config.ChunkStrategy) {
	case KnowledgeChunkStrategyFixedSize:
		return buildFixedSizeKnowledgeDocumentChunks(content, config.ChunkSize, config.ChunkOverlap)
	case KnowledgeChunkStrategyQASplit:
		return buildQAKnowledgeDocumentChunks(content, config.ChunkSize)
	case KnowledgeChunkStrategyTemplate:
		return buildTemplateKnowledgeDocumentChunks(content, config.ChunkSize)
	case KnowledgeChunkStrategySemantic:
		return buildSemanticKnowledgeDocumentChunks(content, config.ChunkSize)
	}
	return buildSemanticKnowledgeDocumentChunks(content, config.ChunkSize)
}

func normalizeKnowledgeChunkStrategy(strategy string) string {
	switch strings.TrimSpace(strategy) {
	case KnowledgeChunkStrategyFixedSize:
		return KnowledgeChunkStrategyFixedSize
	case KnowledgeChunkStrategyQASplit:
		return KnowledgeChunkStrategyQASplit
	case KnowledgeChunkStrategyTemplate, KnowledgeChunkStrategyTemplateBased:
		return KnowledgeChunkStrategyTemplate
	case KnowledgeChunkStrategySemantic:
		return KnowledgeChunkStrategySemantic
	default:
		return KnowledgeChunkStrategySemantic
	}
}

func buildSemanticKnowledgeDocumentChunks(content string, chunkSize int) []configuredKnowledgeChunk {
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	if normalized == "" {
		return nil
	}

	segments := strings.Split(normalized, "\n\n")
	if len(segments) == 1 {
		segments = strings.Split(normalized, "\n")
	}

	chunks := []configuredKnowledgeChunk{}
	runeOffset := 0
	for _, segment := range segments {
		cleaned := strings.Join(strings.Fields(segment), " ")
		if cleaned == "" {
			runeOffset += len([]rune(segment)) + 2
			continue
		}

		chunks = append(chunks, splitConfiguredKnowledgeChunk(cleaned, chunkSize, runeOffset)...)
		runeOffset += len([]rune(segment)) + 2
	}

	return chunks
}

func buildQAKnowledgeDocumentChunks(content string, chunkSize int) []configuredKnowledgeChunk {
	semanticChunks := buildSemanticKnowledgeDocumentChunks(content, chunkSize)
	if len(semanticChunks) == 0 {
		return nil
	}

	chunks := []configuredKnowledgeChunk{}
	group := []configuredKnowledgeChunk{}
	flush := func() {
		if len(group) == 0 {
			return
		}
		parts := make([]string, 0, len(group))
		for _, chunk := range group {
			if text := strings.TrimSpace(chunk.Content); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			chunks = append(chunks, configuredKnowledgeChunk{
				Content:   strings.Join(parts, " "),
				StartRune: group[0].StartRune,
				EndRune:   group[len(group)-1].EndRune,
			})
		}
		group = nil
	}

	for _, chunk := range semanticChunks {
		lower := strings.ToLower(strings.TrimSpace(chunk.Content))
		if strings.HasPrefix(lower, "q:") {
			flush()
		}
		group = append(group, chunk)
		if strings.HasPrefix(lower, "a:") {
			flush()
		}
	}
	flush()
	return chunks
}

func buildTemplateKnowledgeDocumentChunks(content string, chunkSize int) []configuredKnowledgeChunk {
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	if normalized == "" {
		return nil
	}

	sections := splitKnowledgeTemplateSections(normalized)
	chunks := []configuredKnowledgeChunk{}
	runeOffset := 0
	for _, section := range sections {
		text := strings.TrimSpace(section)
		if text == "" {
			runeOffset += len([]rune(section)) + 1
			continue
		}
		chunks = append(chunks, splitConfiguredKnowledgeChunk(text, chunkSize, runeOffset)...)
		runeOffset += len([]rune(section)) + 1
	}
	return chunks
}

func splitKnowledgeTemplateSections(text string) []string {
	lines := strings.Split(text, "\n")
	sections := []string{}
	var current strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") && len(trimmed) > 1 && current.Len() > 0 {
			sections = append(sections, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}
	if current.Len() > 0 {
		sections = append(sections, current.String())
	}
	return sections
}

func buildFixedSizeKnowledgeDocumentChunks(content string, chunkSize, overlap int) []configuredKnowledgeChunk {
	runes := []rune(strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n")))
	if len(runes) == 0 {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize - 1
	}

	chunks := []configuredKnowledgeChunk{}
	for start := 0; start < len(runes); {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		text := strings.TrimSpace(string(runes[start:end]))
		if text != "" {
			chunks = append(chunks, configuredKnowledgeChunk{Content: text, StartRune: start, EndRune: end})
		}
		if end == len(runes) {
			break
		}
		start = end - overlap
	}
	return chunks
}

func splitConfiguredKnowledgeChunk(content string, maxLength, baseOffset int) []configuredKnowledgeChunk {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return nil
	}
	if maxLength <= 0 {
		maxLength = knowledgeDocumentChunkSize
	}

	if len(runes) <= maxLength {
		return []configuredKnowledgeChunk{{Content: string(runes), StartRune: baseOffset, EndRune: baseOffset + len(runes)}}
	}

	chunks := []configuredKnowledgeChunk{}
	start := 0
	for start < len(runes) {
		end := start + maxLength
		if end >= len(runes) {
			text := strings.TrimSpace(string(runes[start:]))
			if text != "" {
				chunks = append(chunks, configuredKnowledgeChunk{Content: text, StartRune: baseOffset + start, EndRune: baseOffset + len(runes)})
			}
			break
		}

		splitAt := end
		for splitAt > start && !unicode.IsSpace(runes[splitAt-1]) {
			splitAt--
		}
		if splitAt == start {
			splitAt = end
		}

		text := strings.TrimSpace(string(runes[start:splitAt]))
		if text != "" {
			chunks = append(chunks, configuredKnowledgeChunk{Content: text, StartRune: baseOffset + start, EndRune: baseOffset + splitAt})
		}
		start = splitAt
		for start < len(runes) && unicode.IsSpace(runes[start]) {
			start++
		}
	}

	return chunks
}

func splitChunk(content string, maxLength int) []string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return nil
	}

	if len(runes) <= maxLength {
		return []string{string(runes)}
	}

	chunks := []string{}
	start := 0
	for start < len(runes) {
		end := start + maxLength
		if end >= len(runes) {
			chunks = append(chunks, strings.TrimSpace(string(runes[start:])))
			break
		}

		splitAt := end
		for splitAt > start && !unicode.IsSpace(runes[splitAt-1]) {
			splitAt--
		}
		if splitAt == start {
			splitAt = end
		}

		chunks = append(chunks, strings.TrimSpace(string(runes[start:splitAt])))
		start = splitAt
		for start < len(runes) && unicode.IsSpace(runes[start]) {
			start++
		}
	}

	return chunks
}

func buildKnowledgeSnippet(content, query string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if normalized == "" {
		return ""
	}

	contentRunes := []rune(normalized)
	lowerContent := strings.ToLower(normalized)
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	matchIndex := strings.Index(lowerContent, lowerQuery)
	if matchIndex == -1 {
		if len(contentRunes) <= knowledgeSnippetSize {
			return normalized
		}
		return strings.TrimSpace(string(contentRunes[:knowledgeSnippetSize])) + "..."
	}

	windowSize := knowledgeSnippetSize
	if len(contentRunes) <= knowledgeSnippetSize {
		if len(contentRunes) > knowledgeSnippetSize/2 {
			windowSize = knowledgeSnippetSize / 2
		} else {
			windowSize = len(contentRunes)
		}
	}

	matchRunes := []rune(normalized[:matchIndex])
	queryRunes := []rune(query)
	start := len(matchRunes) - windowSize/3
	if start < 0 {
		start = 0
	}
	end := start + windowSize
	if end < len(matchRunes)+len(queryRunes) {
		end = len(matchRunes) + len(queryRunes)
	}
	if end > len(contentRunes) {
		end = len(contentRunes)
	}
	if end-start > windowSize && end == len(contentRunes) {
		start = max(0, end-windowSize)
	}

	snippet := strings.TrimSpace(string(contentRunes[start:end]))
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(contentRunes) {
		snippet += "..."
	}
	return snippet
}

func escapeLikePattern(query string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(query)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
