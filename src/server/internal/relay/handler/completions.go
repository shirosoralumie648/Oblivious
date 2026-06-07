package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	relaycache "oblivious/server/internal/relay/cache"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

// LegacyCompletionsHandler Legacy Completions 处理
type LegacyCompletionsHandler struct {
	pool                  *types.ChannelPoolInterface
	adapter               *channel.OpenAIAdapter
	semanticCacheEmbedder SemanticCacheEmbedder
}

func NewLegacyCompletionsHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *LegacyCompletionsHandler {
	return &LegacyCompletionsHandler{pool: p, adapter: a}
}

func NewLegacyCompletionsHandlerWithSemanticCacheEmbedder(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter, embedder SemanticCacheEmbedder) *LegacyCompletionsHandler {
	return &LegacyCompletionsHandler{pool: p, adapter: a, semanticCacheEmbedder: embedder}
}

func (h *LegacyCompletionsHandler) Handle(c *gin.Context) error {
	var rawReq map[string]any
	if err := json.NewDecoder(c.Request.Body).Decode(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "invalid JSON body"}})
		return nil
	}

	model := getString(rawReq, "model")
	req := &channel.ProviderRequest{
		APIType:   types.APITypeCompletions,
		Model:     model,
		Prompt:    getString(rawReq, "prompt"),
		MaxTokens: parseInt(rawReq["max_tokens"]),
	}

	applyTrustedInternalIdentity(c)

	usage := h.adapter.EstimateUsage(req)
	if cacheReq, cacheable := semanticCacheRequestFromCompletion(c.Request.Context(), req); cacheable {
		cacheReq = attachSemanticCacheEmbedding(c.Request.Context(), cacheReq, h.semanticCacheEmbedder)
		c.Request = c.Request.WithContext(types.WithSemanticCacheRequest(c.Request.Context(), cacheReq))
	}

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

func (h *LegacyCompletionsHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func semanticCacheRequestFromCompletion(ctx context.Context, req *channel.ProviderRequest) (types.SemanticCacheRequest, bool) {
	if req == nil || strings.TrimSpace(req.Model) == "" || strings.TrimSpace(req.Prompt) == "" {
		return types.SemanticCacheRequest{}, false
	}
	query := canonicalCompletionSemanticCacheQuery(req)
	if strings.TrimSpace(query) == "" {
		return types.SemanticCacheRequest{}, false
	}
	organizationID, hasOrganizationID := types.TrustedOrganizationIDFromContext(ctx)
	userID, hasUserID := types.TrustedUserIDFromContext(ctx)
	cacheReq := types.SemanticCacheRequest{
		OrganizationID: organizationID,
		UserID:         userID,
		Model:          req.Model,
		Query:          query,
	}
	if hasOrganizationID || hasUserID {
		cacheReq.UserScoped = relaycache.IsSensitiveSemanticCacheRequest(cacheReq)
	}
	return cacheReq, true
}

func canonicalCompletionSemanticCacheQuery(req *channel.ProviderRequest) string {
	payload := struct {
		APIType   string `json:"api_type"`
		Route     string `json:"route"`
		Prompt    string `json:"prompt"`
		MaxTokens int    `json:"max_tokens,omitempty"`
	}{
		APIType:   req.APIType.String(),
		Route:     "/v1/completions",
		Prompt:    req.Prompt,
		MaxTokens: req.MaxTokens,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(body)
}

func (h *LegacyCompletionsHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	router := GetRouter()
	if router == nil {
		return nil, types.ErrNoAvailableChannel
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("comp_%d", time.Now().UnixNano())
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
				APIType:   req.APIType,
				Model:     req.Model,
				URL:       upstreamURL,
				Prompt:    req.Prompt,
				MaxTokens: req.MaxTokens,
				Headers:   headers,
			}

			return executeProviderAdapterRequest(c.Request.Context(), adapter, providerReq)
		},
	)
}
