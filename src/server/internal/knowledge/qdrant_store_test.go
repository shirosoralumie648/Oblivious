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
