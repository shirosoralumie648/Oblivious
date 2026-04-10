package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
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

	req := &channel.ProviderRequest{
		APIType: types.APITypeImageGen,
		Model:   model,
	}
	body, _ := io.ReadAll(c.Request.Body)
	req.Body = body

	resp, err := h.executeRequestRaw(c, req, "application/json")
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
	req := &channel.ProviderRequest{
		APIType: types.APITypeImageEdit,
		Model:   model,
	}
	body, _ := io.ReadAll(c.Request.Body)
	req.Body = body

	resp, err := h.executeRequestRaw(c, req, "application/json")
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
	req := &channel.ProviderRequest{
		APIType: types.APITypeImageVar,
		Model:   model,
	}
	body, _ := io.ReadAll(c.Request.Body)
	req.Body = body

	resp, err := h.executeRequestRaw(c, req, "application/json")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}
	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

func (h *ImagesHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	router := GetRouter()
	if router == nil {
		return nil, types.ErrNoAvailableChannel
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("img_%d", time.Now().UnixNano())
	}

	return router.RouteWithBilling(
		c.Request.Context(),
		req.APIType,
		req.Model,
		"",
		idempotencyKey,
		usage,
		func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
			upstreamURL, _ := h.adapter.BuildURL(req.Model, req.APIType)
			headers, _ := h.adapter.BuildHeaders(c.Request.Context(), req.Model, req.APIType)

			upstreamReq, err := http.NewRequest("POST", upstreamURL, bytes.NewReader(req.Body))
			if err != nil {
				return nil, err
			}
			upstreamReq.Header = headers.Clone()
			upstreamReq.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(upstreamReq)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			bodyOut, _ := io.ReadAll(resp.Body)
			return &types.ProviderResponse{StatusCode: resp.StatusCode, Content: bodyOut}, nil
		},
	)
}

func (h *ImagesHandler) executeRequestRaw(c *gin.Context, req *channel.ProviderRequest, contentType string) (*types.ProviderResponse, error) {
	return h.executeRequest(c, req, nil)
}
