package http

import (
	"context"
	"reflect"
	"testing"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	relaytypes "oblivious/server/internal/relay/types"
)

func TestChatMemoryContextProviderUsesVectorSearchForTopFiveLongTermMemories(t *testing.T) {
	embedder := &recordingChatMemoryEmbedder{embedding: []float32{0.1, 0.2}}
	vector := &recordingAgentMemoryVectorStore{
		results: []*agent.MemorySearchResult{
			{Memory: agent.Memory{Content: "Prefer concise answers."}, Score: 0.9},
			{Memory: agent.Memory{Content: "Project codename is Atlas."}, Score: 0.8},
			{Memory: agent.Memory{Content: "Billing launch depends on invoice checks."}, Score: 0.7},
			{Memory: agent.Memory{Content: "Use Chinese status updates."}, Score: 0.6},
			{Memory: agent.Memory{Content: "Always call out rollout risk."}, Score: 0.55},
			{Memory: agent.Memory{Content: "Sixth memory should be ignored."}, Score: 0.51},
		},
	}
	list := &recordingAgentMemoryListService{}
	provider := chatMemoryContextProvider{embedder: embedder, vector: vector, list: list}

	session := auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}}
	memories, err := provider.RelevantMemories(context.Background(), session, "launch notes", 10)
	if err != nil {
		t.Fatalf("RelevantMemories returned error: %v", err)
	}

	if len(memories) != 5 {
		t.Fatalf("expected top 5 memories, got %+v", memories)
	}
	if memories[0].Content != "Prefer concise answers." || memories[4].Content != "Always call out rollout risk." {
		t.Fatalf("unexpected memory contexts: %+v", memories)
	}
	if !reflect.DeepEqual(embedder.texts, []string{"launch notes"}) {
		t.Fatalf("expected query to be embedded, got %+v", embedder.texts)
	}
	if embedder.userID != "user_1" || embedder.organizationID != "org_1" {
		t.Fatalf("expected trusted relay identity in embedding context, got user=%q org=%q", embedder.userID, embedder.organizationID)
	}
	if vector.organizationID != "org_1" || vector.userID != "user_1" {
		t.Fatalf("unexpected vector search scope: org=%q user=%q", vector.organizationID, vector.userID)
	}
	if vector.req.Type != agent.MemoryTypeLongTerm || vector.req.Limit != 5 || vector.req.MinScore != 0.5 {
		t.Fatalf("unexpected vector search request: %+v", vector.req)
	}
	if !reflect.DeepEqual(vector.req.Embedding, []float32{0.1, 0.2}) {
		t.Fatalf("expected vector search embedding, got %+v", vector.req.Embedding)
	}
	if list.calls != 0 {
		t.Fatalf("expected list fallback not to run when vector search returns results, got %d calls", list.calls)
	}
}

func TestChatMemoryContextProviderFallsBackToTextListWhenVectorSearchHasNoResults(t *testing.T) {
	embedder := &recordingChatMemoryEmbedder{embedding: []float32{0.1, 0.2}}
	vector := &recordingAgentMemoryVectorStore{}
	list := &recordingAgentMemoryListService{
		memories: []*agent.Memory{
			{Content: "Fallback memory."},
		},
	}
	provider := chatMemoryContextProvider{embedder: embedder, vector: vector, list: list}

	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}
	memories, err := provider.RelevantMemories(context.Background(), session, "fallback query", 5)
	if err != nil {
		t.Fatalf("RelevantMemories returned error: %v", err)
	}

	if len(memories) != 1 || memories[0].Content != "Fallback memory." {
		t.Fatalf("expected fallback memory, got %+v", memories)
	}
	if list.session.OrganizationID != "org_1" || list.session.User.ID != "user_1" {
		t.Fatalf("unexpected fallback scope: %+v", list.session)
	}
	if list.req.Type != agent.MemoryTypeLongTerm || list.req.Query != "fallback query" || list.req.Limit != 5 {
		t.Fatalf("unexpected fallback request: %+v", list.req)
	}
}

type recordingChatMemoryEmbedder struct {
	embedding      []float32
	organizationID string
	texts          []string
	userID         string
}

func (e *recordingChatMemoryEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.texts = append(e.texts, text)
	userID, _ := relaytypes.TrustedUserIDFromContext(ctx)
	organizationID, _ := relaytypes.TrustedOrganizationIDFromContext(ctx)
	e.userID = userID
	e.organizationID = organizationID
	return append([]float32(nil), e.embedding...), nil
}

type recordingAgentMemoryVectorStore struct {
	organizationID string
	req            agent.SearchMemoriesRequest
	results        []*agent.MemorySearchResult
	userID         string
}

func (s *recordingAgentMemoryVectorStore) SearchMemories(ctx context.Context, organizationID, userID string, req agent.SearchMemoriesRequest) ([]*agent.MemorySearchResult, error) {
	s.organizationID = organizationID
	s.userID = userID
	s.req = req
	return append([]*agent.MemorySearchResult(nil), s.results...), nil
}

type recordingAgentMemoryListService struct {
	calls    int
	memories []*agent.Memory
	req      agent.ListMemoriesRequest
	session  auth.Session
}

func (s *recordingAgentMemoryListService) ListMemories(ctx context.Context, session auth.Session, req agent.ListMemoriesRequest) ([]*agent.Memory, error) {
	s.calls++
	s.session = session
	s.req = req
	return append([]*agent.Memory(nil), s.memories...), nil
}
