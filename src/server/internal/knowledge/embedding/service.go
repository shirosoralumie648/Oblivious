package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultModel        = "text-embedding-3-small"
	defaultBaseURL      = ""
	embeddingEndpoint   = "/embeddings"
	maxBatchSize        = 2048
	defaultHTTPTimeout  = 30 * time.Second
)

// Service wraps the OpenAI-compatible embeddings API.  It is designed to be
// called through the project's relay layer or directly against a provider.
// Safe for concurrent use.
type Service struct {
	model   string
	baseURL string
	apiKey  string
	client  *http.Client

	mu       sync.RWMutex
	embedFn  func(ctx context.Context, model, input string) ([]float32, error)
	batchFn  func(ctx context.Context, model string, inputs []string) ([][]float32, error)
}

// Config holds the configuration for the embedding service.
type Config struct {
	Model   string `json:"model"`
	BaseURL string `json:"baseUrl,omitempty"`
	APIKey  string `json:"-"`
	Timeout time.Duration `json:"-"`
}

// Option is a functional option for configuring the Service.
type Option func(*Service)

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(s *Service) { s.client = c }
}

// WithEmbedFunc overrides the single-text embedding call (useful for relay integration).
func WithEmbedFunc(fn func(ctx context.Context, model, input string) ([]float32, error)) Option {
	return func(s *Service) {
		s.mu.Lock()
		s.embedFn = fn
		s.mu.Unlock()
	}
}

// WithBatchFunc overrides the batch embedding call (useful for relay integration).
func WithBatchFunc(fn func(ctx context.Context, model string, inputs []string) ([][]float32, error)) Option {
	return func(s *Service) {
		s.mu.Lock()
		s.batchFn = fn
		s.mu.Unlock()
	}
}

// NewService creates a new embedding Service.
func NewService(cfg Config, opts ...Option) *Service {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}

	s := &Service{
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  cfg.APIKey,
		client:  &http.Client{Timeout: timeout},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Embed computes the embedding vector for a single text.
func (s *Service) Embed(ctx context.Context, text string) ([]float32, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("embedding: empty text")
	}
	s.mu.RLock()
	fn := s.embedFn
	s.mu.RUnlock()
	if fn != nil {
		return fn(ctx, s.model, text)
	}
	results, err := s.callAPI(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("embedding: empty response")
	}
	return results[0], nil
}

// EmbedBatch computes embeddings for multiple texts in a single request.
// Large batches are automatically split into sub-batches of maxBatchSize.
func (s *Service) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	s.mu.RLock()
	fn := s.batchFn
	s.mu.RUnlock()
	if fn != nil {
		return fn(ctx, s.model, texts)
	}

	allResults := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[start:end]
		results, err := s.callAPI(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embedding batch [%d:%d]: %w", start, end, err)
		}
		allResults = append(allResults, results...)
	}
	return allResults, nil
}

// Model returns the configured embedding model name.
func (s *Service) Model() string { return s.model }

// ---------------------------------------------------------------------------
// OpenAI-compatible HTTP API
// ---------------------------------------------------------------------------

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []embeddingData `json:"data"`
}

type embeddingData struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

func (s *Service) callAPI(ctx context.Context, inputs []string) ([][]float32, error) {
	body, err := json.Marshal(embeddingRequest{
		Model: s.model,
		Input: inputs,
	})
	if err != nil {
		return nil, err
	}

	url := s.baseURL + embeddingEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("embedding read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding api %d: %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("embedding decode response: %w", err)
	}

	results := make([][]float32, len(inputs))
	for _, d := range embResp.Data {
		if d.Index >= 0 && d.Index < len(results) {
			results[d.Index] = append([]float32(nil), d.Embedding...)
		}
	}
	// Validate that every input got an embedding.
	for i, r := range results {
		if len(r) == 0 {
			return nil, fmt.Errorf("embedding: missing result for input %d", i)
		}
	}
	return results, nil
}
