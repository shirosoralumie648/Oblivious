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
	"oblivious/server/internal/chat"
)

type chatFakeStore struct {
	config               chat.ConversationConfig
	lastConversationID   string
	messages             []chat.Message
	lastWorkspaceID      string
	lastKnowledgeBaseIDs []string
}

func (f *chatFakeStore) CreateConversation(ctx context.Context, workspaceID, title, defaultModelID string) (chat.Conversation, error) {
	return chat.Conversation{}, nil
}

func (f *chatFakeStore) CreateMessage(ctx context.Context, conversationID, role, content string) (chat.Message, error) {
	return chat.Message{}, nil
}

func (f *chatFakeStore) GetConversationConfig(ctx context.Context, conversationID, workspaceID, defaultModelID string) (chat.ConversationConfig, error) {
	f.lastConversationID = conversationID
	f.lastWorkspaceID = workspaceID
	return f.config, nil
}

func (f *chatFakeStore) ListConversations(ctx context.Context, workspaceID string) ([]chat.Conversation, error) {
	return nil, nil
}

func (f *chatFakeStore) ListMessages(ctx context.Context, conversationID, workspaceID string) ([]chat.Message, error) {
	f.lastConversationID = conversationID
	f.lastWorkspaceID = workspaceID
	return append([]chat.Message(nil), f.messages...), nil
}

func (f *chatFakeStore) UpdateConversationConfig(
	ctx context.Context,
	conversationID,
	workspaceID,
	modelID,
	systemPromptOverride string,
	temperature float64,
	maxOutputTokens int,
	toolsEnabled bool,
	knowledgeBaseIDs []string,
) (chat.ConversationConfig, error) {
	f.lastConversationID = conversationID
	f.lastWorkspaceID = workspaceID
	f.lastKnowledgeBaseIDs = append([]string(nil), knowledgeBaseIDs...)

	return chat.ConversationConfig{
		ConversationID:       conversationID,
		ModelID:              modelID,
		SystemPromptOverride: systemPromptOverride,
		Temperature:          temperature,
		MaxOutputTokens:      maxOutputTokens,
		ToolsEnabled:         toolsEnabled,
		KnowledgeBaseIDs:     append([]string(nil), knowledgeBaseIDs...),
		UpdatedAt:            time.Date(2026, time.April, 3, 14, 0, 0, 0, time.UTC),
	}, nil
}

type noopReplyGenerator struct{}

func (noopReplyGenerator) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	return "", nil
}

func TestChatHandlerGetConversationConfigReturnsKnowledgeBaseIDs(t *testing.T) {
	store := &chatFakeStore{
		config: chat.ConversationConfig{
			ConversationID:       "conversation_1",
			ModelID:              "quality-chat",
			SystemPromptOverride: "Use workspace docs",
			Temperature:          0.6,
			MaxOutputTokens:      1536,
			ToolsEnabled:         true,
			KnowledgeBaseIDs:     []string{"kb_1", "kb_3"},
			UpdatedAt:            time.Date(2026, time.April, 3, 13, 30, 0, 0, time.UTC),
		},
	}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/conversations/conversation_1/config", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.getConversationConfig(recorder, request, "conversation_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.lastConversationID != "conversation_1" || store.lastWorkspaceID != "workspace_1" {
		t.Fatalf("unexpected lookup target: conversation=%s workspace=%s", store.lastConversationID, store.lastWorkspaceID)
	}

	var response struct {
		Data chat.ConversationConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.KnowledgeBaseIDs) != 2 || response.Data.KnowledgeBaseIDs[0] != "kb_1" || response.Data.KnowledgeBaseIDs[1] != "kb_3" {
		t.Fatalf("expected knowledge ids [kb_1 kb_3], got %+v", response.Data.KnowledgeBaseIDs)
	}
}

func TestChatHandlerUpdateConversationConfigAcceptsKnowledgeBaseIDs(t *testing.T) {
	store := &chatFakeStore{}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(
		stdhttp.MethodPut,
		"/api/v1/app/conversations/conversation_1/config",
		strings.NewReader(`{"modelId":"quality-chat","systemPromptOverride":"Use docs","temperature":0.7,"maxOutputTokens":1024,"toolsEnabled":true,"knowledgeBaseIds":["kb_2","kb_4"]}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.updateConversationConfig(recorder, request, "conversation_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(store.lastKnowledgeBaseIDs) != 2 || store.lastKnowledgeBaseIDs[0] != "kb_2" || store.lastKnowledgeBaseIDs[1] != "kb_4" {
		t.Fatalf("expected knowledge ids [kb_2 kb_4], got %+v", store.lastKnowledgeBaseIDs)
	}

	var response struct {
		Data chat.ConversationConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.KnowledgeBaseIDs) != 2 || response.Data.KnowledgeBaseIDs[0] != "kb_2" || response.Data.KnowledgeBaseIDs[1] != "kb_4" {
		t.Fatalf("expected response knowledge ids [kb_2 kb_4], got %+v", response.Data.KnowledgeBaseIDs)
	}
}

func TestChatHandlerConvertConversationToTaskReturnsDraft(t *testing.T) {
	store := &chatFakeStore{
		config: chat.ConversationConfig{
			ConversationID:   "conversation_1",
			KnowledgeBaseIDs: []string{"kb_2"},
		},
		messages: []chat.Message{
			{ID: "message_1", Role: "assistant", Content: "Let's turn this into a task."},
			{ID: "message_2", Role: "user", Content: "Draft a launch checklist from this thread."},
		},
	}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/convert-to-task", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.convertConversationToTask(recorder, request, "conversation_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data chat.TaskDraft `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.DraftTaskGoal != "Draft a launch checklist from this thread." {
		t.Fatalf("unexpected task draft: %+v", response.Data)
	}
	if response.Data.SuggestedBudget != 20 {
		t.Fatalf("unexpected task draft budget: %+v", response.Data)
	}
	if len(response.Data.RelatedKnowledgeBaseIDs) != 1 || response.Data.RelatedKnowledgeBaseIDs[0] != "kb_2" {
		t.Fatalf("unexpected related knowledge bases: %+v", response.Data)
	}
}
