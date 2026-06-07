package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	relaycache "oblivious/server/internal/relay/cache"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

// ResponsesHandler Responses API 处理
type ResponsesHandler struct {
	pool                  *types.ChannelPoolInterface
	adapter               *channel.OpenAIAdapter
	semanticCacheEmbedder SemanticCacheEmbedder
}

func NewResponsesHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *ResponsesHandler {
	return &ResponsesHandler{pool: p, adapter: a}
}

func NewResponsesHandlerWithSemanticCacheEmbedder(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter, embedder SemanticCacheEmbedder) *ResponsesHandler {
	return &ResponsesHandler{pool: p, adapter: a, semanticCacheEmbedder: embedder}
}

func (h *ResponsesHandler) Handle(c *gin.Context) error {
	var rawReq map[string]any
	if err := json.NewDecoder(c.Request.Body).Decode(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "invalid JSON body"}})
		return nil
	}

	model := getString(rawReq, "model")
	if model == "" {
		model = "gpt-4o"
	}

	messages := parseMessages(rawReq)
	if len(messages) == 0 {
		messages = parseResponsesInputMessages(rawReq["input"])
	}

	req := &channel.ProviderRequest{
		APIType:   types.APITypeResponses,
		Model:     model,
		Stream:    getBool(rawReq, "stream"),
		Messages:  messages,
		MaxTokens: parseInt(rawReq["max_tokens"]),
	}

	applyTrustedInternalIdentity(c)

	usage := h.adapter.EstimateUsage(req)

	if cacheReq, cacheable := semanticCacheRequestFromResponses(c.Request.Context(), req); cacheable {
		cacheReq = attachSemanticCacheEmbedding(c.Request.Context(), cacheReq, h.semanticCacheEmbedder)
		c.Request = c.Request.WithContext(types.WithSemanticCacheRequest(c.Request.Context(), cacheReq))
	}

	if req.Stream {
		return h.handleStream(c, req, usage)
	}

	resp, err := h.executeRequest(c, req, usage)
	if err != nil {
		writeRelayHandlerError(c, resp, err)
		return nil
	}
	statusCode := resp.StatusCode
	if statusCode < http.StatusContinue {
		statusCode = http.StatusOK
	}
	c.Data(statusCode, "application/json", resp.Content)
	return nil
}

func (h *ResponsesHandler) handleStream(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) error {
	resp, err := h.executeRequest(c, req, usage)
	if err != nil {
		writeRelayHandlerError(c, resp, err)
		return nil
	}

	statusCode := resp.StatusCode
	if statusCode < http.StatusContinue {
		statusCode = http.StatusOK
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Data(statusCode, "text/event-stream", resp.Content)
	return nil
}

func (h *ResponsesHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func semanticCacheRequestFromResponses(ctx context.Context, req *channel.ProviderRequest) (types.SemanticCacheRequest, bool) {
	if req == nil || req.Stream || strings.TrimSpace(req.Model) == "" || len(req.Messages) == 0 {
		return types.SemanticCacheRequest{}, false
	}
	query := canonicalResponsesSemanticCacheQuery(req)
	if strings.TrimSpace(query) == "" {
		return types.SemanticCacheRequest{}, false
	}
	organizationID, hasOrganizationID := types.TrustedOrganizationIDFromContext(ctx)
	userID, hasUserID := types.TrustedUserIDFromContext(ctx)
	cacheReq := types.SemanticCacheRequest{
		OrganizationID: organizationID,
		UserID:         userID,
		Model:          req.Model,
		Query:          query,
	}
	if hasOrganizationID || hasUserID {
		cacheReq.UserScoped = relaycache.IsSensitiveSemanticCacheRequest(cacheReq)
	}
	return cacheReq, true
}

func canonicalResponsesSemanticCacheQuery(req *channel.ProviderRequest) string {
	payload := struct {
		APIType   string          `json:"api_type"`
		Route     string          `json:"route"`
		Messages  []types.Message `json:"messages"`
		MaxTokens int             `json:"max_tokens,omitempty"`
	}{
		APIType:   req.APIType.String(),
		Route:     "/v1/responses",
		Messages:  req.Messages,
		MaxTokens: req.MaxTokens,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(body)
}

func (h *ResponsesHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	router := GetRouter()
	if router == nil {
		return nil, types.ErrNoAvailableChannel
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("resp_%d", time.Now().UnixNano())
	}

	return router.RouteWithBilling(
		c.Request.Context(),
		req.APIType,
		req.Model,
		"",
		idempotencyKey,
		usage,
		func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
			if ch == nil || ch.Channel == nil {
				return nil, types.ErrNoAvailableChannel
			}
			adapter, err := channel.AdapterForChannel(ch.Channel)
			if err != nil {
				return nil, err
			}
			upstreamURL, err := adapter.BuildURL(req.Model, req.APIType)
			if err != nil {
				return nil, err
			}
			headers, err := adapter.BuildHeaders(c.Request.Context(), req.Model, req.APIType)
			if err != nil {
				return nil, err
			}

			providerReq := &channel.ProviderRequest{
				APIType:   req.APIType,
				Model:     req.Model,
				URL:       upstreamURL,
				Stream:    req.Stream,
				Messages:  req.Messages,
				MaxTokens: req.MaxTokens,
				Headers:   headers,
			}

			return executeProviderAdapterRequest(c.Request.Context(), adapter, providerReq)
		},
	)
}

func parseResponsesInputMessages(input any) []channel.Message {
	switch v := input.(type) {
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return nil
		}
		return []channel.Message{{Role: "user", Content: text}}
	case []any:
		messages := make([]channel.Message, 0, len(v))
		for _, item := range v {
			msg, ok := parseResponsesInputMessage(item)
			if ok {
				messages = append(messages, msg)
			}
		}
		return messages
	case map[string]any:
		msg, ok := parseResponsesInputMessage(v)
		if !ok {
			return nil
		}
		return []channel.Message{msg}
	default:
		return nil
	}
}

func parseResponsesInputMessage(input any) (channel.Message, bool) {
	item, ok := input.(map[string]any)
	if !ok {
		if text, ok := input.(string); ok && strings.TrimSpace(text) != "" {
			return channel.Message{Role: "user", Content: strings.TrimSpace(text)}, true
		}
		return channel.Message{}, false
	}

	role := getString(item, "role")
	if role == "" {
		role = "user"
	}
	text := extractResponsesInputText(item)
	if text == "" {
		return channel.Message{}, false
	}
	return channel.Message{Role: role, Content: text}, true
}

func extractResponsesInputText(item map[string]any) string {
	if text := strings.TrimSpace(getString(item, "text")); text != "" {
		return text
	}
	if text := strings.TrimSpace(getString(item, "input_text")); text != "" {
		return text
	}
	switch content := item["content"].(type) {
	case string:
		return strings.TrimSpace(content)
	case []any:
		parts := make([]string, 0, len(content))
		for _, part := range content {
			switch p := part.(type) {
			case string:
				if text := strings.TrimSpace(p); text != "" {
					parts = append(parts, text)
				}
			case map[string]any:
				if text := strings.TrimSpace(getString(p, "text")); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		return strings.TrimSpace(getString(content, "text"))
	default:
		return ""
	}
}
