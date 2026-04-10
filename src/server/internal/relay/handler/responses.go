package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/relay/channel"
)

// ResponsesHandler Responses API 处理
type ResponsesHandler struct {
	pool    *types.ChannelPoolInterface
	adapter *channel.OpenAIAdapter
}

func NewResponsesHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *ResponsesHandler {
	return &ResponsesHandler{pool: p, adapter: a}
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

	url, _ := h.adapter.BuildURL(model, types.APITypeResponses)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeResponses)

	req := &channel.ProviderRequest{
		APIType:   types.APITypeResponses,
		Model:     model,
		URL:       url,
		Stream:    getBool(rawReq, "stream"),
		Messages:  parseMessages(rawReq),
		MaxTokens: parseInt(rawReq["max_tokens"]),
		Headers:   headers,
	}

	if req.Stream {
		return h.handleStream(c, req)
	}

	resp, err := h.executeRequest(c, req, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}
	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

func (h *ResponsesHandler) handleStream(c *gin.Context, req *channel.ProviderRequest) error {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	// TODO: 实现 Responses SSE 流式处理（与 ChatHandler 类似）
	return nil
}

func (h *ResponsesHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *ResponsesHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	return nil, types.ErrNoAvailableChannel
}
