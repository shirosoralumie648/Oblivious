package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/relay/channel"
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

	url, _ := h.adapter.BuildURL(model, types.APITypeAudioSpeech)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeAudioSpeech)

	req := &channel.ProviderRequest{
		APIType:     types.APITypeAudioSpeech,
		Model:       model,
		URL:         url,
		Input:       getString(rawReq, "input"),
		AudioFormat: getString(rawReq, "response_format"),
		Headers:     headers,
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

	url, _ := h.adapter.BuildURL(model, types.APITypeAudioSTT)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeAudioSTT)

	req := &channel.ProviderRequest{
		APIType: types.APITypeAudioSTT,
		Model:   model,
		URL:     url,
		Headers: headers,
	}
	body, _ := io.ReadAll(c.Request.Body)
	req.Body = body

	resp, err := h.executeRequest(c, req, nil)
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

	url, _ := h.adapter.BuildURL(model, types.APITypeAudioTranslate)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeAudioTranslate)

	req := &channel.ProviderRequest{
		APIType: types.APITypeAudioTranslate,
		Model:   model,
		URL:     url,
		Headers: headers,
	}
	body, _ := io.ReadAll(c.Request.Body)
	req.Body = body

	resp, err := h.executeRequest(c, req, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}
	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

func (h *AudioHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	return nil, types.ErrNoAvailableChannel
}
