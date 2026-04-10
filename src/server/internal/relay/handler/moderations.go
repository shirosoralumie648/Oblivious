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

	req := &channel.ProviderRequest{
		APIType: types.APITypeModeration,
		Model:   model,
		Input:   getString(rawReq, "input"),
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
	router := GetRouter()
	if router == nil {
		return nil, types.ErrNoAvailableChannel
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("mod_%d", time.Now().UnixNano())
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

			providerReq := &channel.ProviderRequest{
				APIType: req.APIType,
				Model:   req.Model,
				URL:     upstreamURL,
				Input:   req.Input,
				Headers: headers,
			}

			return h.doUpstreamRequest(providerReq)
		},
	)
}

func (h *ModerationsHandler) doUpstreamRequest(req *channel.ProviderRequest) (*types.ProviderResponse, error) {
	body, err := marshalRequest(req)
	if err != nil {
		return nil, err
	}
	upstreamReq, err := http.NewRequest("POST", req.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	upstreamReq.Header = req.Headers.Clone()
	upstreamReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyOut, _ := io.ReadAll(resp.Body)
	return &types.ProviderResponse{StatusCode: resp.StatusCode, Content: bodyOut}, nil
}
