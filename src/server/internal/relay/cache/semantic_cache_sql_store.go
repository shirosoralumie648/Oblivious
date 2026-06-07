package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type SQLSemanticCacheStore struct {
	db *sql.DB
}

func NewSQLSemanticCacheStore(db *sql.DB) *SQLSemanticCacheStore {
	return &SQLSemanticCacheStore{db: db}
}

func (s *SQLSemanticCacheStore) Get(ctx context.Context, key SemanticCacheKey) (*SemanticCacheEntry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT cache_scope, organization_id, model_id, query_hash, query_text, query_embedding::text, response, hit_count, created_at, expires_at
		FROM relay_semantic_cache
		WHERE cache_scope = $1
		  AND COALESCE(organization_id, '') = $2
		  AND model_id = $3
		  AND query_hash = $4
	`, string(key.Scope), key.OrganizationID, key.Model, key.QueryHash)
	return scanSemanticCacheEntry(row)
}

func (s *SQLSemanticCacheStore) Put(ctx context.Context, entry SemanticCacheEntry) error {
	response := json.RawMessage(`null`)
	if entry.Response != nil {
		response = entry.Response
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO relay_semantic_cache (
			cache_scope, organization_id, model_id, query_hash, query_text, query_embedding, response, hit_count, created_at, expires_at
		)
		VALUES ($1, NULLIF($2, ''), $3, $4, $5, NULLIF($6, '')::vector, $7::jsonb, $8, $9, $10)
		ON CONFLICT (cache_scope, (COALESCE(organization_id, '')), model_id, query_hash)
		DO UPDATE SET
			query_text = EXCLUDED.query_text,
			query_embedding = EXCLUDED.query_embedding,
			response = EXCLUDED.response,
			hit_count = EXCLUDED.hit_count,
			created_at = EXCLUDED.created_at,
			expires_at = EXCLUDED.expires_at
	`, string(entry.Key.Scope), entry.Key.OrganizationID, entry.Key.Model, entry.Key.QueryHash, entry.Query, semanticCacheEmbeddingToVector(entry.QueryEmbedding), string(response), entry.HitCount, entry.CreatedAt, entry.ExpiresAt)
	return err
}

func (s *SQLSemanticCacheStore) IncrementHit(ctx context.Context, key SemanticCacheKey) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE relay_semantic_cache
		SET hit_count = hit_count + 1
		WHERE cache_scope = $1
		  AND COALESCE(organization_id, '') = $2
		  AND model_id = $3
		  AND query_hash = $4
	`, string(key.Scope), key.OrganizationID, key.Model, key.QueryHash)
	return err
}

func (s *SQLSemanticCacheStore) FindSimilar(ctx context.Context, key SemanticCacheKey, _ string) (*SemanticCacheEntry, error) {
	return nil, nil
}

func (s *SQLSemanticCacheStore) FindSimilarByEmbedding(ctx context.Context, key SemanticCacheKey, queryEmbedding []float32) (*SemanticCacheEntry, error) {
	if len(queryEmbedding) == 0 {
		return nil, nil
	}
	vector := semanticCacheEmbeddingToVector(queryEmbedding)
	row := s.db.QueryRowContext(ctx, `
		SELECT cache_scope, organization_id, model_id, query_hash, query_text, query_embedding::text, response, hit_count, created_at, expires_at
		FROM relay_semantic_cache
		WHERE cache_scope = $1
		  AND COALESCE(organization_id, '') = $2
		  AND model_id = $3
		  AND query_hash <> $4
		  AND query_embedding IS NOT NULL
		  AND expires_at > NOW()
		  AND (1 - (query_embedding <=> $5::vector)) >= $6
		ORDER BY query_embedding <=> $5::vector, created_at DESC
		LIMIT 1
	`, string(key.Scope), key.OrganizationID, key.Model, key.QueryHash, vector, semanticCacheEmbeddingSimilarityThreshold)
	return scanSemanticCacheEntry(row)
}

type semanticCacheEntryScanner interface {
	Scan(dest ...any) error
}

func scanSemanticCacheEntry(scanner semanticCacheEntryScanner) (*SemanticCacheEntry, error) {
	var (
		scope          string
		organizationID sql.NullString
		queryEmbedding sql.NullString
		response       []byte
		entry          SemanticCacheEntry
	)
	err := scanner.Scan(
		&scope,
		&organizationID,
		&entry.Key.Model,
		&entry.Key.QueryHash,
		&entry.Query,
		&queryEmbedding,
		&response,
		&entry.HitCount,
		&entry.CreatedAt,
		&entry.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entry.Key.Scope = SemanticCacheScope(scope)
	if organizationID.Valid {
		entry.Key.OrganizationID = organizationID.String
	}
	if queryEmbedding.Valid {
		entry.QueryEmbedding = semanticCacheVectorToEmbedding(queryEmbedding.String)
	}
	entry.Response = cloneRawMessage(json.RawMessage(response))
	return &entry, nil
}

func semanticCacheEmbeddingToVector(embedding []float32) string {
	if len(embedding) == 0 {
		return ""
	}
	parts := make([]string, len(embedding))
	for i, value := range embedding {
		parts[i] = fmt.Sprintf("%f", value)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func semanticCacheVectorToEmbedding(vector string) []float32 {
	vector = strings.TrimSpace(vector)
	if vector == "" || vector == "[]" {
		return nil
	}
	vector = strings.TrimPrefix(strings.TrimSuffix(vector, "]"), "[")
	if strings.TrimSpace(vector) == "" {
		return nil
	}
	parts := strings.Split(vector, ",")
	embedding := make([]float32, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 32)
		if err != nil {
			return nil
		}
		embedding = append(embedding, float32(value))
	}
	return embedding
}
