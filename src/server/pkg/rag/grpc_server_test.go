package rag

import (
	"context"
	"testing"

	ragv1 "oblivious/server/api/proto/rag/v1"
)

func TestCreateKnowledgeBase(t *testing.T) {
	s := NewServer()
	req := &ragv1.CreateKnowledgeBaseRequest{
		Name:           "test-kb",
		Description:    "test description",
		OrganizationId: "org-123",
	}

	resp, err := s.CreateKnowledgeBase(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateKnowledgeBase failed: %v", err)
	}

	if resp.KbId != "kb-test-kb" {
		t.Errorf("expected kb_id 'kb-test-kb', got '%s'", resp.KbId)
	}
	if resp.Status != "created" {
		t.Errorf("expected status 'created', got '%s'", resp.Status)
	}
}

func TestUploadDocument(t *testing.T) {
	s := NewServer()
	req := &ragv1.UploadDocumentRequest{
		KbId:           "kb-test",
		DocumentName:   "test.pdf",
		Content:        []byte("test content"),
		OrganizationId: "org-123",
	}

	resp, err := s.UploadDocument(context.Background(), req)
	if err != nil {
		t.Fatalf("UploadDocument failed: %v", err)
	}

	if resp.DocumentId != "doc-test.pdf" {
		t.Errorf("expected document_id 'doc-test.pdf', got '%s'", resp.DocumentId)
	}
	if resp.Status != "uploaded" {
		t.Errorf("expected status 'uploaded', got '%s'", resp.Status)
	}
}

func TestRetrieve(t *testing.T) {
	s := NewServer()
	req := &ragv1.RetrieveRequest{
		KbId:           "kb-test",
		Query:          "test query",
		TopK:           5,
		OrganizationId: "org-123",
	}

	resp, err := s.Retrieve(context.Background(), req)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	if len(resp.Chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	chunk := resp.Chunks[0]
	if chunk.DocumentId != "doc-1" {
		t.Errorf("expected document_id 'doc-1', got '%s'", chunk.DocumentId)
	}
	if chunk.Score != 0.95 {
		t.Errorf("expected score 0.95, got %f", chunk.Score)
	}
}
