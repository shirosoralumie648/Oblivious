package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/config"
	"oblivious/server/internal/knowledge"
)

type routerKnowledgeStore struct {
	createdBase knowledge.KnowledgeBase
}

func (s *routerKnowledgeStore) CreateKnowledgeBase(ctx context.Context, workspaceID, name string) (knowledge.KnowledgeBase, error) {
	return s.createdBase, nil
}

func TestNewKnowledgeServiceWiresQdrantVectorStore(t *testing.T) {
	var received struct {
		method string
		path   string
		apiKey string
	}
	qdrantServer := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		received.method = r.Method
		received.path = r.URL.Path
		received.apiKey = r.Header.Get("api-key")
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer qdrantServer.Close()

	service := newKnowledgeService(config.Config{
		Port:             8080,
		QdrantAPIKey:     "qdrant-secret",
		QdrantURL:        qdrantServer.URL,
		QdrantVectorSize: 3072,
		RelayEnabled:     false,
	}, &routerKnowledgeStore{
		createdBase: knowledge.KnowledgeBase{ID: "kb_product_docs", Name: "Product Docs"},
	})

	if _, err := service.CreateWithConfig(context.Background(), auth.Session{OrganizationID: "org_acme", WorkspaceID: "workspace_1"}, "Product Docs", knowledge.KnowledgeBaseConfig{}); err != nil {
		t.Fatalf("create knowledge base with router-wired qdrant: %v", err)
	}

	if received.method != stdhttp.MethodPut {
		t.Fatalf("expected qdrant PUT, got %q", received.method)
	}
	if received.path != "/collections/kb_org_acme_kb_product_docs" {
		t.Fatalf("expected tenant collection path, got %q", received.path)
	}
	if received.apiKey != "qdrant-secret" {
		t.Fatalf("expected qdrant api-key header, got %q", received.apiKey)
	}
}
