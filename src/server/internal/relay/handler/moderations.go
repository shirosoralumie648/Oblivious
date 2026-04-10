package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/relay/channel"
)

// ModerationsHandler Moderations 处理
type ModerationsHandler struct {
	pool    *types.ChannelPoolInterface
	adapter *channel.OpenAIAdapter
}

func NewModerationsHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *ModerationsHandler {
	return &ModerationsHandler{pool: p, adapter: a}
}

func (h *ModerationsHandler) Handle(c *gin.Context) error {
	var rawReq map[string]any
	if err := json.NewDecoder(c.Request.Body).Decode(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "invalid JSON body"}})
		return nil
	}

	model := getString(rawReq, "model")
	if model == "" {
		model = "omni-moderation-latest"
	}

	url, _ := h.adapter.BuildURL(model, types.APITypeModeration)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeModeration)

	req := &channel.ProviderRequest{
		APIType: types.APITypeModeration,
		Model:   model,
		URL:     url,
		Input:   getString(rawReq, "input"),
		Headers: headers,
	}

	resp, err := h.executeRequest(c, req, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}
	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

func (h *ModerationsHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *ModerationsHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	return nil, types.ErrNoAvailableChannel
}
