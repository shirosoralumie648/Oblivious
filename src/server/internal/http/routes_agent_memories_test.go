package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/agent"
)

func TestRegisterAgentMemoryRoutesDispatchesCreateAndSearch(t *testing.T) {
	store := newFakeAgentRunsStore()
	store.agent = &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
	}
	store.memories = []*agent.Memory{
		{ID: "memory_1", OrganizationID: "org_1", UserID: "user_1", Content: "remember memory match", Type: agent.MemoryTypeLongTerm, Importance: 3},
	}
	handler := newAgentMemoriesHandler(agent.NewService(store, &fakeAgentRunsGateway{}))
	mux := stdhttp.NewServeMux()
	authMiddleware := &recordingSessionMiddleware{}
	registerAgentMemoryRoutes(mux, authMiddleware, handler)

	create := httptest.NewRecorder()
	mux.ServeHTTP(create, newAgentMemoriesRequest(stdhttp.MethodPost, "/api/v1/agent/memories", `{"content":"remember this","agent_id":"agent_1"}`))
	if create.Code != stdhttp.StatusCreated {
		t.Fatalf("POST expected 201, got %d with body %s", create.Code, create.Body.String())
	}
	if len(store.memories) != 2 || store.memories[1].AgentID != "agent_1" {
		t.Fatalf("expected create dispatch to store agent memory, got %+v", store.memories)
	}

	search := httptest.NewRecorder()
	mux.ServeHTTP(search, newAgentMemoriesRequest(stdhttp.MethodGet, "/api/v1/agent/memories?query=remember", ""))
	if search.Code != stdhttp.StatusOK {
		t.Fatalf("GET expected 200, got %d with body %s", search.Code, search.Body.String())
	}
	if !strings.Contains(search.Body.String(), "remember memory match") {
		t.Fatalf("expected search result response, got %s", search.Body.String())
	}
	if authMiddleware.requestCalls != 2 {
		t.Fatalf("expected session middleware for both agent memory routes, got %d", authMiddleware.requestCalls)
	}
}

func TestRegisterAgentMemoryRoutesDispatchesImport(t *testing.T) {
	store := newFakeAgentRunsStore()
	handler := newAgentMemoriesHandler(agent.NewService(store, &fakeAgentRunsGateway{}))
	mux := stdhttp.NewServeMux()
	authMiddleware := &recordingSessionMiddleware{}
	registerAgentMemoryRoutes(mux, authMiddleware, handler)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, newAgentMemoriesRequest(stdhttp.MethodPost, "/api/v1/agent/memories/import", `{
		"memories": [
			{"content":"Imported route memory one.","type":"user_managed"},
			{"content":"Imported route memory two.","type":"user_managed"}
		]
	}`))

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("import expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(store.memories) != 2 || store.memories[0].Content != "Imported route memory one." {
		t.Fatalf("expected import dispatch to create memories, got %+v", store.memories)
	}
	if authMiddleware.requestCalls != 1 {
		t.Fatalf("expected session middleware for import route, got %d", authMiddleware.requestCalls)
	}
}

func TestRegisterAgentMemoryRoutesDispatchesUpdateAndDelete(t *testing.T) {
	store := newFakeAgentMemoriesStore()
	store.memories = []*agent.Memory{
		{ID: "memory_1", OrganizationID: "org_1", UserID: "user_1", Content: "old memory", Type: agent.MemoryTypeUserManaged, Importance: 2},
	}
	handler := newAgentMemoriesHandler(agent.NewService(store, &fakeAgentRunsGateway{}))
	mux := stdhttp.NewServeMux()
	authMiddleware := &recordingSessionMiddleware{}
	registerAgentMemoryRoutes(mux, authMiddleware, handler)

	update := httptest.NewRecorder()
	mux.ServeHTTP(update, newAgentMemoriesRequest(stdhttp.MethodPatch, "/api/v1/agent/memories/memory_1", `{
		"content":"new memory",
		"importance":5
	}`))
	if update.Code != stdhttp.StatusOK {
		t.Fatalf("PATCH expected 200, got %d with body %s", update.Code, update.Body.String())
	}
	if store.memories[0].Content != "new memory" || store.memories[0].Importance != 5 {
		t.Fatalf("expected patch dispatch to update memory, got %+v", store.memories[0])
	}

	remove := httptest.NewRecorder()
	mux.ServeHTTP(remove, newAgentMemoriesRequest(stdhttp.MethodDelete, "/api/v1/agent/memories/memory_1", ""))
	if remove.Code != stdhttp.StatusNoContent {
		t.Fatalf("DELETE expected 204, got %d with body %s", remove.Code, remove.Body.String())
	}
	if len(store.memories) != 0 {
		t.Fatalf("expected delete dispatch to remove memory, got %+v", store.memories)
	}
	if authMiddleware.requestCalls != 2 {
		t.Fatalf("expected session middleware for patch and delete routes, got %d", authMiddleware.requestCalls)
	}
}

func TestRegisterAgentMemoryRoutesRejectsUnsupportedMethod(t *testing.T) {
	handler := newAgentMemoriesHandler(agent.NewService(newFakeAgentRunsStore(), &fakeAgentRunsGateway{}))
	mux := stdhttp.NewServeMux()
	registerAgentMemoryRoutes(mux, passThroughAuthMiddleware{}, handler)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, newAgentMemoriesRequest(stdhttp.MethodPut, "/api/v1/agent/memories", `{}`))

	if recorder.Code != stdhttp.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"method_not_allowed"`) {
		t.Fatalf("expected method_not_allowed response, got %s", recorder.Body.String())
	}
}
