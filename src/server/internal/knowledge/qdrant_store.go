package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type QdrantVectorStore struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewQdrantVectorStore(baseURL, apiKey string) *QdrantVectorStore {
	return &QdrantVectorStore{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *QdrantVectorStore) EnsureKnowledgeBaseCollection(ctx context.Context, organizationID, knowledgeBaseID string, vectorSize int) error {
	if vectorSize <= 0 {
		vectorSize = 1536
	}
	payload := qdrantCreateCollectionRequest{
		Vectors: qdrantVectorConfig{
			Distance: "Cosine",
			Size:     vectorSize,
		},
	}
	return s.doCollectionRequest(ctx, http.MethodPut, organizationID, knowledgeBaseID, payload)
}

func (s *QdrantVectorStore) DeleteKnowledgeBaseCollection(ctx context.Context, organizationID, knowledgeBaseID string) error {
	return s.doCollectionRequest(ctx, http.MethodDelete, organizationID, knowledgeBaseID, nil)
}

func (s *QdrantVectorStore) DeleteKnowledgeDocumentChunks(ctx context.Context, organizationID, knowledgeBaseID, documentID string) error {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil
	}
	return s.doQdrantRequest(ctx, http.MethodPost, qdrantCollectionPath(organizationID, knowledgeBaseID)+"/points/delete", qdrantDeletePointsRequest{
		Filter: qdrantPayloadFilter{
			Must: []qdrantFieldCondition{
				{
					Key: "document_id",
					Match: qdrantMatchValue{
						Value: documentID,
					},
				},
			},
		},
	}, 0)
}

func (s *QdrantVectorStore) UpsertKnowledgeDocumentChunks(ctx context.Context, organizationID, knowledgeBaseID, documentID string, chunks []KnowledgeDocumentChunk) error {
	points := make([]qdrantPoint, 0, len(chunks))
	for _, chunk := range chunks {
		if len(chunk.Embedding) == 0 {
			continue
		}
		version := strings.TrimSpace(chunk.DocumentVersion)
		if version == "" {
			version = strings.TrimSpace(chunk.Metadata.DocumentVersion)
		}
		pointID := qdrantPointID(documentID, chunk.ChunkIndex, version)
		documentTitle := strings.TrimSpace(chunk.DocumentTitle)
		points = append(points, qdrantPoint{
			ID:     pointID,
			Vector: append([]float32(nil), chunk.Embedding...),
			Payload: map[string]any{
				"chunk_index":           chunk.ChunkIndex,
				"content":               chunk.Content,
				"document_id":           documentID,
				"document_title":        documentTitle,
				"document_version":      version,
				"estimated_token_count": chunk.EstimatedTokenCount,
				"page_number":           chunk.Metadata.PageNumber,
				"source_url":            chunk.Metadata.SourceURL,
				"start_rune":            chunk.Metadata.StartRune,
				"end_rune":              chunk.Metadata.EndRune,
			},
		})
	}
	if len(points) == 0 {
		return nil
	}
	return s.doPointsRequest(ctx, organizationID, knowledgeBaseID, qdrantUpsertPointsRequest{Points: points})
}

func (s *QdrantVectorStore) SearchKnowledgeChunks(ctx context.Context, organizationID, knowledgeBaseID, query string, queryEmbedding []float32, options KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, error) {
	options, err := normalizeKnowledgeRetrievalOptions(options)
	if err != nil {
		return nil, err
	}
	if len(queryEmbedding) == 0 {
		return []KnowledgeRetrievalResult{}, nil
	}
	response := qdrantSearchResponse{}
	if err := s.doSearchRequest(ctx, organizationID, knowledgeBaseID, qdrantSearchRequest{
		Limit:          options.Limit,
		ScoreThreshold: options.MinScore,
		Vector:         append([]float32(nil), queryEmbedding...),
		WithPayload:    true,
	}, &response); err != nil {
		return nil, err
	}
	results := make([]KnowledgeRetrievalResult, 0, len(response.Result))
	for _, point := range response.Result {
		payload := point.Payload
		chunkID := fmt.Sprint(point.ID)
		chunkIndex := intFromQdrantPayload(payload["chunk_index"])
		content := stringFromQdrantPayload(payload["content"])
		documentID := stringFromQdrantPayload(payload["document_id"])
		documentTitle := stringFromQdrantPayload(payload["document_title"])
		documentVersion := stringFromQdrantPayload(payload["document_version"])
		source := KnowledgeCitation{
			ChunkID:            chunkID,
			ChunkIndex:         chunkIndex,
			DocumentID:         documentID,
			DocumentTitle:      documentTitle,
			DocumentVersion:    documentVersion,
			HighlightPositions: buildKnowledgeHighlightPositions(content, query),
			MatchedSnippet:     buildKnowledgeSnippet(content, query),
			OriginalText:       content,
			PageNumber:         intFromQdrantPayload(payload["page_number"]),
			SourceURL:          stringFromQdrantPayload(payload["source_url"]),
			ConfidenceScore:    point.Score,
		}
		results = append(results, KnowledgeRetrievalResult{
			ChunkID:         chunkID,
			ChunkIndex:      chunkIndex,
			DocumentID:      documentID,
			DocumentTitle:   documentTitle,
			DocumentVersion: documentVersion,
			RetrievalMethod: KnowledgeRetrievalMethodEmbeddingRAG,
			RetrievalMode:   options.Mode,
			Score:           point.Score,
			Similarity:      point.Score,
			Snippet:         source.MatchedSnippet,
			Source:          source,
		})
	}
	return results, nil
}

type qdrantCreateCollectionRequest struct {
	Vectors qdrantVectorConfig `json:"vectors"`
}

type qdrantVectorConfig struct {
	Distance string `json:"distance"`
	Size     int    `json:"size"`
}

type qdrantUpsertPointsRequest struct {
	Points []qdrantPoint `json:"points"`
}

type qdrantDeletePointsRequest struct {
	Filter qdrantPayloadFilter `json:"filter"`
}

type qdrantPayloadFilter struct {
	Must []qdrantFieldCondition `json:"must"`
}

type qdrantFieldCondition struct {
	Key   string           `json:"key"`
	Match qdrantMatchValue `json:"match"`
}

type qdrantMatchValue struct {
	Value any `json:"value"`
}

type qdrantPoint struct {
	ID      string         `json:"id"`
	Payload map[string]any `json:"payload"`
	Vector  []float32      `json:"vector"`
}

type qdrantSearchRequest struct {
	Limit          int       `json:"limit"`
	ScoreThreshold float64   `json:"score_threshold,omitempty"`
	Vector         []float32 `json:"vector"`
	WithPayload    bool      `json:"with_payload"`
}

type qdrantSearchResponse struct {
	Result []qdrantSearchPoint `json:"result"`
}

type qdrantSearchPoint struct {
	ID      any            `json:"id"`
	Payload map[string]any `json:"payload"`
	Score   float64        `json:"score"`
}

func (s *QdrantVectorStore) doCollectionRequest(ctx context.Context, method, organizationID, knowledgeBaseID string, body any) error {
	return s.doQdrantRequest(ctx, method, qdrantCollectionPath(organizationID, knowledgeBaseID), body, http.StatusNotFound)
}

func (s *QdrantVectorStore) doPointsRequest(ctx context.Context, organizationID, knowledgeBaseID string, body any) error {
	return s.doQdrantRequest(ctx, http.MethodPut, qdrantCollectionPath(organizationID, knowledgeBaseID)+"/points", body, 0)
}

func (s *QdrantVectorStore) doSearchRequest(ctx context.Context, organizationID, knowledgeBaseID string, body any, out any) error {
	responseBody, err := s.doQdrantRequestWithBody(ctx, http.MethodPost, qdrantCollectionPath(organizationID, knowledgeBaseID)+"/points/search", body, 0)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(responseBody, out); err != nil {
		return fmt.Errorf("decode qdrant search response: %w", err)
	}
	return nil
}

func (s *QdrantVectorStore) doQdrantRequest(ctx context.Context, method, path string, body any, ignoredStatus int) error {
	_, err := s.doQdrantRequestWithBody(ctx, method, path, body, ignoredStatus)
	return err
}

func (s *QdrantVectorStore) doQdrantRequestWithBody(ctx context.Context, method, path string, body any, ignoredStatus int) ([]byte, error) {
	if s == nil || s.baseURL == "" {
		return nil, fmt.Errorf("qdrant vector store is not configured")
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode qdrant collection request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	requestURL := s.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return nil, fmt.Errorf("create qdrant request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}
	client := s.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant collection request failed: %w", err)
	}
	defer resp.Body.Close()
	if ignoredStatus != 0 && resp.StatusCode == ignoredStatus {
		return nil, nil
	}
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("qdrant collection request returned %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}
	return responseBody, nil
}

func qdrantCollectionPath(organizationID, knowledgeBaseID string) string {
	return "/collections/" + url.PathEscape(KnowledgeVectorCollectionName(organizationID, knowledgeBaseID))
}

func qdrantPointID(documentID string, chunkIndex int, version string) string {
	parts := []string{documentID, fmt.Sprintf("%d", chunkIndex)}
	if strings.TrimSpace(version) != "" {
		parts = append(parts, version)
	}
	return sanitizeKnowledgeVectorScope(strings.Join(parts, "_"))
}

func stringFromQdrantPayload(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func intFromQdrantPayload(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}
