package knowledge

import (
	"context"
	"testing"
	"time"

	"oblivious/server/internal/auth"
)

type fakeStore struct {
	createdName  string
	createdBase  KnowledgeBase
	createdDoc   KnowledgeDocument
	detailBase   KnowledgeBase
	documents    []KnowledgeDocument
	listBases    []KnowledgeBase
	requestedDoc KnowledgeDocument
	requestedID  string
	workspaceID  string
}

func (f *fakeStore) CreateKnowledgeBase(ctx context.Context, workspaceID, name string) (KnowledgeBase, error) {
	f.workspaceID = workspaceID
	f.createdName = name
	return f.createdBase, nil
}

func (f *fakeStore) ListKnowledgeBases(ctx context.Context, workspaceID string) ([]KnowledgeBase, error) {
	f.workspaceID = workspaceID
	return f.listBases, nil
}

func (f *fakeStore) GetKnowledgeBase(ctx context.Context, workspaceID, knowledgeBaseID string) (KnowledgeBase, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	return f.detailBase, nil
}

func (f *fakeStore) ListKnowledgeDocuments(ctx context.Context, workspaceID, knowledgeBaseID string) ([]KnowledgeDocument, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	return f.documents, nil
}

func (f *fakeStore) CreateKnowledgeDocument(ctx context.Context, workspaceID, knowledgeBaseID, title, content string) (KnowledgeDocument, error) {
	f.workspaceID = workspaceID
	f.requestedID = knowledgeBaseID
	f.requestedDoc = KnowledgeDocument{
		Title:   title,
		Content: content,
	}
	return f.createdDoc, nil
}

func TestListReturnsWorkspaceKnowledgeBases(t *testing.T) {
	store := &fakeStore{
		listBases: []KnowledgeBase{
			{
				DocumentCount: 3,
				ID:            "kb_1",
				Name:          "Product Docs",
				UpdatedAt:     time.Date(2026, time.April, 2, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(store)

	bases, err := service.List(context.Background(), auth.Session{WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("list knowledge bases: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if len(bases) != 1 {
		t.Fatalf("expected 1 knowledge base, got %d", len(bases))
	}
	if bases[0].Name != "Product Docs" {
		t.Fatalf("expected Product Docs, got %s", bases[0].Name)
	}
}

func TestCreateCreatesKnowledgeBaseInWorkspace(t *testing.T) {
	store := &fakeStore{
		createdBase: KnowledgeBase{
			DocumentCount: 0,
			ID:            "kb_1",
			Name:          "Research Vault",
			UpdatedAt:     time.Date(2026, time.April, 3, 8, 0, 0, 0, time.UTC),
		},
	}
	service := NewService(store)

	base, err := service.Create(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "Research Vault")
	if err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.createdName != "Research Vault" {
		t.Fatalf("expected created name Research Vault, got %s", store.createdName)
	}
	if base.ID != "kb_1" {
		t.Fatalf("expected base id kb_1, got %s", base.ID)
	}
}

func TestGetReturnsKnowledgeBaseFromWorkspace(t *testing.T) {
	store := &fakeStore{
		detailBase: KnowledgeBase{
			DocumentCount: 7,
			ID:            "kb_7",
			Name:          "Customer Notes",
			UpdatedAt:     time.Date(2026, time.April, 3, 11, 0, 0, 0, time.UTC),
		},
	}
	service := NewService(store)

	base, err := service.Get(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7")
	if err != nil {
		t.Fatalf("get knowledge base: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.requestedID != "kb_7" {
		t.Fatalf("expected requested id kb_7, got %s", store.requestedID)
	}
	if base.Name != "Customer Notes" {
		t.Fatalf("expected Customer Notes, got %s", base.Name)
	}
}

func TestListDocumentsReturnsKnowledgeBaseDocuments(t *testing.T) {
	store := &fakeStore{
		documents: []KnowledgeDocument{
			{
				Content:   "Deployment notes",
				ID:        "doc_1",
				Title:     "Runbook",
				UpdatedAt: time.Date(2026, time.April, 3, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(store)

	documents, err := service.ListDocuments(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7")
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.requestedID != "kb_7" {
		t.Fatalf("expected requested id kb_7, got %s", store.requestedID)
	}
	if len(documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(documents))
	}
}

func TestCreateDocumentCreatesDocumentInKnowledgeBase(t *testing.T) {
	store := &fakeStore{
		createdDoc: KnowledgeDocument{
			Content:   "Initial architecture outline",
			ID:        "doc_9",
			Title:     "Architecture Draft",
			UpdatedAt: time.Date(2026, time.April, 3, 12, 30, 0, 0, time.UTC),
		},
	}
	service := NewService(store)

	document, err := service.CreateDocument(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "kb_7", "Architecture Draft", "Initial architecture outline")
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.requestedID != "kb_7" {
		t.Fatalf("expected requested id kb_7, got %s", store.requestedID)
	}
	if store.requestedDoc.Title != "Architecture Draft" {
		t.Fatalf("expected title Architecture Draft, got %s", store.requestedDoc.Title)
	}
	if document.ID != "doc_9" {
		t.Fatalf("expected doc id doc_9, got %s", document.ID)
	}
}
