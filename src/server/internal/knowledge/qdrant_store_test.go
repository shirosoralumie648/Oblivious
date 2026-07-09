package knowledge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQdrantVectorStoreEnsuresTenantCollection(t *testing.T) {
	var received struct {
		method string
		path   string
		apiKey string
		body   struct {
			Vectors struct {
				Size     int    `json:"size"`
				Distance string `json:"distance"`
			} `json:"vectors"`
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.method = r.Method
		received.path = r.URL.Path
		received.apiKey = r.Header.Get("api-key")
		if err := json.NewDecoder(r.Body).Decode(&received.body); err != nil {
			t.Fatalf("decode qdrant request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := NewQdrantVectorStore(server.URL, "qdrant-secret")
	if err := store.EnsureKnowledgeBaseCollection(context.Background(), "Org 123/Prod", "KB:Main", 3072); err != nil {
		t.Fatalf("ensure qdrant collection: %v", err)
	}

	if received.method != http.MethodPut {
		t.Fatalf("expected PUT, got %s", received.method)
	}
	if received.path != "/collections/kb_org_123_prod_kb_main" {
		t.Fatalf("unexpected path %q", received.path)
	}
	if received.apiKey != "qdrant-secret" {
		t.Fatalf("expected qdrant api key header, got %q", received.apiKey)
	}
	if received.body.Vectors.Size != 3072 || received.body.Vectors.Distance != "Cosine" {
		t.Fatalf("unexpected vector config %+v", received.body.Vectors)
	}
}

func TestQdrantVectorStoreDeletesTenantCollection(t *testing.T) {
	var received struct {
		method string
		path   string
		apiKey string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.method = r.Method
		received.path = r.URL.Path
		received.apiKey = r.Header.Get("api-key")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := NewQdrantVectorStore(server.URL, "qdrant-secret")
	if err := store.DeleteKnowledgeBaseCollection(context.Background(), "org_acme", "kb_product_docs"); err != nil {
		t.Fatalf("delete qdrant collection: %v", err)
	}

	if received.method != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", received.method)
	}
	if received.path != "/collections/kb_org_acme_kb_product_docs" {
		t.Fatalf("unexpected path %q", received.path)
	}
	if received.apiKey != "qdrant-secret" {
		t.Fatalf("expected qdrant api key header, got %q", received.apiKey)
	}
}

func TestQdrantVectorStoreDeletesDocumentPointsByPayloadFilter(t *testing.T) {
	var received struct {
		method string
		path   string
		apiKey string
		body   struct {
			Filter struct {
				Must []struct {
					Key   string `json:"key"`
					Match struct {
						Value string `json:"value"`
					} `json:"match"`
				} `json:"must"`
			} `json:"filter"`
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.method = r.Method
		received.path = r.URL.Path
		received.apiKey = r.Header.Get("api-key")
		if err := json.NewDecoder(r.Body).Decode(&received.body); err != nil {
			t.Fatalf("decode qdrant delete body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := NewQdrantVectorStore(server.URL, "qdrant-secret")
	if err := store.DeleteKnowledgeDocumentChunks(context.Background(), "org_acme", "kb_product_docs", "doc_1"); err != nil {
		t.Fatalf("delete qdrant document chunks: %v", err)
	}

	if received.method != http.MethodPost {
		t.Fatalf("expected POST, got %s", received.method)
	}
	if received.path != "/collections/kb_org_acme_kb_product_docs/points/delete" {
		t.Fatalf("unexpected path %q", received.path)
	}
	if received.apiKey != "qdrant-secret" {
		t.Fatalf("expected qdrant api key header, got %q", received.apiKey)
	}
	if len(received.body.Filter.Must) != 1 {
		t.Fatalf("expected one delete filter condition, got %+v", received.body.Filter.Must)
	}
	condition := received.body.Filter.Must[0]
	if condition.Key != "document_id" || condition.Match.Value != "doc_1" {
		t.Fatalf("unexpected delete filter condition %+v", condition)
	}
}

func TestQdrantVectorStoreUpsertsTenantChunkPoints(t *testing.T) {
	var received struct {
		method string
		path   string
		apiKey string
		body   struct {
			Points []struct {
				ID      string         `json:"id"`
				Vector  []float32      `json:"vector"`
				Payload map[string]any `json:"payload"`
			} `json:"points"`
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.method = r.Method
		received.path = r.URL.Path
		received.apiKey = r.Header.Get("api-key")
		if err := json.NewDecoder(r.Body).Decode(&received.body); err != nil {
			t.Fatalf("decode qdrant upsert body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := NewQdrantVectorStore(server.URL, "qdrant-secret")
	err := store.UpsertKnowledgeDocumentChunks(context.Background(), "org_acme", "kb_product_docs", "doc_1", []KnowledgeDocumentChunk{
		{
			ChunkIndex:          2,
			Content:             "Deployment rollback content",
			DocumentTitle:       "Deployment Runbook",
			DocumentVersion:     "v2",
			Embedding:           []float32{0.1, 0.2, 0.3},
			EstimatedTokenCount: 42,
			Metadata: KnowledgeChunkMetadata{
				PageNumber: 7,
				SourceURL:  "https://docs.example/runbook.md",
				StartRune:  10,
				EndRune:    38,
			},
		},
	})
	if err != nil {
		t.Fatalf("upsert qdrant chunks: %v", err)
	}

	if received.method != http.MethodPut {
		t.Fatalf("expected PUT, got %s", received.method)
	}
	if received.path != "/collections/kb_org_acme_kb_product_docs/points" {
		t.Fatalf("unexpected path %q", received.path)
	}
	if received.apiKey != "qdrant-secret" {
		t.Fatalf("expected qdrant api key header, got %q", received.apiKey)
	}
	if len(received.body.Points) != 1 {
		t.Fatalf("expected one qdrant point, got %+v", received.body.Points)
	}
	point := received.body.Points[0]
	if point.ID != "doc_1_2_v2" {
		t.Fatalf("expected deterministic point id doc_1_2_v2, got %q", point.ID)
	}
	if len(point.Vector) != 3 || point.Vector[0] != 0.1 {
		t.Fatalf("unexpected vector payload %+v", point.Vector)
	}
	if point.Payload["document_id"] != "doc_1" || point.Payload["chunk_index"] != float64(2) || point.Payload["document_version"] != "v2" {
		t.Fatalf("unexpected qdrant payload %+v", point.Payload)
	}
	if point.Payload["document_title"] != "Deployment Runbook" {
		t.Fatalf("expected document_title payload, got %+v", point.Payload)
	}
	if point.Payload["source_url"] != "https://docs.example/runbook.md" || point.Payload["page_number"] != float64(7) {
		t.Fatalf("expected source metadata payload, got %+v", point.Payload)
	}
}

func TestQdrantVectorStoreSearchesTenantChunkPoints(t *testing.T) {
	var received struct {
		method string
		path   string
		apiKey string
		body   struct {
			Limit       int       `json:"limit"`
			ScoreLimit  float64   `json:"score_threshold"`
			Vector      []float32 `json:"vector"`
			WithPayload bool      `json:"with_payload"`
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.method = r.Method
		received.path = r.URL.Path
		received.apiKey = r.Header.Get("api-key")
		if err := json.NewDecoder(r.Body).Decode(&received.body); err != nil {
			t.Fatalf("decode qdrant search body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"result": [
				{
					"id": "doc_1_2_v2",
					"score": 0.93,
					"payload": {
						"chunk_index": 2,
						"content": "Deployment rollback content",
						"document_id": "doc_1",
						"document_title": "Deployment Runbook",
						"document_version": "v2",
						"page_number": 7,
						"source_url": "https://docs.example/runbook.md"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	store := NewQdrantVectorStore(server.URL, "qdrant-secret")
	results, err := store.SearchKnowledgeChunks(context.Background(), "org_acme", "kb_product_docs", "deployment rollback", []float32{0.1, 0.2, 0.3}, KnowledgeRetrievalOptions{Limit: 4, MinScore: 0.42})
	if err != nil {
		t.Fatalf("search qdrant chunks: %v", err)
	}

	if received.method != http.MethodPost {
		t.Fatalf("expected POST, got %s", received.method)
	}
	if received.path != "/collections/kb_org_acme_kb_product_docs/points/search" {
		t.Fatalf("unexpected path %q", received.path)
	}
	if received.apiKey != "qdrant-secret" {
		t.Fatalf("expected qdrant api key header, got %q", received.apiKey)
	}
	if received.body.Limit != 4 || received.body.ScoreLimit != 0.42 || !received.body.WithPayload || len(received.body.Vector) != 3 {
		t.Fatalf("unexpected qdrant search body %+v", received.body)
	}
	if len(results) != 1 {
		t.Fatalf("expected one search result, got %+v", results)
	}
	result := results[0]
	if result.DocumentID != "doc_1" || result.ChunkID != "doc_1_2_v2" || result.ChunkIndex != 2 || result.Score != 0.93 {
		t.Fatalf("unexpected qdrant search result %+v", result)
	}
	if result.Source.PageNumber != 7 || result.Source.SourceURL != "https://docs.example/runbook.md" {
		t.Fatalf("expected source metadata from qdrant payload, got %+v", result.Source)
	}
}

func TestQdrantVectorStoreReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "collection failure", http.StatusBadGateway)
	}))
	defer server.Close()

	store := NewQdrantVectorStore(server.URL, "")
	if err := store.EnsureKnowledgeBaseCollection(context.Background(), "org", "kb", 1536); err == nil {
		t.Fatal("expected qdrant HTTP error")
	}
}
