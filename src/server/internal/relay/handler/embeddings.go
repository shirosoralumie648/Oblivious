package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
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

	inputText, body, err := parseEmbeddingsInput(rawReq, model)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return nil
	}

	req := &channel.ProviderRequest{
		APIType: types.APITypeEmbeddings,
		Model:   model,
		Input:   inputText,
		Body:    body,
	}

	usage := h.adapter.EstimateUsage(req)
	resp, err := h.executeRequest(c, req, usage)
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

func (h *EmbeddingsHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *EmbeddingsHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	router := GetRouter()
	if router == nil {
		return nil, types.ErrNoAvailableChannel
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("emb_%d", time.Now().UnixNano())
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

			providerReq := &channel.ProviderRequest{
				APIType: req.APIType,
				Model:   req.Model,
				URL:     upstreamURL,
				Input:   req.Input,
				Body:    req.Body,
				Headers: headers,
			}

			return executeProviderAdapterRequest(c.Request.Context(), adapter, providerReq)
		},
	)
}

func parseEmbeddingsInput(rawReq map[string]any, model string) (string, []byte, error) {
	switch input := rawReq["input"].(type) {
	case string:
		return input, nil, nil
	case []any:
		values := make([]string, 0, len(input))
		for _, item := range input {
			text, ok := item.(string)
			if !ok {
				return "", nil, fmt.Errorf("embedding input array must contain strings")
			}
			values = append(values, text)
		}
		body, err := json.Marshal(map[string]any{
			"model": model,
			"input": values,
		})
		if err != nil {
			return "", nil, err
		}
		return strings.Join(values, "\n"), body, nil
	default:
		return "", nil, nil
	}
}
