package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type TavilyWebSearchProviderConfig struct {
	Endpoint    string
	APIKey      string
	ResultLimit int
	HTTPClient  *http.Client
}

type tavilyWebSearchProvider struct {
	endpoint    string
	apiKey      string
	resultLimit int
	client      *http.Client
}

func NewTavilyWebSearchProvider(config TavilyWebSearchProviderConfig) WebSearchProvider {
	endpoint := strings.TrimSpace(config.Endpoint)
	apiKey := strings.TrimSpace(config.APIKey)
	if endpoint == "" || apiKey == "" {
		return nil
	}
	resultLimit := config.ResultLimit
	if resultLimit < 1 {
		resultLimit = 5
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &tavilyWebSearchProvider{
		endpoint:    endpoint,
		apiKey:      apiKey,
		resultLimit: resultLimit,
		client:      client,
	}
}

func (p *tavilyWebSearchProvider) Search(ctx context.Context, query string) ([]WebSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	payload := map[string]any{
		"api_key":     p.apiKey,
		"query":       query,
		"max_results": p.resultLimit,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal web search request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create web search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("web search request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read web search response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("web search provider error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
			Snippet string `json:"snippet"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("parse web search response: %w", err)
	}

	results := make([]WebSearchResult, 0, len(decoded.Results))
	for _, item := range decoded.Results {
		title := strings.TrimSpace(item.Title)
		url := strings.TrimSpace(item.URL)
		snippet := strings.TrimSpace(item.Content)
		if snippet == "" {
			snippet = strings.TrimSpace(item.Snippet)
		}
		if title == "" && url == "" && snippet == "" {
			continue
		}
		results = append(results, WebSearchResult{
			Title:   title,
			URL:     url,
			Snippet: snippet,
		})
	}
	return results, nil
}
