package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

// BatchHandler Batch 处理（submit 原生 + status 透传）
type BatchHandler struct {
	pool                       *types.ChannelPoolInterface
	adapter                    *channel.OpenAIAdapter
	pollingRegistrar           BatchPollingRegistrar
	commercialLifecycleEnabled bool
}

func NewBatchHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *BatchHandler {
	return &BatchHandler{pool: p, adapter: a}
}

type BatchPollingRegistration struct {
	BatchID                  string
	RequestID                string
	UserID                   string
	OrganizationID           string
	APITokenID               string
	FeatureType              string
	Model                    string
	APIType                  types.APIType
	BillingSessionID         string
	PreauthorizedAmount      float64
	TokenPreauthorizedAmount float64
}

type BatchPollingRegistrar interface {
	RegisterBatchPolling(ctx context.Context, task BatchPollingRegistration) error
}

func (h *BatchHandler) WithPollingRegistrar(registrar BatchPollingRegistrar) *BatchHandler {
	h.pollingRegistrar = registrar
	return h
}

func (h *BatchHandler) WithCommercialLifecycleEnabled(enabled bool) *BatchHandler {
	h.commercialLifecycleEnabled = enabled
	return h
}

func (h *BatchHandler) Handle(c *gin.Context) error {
	if !h.lifecycleEnabled() {
		h.writeDisabled(c)
		return nil
	}
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
	if !h.lifecycleEnabled() {
		h.writeDisabled(c)
		return nil
	}
	body, _ := io.ReadAll(c.Request.Body)
	model := extractBatchSubmitModel(body)
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "batch_model_required", "message": "batch model is required"}})
		return nil
	}

	url, err := h.adapter.BuildURL(model, types.APITypeBatch)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return nil
	}
	headers, err := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeBatch)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return nil
	}

	req := &channel.ProviderRequest{
		APIType:   types.APITypeBatch,
		Model:     model,
		URL:       url,
		Headers:   headers,
		RequestID: c.GetHeader("X-Request-ID"),
		Body:      body,
	}

	if GetRouter() != nil {
		resp, err := h.executeRequest(c, req, estimateBatchSubmitUsage(h.adapter, req))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
			return nil
		}
		if err := h.registerBatchPolling(c, resp, model); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "batch_polling_registration_failed", "message": err.Error()}})
			return nil
		}
		contentType := "application/json"
		if resp.Headers != nil && resp.Headers.Get("Content-Type") != "" {
			contentType = resp.Headers.Get("Content-Type")
		}
		statusCode := resp.StatusCode
		if statusCode < http.StatusContinue {
			statusCode = http.StatusOK
		}
		c.Data(statusCode, contentType, resp.Content)
		return nil
	}

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

	if err := h.registerBatchPolling(c, &types.ProviderResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Content:    bodyOut,
	}, model); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "batch_polling_registration_failed", "message": err.Error()}})
		return nil
	}

	c.Data(resp.StatusCode, "application/json", bodyOut)
	return nil
}

func extractBatchSubmitModel(body []byte) string {
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Model)
}

func estimateBatchSubmitUsage(adapter *channel.OpenAIAdapter, req *channel.ProviderRequest) *types.Usage {
	if adapter != nil {
		if usage := adapter.EstimateUsage(req); usage != nil {
			return usage
		}
	}
	return &types.Usage{}
}

func (h *BatchHandler) registerBatchPolling(c *gin.Context, resp *types.ProviderResponse, model string) error {
	if h.pollingRegistrar == nil {
		return nil
	}
	if resp == nil {
		return fmt.Errorf("upstream batch response is empty")
	}
	batchID := extractBatchID(resp.Content)
	if batchID == "" {
		return fmt.Errorf("upstream batch response did not include id")
	}
	ctx := c.Request.Context()
	userID, _ := types.TrustedUserIDFromContext(ctx)
	organizationID, _ := types.TrustedOrganizationIDFromContext(ctx)
	apiTokenID, _ := types.TrustedAPITokenIDFromContext(ctx)
	featureType, _ := types.TrustedFeatureTypeFromContext(ctx)
	return h.pollingRegistrar.RegisterBatchPolling(ctx, BatchPollingRegistration{
		BatchID:                  batchID,
		RequestID:                c.GetHeader(types.HeaderRequestID),
		UserID:                   userID,
		OrganizationID:           organizationID,
		APITokenID:               apiTokenID,
		FeatureType:              featureType,
		Model:                    model,
		APIType:                  types.APITypeBatch,
		BillingSessionID:         resp.BillingSessionID,
		PreauthorizedAmount:      resp.PreauthorizedAmount,
		TokenPreauthorizedAmount: resp.TokenPreauthorizedAmount,
	})
}

func extractBatchID(body []byte) string {
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.ID)
}

// GET /v1/batches (透传)
func (h *BatchHandler) HandleList(c *gin.Context) {
	if !h.lifecycleEnabled() {
		h.writeDisabled(c)
		return
	}
	h.passthrough(c, "GET", "/v1/batches", nil)
}

// GET /v1/batches/:id (透传)
func (h *BatchHandler) HandleGet(c *gin.Context) {
	if !h.lifecycleEnabled() {
		h.writeDisabled(c)
		return
	}
	id := c.Param("id")
	h.passthrough(c, "GET", "/v1/batches/"+id, nil)
}

func (h *BatchHandler) lifecycleEnabled() bool {
	return h != nil && h.commercialLifecycleEnabled
}

func (h *BatchHandler) writeDisabled(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{
		"code":    "unsupported_api",
		"message": "batch API is disabled until the commercial polling, settlement, refund, and audit lifecycle is enabled",
	}})
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
			return &types.ProviderResponse{StatusCode: resp.StatusCode, Headers: resp.Header.Clone(), Content: bodyOut}, nil
		},
	)
}
