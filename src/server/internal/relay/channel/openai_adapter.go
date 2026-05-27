package channel

import (
	"context"
	"net/http"
	"strings"

	"oblivious/server/internal/relay/types"
)

// OpenAIAdapter OpenAI Provider 适配器实现
type OpenAIAdapter struct {
	baseURL string
	apiKey  string
}

var _ types.ProviderAdapter = (*OpenAIAdapter)(nil)

// Name returns the adapter name
func (a *OpenAIAdapter) Name() string { return "openai-adapter" }

// Provider returns the provider name
func (a *OpenAIAdapter) Provider() string { return "openai" }

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
	return a.baseURL + "/v1/" + apiType.String(), nil
}

// BuildHeaders builds the request headers
func (a *OpenAIAdapter) BuildHeaders(ctx context.Context, model string, apiType types.APIType) (http.Header, error) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+a.apiKey)
	headers.Set("Content-Type", "application/json")
	return headers, nil
}

// ConvertRequest converts the request
func (a *OpenAIAdapter) ConvertRequest(req *types.ProviderRequest) (*types.ProviderRequest, error) {
	return req, nil
}

// ConvertResponse converts the response
func (a *OpenAIAdapter) ConvertResponse(resp []byte, isStream bool) (*types.ProviderResponse, error) {
	return &types.ProviderResponse{StatusCode: 200, Content: resp, Done: true}, nil
}

// DoRequest executes the HTTP request
func (a *OpenAIAdapter) DoRequest(ctx context.Context, req *types.ProviderRequest) (*http.Response, error) {
	return nil, nil
}

// HealthCheck checks if the provider is healthy
func (a *OpenAIAdapter) HealthCheck(ctx context.Context) error {
	return nil
}

// MapError maps HTTP status to provider error
func (a *OpenAIAdapter) MapError(statusCode int, body []byte) *types.ProviderError {
	return nil
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
