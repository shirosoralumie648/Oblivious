package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

func TestAudioSpeechHandlerUsesSelectedOpenAICompatibleAdapter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamAuth string
	var upstreamBody struct {
		Model          string `json:"model"`
		Input          string `json:"input"`
		ResponseFormat string `json:"response_format"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("mp3-bytes"))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-openrouter-audio",
			Name:     "OpenRouter audio",
			Provider: "openrouter",
			BaseURL:  upstream.URL,
			APIKey:   "sk-openrouter-audio",
			Enabled:  true,
		},
		ChannelID: "ch-openrouter-audio",
		Enabled:   true,
		Healthy:   true,
	}
	restoreRouter := setRouterForChatTest(&chatTestRouter{selected: selectedChannel})
	t.Cleanup(restoreRouter)

	handler := NewAudioHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(`{
		"model":"tts-1",
		"input":"say this",
		"response_format":"mp3"
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1/audio/speech" {
		t.Fatalf("upstream path = %q, want /v1/audio/speech", upstreamPath)
	}
	if upstreamAuth != "Bearer sk-openrouter-audio" {
		t.Fatalf("authorization = %q, want selected channel key", upstreamAuth)
	}
	if upstreamBody.Model != "tts-1" || upstreamBody.Input != "say this" || upstreamBody.ResponseFormat != "mp3" {
		t.Fatalf("audio speech body not preserved: %+v", upstreamBody)
	}
	if rec.Body.String() != "mp3-bytes" {
		t.Fatalf("expected upstream audio bytes, got %q", rec.Body.String())
	}
}

func TestAudioTranscriptionAndTranslationPassEstimatedUsageToBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "transcription", path: "/v1/audio/transcriptions"},
		{name: "translation", path: "/v1/audio/translations"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.path {
					t.Fatalf("upstream path = %q, want %s", r.URL.Path, tt.path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"text":"transcribed"}`))
			}))
			t.Cleanup(upstream.Close)

			selectedChannel := &types.RouteChannel{
				Channel: &types.Channel{
					ID:       "ch-audio",
					Name:     "Audio channel",
					Provider: "openai",
					BaseURL:  upstream.URL,
					APIKey:   "sk-audio",
					Enabled:  true,
				},
				ChannelID: "ch-audio",
				Enabled:   true,
				Healthy:   true,
			}
			testRouter := &chatTestRouter{selected: selectedChannel}
			restoreRouter := setRouterForChatTest(testRouter)
			t.Cleanup(restoreRouter)

			handler := NewAudioHandler(nil, &channel.OpenAIAdapter{})
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader("fake-audio-bytes"))
			ctx.Request.Header.Set("Content-Type", "audio/mp3")

			if err := handler.Handle(ctx); err != nil {
				t.Fatalf("Handle returned error: %v", err)
			}

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
			}
			if testRouter.lastPrebillUsage == nil || testRouter.lastPrebillUsage.AudioSeconds <= 0 {
				t.Fatalf("expected positive audio usage to reach billing, got %+v", testRouter.lastPrebillUsage)
			}
		})
	}
}
