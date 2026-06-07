package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	relaycache "oblivious/server/internal/relay/cache"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ChatHandler Chat Completions 处理
type ChatHandler struct {
	pool                  *types.ChannelPoolInterface
	adapter               *channel.OpenAIAdapter
	semanticCacheEmbedder SemanticCacheEmbedder
}

func NewChatHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *ChatHandler {
	return &ChatHandler{pool: p, adapter: a}
}

func NewChatHandlerWithSemanticCacheEmbedder(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter, embedder SemanticCacheEmbedder) *ChatHandler {
	return &ChatHandler{pool: p, adapter: a, semanticCacheEmbedder: embedder}
}

// Handle 同步请求
func (h *ChatHandler) Handle(c *gin.Context) error {
	var rawReq map[string]any
	if err := json.NewDecoder(c.Request.Body).Decode(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "invalid JSON body"}})
		return nil
	}

	model, _ := rawReq["model"].(string)
	if model == "" {
		model = "gpt-4o"
	}
	stream, _ := rawReq["stream"].(bool)

	// Parse tools array for tool-calling support.
	var tools []map[string]any
	if toolsRaw, ok := rawReq["tools"].([]any); ok {
		tools = make([]map[string]any, 0, len(toolsRaw))
		for _, t := range toolsRaw {
			if tm, ok := t.(map[string]any); ok {
				tools = append(tools, tm)
			}
		}
	}

	// 构建 ProviderRequest
	req := &channel.ProviderRequest{
		APIType:    types.APITypeChat,
		Model:      model,
		Stream:     stream,
		Messages:   parseMessages(rawReq),
		MaxTokens:  parseInt(rawReq["max_tokens"]),
		Tools:      tools,
		ToolChoice: rawReq["tool_choice"],
	}

	applyTrustedInternalIdentity(c)

	// 估算用量
	usage := h.adapter.EstimateUsage(req)

	if cacheReq, cacheable := semanticCacheRequestFromChat(c.Request.Context(), req); cacheable {
		cacheReq = attachSemanticCacheEmbedding(c.Request.Context(), cacheReq, h.semanticCacheEmbedder)
		c.Request = c.Request.WithContext(types.WithSemanticCacheRequest(c.Request.Context(), cacheReq))
	}

	// 通过 executeRequest 路由
	resp, err := h.executeRequest(c, req, usage)
	if err != nil {
		writeRelayHandlerError(c, resp, err)
		return nil
	}

	if req.Stream {
		h.handleStream(c, req, resp)
		return nil
	}

	statusCode := resp.StatusCode
	if statusCode < http.StatusContinue {
		statusCode = http.StatusOK
	}
	c.Data(statusCode, "application/json", resp.Content)
	return nil
}

// handleStream SSE 流式处理
func (h *ChatHandler) handleStream(c *gin.Context, req *channel.ProviderRequest, resp *types.ProviderResponse) {
	statusCode := resp.StatusCode
	if statusCode < http.StatusContinue {
		statusCode = http.StatusOK
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// resp.Content 在真实实现中是 upstream response body
	// 这里简化处理；真实场景需要流式代理
	if len(resp.Content) > 0 {
		c.Data(statusCode, "text/event-stream", resp.Content)
		return
	}
	c.Status(statusCode)
}

// HandleStream 实现 Handler 接口（用于 WebSocket，但 Chat 用 Handle）
func (h *ChatHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func semanticCacheRequestFromChat(ctx context.Context, req *channel.ProviderRequest) (types.SemanticCacheRequest, bool) {
	if req == nil || req.Stream || strings.TrimSpace(req.Model) == "" {
		return types.SemanticCacheRequest{}, false
	}
	query := canonicalChatSemanticCacheQuery(req)
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

func canonicalChatSemanticCacheQuery(req *channel.ProviderRequest) string {
	if len(req.Messages) == 0 {
		return ""
	}
	payload := struct {
		Messages   []types.Message  `json:"messages"`
		MaxTokens  int              `json:"max_tokens,omitempty"`
		Tools      []map[string]any `json:"tools,omitempty"`
		ToolChoice any              `json:"tool_choice,omitempty"`
	}{
		Messages:   req.Messages,
		MaxTokens:  req.MaxTokens,
		Tools:      req.Tools,
		ToolChoice: req.ToolChoice,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(body)
}

func (h *ChatHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	router := GetRouter()
	if router == nil {
		return nil, types.ErrNoAvailableChannel
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("chat_%d", time.Now().UnixNano())
	}

	return router.RouteWithBilling(
		c.Request.Context(),
		req.APIType,
		req.Model,
		"", // channel selected by router
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
				APIType:    req.APIType,
				Model:      req.Model,
				URL:        upstreamURL,
				Stream:     req.Stream,
				Messages:   req.Messages,
				MaxTokens:  req.MaxTokens,
				Headers:    headers,
				Tools:      req.Tools,
				ToolChoice: req.ToolChoice,
			}

			return executeProviderAdapterRequest(c.Request.Context(), adapter, providerReq)
		},
	)
}
