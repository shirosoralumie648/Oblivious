package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ChatHandler Chat Completions 处理
type ChatHandler struct {
	pool    *types.ChannelPoolInterface
	adapter *channel.OpenAIAdapter
}

func NewChatHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *ChatHandler {
	return &ChatHandler{pool: p, adapter: a}
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

	url, _ := h.adapter.BuildURL(model, types.APITypeChat)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeChat)

	// 构建 ProviderRequest
	req := &channel.ProviderRequest{
		APIType:   types.APITypeChat,
		Model:     model,
		URL:       url,
		Stream:    stream,
		Messages:  parseMessages(rawReq),
		MaxTokens: parseInt(rawReq["max_tokens"]),
		Headers:   headers,
	}

	// 估算用量
	usage := h.adapter.EstimateUsage(req)
	_ = usage

	// 通过 executeRequest 路由
	resp, err := h.executeRequest(c, req, usage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}

	if stream {
		h.handleStream(c, req, resp)
		return nil
	}

	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

// handleStream SSE 流式处理
func (h *ChatHandler) handleStream(c *gin.Context, req *channel.ProviderRequest, resp *types.ProviderResponse) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// resp.Content 在真实实现中是 upstream response body
	// 这里简化处理；真实场景需要流式代理
	if len(resp.Content) > 0 {
		c.Writer.Write(resp.Content)
		c.Writer.(http.Flusher).Flush()
	}
}

// HandleStream 实现 Handler 接口（用于 WebSocket，但 Chat 用 Handle）
func (h *ChatHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *ChatHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	// TODO: 调用 Router.Execute(req) — Plan C 实现
	// Plan C 之前返回错误以便编译通过
	return nil, types.ErrNoAvailableChannel
}
