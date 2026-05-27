package chat

import (
	"context"
	"errors"
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
	defaultName string
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

// NewHTTPReplyGenerator is retained for non-Relay development demo behavior.
// Commercial provider calls must use RelayGateway, not this fallback.
func NewHTTPReplyGenerator(baseURL, apiKey, defaultName string, timeout time.Duration) *HTTPReplyGenerator {
	_ = baseURL
	_ = apiKey
	_ = timeout
	return &HTTPReplyGenerator{
		defaultName: defaultName,
	}
}

func (g *HTTPReplyGenerator) GenerateReply(ctx context.Context, messages []Message, config ConversationConfig) (string, error) {
	_ = ctx
	_ = config
	return formatDemoReply(messages), nil
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
