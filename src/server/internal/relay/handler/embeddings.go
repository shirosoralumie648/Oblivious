package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/relay/channel"
)

// EmbeddingsHandler Embeddings 处理
type EmbeddingsHandler struct {
	pool    *types.ChannelPoolInterface
	adapter *channel.OpenAIAdapter
}

func NewEmbeddingsHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *EmbeddingsHandler {
	return &EmbeddingsHandler{pool: p, adapter: a}
}

func (h *EmbeddingsHandler) Handle(c *gin.Context) error {
	var rawReq map[string]any
	if err := json.NewDecoder(c.Request.Body).Decode(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "invalid JSON body"}})
		return nil
	}

	model := getString(rawReq, "model")
	if model == "" {
		model = "text-embedding-3-small"
	}

	url, _ := h.adapter.BuildURL(model, types.APITypeEmbeddings)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeEmbeddings)

	req := &channel.ProviderRequest{
		APIType: types.APITypeEmbeddings,
		Model:   model,
		URL:     url,
		Input:   getString(rawReq, "input"),
		Headers: headers,
	}

	usage := h.adapter.EstimateUsage(req)
	_ = usage
	resp, err := h.executeRequest(c, req, usage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}

	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

func (h *EmbeddingsHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *EmbeddingsHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	return nil, types.ErrNoAvailableChannel
}
