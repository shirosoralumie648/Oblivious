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

	resp, err := h.executeRequest(c, req, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}

	// TTS 返回二进制音频
	c.Data(http.StatusOK, "audio/mp3", resp.Content)
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

	resp, err := h.executeRequestRaw(c, req, "audio/mp3")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
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

	resp, err := h.executeRequestRaw(c, req, "audio/mp3")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
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
			upstreamURL, _ := h.adapter.BuildURL(req.Model, req.APIType)
			headers, _ := h.adapter.BuildHeaders(c.Request.Context(), req.Model, req.APIType)

			providerReq := &channel.ProviderRequest{
				APIType:     req.APIType,
				Model:       req.Model,
				URL:         upstreamURL,
				Input:       req.Input,
				AudioFormat: req.AudioFormat,
				Headers:     headers,
			}

			return h.doUpstreamRequest(providerReq)
		},
	)
}

func (h *AudioHandler) executeRequestRaw(c *gin.Context, req *channel.ProviderRequest, contentType string) (*types.ProviderResponse, error) {
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
		nil,
		func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
			upstreamURL, _ := h.adapter.BuildURL(req.Model, req.APIType)
			headers, _ := h.adapter.BuildHeaders(c.Request.Context(), req.Model, req.APIType)

			upstreamReq, err := http.NewRequest("POST", upstreamURL, bytes.NewReader(req.Body))
			if err != nil {
				return nil, err
			}
			upstreamReq.Header = headers.Clone()
			upstreamReq.Header.Set("Content-Type", contentType)

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

func (h *AudioHandler) doUpstreamRequest(req *channel.ProviderRequest) (*types.ProviderResponse, error) {
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

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyOut, _ := io.ReadAll(resp.Body)
	return &types.ProviderResponse{StatusCode: resp.StatusCode, Content: bodyOut}, nil
}
