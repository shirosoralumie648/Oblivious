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

type GeminiAdapter struct {
	baseURL string
	apiKey  string
}

var _ types.ProviderAdapter = (*GeminiAdapter)(nil)

func NewGeminiAdapter(baseURL, apiKey string) *GeminiAdapter {
	return &GeminiAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
	}
}

func (a *GeminiAdapter) Name() string { return "gemini-adapter" }

func (a *GeminiAdapter) Provider() string { return "gemini" }

func (a *GeminiAdapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SupportsChat:       true,
		SupportsStreaming:  true,
		SupportsEmbeddings: true,
	}
}

func (a *GeminiAdapter) BuildURL(model string, apiType types.APIType) (string, error) {
	if strings.TrimSpace(a.baseURL) == "" {
		return "", fmt.Errorf("missing upstream base URL")
	}
	model = normalizeGeminiModel(model)
	if model == "" {
		return "", fmt.Errorf("missing Gemini model")
	}

	switch apiType {
	case types.APITypeChat, types.APITypeResponses, types.APITypeCompletions:
		return strings.TrimRight(a.baseURL, "/") + "/v1beta/models/" + model + ":generateContent", nil
	case types.APITypeEmbeddings:
		return strings.TrimRight(a.baseURL, "/") + "/v1beta/models/" + model + ":embedContent", nil
	default:
		return "", fmt.Errorf("gemini adapter does not support %s", apiType.String())
	}
}

func normalizeGeminiModel(model string) string {
	model = strings.TrimSpace(model)
	model = strings.TrimPrefix(model, "models/")
	return model
}

func (a *GeminiAdapter) BuildHeaders(ctx context.Context, model string, apiType types.APIType) (http.Header, error) {
	headers := http.Header{}
	if strings.TrimSpace(a.apiKey) != "" {
		headers.Set("x-goog-api-key", a.apiKey)
	}
	headers.Set("Content-Type", "application/json")
	return headers, nil
}

func (a *GeminiAdapter) ConvertRequest(req *types.ProviderRequest) (*types.ProviderRequest, error) {
	return req, nil
}

func (a *GeminiAdapter) ConvertResponse(resp []byte, isStream bool) (*types.ProviderResponse, error) {
	var payload struct {
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	_ = json.Unmarshal(resp, &payload)

	var usage *types.Usage
	if payload.UsageMetadata.PromptTokenCount > 0 ||
		payload.UsageMetadata.CandidatesTokenCount > 0 ||
		payload.UsageMetadata.TotalTokenCount > 0 {
		total := payload.UsageMetadata.TotalTokenCount
		if total == 0 {
			total = payload.UsageMetadata.PromptTokenCount + payload.UsageMetadata.CandidatesTokenCount
		}
		usage = &types.Usage{
			PromptTokens:     payload.UsageMetadata.PromptTokenCount,
			CompletionTokens: payload.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      total,
		}
	}

	return &types.ProviderResponse{StatusCode: http.StatusOK, Content: resp, Done: true, Usage: usage}, nil
}

func (a *GeminiAdapter) DoRequest(ctx context.Context, req *types.ProviderRequest) (*http.Response, error) {
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
		marshaled, err := marshalGeminiProviderRequest(req)
		if err != nil {
			return nil, err
		}
		body = marshaled
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	upstreamReq.Header = req.Headers.Clone()
	if upstreamReq.Header == nil {
		upstreamReq.Header = http.Header{}
	}
	if upstreamReq.Header.Get("x-goog-api-key") == "" {
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
	if upstreamReq.Header.Get("Content-Type") == "" {
		upstreamReq.Header.Set("Content-Type", "application/json")
	}

	return (&http.Client{Timeout: 60 * time.Second}).Do(upstreamReq)
}

func (a *GeminiAdapter) HealthCheck(ctx context.Context) error {
	_, err := a.ListModels(ctx)
	return err
}

func (a *GeminiAdapter) ListModels(ctx context.Context) ([]string, error) {
	if strings.TrimSpace(a.baseURL) == "" {
		return nil, fmt.Errorf("missing upstream base URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(a.baseURL, "/")+"/v1beta/models", nil)
	if err != nil {
		return nil, err
	}
	headers, err := a.BuildHeaders(ctx, "", types.APITypeUnknown)
	if err != nil {
		return nil, err
	}
	req.Header = headers

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
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
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse Gemini model list: %w", err)
	}
	models := make([]string, 0, len(payload.Models))
	for _, model := range payload.Models {
		name := normalizeGeminiModel(model.Name)
		if name != "" {
			models = append(models, name)
		}
	}
	return models, nil
}

func (a *GeminiAdapter) MapError(statusCode int, body []byte) *types.ProviderError {
	if statusCode < http.StatusBadRequest {
		return nil
	}

	var payload struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)

	code := payload.Error.Status
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

func (a *GeminiAdapter) EstimateUsage(req *types.ProviderRequest) *types.Usage {
	return (&OpenAIAdapter{}).EstimateUsage(req)
}

func marshalGeminiProviderRequest(req *types.ProviderRequest) ([]byte, error) {
	if req.APIType == types.APITypeEmbeddings {
		text := strings.TrimSpace(req.Input)
		if text == "" {
			text = strings.TrimSpace(req.Prompt)
		}
		if text == "" {
			for _, msg := range req.Messages {
				if strings.TrimSpace(msg.Content) != "" {
					text = strings.TrimSpace(msg.Content)
					break
				}
			}
		}
		if text == "" {
			return nil, fmt.Errorf("Gemini embedding request requires input")
		}
		return json.Marshal(map[string]any{
			"content": map[string]any{
				"parts": []map[string]string{{
					"text": text,
				}},
			},
		})
	}

	contents := make([]map[string]any, 0, len(req.Messages))
	for _, msg := range req.Messages {
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}

		role := "user"
		if msg.Role == "assistant" || msg.Role == "model" {
			role = "model"
		}

		contents = append(contents, map[string]any{
			"role": role,
			"parts": []map[string]string{{
				"text": text,
			}},
		})
	}

	payload := map[string]any{
		"contents": contents,
	}
	if req.MaxTokens > 0 {
		payload["generationConfig"] = map[string]any{
			"maxOutputTokens": req.MaxTokens,
		}
	}
	return json.Marshal(payload)
}
