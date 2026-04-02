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

type ReplyGenerator interface {
	GenerateReply(ctx context.Context, messages []Message, config ConversationConfig) (string, error)
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
	Content string `json:"content"`
	Role    string `json:"role"`
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
		if role != "assistant" && role != "user" {
			role = "user"
		}
		result = append(result, openAIMessage{
			Content: message.Content,
			Role:    role,
		})
	}

	return result
}
