package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"oblivious/server/internal/relay/types"
)

type VertexAdapter struct {
	baseURL string
	apiKey  string
}

var _ types.ProviderAdapter = (*VertexAdapter)(nil)

func NewVertexAdapter(baseURL, apiKey string) *VertexAdapter {
	return &VertexAdapter{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
	}
}

func (a *VertexAdapter) Name() string { return "vertex-adapter" }

func (a *VertexAdapter) Provider() string { return "vertex" }

func (a *VertexAdapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SupportsChat:      true,
		SupportsStreaming: true,
	}
}

func (a *VertexAdapter) BuildURL(model string, apiType types.APIType) (string, error) {
	if apiType != types.APITypeChat && apiType != types.APITypeResponses && apiType != types.APITypeCompletions {
		return "", fmt.Errorf("vertex adapter does not support %s", apiType.String())
	}
	model = normalizeVertexModel(model)
	if model == "" {
		return "", fmt.Errorf("missing Vertex model")
	}

	key, projectID, region := parseVertexAPIKey(a.apiKey)
	baseURL := buildVertexAPIBaseURL(a.baseURL, "v1", projectID, region)
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("missing Vertex base URL")
	}

	endpoint := baseURL + "/publishers/google/models/" + model + ":generateContent"
	if key != "" {
		separator := "?"
		if strings.Contains(endpoint, "?") {
			separator = "&"
		}
		endpoint += separator + "key=" + url.QueryEscape(key)
	}
	return endpoint, nil
}

func (a *VertexAdapter) BuildHeaders(ctx context.Context, model string, apiType types.APIType) (http.Header, error) {
	headers := http.Header{}
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	return headers, nil
}

func (a *VertexAdapter) ConvertRequest(req *types.ProviderRequest) (*types.ProviderRequest, error) {
	return req, nil
}

func (a *VertexAdapter) ConvertResponse(resp []byte, isStream bool) (*types.ProviderResponse, error) {
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

func (a *VertexAdapter) DoRequest(ctx context.Context, req *types.ProviderRequest) (*http.Response, error) {
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
	if err := validateProviderUpstreamURL(upstreamReq.URL.String()); err != nil {
		return nil, err
	}
	upstreamReq.Header = req.Headers.Clone()
	if upstreamReq.Header == nil {
		upstreamReq.Header = http.Header{}
	}
	headers, err := a.BuildHeaders(ctx, req.Model, req.APIType)
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
		if upstreamReq.Header.Get(key) != "" {
			continue
		}
		for _, value := range values {
			upstreamReq.Header.Add(key, value)
		}
	}

	return newProviderHTTPClient(60*time.Second, nil).Do(upstreamReq)
}

func (a *VertexAdapter) HealthCheck(ctx context.Context) error {
	key, _, _ := parseVertexAPIKey(a.apiKey)
	if key == "" {
		return fmt.Errorf("missing Vertex API key")
	}
	_, err := a.BuildURL("gemini-1.5-flash", types.APITypeChat)
	return err
}

func (a *VertexAdapter) MapError(statusCode int, body []byte) *types.ProviderError {
	if statusCode < http.StatusBadRequest {
		return nil
	}

	var payload struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)

	code := firstNonEmptyString(payload.Error.Status, payload.Error.Code)
	if code == "" {
		code = strings.ToLower(strings.ReplaceAll(http.StatusText(statusCode), " ", "_"))
	}
	if code == "" {
		code = "provider_error"
	}
	message := firstNonEmptyString(payload.Error.Message, strings.TrimSpace(string(body)), http.StatusText(statusCode))
	return &types.ProviderError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Retryable:  isRetryableProviderStatus(statusCode),
	}
}

func (a *VertexAdapter) EstimateUsage(req *types.ProviderRequest) *types.Usage {
	return (&OpenAIAdapter{}).EstimateUsage(req)
}

func parseVertexAPIKey(apiKey string) (string, string, string) {
	parts := strings.Split(strings.TrimSpace(apiKey), "|")
	if len(parts) == 0 {
		return "", "", "global"
	}
	key := strings.TrimSpace(parts[0])
	projectID := ""
	region := "global"
	if len(parts) == 2 {
		region = firstNonEmptyString(parts[1], "global")
	} else if len(parts) >= 3 {
		projectID = strings.TrimSpace(parts[1])
		region = firstNonEmptyString(parts[2], "global")
	}
	return key, projectID, region
}

func buildVertexAPIBaseURL(baseURL, version, projectID, region string) string {
	version = strings.Trim(strings.TrimSpace(version), "/")
	if version == "" {
		version = "v1"
	}
	region = firstNonEmptyString(region, "global")

	normalizedBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if normalizedBase == "" {
		if region == "global" {
			normalizedBase = "https://aiplatform.googleapis.com"
		} else {
			normalizedBase = "https://" + region + "-aiplatform.googleapis.com"
		}
	}
	if !strings.HasSuffix(normalizedBase, "/"+version) {
		normalizedBase += "/" + version
	}
	if strings.TrimSpace(projectID) != "" {
		normalizedBase += "/projects/" + strings.TrimSpace(projectID) + "/locations/" + region
	}
	return normalizedBase
}

func normalizeVertexModel(model string) string {
	return strings.TrimPrefix(strings.TrimSpace(model), "models/")
}
