package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"oblivious/server/internal/relay/types"
)

// OpenAIAdapter OpenAI Provider 适配器实现
type OpenAIAdapter struct {
	provider string
	baseURL  string
	apiKey   string
}

var _ types.ProviderAdapter = (*OpenAIAdapter)(nil)

func NewOpenAIAdapter(baseURL, apiKey string) *OpenAIAdapter {
	return NewOpenAICompatibleAdapter("openai", baseURL, apiKey)
}

func NewOpenAICompatibleAdapter(provider, baseURL, apiKey string) *OpenAIAdapter {
	normalized := NormalizeProvider(provider)
	if normalized == "" {
		normalized = "openai"
	}
	return &OpenAIAdapter{
		provider: normalized,
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
	}
}

func (a *OpenAIAdapter) ForChannel(ch *types.Channel) *OpenAIAdapter {
	if ch == nil {
		return NewOpenAICompatibleAdapter(a.Provider(), a.baseURL, a.apiKey)
	}
	return NewOpenAICompatibleAdapter(ch.Provider, ch.BaseURL, ch.APIKey)
}

// Name returns the adapter name
func (a *OpenAIAdapter) Name() string { return "openai-adapter" }

// Provider returns the provider name
func (a *OpenAIAdapter) Provider() string {
	if a.provider == "" {
		return "openai"
	}
	return a.provider
}

// Capabilities returns the capabilities
func (a *OpenAIAdapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SupportsChat:       true,
		SupportsStreaming:  true,
		SupportsEmbeddings: true,
		SupportsImages:     true,
		SupportsAudio:      true,
		SupportsRealtime:   true,
		SupportsAssistants: true,
	}
}

// BuildURL builds the request URL
func (a *OpenAIAdapter) BuildURL(model string, apiType types.APIType) (string, error) {
	return a.buildEndpointURL(openAIPath(apiType))
}

func (a *OpenAIAdapter) buildEndpointURL(path string) (string, error) {
	if strings.TrimSpace(a.baseURL) == "" {
		return "", fmt.Errorf("missing upstream base URL")
	}
	baseURL := strings.TrimRight(a.baseURL, "/")
	versionedPrefix := "/v1"
	if hasOpenAIVersionSuffix(baseURL) {
		versionedPrefix = ""
	}
	return baseURL + versionedPrefix + path, nil
}

func hasOpenAIVersionSuffix(baseURL string) bool {
	trimmed := strings.ToLower(strings.TrimRight(baseURL, "/"))
	if trimmed == "" {
		return false
	}
	segment := trimmed
	if slash := strings.LastIndex(trimmed, "/"); slash >= 0 {
		segment = trimmed[slash+1:]
	}
	if len(segment) < 2 || segment[0] != 'v' {
		return false
	}
	hasDigit := false
	for _, r := range segment[1:] {
		if r >= '0' && r <= '9' {
			hasDigit = true
			continue
		}
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return hasDigit
}

// BuildHeaders builds the request headers
func (a *OpenAIAdapter) BuildHeaders(ctx context.Context, model string, apiType types.APIType) (http.Header, error) {
	headers := http.Header{}
	if strings.TrimSpace(a.apiKey) != "" {
		headers.Set("Authorization", "Bearer "+a.apiKey)
	}
	headers.Set("Content-Type", "application/json")
	return headers, nil
}

func openAIPath(apiType types.APIType) string {
	switch apiType {
	case types.APITypeChat:
		return "/chat/completions"
	case types.APITypeResponses:
		return "/responses"
	case types.APITypeEmbeddings:
		return "/embeddings"
	case types.APITypeImageGen:
		return "/images/generations"
	case types.APITypeImageEdit:
		return "/images/edits"
	case types.APITypeImageVar:
		return "/images/variations"
	case types.APITypeAudioSpeech:
		return "/audio/speech"
	case types.APITypeAudioSTT:
		return "/audio/transcriptions"
	case types.APITypeAudioTranslate:
		return "/audio/translations"
	case types.APITypeModeration:
		return "/moderations"
	case types.APITypeCompletions:
		return "/completions"
	case types.APITypeRealtime:
		return "/realtime"
	case types.APITypeBatch:
		return "/batch"
	case types.APITypeFiles:
		return "/files"
	case types.APITypeFineTuning:
		return "/fine_tuning/jobs"
	case types.APITypeAssistants:
		return "/assistants"
	case types.APITypeThreads:
		return "/threads"
	default:
		return "/" + apiType.String()
	}
}

// ConvertRequest converts the request
func (a *OpenAIAdapter) ConvertRequest(req *types.ProviderRequest) (*types.ProviderRequest, error) {
	return req, nil
}

// ConvertResponse converts the response
func (a *OpenAIAdapter) ConvertResponse(resp []byte, isStream bool) (*types.ProviderResponse, error) {
	return &types.ProviderResponse{StatusCode: 200, Content: resp, Done: true, Usage: parseOpenAIUsage(resp, isStream)}, nil
}

func parseOpenAIUsage(resp []byte, isStream bool) *types.Usage {
	if isStream {
		return parseOpenAIStreamUsage(resp)
	}
	var payload struct {
		Usage *types.Usage `json:"usage"`
	}
	_ = json.Unmarshal(resp, &payload)
	return payload.Usage
}

func parseOpenAIStreamUsage(resp []byte) *types.Usage {
	var usage *types.Usage
	for _, line := range strings.Split(string(resp), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var payload struct {
			Usage *types.Usage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &payload); err == nil && payload.Usage != nil {
			usage = payload.Usage
		}
	}
	return usage
}

// DoRequest executes the HTTP request
func (a *OpenAIAdapter) DoRequest(ctx context.Context, req *types.ProviderRequest) (*http.Response, error) {
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
		marshaled, err := marshalOpenAIProviderRequest(req)
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
	if upstreamReq.Header.Get("Content-Type") == "" {
		upstreamReq.Header.Set("Content-Type", "application/json")
	}

	client := newProviderHTTPClient(60*time.Second, nil)
	return client.Do(upstreamReq)
}

// HealthCheck checks if the provider is healthy
func (a *OpenAIAdapter) HealthCheck(ctx context.Context) error {
	_, err := a.ListModels(ctx)
	return err
}

func (a *OpenAIAdapter) ListModels(ctx context.Context) ([]string, error) {
	upstreamURL, err := a.buildEndpointURL("/models")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
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
		return nil, fmt.Errorf("parse provider model list: %w", err)
	}
	models := make([]string, 0, len(payload.Data))
	for _, model := range payload.Data {
		if strings.TrimSpace(model.ID) != "" {
			models = append(models, model.ID)
		}
	}
	return models, nil
}

func (a *OpenAIAdapter) CheckBalance(ctx context.Context) (*ProviderBalance, error) {
	upstreamURL, err := a.balanceURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
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
		return nil, fmt.Errorf("provider balance check failed with status %d", resp.StatusCode)
	}

	switch a.Provider() {
	case "openrouter":
		return parseOpenRouterBalance(body)
	case "deepseek":
		return parseDeepSeekBalance(body)
	default:
		return parseOpenAICreditBalance(body)
	}
}

func (a *OpenAIAdapter) balanceURL() (string, error) {
	switch a.Provider() {
	case "openrouter":
		return a.buildEndpointURL("/credits")
	case "deepseek":
		return a.buildEndpointURL("/user/balance")
	default:
		return a.buildEndpointURL("/dashboard/billing/credit_grants")
	}
}

func parseOpenAICreditBalance(body []byte) (*ProviderBalance, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse provider balance: %w", err)
	}

	if amount, ok := parseJSONNumber(raw["total_available"]); ok {
		return &ProviderBalance{Amount: amount, Currency: "USD", Source: "openai_credit_grants"}, nil
	}
	granted, hasGranted := parseJSONNumber(raw["total_granted"])
	used, hasUsed := parseJSONNumber(raw["total_used"])
	if hasGranted {
		if !hasUsed {
			used = 0
		}
		return &ProviderBalance{Amount: granted - used, Currency: "USD", Source: "openai_credit_grants"}, nil
	}

	return nil, fmt.Errorf("provider balance payload did not include a supported amount field")
}

func parseOpenRouterBalance(body []byte) (*ProviderBalance, error) {
	var payload struct {
		Data struct {
			TotalCredits json.RawMessage `json:"total_credits"`
			TotalUsage   json.RawMessage `json:"total_usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse OpenRouter balance: %w", err)
	}

	credits, hasCredits := parseJSONNumber(payload.Data.TotalCredits)
	usage, hasUsage := parseJSONNumber(payload.Data.TotalUsage)
	if !hasCredits {
		return nil, fmt.Errorf("OpenRouter balance payload did not include total_credits")
	}
	if !hasUsage {
		usage = 0
	}
	return &ProviderBalance{Amount: credits - usage, Currency: "USD", Source: "openrouter_credits"}, nil
}

func parseDeepSeekBalance(body []byte) (*ProviderBalance, error) {
	var payload struct {
		BalanceInfos []struct {
			Currency     string          `json:"currency"`
			TotalBalance json.RawMessage `json:"total_balance"`
		} `json:"balance_infos"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse DeepSeek balance: %w", err)
	}
	if len(payload.BalanceInfos) == 0 {
		return nil, fmt.Errorf("DeepSeek balance payload did not include balance_infos")
	}

	info := payload.BalanceInfos[0]
	amount, ok := parseJSONNumber(info.TotalBalance)
	if !ok {
		return nil, fmt.Errorf("DeepSeek balance payload did not include total_balance")
	}
	currency := strings.ToUpper(strings.TrimSpace(info.Currency))
	if currency == "" {
		currency = "USD"
	}
	return &ProviderBalance{Amount: amount, Currency: currency, Source: "deepseek_balance"}, nil
}

func parseJSONNumber(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}

	var numeric float64
	if err := json.Unmarshal(raw, &numeric); err == nil {
		return numeric, true
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		value, parseErr := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if parseErr == nil {
			return value, true
		}
	}

	return 0, false
}

// MapError maps HTTP status to provider error
func (a *OpenAIAdapter) MapError(statusCode int, body []byte) *types.ProviderError {
	if statusCode < http.StatusBadRequest {
		return nil
	}

	var payload struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)

	code := payload.Error.Code
	if code == "" {
		code = payload.Error.Type
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
	if message == "" {
		message = fmt.Sprintf("provider returned status %d", statusCode)
	}

	return &types.ProviderError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Retryable:  isRetryableProviderStatus(statusCode),
	}
}

func marshalOpenAIProviderRequest(req *types.ProviderRequest) ([]byte, error) {
	payload := map[string]any{
		"model":  req.Model,
		"stream": req.Stream,
	}
	if req.Stream && req.APIType == types.APITypeChat {
		payload["stream_options"] = map[string]any{"include_usage": true}
	}
	if len(req.Messages) > 0 {
		payload["messages"] = req.Messages
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if req.Input != "" {
		payload["input"] = req.Input
	}
	if req.Prompt != "" {
		payload["prompt"] = req.Prompt
	}
	if len(req.Tools) > 0 {
		payload["tools"] = req.Tools
	}
	if req.ToolChoice != nil {
		payload["tool_choice"] = req.ToolChoice
	}
	if req.AudioFormat != "" {
		payload["response_format"] = req.AudioFormat
	}
	if req.AudioVoice != "" {
		payload["voice"] = req.AudioVoice
	}
	if req.ImageURL != "" {
		payload["image"] = req.ImageURL
	}
	if req.FileURL != "" {
		payload["file"] = req.FileURL
	}
	return json.Marshal(payload)
}

func isRetryableProviderStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusConflict ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}

// EstimateUsage estimates usage for pre-billing
func (a *OpenAIAdapter) EstimateUsage(req *types.ProviderRequest) *types.Usage {
	if req == nil {
		return nil
	}

	switch req.APIType {
	case types.APITypeChat, types.APITypeResponses:
		promptTokens := 0
		for _, msg := range req.Messages {
			promptTokens += estimateTextTokens(msg.Role)
			promptTokens += estimateTextTokens(msg.Content)
			for _, toolCall := range msg.ToolCalls {
				promptTokens += estimateTextTokens(toolCall.Function.Name)
				promptTokens += estimateTextTokens(toolCall.Function.Arguments)
			}
		}
		completionTokens := req.MaxTokens
		if completionTokens <= 0 {
			completionTokens = 512
		}
		return &types.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	case types.APITypeCompletions:
		promptTokens := estimateTextTokens(req.Prompt)
		completionTokens := req.MaxTokens
		if completionTokens <= 0 {
			completionTokens = 512
		}
		return &types.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	case types.APITypeEmbeddings:
		promptTokens := estimateTextTokens(req.Input)
		if promptTokens == 0 {
			promptTokens = 1
		}
		return &types.Usage{PromptTokens: promptTokens, TotalTokens: promptTokens}
	case types.APITypeImageGen, types.APITypeImageEdit, types.APITypeImageVar:
		return &types.Usage{ImageCount: 1}
	case types.APITypeAudioSpeech:
		seconds := float64(estimateTextTokens(req.Input))
		if seconds < 1 {
			seconds = 1
		}
		return &types.Usage{AudioSeconds: seconds}
	case types.APITypeAudioSTT, types.APITypeAudioTranslate:
		return &types.Usage{AudioSeconds: 60}
	case types.APITypeModeration:
		promptTokens := estimateTextTokens(req.Input)
		if promptTokens == 0 {
			promptTokens = 1
		}
		return &types.Usage{PromptTokens: promptTokens, TotalTokens: promptTokens}
	default:
		return nil
	}
}

func estimateTextTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	byRune := (len([]rune(text)) + 3) / 4
	byWords := len(strings.Fields(text))
	if byWords > byRune {
		return byWords
	}
	if byRune < 1 {
		return 1
	}
	return byRune
}
