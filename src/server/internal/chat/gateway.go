package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var ErrModelGatewayUnavailable = errors.New("model gateway unavailable")

type relayRequestMetadataKey struct{}

type ReplyGenerator interface {
	GenerateReply(ctx context.Context, messages []Message, config ConversationConfig) (string, error)
}

type StructuredReplyGenerator interface {
	GenerateStructuredReply(ctx context.Context, messages []Message, config ConversationConfig, tools []map[string]any) (*CompletionResponse, error)
}

type HTTPReplyGenerator struct {
	apiKey      string
	baseURL     string
	defaultName string
	httpClient  *http.Client
}

type openAIChatCompletionsRequest struct {
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Messages    []openAIMessage `json:"messages"`
	Model       string          `json:"model"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openAIChatCompletionsResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

type openAIMessage struct {
	Content    string     `json:"content,omitempty"`
	Role       string     `json:"role"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function ToolFunction `json:"function,omitempty"`
}

type ToolFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type CompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type CompletionResponse struct {
	Content      string           `json:"content,omitempty"`
	ToolCalls    []ToolCall       `json:"toolCalls,omitempty"`
	FinishReason string           `json:"finishReason,omitempty"`
	Usage        *CompletionUsage `json:"usage,omitempty"`
}

type RelayRequestMetadata struct {
	OrganizationID string
	UserID         string
	WorkspaceID    string
	RequestID      string
}

func WithRelayRequestMetadata(ctx context.Context, metadata RelayRequestMetadata) context.Context {
	return context.WithValue(ctx, relayRequestMetadataKey{}, metadata)
}

func RelayRequestMetadataFromContext(ctx context.Context) (RelayRequestMetadata, bool) {
	metadata, ok := ctx.Value(relayRequestMetadataKey{}).(RelayRequestMetadata)
	return metadata, ok
}

func NewHTTPReplyGenerator(baseURL, apiKey, defaultName string, timeout time.Duration) *HTTPReplyGenerator {
	return &HTTPReplyGenerator{
		apiKey:      apiKey,
		baseURL:     strings.TrimRight(baseURL, "/"),
		defaultName: defaultName,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (g *HTTPReplyGenerator) GenerateReply(ctx context.Context, messages []Message, config ConversationConfig) (string, error) {
	if g.baseURL == "" || g.apiKey == "" {
		return formatDemoReply(messages), nil
	}

	requestBody, err := json.Marshal(openAIChatCompletionsRequest{
		MaxTokens:   config.MaxOutputTokens,
		Messages:    toOpenAIMessages(messages, config.SystemPromptOverride, config.ToolsEnabled),
		Model:       selectModelID(config.ModelID, g.defaultName),
		Temperature: config.Temperature,
	})
	if err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/chat/completions", bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+g.apiKey)

	response, err := g.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("model gateway returned status %d", response.StatusCode)
	}

	var payload openAIChatCompletionsResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 || strings.TrimSpace(payload.Choices[0].Message.Content) == "" {
		return "", ErrModelGatewayUnavailable
	}

	return payload.Choices[0].Message.Content, nil
}

func formatDemoReply(messages []Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == "user" {
			return "Assistant reply: " + messages[index].Content
		}
	}

	return "Assistant reply"
}

func selectModelID(modelID, fallback string) string {
	if strings.TrimSpace(modelID) != "" {
		return modelID
	}

	return fallback
}

func toOpenAIMessages(messages []Message, systemPromptOverride string, toolsEnabled bool) []openAIMessage {
	result := make([]openAIMessage, 0, len(messages)+2)
	if strings.TrimSpace(systemPromptOverride) != "" {
		result = append(result, openAIMessage{
			Content: systemPromptOverride,
			Role:    "system",
		})
	}
	if toolsEnabled {
		result = append(result, openAIMessage{
			Content: "Tools are enabled for this conversation.",
			Role:    "system",
		})
	}
	for _, message := range messages {
		role := message.Role
		if role != "assistant" && role != "user" && role != "system" && role != "tool" {
			role = "user"
		}
		result = append(result, openAIMessage{
			Content:    message.Content,
			Role:       role,
			ToolCallID: message.ToolCallID,
			ToolCalls:  append([]ToolCall(nil), message.ToolCalls...),
		})
	}

	return result
}
