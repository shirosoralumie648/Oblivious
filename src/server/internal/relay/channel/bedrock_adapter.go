package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"oblivious/server/internal/relay/types"
)

type BedrockAdapter struct {
	baseURL string
	apiKey  string
}

var _ types.ProviderAdapter = (*BedrockAdapter)(nil)

func NewBedrockAdapter(baseURL, apiKey string) *BedrockAdapter {
	return &BedrockAdapter{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
	}
}

func (a *BedrockAdapter) Name() string { return "bedrock-adapter" }

func (a *BedrockAdapter) Provider() string { return "bedrock" }

func (a *BedrockAdapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SupportsChat: true,
	}
}

func (a *BedrockAdapter) BuildURL(model string, apiType types.APIType) (string, error) {
	if apiType != types.APITypeChat {
		return "", fmt.Errorf("bedrock adapter does not support %s", apiType.String())
	}
	modelID := normalizeBedrockModel(model)
	if modelID == "" {
		return "", fmt.Errorf("missing Bedrock model")
	}
	baseURL, err := a.endpointBaseURL()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(baseURL, "/") + "/model/" + modelID + "/converse", nil
}

func (a *BedrockAdapter) BuildHeaders(ctx context.Context, model string, apiType types.APIType) (http.Header, error) {
	token, _ := parseBedrockAPIKey(a.apiKey)
	headers := http.Header{}
	if token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	return headers, nil
}

func (a *BedrockAdapter) ConvertRequest(req *types.ProviderRequest) (*types.ProviderRequest, error) {
	return req, nil
}

func (a *BedrockAdapter) ConvertResponse(resp []byte, isStream bool) (*types.ProviderResponse, error) {
	var payload struct {
		Usage struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
			TotalTokens  int `json:"totalTokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(resp, &payload)

	var usage *types.Usage
	if payload.Usage.InputTokens > 0 || payload.Usage.OutputTokens > 0 || payload.Usage.TotalTokens > 0 {
		total := payload.Usage.TotalTokens
		if total == 0 {
			total = payload.Usage.InputTokens + payload.Usage.OutputTokens
		}
		usage = &types.Usage{
			PromptTokens:     payload.Usage.InputTokens,
			CompletionTokens: payload.Usage.OutputTokens,
			TotalTokens:      total,
		}
	}

	return &types.ProviderResponse{StatusCode: http.StatusOK, Content: resp, Done: true, Usage: usage}, nil
}

func (a *BedrockAdapter) DoRequest(ctx context.Context, req *types.ProviderRequest) (*http.Response, error) {
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
		marshaled, err := marshalBedrockConverseRequest(req)
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
	if upstreamReq.Header.Get("Authorization") == "" {
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
	if upstreamReq.Header.Get("Accept") == "" {
		upstreamReq.Header.Set("Accept", "application/json")
	}
	if upstreamReq.Header.Get("Content-Type") == "" {
		upstreamReq.Header.Set("Content-Type", "application/json")
	}

	return (&http.Client{Timeout: 60 * time.Second}).Do(upstreamReq)
}

func (a *BedrockAdapter) HealthCheck(ctx context.Context) error {
	token, region := parseBedrockAPIKey(a.apiKey)
	if token == "" {
		return fmt.Errorf("missing Bedrock API key")
	}
	if strings.TrimSpace(a.baseURL) == "" && region == "" {
		return fmt.Errorf("missing Bedrock region in API key")
	}
	_, err := a.BuildURL("claude-3-haiku-20240307", types.APITypeChat)
	return err
}

func (a *BedrockAdapter) ListModels(ctx context.Context) ([]string, error) {
	token, region := parseBedrockAPIKey(a.apiKey)
	if token == "" {
		return nil, fmt.Errorf("missing Bedrock API key")
	}
	if strings.TrimSpace(a.baseURL) == "" && region == "" {
		return nil, fmt.Errorf("missing Bedrock region in API key")
	}

	models := make([]string, 0, len(bedrockModelIDMap))
	for model := range bedrockModelIDMap {
		models = append(models, model)
	}
	sort.Strings(models)
	return models, nil
}

func (a *BedrockAdapter) MapError(statusCode int, body []byte) *types.ProviderError {
	if statusCode < http.StatusBadRequest {
		return nil
	}

	var payload struct {
		Type         string `json:"__type"`
		Code         string `json:"code"`
		Message      string `json:"message"`
		MessageUpper string `json:"Message"`
		Error        struct {
			Type    string `json:"type"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)

	code := firstNonEmptyString(payload.Error.Code, payload.Error.Type, payload.Code, payload.Type)
	if code == "" {
		code = strings.ToLower(strings.ReplaceAll(http.StatusText(statusCode), " ", "_"))
	}
	if code == "" {
		code = "provider_error"
	}

	message := firstNonEmptyString(payload.Error.Message, payload.Message, payload.MessageUpper, strings.TrimSpace(string(body)), http.StatusText(statusCode))
	return &types.ProviderError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Retryable:  isRetryableProviderStatus(statusCode),
	}
}

func (a *BedrockAdapter) EstimateUsage(req *types.ProviderRequest) *types.Usage {
	return (&OpenAIAdapter{}).EstimateUsage(req)
}

func (a *BedrockAdapter) endpointBaseURL() (string, error) {
	if strings.TrimSpace(a.baseURL) != "" {
		return strings.TrimRight(a.baseURL, "/"), nil
	}
	_, region := parseBedrockAPIKey(a.apiKey)
	region = strings.TrimSpace(region)
	if region == "" {
		return "", fmt.Errorf("missing Bedrock region in API key")
	}
	return "https://bedrock-runtime." + region + ".amazonaws.com", nil
}

func parseBedrockAPIKey(apiKey string) (string, string) {
	parts := strings.Split(strings.TrimSpace(apiKey), "|")
	if len(parts) == 0 {
		return "", ""
	}
	token := strings.TrimSpace(parts[0])
	region := ""
	if len(parts) > 1 {
		region = strings.TrimSpace(parts[1])
	}
	return token, region
}

var bedrockModelIDMap = map[string]string{
	"claude-3-haiku-20240307":    "anthropic.claude-3-haiku-20240307-v1:0",
	"claude-3-sonnet-20240229":   "anthropic.claude-3-sonnet-20240229-v1:0",
	"claude-3-opus-20240229":     "anthropic.claude-3-opus-20240229-v1:0",
	"claude-3-5-sonnet-20240620": "anthropic.claude-3-5-sonnet-20240620-v1:0",
	"claude-3-5-sonnet-20241022": "anthropic.claude-3-5-sonnet-20241022-v2:0",
	"claude-3-5-haiku-20241022":  "anthropic.claude-3-5-haiku-20241022-v1:0",
	"claude-3-7-sonnet-20250219": "anthropic.claude-3-7-sonnet-20250219-v1:0",
	"claude-sonnet-4-20250514":   "anthropic.claude-sonnet-4-20250514-v1:0",
	"claude-opus-4-20250514":     "anthropic.claude-opus-4-20250514-v1:0",
	"nova-micro-v1:0":            "amazon.nova-micro-v1:0",
	"nova-lite-v1:0":             "amazon.nova-lite-v1:0",
	"nova-pro-v1:0":              "amazon.nova-pro-v1:0",
	"amazon.nova-micro-v1:0":     "amazon.nova-micro-v1:0",
	"amazon.nova-lite-v1:0":      "amazon.nova-lite-v1:0",
	"amazon.nova-pro-v1:0":       "amazon.nova-pro-v1:0",
}

func normalizeBedrockModel(model string) string {
	model = strings.TrimSpace(model)
	if mapped, ok := bedrockModelIDMap[model]; ok {
		return mapped
	}
	return model
}

func marshalBedrockConverseRequest(req *types.ProviderRequest) ([]byte, error) {
	type bedrockContent struct {
		Text string `json:"text"`
	}
	type bedrockMessage struct {
		Role    string           `json:"role"`
		Content []bedrockContent `json:"content"`
	}
	payload := struct {
		System          []bedrockContent `json:"system,omitempty"`
		Messages        []bedrockMessage `json:"messages"`
		InferenceConfig *struct {
			MaxTokens int `json:"maxTokens,omitempty"`
		} `json:"inferenceConfig,omitempty"`
	}{
		Messages: make([]bedrockMessage, 0, len(req.Messages)),
	}

	for _, msg := range req.Messages {
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}
		if msg.Role == "system" {
			payload.System = append(payload.System, bedrockContent{Text: text})
			continue
		}
		role := "user"
		if msg.Role == "assistant" {
			role = "assistant"
		}
		payload.Messages = append(payload.Messages, bedrockMessage{
			Role:    role,
			Content: []bedrockContent{{Text: text}},
		})
	}

	if len(payload.Messages) == 0 && strings.TrimSpace(req.Prompt) != "" {
		payload.Messages = append(payload.Messages, bedrockMessage{
			Role:    "user",
			Content: []bedrockContent{{Text: strings.TrimSpace(req.Prompt)}},
		})
	}
	if len(payload.Messages) == 0 {
		return nil, fmt.Errorf("Bedrock Converse request requires at least one message")
	}
	if req.MaxTokens > 0 {
		payload.InferenceConfig = &struct {
			MaxTokens int `json:"maxTokens,omitempty"`
		}{MaxTokens: req.MaxTokens}
	}

	return json.Marshal(payload)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
