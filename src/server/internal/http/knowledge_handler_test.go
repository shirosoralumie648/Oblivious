package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/knowledge"
)

type knowledgeFakeStore struct {
	createdName string
	createdBase knowledge.KnowledgeBase
	detailBase  knowledge.KnowledgeBase
	listBases   []knowledge.KnowledgeBase
	requestedID string
	workspaceID string
}

func (f *knowledgeFakeStore) CreateKnowledgeBase(ctx context.Context, workspaceID, name string) (knowledge.KnowledgeBase, error) {
	f.workspaceID = workspaceID
	f.createdName = name
	return f.createdBase, nil
}

func (f *knowledgeFakeStore) ListKnowledgeBases(ctx context.Context, workspaceID string) ([]knowledge.KnowledgeBase, error) {
	f.workspaceID = workspaceID
	return f.listBases, nil
}

func (f *knowledgeFakeStore) GetKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) (knowledge.KnowledgeBase, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	return f.detailBase, nil
}

func TestKnowledgeHandlerListReturnsWorkspaceBases(t *testing.T) {
	store := &knowledgeFakeStore{
		listBases: []knowledge.KnowledgeBase{
			{
				DocumentCount: 2,
				ID:            "kb_1",
				Name:          "Ops Runbooks",
				UpdatedAt:     time.Date(2026, time.April, 3, 9, 0, 0, 0, time.UTC),
			},
		},
	}
	handler := newKnowledgeHandler(knowledge.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/knowledge-bases", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.listKnowledgeBases(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}

	var response struct {
		Data []knowledge.KnowledgeBase `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 {
		t.Fatalf("expected 1 knowledge base, got %d", len(response.Data))
	}
}

func TestKnowledgeHandlerCreateCreatesKnowledgeBase(t *testing.T) {
	store := &knowledgeFakeStore{
		createdBase: knowledge.KnowledgeBase{
			DocumentCount: 0,
			ID:            "kb_1",
			Name:          "Roadmap Notes",
			UpdatedAt:     time.Date(2026, time.April, 3, 9, 30, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeHandler(knowledge.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases", strings.NewReader(`{"name":"Roadmap Notes"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createKnowledgeBase(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.createdName != "Roadmap Notes" {
		t.Fatalf("expected created name Roadmap Notes, got %s", store.createdName)
	}
}

func TestKnowledgeHandlerGetReturnsKnowledgeBase(t *testing.T) {
	store := &knowledgeFakeStore{
		detailBase: knowledge.KnowledgeBase{
			DocumentCount: 5,
			ID:            "kb_2",
			Name:          "Architecture Notes",
			UpdatedAt:     time.Date(2026, time.April, 3, 11, 30, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeHandler(knowledge.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_2", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.getKnowledgeBase(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected requested id kb_2, got %s", store.requestedID)
	}
}
