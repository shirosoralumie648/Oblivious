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

// AudioHandler Audio 处理（speech TTS / transcriptions STT / translations）
type AudioHandler struct {
	pool    *types.ChannelPoolInterface
	adapter *channel.OpenAIAdapter
}

func NewAudioHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *AudioHandler {
	return &AudioHandler{pool: p, adapter: a}
}

func (h *AudioHandler) Handle(c *gin.Context) error {
	path := c.Request.URL.Path
	if path == "/v1/audio/speech" {
		return h.HandleSpeech(c)
	}
	if path == "/v1/audio/transcriptions" {
		return h.HandleTranscriptions(c)
	}
	if path == "/v1/audio/translations" {
		return h.HandleTranslations(c)
	}
	c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "unknown audio path"}})
	return nil
}

func (h *AudioHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

// POST /v1/audio/speech (TTS)
func (h *AudioHandler) HandleSpeech(c *gin.Context) error {
	var rawReq map[string]any
	if err := json.NewDecoder(c.Request.Body).Decode(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "invalid JSON body"}})
		return nil
	}

	model := getString(rawReq, "model")
	if model == "" {
		model = "tts-1"
	}

	req := &channel.ProviderRequest{
		APIType:     types.APITypeAudioSpeech,
		Model:       model,
		Input:       getString(rawReq, "input"),
		AudioFormat: getString(rawReq, "response_format"),
	}

	resp, err := h.executeRequest(c, req, h.adapter.EstimateUsage(req))
	if err != nil {
		writeRelayHandlerError(c, resp, err)
		return nil
	}

	// TTS 返回二进制音频
	statusCode := resp.StatusCode
	if statusCode < http.StatusContinue {
		statusCode = http.StatusOK
	}
	c.Data(statusCode, "audio/mp3", resp.Content)
	return nil
}

// POST /v1/audio/transcriptions (Whisper STT)
func (h *AudioHandler) HandleTranscriptions(c *gin.Context) error {
	model := getString(map[string]any{}, "model")
	if model == "" {
		model = "whisper-1"
	}

	req := &channel.ProviderRequest{
		APIType: types.APITypeAudioSTT,
		Model:   model,
	}
	body, _ := io.ReadAll(c.Request.Body)
	req.Body = body

	resp, err := h.executeRequestRaw(c, req, "audio/mp3", h.adapter.EstimateUsage(req))
	if err != nil {
		writeRelayHandlerError(c, resp, err)
		return nil
	}
	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

// POST /v1/audio/translations (Whisper Translation)
func (h *AudioHandler) HandleTranslations(c *gin.Context) error {
	model := getString(map[string]any{}, "model")
	if model == "" {
		model = "whisper-1"
	}

	req := &channel.ProviderRequest{
		APIType: types.APITypeAudioTranslate,
		Model:   model,
	}
	body, _ := io.ReadAll(c.Request.Body)
	req.Body = body

	resp, err := h.executeRequestRaw(c, req, "audio/mp3", h.adapter.EstimateUsage(req))
	if err != nil {
		writeRelayHandlerError(c, resp, err)
		return nil
	}
	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

func (h *AudioHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	router := GetRouter()
	if router == nil {
		return nil, types.ErrNoAvailableChannel
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("audio_%d", time.Now().UnixNano())
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
				APIType:     req.APIType,
				Model:       req.Model,
				URL:         upstreamURL,
				Input:       req.Input,
				AudioFormat: req.AudioFormat,
				Headers:     headers,
			}

			return executeProviderAdapterRequest(c.Request.Context(), adapter, providerReq)
		},
	)
}

func (h *AudioHandler) executeRequestRaw(c *gin.Context, req *channel.ProviderRequest, contentType string, usage *types.Usage) (*types.ProviderResponse, error) {
	router := GetRouter()
	if router == nil {
		return nil, types.ErrNoAvailableChannel
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("audio_%d", time.Now().UnixNano())
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
			headers.Set("Content-Type", contentType)

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
