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

func TestVertexAdapter_DoRequestBuildsGenerateContentRequest(t *testing.T) {
	var gotPath string
	var gotQuery string
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
		gotQuery = r.URL.RawQuery
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"pong"}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}}`))
	}))
	t.Cleanup(upstream.Close)

	adapter := NewVertexAdapter(upstream.URL, "vertex-key|demo-project|us-central1")
	resp, err := adapter.DoRequest(context.Background(), &types.ProviderRequest{
		APIType: types.APITypeChat,
		Model:   "gemini-1.5-pro",
		Messages: []types.Message{
			{Role: "system", Content: "be concise"},
			{Role: "user", Content: "ping"},
			{Role: "assistant", Content: "pong draft"},
		},
		MaxTokens: 24,
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
	if gotPath != "/v1/projects/demo-project/locations/us-central1/publishers/google/models/gemini-1.5-pro:generateContent" {
		t.Fatalf("path = %q, want Vertex publisher model path", gotPath)
	}
	if gotQuery != "key=vertex-key" {
		t.Fatalf("query = %q, want API key query", gotQuery)
	}
	if len(gotBody.Contents) != 3 {
		t.Fatalf("expected 3 Vertex contents, got %+v", gotBody.Contents)
	}
	if gotBody.Contents[0].Role != "user" || !strings.Contains(gotBody.Contents[0].Parts[0].Text, "be concise") {
		t.Fatalf("system prompt should be folded into first user content: %+v", gotBody.Contents[0])
	}
	if gotBody.Contents[2].Role != "model" || gotBody.Contents[2].Parts[0].Text != "pong draft" {
		t.Fatalf("assistant message should map to Vertex/Gemini model role: %+v", gotBody.Contents[2])
	}
	if gotBody.GenerationConfig.MaxOutputTokens != 24 {
		t.Fatalf("maxOutputTokens = %d, want 24", gotBody.GenerationConfig.MaxOutputTokens)
	}
}

func TestVertexAdapter_ConvertResponseExtractsUsage(t *testing.T) {
	adapter := NewVertexAdapter("https://us-central1-aiplatform.googleapis.com", "vertex-key|demo-project|us-central1")

	resp, err := adapter.ConvertResponse([]byte(`{
		"candidates":[{"content":{"parts":[{"text":"pong"}]}}],
		"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}
	}`), false)
	if err != nil {
		t.Fatalf("ConvertResponse returned error: %v", err)
	}
	if resp == nil || resp.Usage == nil {
		t.Fatalf("expected normalized usage, got %+v", resp)
	}
	if resp.Usage.PromptTokens != 5 || resp.Usage.CompletionTokens != 3 || resp.Usage.TotalTokens != 8 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
}
