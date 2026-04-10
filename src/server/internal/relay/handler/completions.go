package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/relay/channel"
)

// LegacyCompletionsHandler Legacy Completions 处理
type LegacyCompletionsHandler struct {
	pool    *types.ChannelPoolInterface
	adapter *channel.OpenAIAdapter
}

func NewLegacyCompletionsHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *LegacyCompletionsHandler {
	return &LegacyCompletionsHandler{pool: p, adapter: a}
}

func (h *LegacyCompletionsHandler) Handle(c *gin.Context) error {
	var rawReq map[string]any
	if err := json.NewDecoder(c.Request.Body).Decode(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "invalid JSON body"}})
		return nil
	}

	model := getString(rawReq, "model")
	url, _ := h.adapter.BuildURL(model, types.APITypeCompletions)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeCompletions)

	req := &channel.ProviderRequest{
		APIType:   types.APITypeCompletions,
		Model:     model,
		URL:       url,
		Prompt:    getString(rawReq, "prompt"),
		MaxTokens: parseInt(rawReq["max_tokens"]),
		Headers:   headers,
	}

	resp, err := h.executeRequest(c, req, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}
	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

func (h *LegacyCompletionsHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *LegacyCompletionsHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	return nil, types.ErrNoAvailableChannel
}
