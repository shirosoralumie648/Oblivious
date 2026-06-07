package agent

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
)

func TestServiceCreateMemoryDefaultsAndScopesToSession(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Name:           "Memory Agent",
		},
	}
	service := NewService(store, &fakeGateway{})
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	memory, err := service.CreateMemory(context.Background(), session, CreateMemoryRequest{
		AgentID:    "agent_1",
		Content:    "  I prefer concise answers  ",
		Importance: 4,
	})
	if err != nil {
		t.Fatalf("CreateMemory returned error: %v", err)
	}
	if memory.ID == "" || memory.OrganizationID != "org_1" || memory.UserID != "user_1" || memory.AgentID != "agent_1" {
		t.Fatalf("expected scoped memory, got %+v", memory)
	}
	if memory.Type != MemoryTypeLongTerm {
		t.Fatalf("expected default long-term memory type, got %q", memory.Type)
	}
	if memory.Content != "I prefer concise answers" {
		t.Fatalf("expected trimmed content, got %q", memory.Content)
	}
	if memory.Importance != 4 {
		t.Fatalf("expected requested importance 4, got %d", memory.Importance)
	}
	if memory.Metadata == nil {
		t.Fatal("expected non-nil metadata map")
	}
}

func TestServiceCreateAndUpdateMemoryWritesEmbeddingsWhenProviderConfigured(t *testing.T) {
	store := &fakeStore{}
	embedder := &fakeAgentMemoryEmbedder{
		embeddings: map[string][]float32{
			"Remember terse answers.":        {1, 0},
			"Remember direct bullet points.": {0, 1},
		},
	}
	service := NewService(store, &fakeGateway{})
	service.SetMemoryEmbedder(embedder)
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	created, err := service.CreateMemory(context.Background(), session, CreateMemoryRequest{
		Content: "Remember terse answers.",
	})
	if err != nil {
		t.Fatalf("CreateMemory returned error: %v", err)
	}
	if !reflect.DeepEqual(store.createMemoryEmbedding, []float32{1, 0}) {
		t.Fatalf("expected create embedding to reach store, got %+v", store.createMemoryEmbedding)
	}

	_, err = service.UpdateMemory(context.Background(), session, created.ID, UpdateMemoryRequest{
		Content: stringPtr("Remember direct bullet points."),
	})
	if err != nil {
		t.Fatalf("UpdateMemory returned error: %v", err)
	}
	if !reflect.DeepEqual(store.updateMemoryEmbedding, []float32{0, 1}) {
		t.Fatalf("expected update embedding to reach store, got %+v", store.updateMemoryEmbedding)
	}
	if len(embedder.texts) != 2 || embedder.texts[0] != "Remember terse answers." || embedder.texts[1] != "Remember direct bullet points." {
		t.Fatalf("expected create and content update texts to be embedded, got %+v", embedder.texts)
	}
}

func (s *fakeStore) GetMemory(ctx context.Context, organizationID, id string) (*Memory, error) {
	for _, memory := range s.memories {
		if memory.ID == id && memory.OrganizationID == organizationID {
			return memory, nil
		}
	}
	return nil, nil
}

func (s *fakeStore) UpdateMemory(ctx context.Context, organizationID, userID, id string, req UpdateMemoryStoreRequest) (*Memory, error) {
	memory, _ := s.GetMemory(ctx, organizationID, id)
	if memory == nil || memory.UserID != userID {
		return nil, errors.New("memory not found")
	}
	s.updateMemoryEmbedding = append([]float32(nil), req.Embedding...)
	if req.Content != nil {
		memory.Content = *req.Content
	}
	if req.Importance != nil {
		memory.Importance = *req.Importance
	}
	memory.UpdatedAt = time.Now().UTC()
	return memory, nil
}

func (s *fakeStore) DeleteMemory(ctx context.Context, organizationID, userID, id string) error {
	for index, memory := range s.memories {
		if memory.ID == id && memory.OrganizationID == organizationID && memory.UserID == userID {
			s.memories = append(s.memories[:index], s.memories[index+1:]...)
			return nil
		}
	}
	return errors.New("memory not found")
}

func TestServiceCreateMemoryDefaultsImportance(t *testing.T) {
	service := NewService(&fakeStore{}, &fakeGateway{})
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	memory, err := service.CreateMemory(context.Background(), session, CreateMemoryRequest{
		Content: "Remember review gates.",
	})
	if err != nil {
		t.Fatalf("CreateMemory returned error: %v", err)
	}
	if memory.Importance != 3 {
		t.Fatalf("expected default importance 3, got %d", memory.Importance)
	}
}

func TestServiceCreateMemoryRejectsInvalidImportance(t *testing.T) {
	service := NewService(&fakeStore{}, &fakeGateway{})
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	_, err := service.CreateMemory(context.Background(), session, CreateMemoryRequest{
		Content:    "Remember invalid stars.",
		Importance: 6,
	})
	if err == nil || !strings.Contains(err.Error(), "importance must be between 1 and 5") {
		t.Fatalf("expected importance validation error, got %v", err)
	}
}

func TestServiceCreateMemoryRejectsCrossTenantAgent(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_other",
			OrganizationID: "org_2",
			UserID:         "user_2",
		},
	}
	service := NewService(store, &fakeGateway{})

	_, err := service.CreateMemory(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, CreateMemoryRequest{
		AgentID: "agent_other",
		Content: "private memory",
	})
	if err == nil || !strings.Contains(err.Error(), "agent not found") {
		t.Fatalf("expected cross-tenant agent not found, got %v", err)
	}
}

func TestServiceListMemoriesFiltersByQueryAndAgent(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		memories: []*Memory{
			{
				ID:             "memory_1",
				OrganizationID: "org_1",
				UserID:         "user_1",
				AgentID:        "agent_1",
				Type:           MemoryTypeLongTerm,
				Content:        "I prefer concise answers",
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			},
			{
				ID:             "memory_2",
				OrganizationID: "org_1",
				UserID:         "user_1",
				AgentID:        "agent_2",
				Type:           MemoryTypeLongTerm,
				Content:        "Use detailed reasoning",
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			},
		},
	}
	service := NewService(store, &fakeGateway{})

	memories, err := service.ListMemories(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, ListMemoriesRequest{
		AgentID: "agent_1",
		Query:   "concise",
		Limit:   5,
	})
	if err != nil {
		t.Fatalf("ListMemories returned error: %v", err)
	}
	if len(memories) != 1 || memories[0].ID != "memory_1" {
		t.Fatalf("expected only matching agent memory, got %+v", memories)
	}
	if store.listMemoryLimit != 5 || store.listMemoryAgentID != "agent_1" || store.listMemoryQuery != "concise" {
		t.Fatalf("expected list filters to reach store, got agent=%q query=%q limit=%d", store.listMemoryAgentID, store.listMemoryQuery, store.listMemoryLimit)
	}
}

func TestServiceUpdateMemoryScopesToSessionAndValidatesImportance(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		memories: []*Memory{
			{
				ID:             "memory_1",
				OrganizationID: "org_1",
				UserID:         "user_1",
				Type:           MemoryTypeUserManaged,
				Content:        "Old memory content",
				Importance:     2,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}
	service := NewService(store, &fakeGateway{})
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	updated, err := service.UpdateMemory(context.Background(), session, "memory_1", UpdateMemoryRequest{
		Content:    stringPtr("  Updated memory content  "),
		Importance: intPtr(5),
	})
	if err != nil {
		t.Fatalf("UpdateMemory returned error: %v", err)
	}
	if updated.Content != "Updated memory content" || updated.Importance != 5 {
		t.Fatalf("expected content and importance update, got %+v", updated)
	}

	_, err = service.UpdateMemory(context.Background(), session, "memory_1", UpdateMemoryRequest{
		Importance: intPtr(0),
	})
	if err == nil || !strings.Contains(err.Error(), "importance must be between 1 and 5") {
		t.Fatalf("expected invalid importance error, got %v", err)
	}

	crossUserSession := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_2"}}
	_, err = service.UpdateMemory(context.Background(), crossUserSession, "memory_1", UpdateMemoryRequest{
		Content: stringPtr("stolen"),
	})
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("expected cross-user access denied, got %v", err)
	}
	if store.memories[0].Content != "Updated memory content" {
		t.Fatalf("expected cross-user update to leave memory unchanged, got %+v", store.memories[0])
	}
}

func TestServiceDeleteMemoryScopesToSession(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		memories: []*Memory{
			{
				ID:             "memory_1",
				OrganizationID: "org_1",
				UserID:         "user_1",
				Type:           MemoryTypeUserManaged,
				Content:        "Do not leak this memory.",
				Importance:     3,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}
	service := NewService(store, &fakeGateway{})

	crossUserSession := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_2"}}
	err := service.DeleteMemory(context.Background(), crossUserSession, "memory_1")
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("expected cross-user access denied, got %v", err)
	}
	if len(store.memories) != 1 {
		t.Fatalf("expected cross-user delete to keep memory, got %+v", store.memories)
	}

	ownerSession := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}
	if err := service.DeleteMemory(context.Background(), ownerSession, "memory_1"); err != nil {
		t.Fatalf("DeleteMemory returned error: %v", err)
	}
	if len(store.memories) != 0 {
		t.Fatalf("expected owner delete to remove memory, got %+v", store.memories)
	}
}
