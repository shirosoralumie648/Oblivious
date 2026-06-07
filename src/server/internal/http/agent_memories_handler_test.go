package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
)

func TestAgentMemoriesHandlerCreateMemoryStoresAgentMetadata(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
	}
	handler := newAgentMemoriesHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.createMemory(recorder, newAgentMemoriesRequest(stdhttp.MethodPost, "/api/v1/agent/memories", `{
		"agentId":"agent_1",
		"type":"user_managed",
		"content":" I prefer concise answers ",
		"importance":5,
		"metadata":{"topic":"style"}
	}`))

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(store.memories) != 1 {
		t.Fatalf("expected one memory to be created, got %+v", store.memories)
	}
	created := store.memories[0]
	if created.UserID != "user_1" || created.OrganizationID != "org_1" {
		t.Fatalf("expected session scope user_1/org_1, got user=%q org=%q", created.UserID, created.OrganizationID)
	}
	if created.Content != "I prefer concise answers" {
		t.Fatalf("expected trimmed content, got %q", created.Content)
	}
	if created.AgentID != "agent_1" || created.Type != agent.MemoryTypeUserManaged || created.Metadata["topic"] != "style" {
		t.Fatalf("expected agent memory metadata, got %+v", created)
	}
	if created.Importance != 5 {
		t.Fatalf("expected requested importance 5, got %d", created.Importance)
	}
	var response struct {
		Data agent.Memory `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ID == "" || response.Data.Type != agent.MemoryTypeUserManaged {
		t.Fatalf("expected created agent memory response, got %+v", response.Data)
	}
	if response.Data.Importance != 5 {
		t.Fatalf("expected response importance 5, got %d", response.Data.Importance)
	}
}

func TestAgentMemoriesHandlerCreateMemoryReturnsNotFoundForUnknownAgent(t *testing.T) {
	handler := newAgentMemoriesHandler(agent.NewService(newFakeAgentRunsStore(), &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.createMemory(recorder, newAgentMemoriesRequest(stdhttp.MethodPost, "/api/v1/agent/memories", `{
		"agentId":"missing_agent",
		"content":"remember this"
	}`))

	if recorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("expected 404, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"not_found"`) {
		t.Fatalf("expected not_found response, got %s", recorder.Body.String())
	}
}

func TestAgentMemoriesHandlerCreateMemoryRequiresContent(t *testing.T) {
	handler := newAgentMemoriesHandler(agent.NewService(newFakeAgentRunsStore(), &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.createMemory(recorder, newAgentMemoriesRequest(stdhttp.MethodPost, "/api/v1/agent/memories", `{"content":" "}`))

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "content is required") {
		t.Fatalf("expected content validation error, got %s", recorder.Body.String())
	}
}

func TestAgentMemoriesHandlerImportsMultipleMemories(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
	}
	handler := newAgentMemoriesHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.importMemories(recorder, newAgentMemoriesRequest(stdhttp.MethodPost, "/api/v1/agent/memories/import", `{
		"memories": [
			{
				"agentId": "agent_1",
				"type": "user_managed",
				"content": " Imported memory one. ",
				"importance": 5,
				"metadata": {"imported": true}
			},
			{
				"type": "user_managed",
				"content": "Imported memory two.",
				"importance": 3
			}
		]
	}`))

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(store.memories) != 2 {
		t.Fatalf("expected two imported memories, got %+v", store.memories)
	}
	if store.memories[0].Content != "Imported memory one." || store.memories[0].AgentID != "agent_1" || store.memories[0].Importance != 5 {
		t.Fatalf("unexpected first imported memory: %+v", store.memories[0])
	}
	if store.memories[1].Content != "Imported memory two." || store.memories[1].Importance != 3 {
		t.Fatalf("unexpected second imported memory: %+v", store.memories[1])
	}
	var response struct {
		Data []agent.Memory `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 2 || response.Data[0].Content != "Imported memory one." {
		t.Fatalf("expected imported memories response, got %+v", response.Data)
	}
}

func TestAgentMemoriesHandlerSearchMemoriesUsesQueryParameters(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
	}
	store.memories = []*agent.Memory{
		{
			ID:             "memory_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			AgentID:        "agent_1",
			Type:           agent.MemoryTypeLongTerm,
			Content:        "I prefer concise answers",
		},
	}
	handler := newAgentMemoriesHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.searchMemories(recorder, newAgentMemoriesRequest(stdhttp.MethodGet, "/api/v1/agent/memories?query=concise&agentId=agent_1&limit=3", ""))

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.listMemoryUserID != "user_1" || store.listMemoryOrganizationID != "org_1" {
		t.Fatalf("expected scoped search user_1/org_1, got user=%q org=%q", store.listMemoryUserID, store.listMemoryOrganizationID)
	}
	if store.listMemoryLimit != 3 || store.listMemoryAgentID != "agent_1" || store.listMemoryQuery != "concise" {
		t.Fatalf("expected agent/query/limit filters, got agent=%q query=%q limit=%d", store.listMemoryAgentID, store.listMemoryQuery, store.listMemoryLimit)
	}
	if !strings.Contains(recorder.Body.String(), "I prefer concise answers") {
		t.Fatalf("expected memory search result, got %s", recorder.Body.String())
	}
}

func TestAgentMemoriesHandlerListsAllMemoriesWithoutQuery(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.memories = []*agent.Memory{
		{
			ID:             "memory_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Type:           agent.MemoryTypeLongTerm,
			Content:        "Company launch date is June 12.",
			Importance:     4,
		},
	}
	handler := newAgentMemoriesHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.searchMemories(recorder, newAgentMemoriesRequest(stdhttp.MethodGet, "/api/v1/agent/memories", ""))

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.listMemoryQuery != "" || store.listMemoryLimit == 0 {
		t.Fatalf("expected blank query list with default limit, got query=%q limit=%d", store.listMemoryQuery, store.listMemoryLimit)
	}
	if !strings.Contains(recorder.Body.String(), "Company launch date is June 12.") {
		t.Fatalf("expected memory list response, got %s", recorder.Body.String())
	}
}

func TestAgentMemoriesHandlerUpdateMemoryEditsContentAndImportance(t *testing.T) {
	store := newFakeAgentMemoriesStore()
	store.memories = []*agent.Memory{
		{
			ID:             "memory_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Type:           agent.MemoryTypeUserManaged,
			Content:        "Old memory content",
			Importance:     2,
		},
	}
	handler := newAgentMemoriesHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.updateMemory(recorder, newAgentMemoriesRequest(stdhttp.MethodPatch, "/api/v1/agent/memories/memory_1", `{
		"content":" Updated memory content ",
		"importance":4
	}`), "memory_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.memories[0].Content != "Updated memory content" || store.memories[0].Importance != 4 {
		t.Fatalf("expected edited memory, got %+v", store.memories[0])
	}
	if !strings.Contains(recorder.Body.String(), `"importance":4`) {
		t.Fatalf("expected importance in response, got %s", recorder.Body.String())
	}
}

func TestAgentMemoriesHandlerDeleteMemoryRemovesOwnedMemory(t *testing.T) {
	store := newFakeAgentMemoriesStore()
	store.memories = []*agent.Memory{
		{
			ID:             "memory_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Type:           agent.MemoryTypeUserManaged,
			Content:        "Memory to delete",
			Importance:     3,
		},
	}
	handler := newAgentMemoriesHandler(agent.NewService(store, &fakeAgentRunsGateway{}))

	recorder := httptest.NewRecorder()
	handler.deleteMemory(recorder, newAgentMemoriesRequest(stdhttp.MethodDelete, "/api/v1/agent/memories/memory_1", ""), "memory_1")

	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(store.memories) != 0 {
		t.Fatalf("expected memory deleted, got %+v", store.memories)
	}
}

type fakeAgentMemoriesStore struct {
	*fakeAgentRunsStore
}

func newFakeAgentMemoriesStore() *fakeAgentMemoriesStore {
	return &fakeAgentMemoriesStore{fakeAgentRunsStore: newFakeAgentRunsStore()}
}

func (s *fakeAgentMemoriesStore) GetMemory(ctx context.Context, organizationID, id string) (*agent.Memory, error) {
	for _, memory := range s.memories {
		if memory.ID == id && memory.OrganizationID == organizationID {
			return memory, nil
		}
	}
	return nil, nil
}

func (s *fakeAgentMemoriesStore) UpdateMemory(ctx context.Context, organizationID, userID, id string, req agent.UpdateMemoryStoreRequest) (*agent.Memory, error) {
	memory, _ := s.GetMemory(ctx, organizationID, id)
	if memory == nil || memory.UserID != userID {
		return nil, errFakeAgentMemoryNotFound()
	}
	if req.Content != nil {
		memory.Content = *req.Content
	}
	if req.Importance != nil {
		memory.Importance = *req.Importance
	}
	return memory, nil
}

func (s *fakeAgentMemoriesStore) DeleteMemory(ctx context.Context, organizationID, userID, id string) error {
	for index, memory := range s.memories {
		if memory.ID == id && memory.OrganizationID == organizationID && memory.UserID == userID {
			s.memories = append(s.memories[:index], s.memories[index+1:]...)
			return nil
		}
	}
	return errFakeAgentMemoryNotFound()
}

func errFakeAgentMemoryNotFound() error {
	return &fakeAgentMemoryError{message: "memory not found"}
}

type fakeAgentMemoryError struct {
	message string
}

func (e *fakeAgentMemoryError) Error() string {
	return e.message
}

func newAgentMemoriesRequest(method, path, body string) *stdhttp.Request {
	reader := strings.NewReader(body)
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	return request.WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		ID:             "session_1",
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		User: auth.User{
			ID:    "user_1",
			Email: "user@example.com",
			Role:  "user",
		},
	}))
}
