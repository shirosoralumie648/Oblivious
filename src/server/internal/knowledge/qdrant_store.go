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

type qdrantCreateCollectionRequest struct {
	Vectors qdrantVectorConfig `json:"vectors"`
}

type qdrantVectorConfig struct {
	Distance string `json:"distance"`
	Size     int    `json:"size"`
}

func (s *QdrantVectorStore) doCollectionRequest(ctx context.Context, method, organizationID, knowledgeBaseID string, body any) error {
	if s == nil || s.baseURL == "" {
		return fmt.Errorf("qdrant vector store is not configured")
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode qdrant collection request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	collectionName := KnowledgeVectorCollectionName(organizationID, knowledgeBaseID)
	requestURL := s.baseURL + "/collections/" + url.PathEscape(collectionName)
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return fmt.Errorf("create qdrant request: %w", err)
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
		return fmt.Errorf("qdrant collection request failed: %w", err)
	}
	defer resp.Body.Close()
	if method == http.MethodDelete && resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("qdrant collection request returned %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}
	return nil
}
