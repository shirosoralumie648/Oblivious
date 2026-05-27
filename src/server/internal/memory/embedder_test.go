package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestRelayEmbedder_Embed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content-type: %s", r.Header.Get("Content-Type"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{
					"object":    "embedding",
					"index":     0,
					"embedding": []float32{0.1, 0.2, 0.3},
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]any{
				"prompt_tokens": 3,
				"total_tokens":  3,
			},
		})
	}))
	defer server.Close()

	embedder := NewRelayEmbedder(server.URL, "text-embedding-3-small")
	embedding, err := embedder.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	if len(embedding) != 3 {
		t.Fatalf("expected 3-dimensional embedding, got %d", len(embedding))
	}
	if embedding[0] != 0.1 || embedding[1] != 0.2 || embedding[2] != 0.3 {
		t.Fatalf("unexpected embedding values: %v", embedding)
	}
}

func TestRelayEmbedder_ForwardsTrustedRelayIdentityHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(types.HeaderInternalAuth); got != types.SharedInternalToken {
			t.Fatalf("expected internal auth header, got %q", got)
		}
		if got := r.Header.Get(types.HeaderInternalUserID); got != "user_1" {
			t.Fatalf("expected user header user_1, got %q", got)
		}
		if got := r.Header.Get(types.HeaderInternalOrganization); got != "org_1" {
			t.Fatalf("expected organization header org_1, got %q", got)
		}
		if got := r.Header.Get(types.HeaderRequestID); got != "req_1" {
			t.Fatalf("expected request id req_1, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"object": "embedding", "index": 0, "embedding": []float32{0.1}},
			},
		})
	}))
	defer server.Close()

	ctx := context.Background()
	ctx = types.WithTrustedUserID(ctx, "user_1")
	ctx = types.WithTrustedOrganizationID(ctx, "org_1")
	ctx = types.WithTrustedRequestID(ctx, "req_1")

	embedder := NewRelayEmbedder(server.URL, "text-embedding-3-small")
	if _, err := embedder.Embed(ctx, "hello world"); err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
}

func TestRelayEmbedder_EmbedBatch_BatchingBehavior(t *testing.T) {
	// Create 250 texts to exceed the default 100 batch size.
	texts := make([]string, 250)
	for i := range texts {
		texts[i] = "text " + string(rune('a'+i%26))
	}

	var batchCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batchCount++

		var req embeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}

		// Each batch should have at most 100 items.
		if len(req.Input) > 100 {
			t.Errorf("batch size %d exceeds max 100", len(req.Input))
		}

		data := make([]map[string]any, len(req.Input))
		for i := range req.Input {
			data[i] = map[string]any{
				"object":    "embedding",
				"index":     i,
				"embedding": []float32{float32(i) * 0.1, 0.5, 0.9},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   data,
		})
	}))
	defer server.Close()

	embedder := NewRelayEmbedder(server.URL, "text-embedding-3-small")
	embeddings, err := embedder.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}
	if len(embeddings) != 250 {
		t.Fatalf("expected 250 embeddings, got %d", len(embeddings))
	}
	// 250 texts with batch size 100 = 3 batches.
	if batchCount != 3 {
		t.Fatalf("expected 3 batches, got %d", batchCount)
	}
}

func TestRelayEmbedder_Embed_EmptyInput(t *testing.T) {
	embedder := NewRelayEmbedder("http://localhost:8080/v1", "text-embedding-3-small")
	embeddings, err := embedder.EmbedBatch(context.Background(), []string{})
	if err != nil {
		t.Fatalf("EmbedBatch with empty input should not error: %v", err)
	}
	if embeddings != nil {
		t.Fatal("expected nil embeddings for empty input")
	}
}

func TestRelayEmbedder_EmbedBatch_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"internal failure","type":"server_error"}}`))
	}))
	defer server.Close()

	embedder := NewRelayEmbedder(server.URL, "text-embedding-3-small")
	_, err := embedder.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestRelayEmbedder_Embed_NetworkError(t *testing.T) {
	embedder := NewRelayEmbedder("http://127.0.0.1:1/v1", "text-embedding-3-small")
	_, err := embedder.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestRelayEmbedder_Embed_APIErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "rate limit exceeded",
				"type":    "rate_limit_error",
				"code":    "429",
			},
		})
	}))
	defer server.Close()

	embedder := NewRelayEmbedder(server.URL, "text-embedding-3-small")
	_, err := embedder.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for API error response, got nil")
	}
}

func TestRelayEmbedder_SetCustomBatchSize(t *testing.T) {
	texts := make([]string, 50)
	for i := range texts {
		texts[i] = "text"
	}

	var batchCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batchCount++
		data := make([]map[string]any, 1)
		data[0] = map[string]any{"object": "embedding", "index": 0, "embedding": []float32{0.1}}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	}))
	defer server.Close()

	embedder := NewRelayEmbedder(server.URL, "text-embedding-3-small")
	embedder.SetBatchSize(25)

	_, err := embedder.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}
	// 50 texts with batch size 25 = 2 batches.
	if batchCount != 2 {
		t.Fatalf("expected 2 batches with batch size 25, got %d", batchCount)
	}
}

func TestRelayEmbedder_DefaultBatchSizeRejectsZero(t *testing.T) {
	embedder := NewRelayEmbedder("http://localhost:8080/v1", "text-embedding-3-small")
	embedder.SetBatchSize(0)
	if embedder.batchSize == 0 {
		t.Fatal("SetBatchSize(0) should be ignored, keeping batchSize > 0")
	}
}
