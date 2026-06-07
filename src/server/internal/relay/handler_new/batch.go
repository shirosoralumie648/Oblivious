package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

// BatchHandler Batch 处理（submit 原生 + status 透传）
type BatchHandler struct {
	pool    *types.ChannelPoolInterface
	adapter *channel.OpenAIAdapter
}

func NewBatchHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *BatchHandler {
	return &BatchHandler{pool: p, adapter: a}
}

func (h *BatchHandler) Handle(c *gin.Context) error {
	path := c.Request.URL.Path
	if path == "/v1/batch" {
		return h.HandleSubmit(c)
	}
	if path == "/v1/batches" {
		h.HandleList(c)
		return nil
	}
	// GET /v1/batches/:id — 透传
	h.HandleGet(c)
	return nil
}

func (h *BatchHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

// POST /v1/batch (原生处理 - 走异步计费)
func (h *BatchHandler) HandleSubmit(c *gin.Context) error {
	model := "gpt-4o"
	body, _ := io.ReadAll(c.Request.Body)

	url, _ := h.adapter.BuildURL(model, types.APITypeBatch)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeBatch)

	req := &channel.ProviderRequest{
		APIType:   types.APITypeBatch,
		Model:     model,
		URL:       url,
		Headers:   headers,
		RequestID: c.GetHeader("X-Request-ID"),
		Body:      body,
	}
	_ = req

	// TODO: PreBill 预扣（Plan D 实现 BillingHook 后启用）
	// session, err := h.billing.PreBill(c.Request.Context(), req, nil)

	// 提交到 OpenAI
	upstreamURL, _ := h.adapter.BuildURL(model, types.APITypeBatch)
	upstreamReq, err := http.NewRequest("POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return nil
	}
	upstreamHeaders, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeBatch)
	upstreamReq.Header = upstreamHeaders
	upstreamReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return nil
	}
	defer resp.Body.Close()

	bodyOut, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		c.Data(resp.StatusCode, "application/json", bodyOut)
		return nil
	}

	// TODO: 提取 batch_id，注册 Asynq polling 任务（Plan D）

	c.Data(resp.StatusCode, "application/json", bodyOut)
	return nil
}

// GET /v1/batches (透传)
func (h *BatchHandler) HandleList(c *gin.Context) {
	h.passthrough(c, "GET", "/v1/batches", nil)
}

// GET /v1/batches/:id (透传)
func (h *BatchHandler) HandleGet(c *gin.Context) {
	id := c.Param("id")
	h.passthrough(c, "GET", "/v1/batches/"+id, nil)
}

func (h *BatchHandler) passthrough(c *gin.Context, method, path string, body []byte) {
	upstreamURL, _ := h.adapter.BuildURL("gpt-4o", types.APITypeBatch)
	upstreamURL = upstreamURL + path
	req, err := http.NewRequest(method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return
	}
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), "gpt-4o", types.APITypeBatch)
	req.Header = headers
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return
	}
	defer resp.Body.Close()

	bodyOut, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", bodyOut)
}

func (h *BatchHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	router := GetRouter()
	if router == nil {
		return nil, types.ErrNoAvailableChannel
	}

	idempotencyKey := c.GetHeader("X-Request-ID")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("batch_%d", time.Now().UnixNano())
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
