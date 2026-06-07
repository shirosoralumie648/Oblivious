package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"oblivious/server/internal/knowledge"
)

const (
	defaultRerankerModel = "bge-reranker-large"
	defaultRerankerTopK  = 5
	rerankerEndpoint     = "/rerank"
	rerankerTimeout      = 30 * time.Second
)

// Reranker calls a bge-reranker-large compatible service to reorder retrieval
// results by semantic relevance. Safe for concurrent use.
type Reranker struct {
	model   string
	baseURL string
	apiKey  string
	topK    int
	client  *http.Client
}

// NewReranker creates a Reranker with the given configuration.
func NewReranker(cfg knowledge.RerankerConfig) *Reranker {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultRerankerModel
	}
	topK := cfg.TopK
	if topK <= 0 {
		topK = defaultRerankerTopK
	}
	client := &http.Client{Timeout: rerankerTimeout}
	return &Reranker{
		model:   model,
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		apiKey:  cfg.APIKey,
		topK:    topK,
		client:  client,
	}
}

// Rerank reorders the provided results using the bge-reranker-large model and
// returns at most limit results. If limit <= 0 the configured topK is used.
func (r *Reranker) Rerank(ctx context.Context, query string, results []knowledge.HybridRetrievalResult, limit int) ([]knowledge.HybridRetrievalResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(results) == 0 {
		return results, nil
	}
	if limit <= 0 {
		limit = r.topK
	}

	choices := make([]string, len(results))
	for i, res := range results {
		choices[i] = res.Snippet
		if choices[i] == "" {
			choices[i] = res.Citation.OriginalText
		}
	}

	scored, err := r.callAPI(ctx, query, choices)
	if err != nil {
		// If the reranker service is unavailable, fall through with original ordering.
		return limitResults(results, limit), nil
	}

	// Build reranked slice preserving the original result objects.
	reranked := make([]knowledge.HybridRetrievalResult, 0, len(scored))
	for _, s := range scored {
		if s.Index < 0 || s.Index >= len(results) {
			continue
		}
		entry := results[s.Index]
		entry.Score = s.Score
		entry.RetrievalMethod = knowledge.RetrievalModeHybridRerank
		entry.Citation.ConfidenceScore = s.Score
		reranked = append(reranked, entry)
		if len(reranked) >= limit {
			break
		}
	}
	return reranked, nil
}

// ---------------------------------------------------------------------------
// Reranker HTTP API (Cohere-compatible /rerank)
// ---------------------------------------------------------------------------

type rerankerAPIRequest struct {
	Model   string   `json:"model"`
	Query   string   `json:"query"`
	Choices []string `json:"documents"`
	TopK    int      `json:"top_n,omitempty"`
}

type rerankerAPIResponse struct {
	Results []rerankerAPIScore `json:"results"`
}

type rerankerAPIScore struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

type rerankerResult struct {
	Index int
	Score float64
}

func (r *Reranker) callAPI(ctx context.Context, query string, choices []string) ([]rerankerResult, error) {
	if r.baseURL == "" {
		return nil, fmt.Errorf("reranker: base URL not configured")
	}

	body, err := json.Marshal(rerankerAPIRequest{
		Model:   r.model,
		Query:   query,
		Choices: choices,
		TopK:    r.topK,
	})
	if err != nil {
		return nil, err
	}

	url := r.baseURL + rerankerEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.apiKey)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reranker request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reranker read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reranker api %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp rerankerAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("reranker decode: %w", err)
	}

	results := make([]rerankerResult, len(apiResp.Results))
	for i, s := range apiResp.Results {
		results[i] = rerankerResult{Index: s.Index, Score: s.RelevanceScore}
	}
	return results, nil
}

func limitResults(results []knowledge.HybridRetrievalResult, limit int) []knowledge.HybridRetrievalResult {
	if limit <= 0 || len(results) <= limit {
		return results
	}
	return results[:limit]
}
