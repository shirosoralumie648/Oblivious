package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/relay/channel"
)

// ImagesHandler Images 处理（generations/edits/variations）
type ImagesHandler struct {
	pool    *types.ChannelPoolInterface
	adapter *channel.OpenAIAdapter
}

func NewImagesHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *ImagesHandler {
	return &ImagesHandler{pool: p, adapter: a}
}

func (h *ImagesHandler) Handle(c *gin.Context) error {
	path := c.Request.URL.Path
	if path == "/v1/images/generations" {
		return h.HandleGenerations(c)
	}
	if path == "/v1/images/edits" {
		return h.HandleEdits(c)
	}
	if path == "/v1/images/variations" {
		return h.HandleVariations(c)
	}
	c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "unknown images path"}})
	return nil
}

func (h *ImagesHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

// /v1/images/generations
func (h *ImagesHandler) HandleGenerations(c *gin.Context) error {
	var rawReq map[string]any
	if err := json.NewDecoder(c.Request.Body).Decode(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "invalid JSON body"}})
		return nil
	}

	model := getString(rawReq, "model")
	if model == "" {
		model = "dall-e-3"
	}

	url, _ := h.adapter.BuildURL(model, types.APITypeImageGen)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeImageGen)

	// 透传整个 body
	body, _ := io.ReadAll(c.Request.Body)

	req := &channel.ProviderRequest{
		APIType: types.APITypeImageGen,
		Model:   model,
		URL:     url,
		Headers: headers,
		Body:    body,
	}

	resp, err := h.executeRequest(c, req, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}
	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

// /v1/images/edits
func (h *ImagesHandler) HandleEdits(c *gin.Context) error {
	model := getString(map[string]any{}, "model")
	if model == "" {
		model = "dall-e-3"
	}
	url, _ := h.adapter.BuildURL(model, types.APITypeImageEdit)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeImageEdit)
	body, _ := io.ReadAll(c.Request.Body)

	req := &channel.ProviderRequest{
		APIType: types.APITypeImageEdit,
		Model:   model,
		URL:     url,
		Headers: headers,
		Body:    body,
	}

	resp, err := h.executeRequest(c, req, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}
	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

// /v1/images/variations
func (h *ImagesHandler) HandleVariations(c *gin.Context) error {
	model := "dall-e-3"
	url, _ := h.adapter.BuildURL(model, types.APITypeImageVar)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeImageVar)
	body, _ := io.ReadAll(c.Request.Body)

	req := &channel.ProviderRequest{
		APIType: types.APITypeImageVar,
		Model:   model,
		URL:     url,
		Headers: headers,
		Body:    body,
	}

	resp, err := h.executeRequest(c, req, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}
	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

func (h *ImagesHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	return nil, types.ErrNoAvailableChannel
}
