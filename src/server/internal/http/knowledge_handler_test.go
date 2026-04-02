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
	createdName  string
	createdBase  knowledge.KnowledgeBase
	createdDoc   knowledge.KnowledgeDocument
	deletedDocID string
	deletedID    string
	detailBase   knowledge.KnowledgeBase
	documents    []knowledge.KnowledgeDocument
	listBases    []knowledge.KnowledgeBase
	requestedDoc knowledge.KnowledgeDocument
	requestedID  string
	updatedBase  knowledge.KnowledgeBase
	updatedDoc   knowledge.KnowledgeDocument
	workspaceID  string
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

func (f *knowledgeFakeStore) ListKnowledgeDocuments(ctx context.Context, workspaceID, knowledgeBaseID string) ([]knowledge.KnowledgeDocument, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	return f.documents, nil
}

func (f *knowledgeFakeStore) CreateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, title, content string) (knowledge.KnowledgeDocument, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.requestedDoc = knowledge.KnowledgeDocument{
		Title:   title,
		Content: content,
	}
	return f.createdDoc, nil
}

func (f *knowledgeFakeStore) UpdateKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID, name string) (knowledge.KnowledgeBase, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.createdName = name
	return f.updatedBase, nil
}

func (f *knowledgeFakeStore) DeleteKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) error {
	f.workspaceID = workspaceID
	f.deletedID = knowledgeBaseID
	return nil
}

func (f *knowledgeFakeStore) UpdateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, documentID, title, content string) (knowledge.KnowledgeDocument, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	f.requestedDoc = knowledge.KnowledgeDocument{
		Title:   title,
		Content: content,
	}
	return f.updatedDoc, nil
}

func (f *knowledgeFakeStore) DeleteKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, documentID string) error {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.deletedDocID = documentID
	return nil
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

func TestKnowledgeHandlerListDocumentsReturnsKnowledgeBaseDocuments(t *testing.T) {
	store := &knowledgeFakeStore{
		documents: []knowledge.KnowledgeDocument{
			{
				Content:   "Deploy checklist",
				ID:        "doc_1",
				Title:     "Runbook",
				UpdatedAt: time.Date(2026, time.April, 3, 12, 45, 0, 0, time.UTC),
			},
		},
	}
	handler := newKnowledgeHandler(knowledge.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_2/documents", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.listKnowledgeDocuments(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected requested id kb_2, got %s", store.requestedID)
	}
}

func TestKnowledgeHandlerCreateDocumentCreatesKnowledgeBaseDocument(t *testing.T) {
	store := &knowledgeFakeStore{
		createdDoc: knowledge.KnowledgeDocument{
			Content:   "Initial plan",
			ID:        "doc_2",
			Title:     "Plan",
			UpdatedAt: time.Date(2026, time.April, 3, 13, 0, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeHandler(knowledge.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_2/documents", strings.NewReader(`{"title":"Plan","content":"Initial plan"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createKnowledgeDocument(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected requested id kb_2, got %s", store.requestedID)
	}
	if store.requestedDoc.Title != "Plan" {
		t.Fatalf("expected title Plan, got %s", store.requestedDoc.Title)
	}
}

func TestKnowledgeHandlerUpdateKnowledgeBaseUpdatesKnowledgeBase(t *testing.T) {
	store := &knowledgeFakeStore{
		updatedBase: knowledge.KnowledgeBase{
			DocumentCount: 2,
			ID:            "kb_2",
			Name:          "Updated Notes",
			UpdatedAt:     time.Date(2026, time.April, 3, 13, 30, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeHandler(knowledge.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_2", strings.NewReader(`{"name":"Updated Notes"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.updateKnowledgeBase(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected requested id kb_2, got %s", store.requestedID)
	}
	if store.createdName != "Updated Notes" {
		t.Fatalf("expected updated name Updated Notes, got %s", store.createdName)
	}
}

func TestKnowledgeHandlerDeleteKnowledgeBaseDeletesKnowledgeBase(t *testing.T) {
	store := &knowledgeFakeStore{}
	handler := newKnowledgeHandler(knowledge.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_2", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.deleteKnowledgeBase(recorder, request, "kb_2")

	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if store.deletedID != "kb_2" {
		t.Fatalf("expected deleted id kb_2, got %s", store.deletedID)
	}
}

func TestKnowledgeHandlerUpdateDocumentUpdatesKnowledgeBaseDocument(t *testing.T) {
	store := &knowledgeFakeStore{
		updatedDoc: knowledge.KnowledgeDocument{
			Content:   "Updated plan",
			ID:        "doc_2",
			Title:     "Plan v2",
			UpdatedAt: time.Date(2026, time.April, 3, 13, 45, 0, 0, time.UTC),
		},
	}
	handler := newKnowledgeHandler(knowledge.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_2/documents/doc_2", strings.NewReader(`{"title":"Plan v2","content":"Updated plan"}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.updateKnowledgeDocument(recorder, request, "kb_2", "doc_2")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected knowledge base id kb_2, got %s", store.requestedID)
	}
	if store.deletedDocID != "doc_2" {
		t.Fatalf("expected document id doc_2, got %s", store.deletedDocID)
	}
	if store.requestedDoc.Title != "Plan v2" {
		t.Fatalf("expected title Plan v2, got %s", store.requestedDoc.Title)
	}
}

func TestKnowledgeHandlerDeleteDocumentDeletesKnowledgeBaseDocument(t *testing.T) {
	store := &knowledgeFakeStore{}
	handler := newKnowledgeHandler(knowledge.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_2/documents/doc_2", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.deleteKnowledgeDocument(recorder, request, "kb_2", "doc_2")

	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if store.requestedID != "kb_2" {
		t.Fatalf("expected knowledge base id kb_2, got %s", store.requestedID)
	}
	if store.deletedDocID != "doc_2" {
		t.Fatalf("expected document id doc_2, got %s", store.deletedDocID)
	}
}
