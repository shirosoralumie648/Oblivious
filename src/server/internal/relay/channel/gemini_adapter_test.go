package channel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestGeminiAdapter_DoRequestBuildsGenerateContentRequest(t *testing.T) {
	var gotPath string
	var gotKey string
	var gotBody struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
		GenerationConfig struct {
			MaxOutputTokens int `json:"maxOutputTokens"`
		} `json:"generationConfig"`
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-goog-api-key")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2,"totalTokenCount":5}}`))
	}))
	t.Cleanup(upstream.Close)

	adapter := NewGeminiAdapter(upstream.URL, "gemini-key")
	resp, err := adapter.DoRequest(context.Background(), &types.ProviderRequest{
		APIType: types.APITypeChat,
		Model:   "gemini-1.5-flash",
		Messages: []types.Message{
			{Role: "system", Content: "be concise"},
			{Role: "user", Content: "ping"},
			{Role: "assistant", Content: "pong"},
		},
		MaxTokens: 12,
	})
	if err != nil {
		t.Fatalf("DoRequest returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("DoRequest returned nil response")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, body)
	}
	if gotPath != "/v1beta/models/gemini-1.5-flash:generateContent" {
		t.Fatalf("path = %q, want Gemini generateContent path", gotPath)
	}
	if gotKey != "gemini-key" {
		t.Fatalf("x-goog-api-key = %q, want adapter API key", gotKey)
	}
	if len(gotBody.Contents) != 3 {
		t.Fatalf("expected 3 Gemini contents, got %+v", gotBody.Contents)
	}
	if gotBody.Contents[0].Role != "user" || !strings.Contains(gotBody.Contents[0].Parts[0].Text, "be concise") {
		t.Fatalf("system prompt should be folded into first user content: %+v", gotBody.Contents[0])
	}
	if gotBody.Contents[2].Role != "model" || gotBody.Contents[2].Parts[0].Text != "pong" {
		t.Fatalf("assistant message should map to Gemini model role: %+v", gotBody.Contents[2])
	}
	if gotBody.GenerationConfig.MaxOutputTokens != 12 {
		t.Fatalf("maxOutputTokens = %d, want 12", gotBody.GenerationConfig.MaxOutputTokens)
	}
}

func TestGeminiAdapter_MapErrorParsesProviderError(t *testing.T) {
	adapter := NewGeminiAdapter("https://generativelanguage.googleapis.com", "gemini-key")

	providerErr := adapter.MapError(http.StatusForbidden, []byte(`{
		"error": {
			"code": 403,
			"message": "API key not valid",
			"status": "PERMISSION_DENIED"
		}
	}`))

	if providerErr == nil {
		t.Fatal("expected provider error")
	}
	if providerErr.Code != "PERMISSION_DENIED" {
		t.Fatalf("code = %q, want PERMISSION_DENIED", providerErr.Code)
	}
	if !strings.Contains(providerErr.Message, "API key not valid") {
		t.Fatalf("message = %q, want upstream message", providerErr.Message)
	}
	if providerErr.Retryable {
		t.Fatal("403 permission denied must not be retryable")
	}
}
