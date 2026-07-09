package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"oblivious/server/internal/relay/types"
)

const defaultClaudeVersion = "2023-06-01"

type ClaudeAdapter struct {
	baseURL string
	apiKey  string
}

var _ types.ProviderAdapter = (*ClaudeAdapter)(nil)

func NewClaudeAdapter(baseURL, apiKey string) *ClaudeAdapter {
	return &ClaudeAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
	}
}

func (a *ClaudeAdapter) Name() string { return "claude-adapter" }

func (a *ClaudeAdapter) Provider() string { return "claude" }

func (a *ClaudeAdapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SupportsChat:      true,
		SupportsStreaming: true,
	}
}

func (a *ClaudeAdapter) BuildURL(_ string, apiType types.APIType) (string, error) {
	if apiType != types.APITypeChat {
		return "", fmt.Errorf("claude adapter does not support %s", apiType.String())
	}
	if strings.TrimSpace(a.baseURL) == "" {
		return "", fmt.Errorf("missing upstream base URL")
	}
	return strings.TrimRight(a.baseURL, "/") + "/v1/messages", nil
}

func (a *ClaudeAdapter) BuildHeaders(ctx context.Context, model string, apiType types.APIType) (http.Header, error) {
	headers := http.Header{}
	if strings.TrimSpace(a.apiKey) != "" {
		headers.Set("x-api-key", a.apiKey)
	}
	headers.Set("anthropic-version", defaultClaudeVersion)
	headers.Set("Content-Type", "application/json")
	return headers, nil
}

func (a *ClaudeAdapter) ConvertRequest(req *types.ProviderRequest) (*types.ProviderRequest, error) {
	return req, nil
}

func (a *ClaudeAdapter) ConvertResponse(resp []byte, isStream bool) (*types.ProviderResponse, error) {
	var payload struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(resp, &payload)
	var usage *types.Usage
	if payload.Usage.InputTokens > 0 || payload.Usage.OutputTokens > 0 {
		usage = &types.Usage{
			PromptTokens:     payload.Usage.InputTokens,
			CompletionTokens: payload.Usage.OutputTokens,
			TotalTokens:      payload.Usage.InputTokens + payload.Usage.OutputTokens,
		}
	}
	return &types.ProviderResponse{StatusCode: http.StatusOK, Content: resp, Done: true, Usage: usage}, nil
}

func (a *ClaudeAdapter) DoRequest(ctx context.Context, req *types.ProviderRequest) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("missing provider request")
	}

	upstreamURL := req.URL
	if upstreamURL == "" {
		var err error
		upstreamURL, err = a.BuildURL(req.Model, req.APIType)
		if err != nil {
			return nil, err
		}
	}

	body := req.Body
	if len(body) == 0 {
		marshaled, err := marshalClaudeProviderRequest(req)
		if err != nil {
			return nil, err
		}
		body = marshaled
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := validateProviderUpstreamURL(upstreamReq.URL.String()); err != nil {
		return nil, err
	}
	upstreamReq.Header = req.Headers.Clone()
	if upstreamReq.Header == nil {
		upstreamReq.Header = http.Header{}
	}
	if upstreamReq.Header.Get("x-api-key") == "" {
		headers, err := a.BuildHeaders(ctx, req.Model, req.APIType)
		if err != nil {
			return nil, err
		}
		for key, values := range headers {
			for _, value := range values {
				upstreamReq.Header.Add(key, value)
			}
		}
	}
	if upstreamReq.Header.Get("anthropic-version") == "" {
		upstreamReq.Header.Set("anthropic-version", defaultClaudeVersion)
	}
	if upstreamReq.Header.Get("Content-Type") == "" {
		upstreamReq.Header.Set("Content-Type", "application/json")
	}

	client := newProviderHTTPClient(60*time.Second, nil)
	return client.Do(upstreamReq)
}

func (a *ClaudeAdapter) HealthCheck(ctx context.Context) error {
	_, err := a.ListModels(ctx)
	return err
}

func (a *ClaudeAdapter) ListModels(ctx context.Context) ([]string, error) {
	if strings.TrimSpace(a.baseURL) == "" {
		return nil, fmt.Errorf("missing upstream base URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(a.baseURL, "/")+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	if err := validateProviderUpstreamURL(req.URL.String()); err != nil {
		return nil, err
	}
	headers, err := a.BuildHeaders(ctx, "", types.APITypeUnknown)
	if err != nil {
		return nil, err
	}
	req.Header = headers

	resp, err := newProviderHTTPClient(10*time.Second, nil).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		if providerErr := a.MapError(resp.StatusCode, body); providerErr != nil {
			return nil, providerErr
		}
		return nil, fmt.Errorf("provider health check failed with status %d", resp.StatusCode)
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse Claude model list: %w", err)
	}
	models := make([]string, 0, len(payload.Data))
	for _, model := range payload.Data {
		if strings.TrimSpace(model.ID) != "" {
			models = append(models, model.ID)
		}
	}
	return models, nil
}

func (a *ClaudeAdapter) MapError(statusCode int, body []byte) *types.ProviderError {
	if statusCode < http.StatusBadRequest {
		return nil
	}

	var payload struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)

	code := payload.Error.Type
	if code == "" {
		code = payload.Type
	}
	if code == "" {
		code = strings.ToLower(strings.ReplaceAll(http.StatusText(statusCode), " ", "_"))
	}
	if code == "" {
		code = "provider_error"
	}

	message := payload.Error.Message
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	if message == "" {
		message = http.StatusText(statusCode)
	}

	return &types.ProviderError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Retryable:  isRetryableProviderStatus(statusCode),
	}
}

func (a *ClaudeAdapter) EstimateUsage(req *types.ProviderRequest) *types.Usage {
	return (&OpenAIAdapter{}).EstimateUsage(req)
}

func marshalClaudeProviderRequest(req *types.ProviderRequest) ([]byte, error) {
	messages := make([]map[string]any, 0, len(req.Messages))
	var system []string
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			if strings.TrimSpace(msg.Content) != "" {
				system = append(system, msg.Content)
			}
		case "assistant", "user":
			messages = append(messages, map[string]any{
				"role":    msg.Role,
				"content": msg.Content,
			})
		default:
			messages = append(messages, map[string]any{
				"role":    "user",
				"content": msg.Content,
			})
		}
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	payload := map[string]any{
		"model":      req.Model,
		"max_tokens": maxTokens,
		"messages":   messages,
		"stream":     req.Stream,
	}
	if len(system) > 0 {
		payload["system"] = strings.Join(system, "\n\n")
	}
	return json.Marshal(payload)
}
