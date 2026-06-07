package handler

import (
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
	body, err := json.Marshal(rawReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "invalid JSON body"}})
		return nil
	}
	req.Body = body

	resp, err := h.executeRequestRaw(c, req, "application/json")
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
			headers.Set("Content-Type", "application/json")

			providerReq := &channel.ProviderRequest{
				APIType: req.APIType,
				Model:   req.Model,
				URL:     upstreamURL,
				Body:    req.Body,
				Headers: headers,
			}

			return executeProviderAdapterRequest(c.Request.Context(), adapter, providerReq)
		},
	)
}

func (h *ImagesHandler) executeRequestRaw(c *gin.Context, req *channel.ProviderRequest, contentType string) (*types.ProviderResponse, error) {
	_ = contentType
	return h.executeRequest(c, req, h.adapter.EstimateUsage(req))
}
