package retrieval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"oblivious/server/internal/knowledge"
)

func TestKnowledgeResultRerankerCallsCohereCompatibleEndpoint(t *testing.T) {
	var received struct {
		Authorization string
		Documents     []string
		Method        string
		Model         string
		Path          string
		Query         string
		TopN          int
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Method = r.Method
		received.Path = r.URL.Path
		received.Authorization = r.Header.Get("Authorization")
		var payload struct {
			Documents []string `json:"documents"`
			Model     string   `json:"model"`
			Query     string   `json:"query"`
			TopN      int      `json:"top_n"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode reranker request: %v", err)
		}
		received.Documents = payload.Documents
		received.Model = payload.Model
		received.Query = payload.Query
		received.TopN = payload.TopN
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"index":1,"relevance_score":0.98},{"index":0,"relevance_score":0.31}]}`))
	}))
	defer server.Close()

	reranker := NewKnowledgeResultReranker(knowledge.RerankerConfig{
		APIKey:  "reranker-secret",
		BaseURL: server.URL,
		Model:   "bge-reranker-base",
		TopK:    2,
	})

	results, err := reranker.Rerank(context.Background(), "deployment rollback", []knowledge.KnowledgeRetrievalResult{
		{
			ChunkID:       "chunk_a",
			DocumentID:    "doc_a",
			DocumentTitle: "Alpha",
			Snippet:       "alpha snippet",
		},
		{
			ChunkID:       "chunk_b",
			DocumentID:    "doc_b",
			DocumentTitle: "Beta",
			Source: knowledge.KnowledgeCitation{
				OriginalText: "beta original text",
			},
		},
	}, 1)
	if err != nil {
		t.Fatalf("rerank knowledge results: %v", err)
	}

	if received.Method != http.MethodPost || received.Path != "/rerank" {
		t.Fatalf("expected POST /rerank, got %s %s", received.Method, received.Path)
	}
	if received.Authorization != "Bearer reranker-secret" {
		t.Fatalf("expected bearer auth, got %q", received.Authorization)
	}
	if received.Model != "bge-reranker-base" || received.Query != "deployment rollback" || received.TopN != 2 {
		t.Fatalf("unexpected reranker payload model=%q query=%q topN=%d", received.Model, received.Query, received.TopN)
	}
	if len(received.Documents) != 2 || received.Documents[0] != "alpha snippet" || received.Documents[1] != "beta original text" {
		t.Fatalf("unexpected reranker documents: %+v", received.Documents)
	}
	if len(results) != 1 || results[0].ChunkID != "chunk_b" {
		t.Fatalf("expected top reranked chunk_b, got %+v", results)
	}
	if results[0].Score != 0.98 || results[0].RetrievalMethod != knowledge.KnowledgeRetrievalModeHybridRerank || results[0].Source.ConfidenceScore != 0.98 {
		t.Fatalf("expected rerank score/method/confidence, got %+v", results[0])
	}
}
