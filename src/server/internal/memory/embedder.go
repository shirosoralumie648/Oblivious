package memory

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

// RelayEmbedder 通过 Relay 调用嵌入 API
type RelayEmbedder struct {
	client    *http.Client
	relayURL  string
	model     string
	batchSize int
}

// NewRelayEmbedder 创建 RelayEmbedder
func NewRelayEmbedder(relayURL, model string) *RelayEmbedder {
	if model == "" {
		model = "text-embedding-3-small"
	}
	if relayURL == "" {
		relayURL = "http://localhost:8080/v1"
	}
	return &RelayEmbedder{
		client:    &http.Client{Timeout: 60 * time.Second},
		relayURL:  strings.TrimSuffix(relayURL, "/"),
		model:     model,
		batchSize: 100, // OpenAI 批量限制
	}
}

// embeddingRequest 嵌入请求
type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embeddingResponse 嵌入响应
type embeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Embed 嵌入单个文本
func (e *RelayEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedBatch 批量嵌入
func (e *RelayEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// 分批处理
	var allEmbeddings [][]float32
	for i := 0; i < len(texts); i += e.batchSize {
		end := i + e.batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := e.embedBatch(ctx, batch)
		if err != nil {
			return nil, err
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// embedBatch 处理单个批次
func (e *RelayEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	req := embeddingRequest{
		Model: e.model,
		Input: texts,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.relayURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("embedding API error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if embResp.Error != nil {
		return nil, fmt.Errorf("embedding error: %s", embResp.Error.Message)
	}

	// 按 index 排序
	embeddings := make([][]float32, len(texts))
	for _, data := range embResp.Data {
		if data.Index >= 0 && data.Index < len(embeddings) {
			embeddings[data.Index] = data.Embedding
		}
	}

	return embeddings, nil
}

// SetClient 设置自定义 HTTP 客户端
func (e *RelayEmbedder) SetClient(client *http.Client) {
	e.client = client
}

// SetBatchSize 设置批量大小
func (e *RelayEmbedder) SetBatchSize(size int) {
	if size > 0 {
		e.batchSize = size
	}
}
