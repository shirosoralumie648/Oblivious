package http

import (
	"context"
	"database/sql"
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
	config                chat.ConversationConfig
	conversation          chat.Conversation
	deletedConversationID string
	lastConversationID    string
	lastMessageID         string
	lastOrganizationID    string
	lastPersonaID         string
	lastBranchMessageID   string
	messages              []chat.Message
	lastWorkspaceID       string
	lastKnowledgeBaseIDs  []string
	personaUpdateErr      error
	personaDeleteErr      error
}

type chatRealtimeNotification struct {
	conversationID string
	eventType      string
	payload        any
}

func (f *chatFakeStore) CreateConversation(ctx context.Context, workspaceID string, args ...string) (chat.Conversation, error) {
	f.lastWorkspaceID = workspaceID
	return chat.Conversation{}, nil
}

func (f *chatFakeStore) CreateMessage(ctx context.Context, conversationID string, args ...string) (chat.Message, error) {
	f.lastConversationID = conversationID
	return chat.Message{}, nil
}

func (f *chatFakeStore) GetConversationConfig(ctx context.Context, conversationID, workspaceID, defaultModelID string) (chat.ConversationConfig, error) {
	f.lastConversationID = conversationID
	f.lastWorkspaceID = workspaceID
	return f.config, nil
}

func (f *chatFakeStore) GetConversation(ctx context.Context, conversationID, scopeID string) (chat.Conversation, error) {
	f.lastConversationID = conversationID
	f.lastOrganizationID = scopeID
	if f.conversation.ID != "" {
		return f.conversation, nil
	}
	return chat.Conversation{ID: conversationID, Title: "Conversation"}, nil
}

func (f *chatFakeStore) UpdateConversation(ctx context.Context, conversationID, scopeID, title string) (chat.Conversation, error) {
	f.lastConversationID = conversationID
	f.lastOrganizationID = scopeID
	return chat.Conversation{ID: conversationID, Title: title}, nil
}

func (f *chatFakeStore) DeleteConversation(ctx context.Context, conversationID, scopeID string) error {
	f.deletedConversationID = conversationID
	f.lastOrganizationID = scopeID
	return nil
}

func (f *chatFakeStore) ListConversations(ctx context.Context, workspaceID string) ([]chat.Conversation, error) {
	return nil, nil
}

func (f *chatFakeStore) ListMessages(ctx context.Context, conversationID, workspaceID string) ([]chat.Message, error) {
	f.lastConversationID = conversationID
	f.lastWorkspaceID = workspaceID
	return append([]chat.Message(nil), f.messages...), nil
}

func (f *chatFakeStore) UpdateMessage(ctx context.Context, conversationID, scopeID, messageID, content string) (chat.Message, error) {
	f.lastConversationID = conversationID
	f.lastOrganizationID = scopeID
	f.lastMessageID = messageID
	return chat.Message{ID: messageID, Role: "user", Content: content}, nil
}

func (f *chatFakeStore) DeleteMessage(ctx context.Context, conversationID, scopeID, messageID string) error {
	f.lastConversationID = conversationID
	f.lastOrganizationID = scopeID
	f.lastMessageID = messageID
	return nil
}

func (f *chatFakeStore) BookmarkMessage(ctx context.Context, conversationID, scopeID, messageID string, bookmarked bool) (chat.Message, error) {
	f.lastConversationID = conversationID
	f.lastOrganizationID = scopeID
	f.lastMessageID = messageID
	return chat.Message{ID: messageID, Bookmarked: bookmarked}, nil
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
	personaIDs ...string,
) (chat.ConversationConfig, error) {
	f.lastConversationID = conversationID
	f.lastWorkspaceID = workspaceID
	f.lastKnowledgeBaseIDs = append([]string(nil), knowledgeBaseIDs...)
	if len(personaIDs) > 0 {
		f.lastPersonaID = personaIDs[0]
	}

	return chat.ConversationConfig{
		ConversationID:       conversationID,
		ModelID:              modelID,
		PersonaID:            f.lastPersonaID,
		SystemPromptOverride: systemPromptOverride,
		Temperature:          temperature,
		MaxOutputTokens:      maxOutputTokens,
		ToolsEnabled:         toolsEnabled,
		KnowledgeBaseIDs:     append([]string(nil), knowledgeBaseIDs...),
		UpdatedAt:            time.Date(2026, time.April, 3, 14, 0, 0, 0, time.UTC),
	}, nil
}

func (f *chatFakeStore) ForkConversation(ctx context.Context, organizationID, workspaceID, sourceConversationID, title, branchFromMessageID string) (chat.Conversation, error) {
	f.lastOrganizationID = organizationID
	f.lastWorkspaceID = workspaceID
	f.lastConversationID = sourceConversationID
	f.lastBranchMessageID = branchFromMessageID
	return chat.Conversation{ID: "forked_1", ParentID: sourceConversationID, Title: title}, nil
}

func (f *chatFakeStore) ListConversationBranches(ctx context.Context, conversationID, workspaceID string) ([]chat.Conversation, error) {
	return nil, nil
}

func (f *chatFakeStore) CreatePersona(ctx context.Context, workspaceID string, persona chat.Persona) (chat.Persona, error) {
	f.lastWorkspaceID = workspaceID
	persona.ID = "persona_1"
	persona.WorkspaceID = workspaceID
	f.lastPersonaID = persona.ID
	return persona, nil
}

func (f *chatFakeStore) GetPersona(ctx context.Context, personaID, workspaceID string) (chat.Persona, error) {
	f.lastPersonaID = personaID
	f.lastWorkspaceID = workspaceID
	return chat.Persona{ID: personaID, WorkspaceID: workspaceID}, nil
}

func (f *chatFakeStore) ListPersonas(ctx context.Context, workspaceID string) ([]chat.Persona, error) {
	f.lastWorkspaceID = workspaceID
	return []chat.Persona{{ID: "persona_1", WorkspaceID: workspaceID, Name: "Helpful Assistant"}}, nil
}

func (f *chatFakeStore) UpdatePersona(ctx context.Context, personaID, workspaceID string, persona chat.Persona) (chat.Persona, error) {
	if f.personaUpdateErr != nil {
		return chat.Persona{}, f.personaUpdateErr
	}
	f.lastPersonaID = personaID
	f.lastWorkspaceID = workspaceID
	persona.ID = personaID
	persona.WorkspaceID = workspaceID
	return persona, nil
}

func (f *chatFakeStore) DeletePersona(ctx context.Context, personaID, workspaceID string) error {
	if f.personaDeleteErr != nil {
		return f.personaDeleteErr
	}
	f.lastPersonaID = personaID
	f.lastWorkspaceID = workspaceID
	return nil
}

func (f *chatFakeStore) CreateMessageWithMetadata(ctx context.Context, conversationID, role, content string, metadata *chat.MessageMetadata) (chat.Message, error) {
	return chat.Message{}, nil
}

func (f *chatFakeStore) AddMessageAttachment(ctx context.Context, attachment chat.MessageAttachment) (chat.MessageAttachment, error) {
	return attachment, nil
}

func (f *chatFakeStore) ListMessageAttachments(ctx context.Context, messageID string) ([]chat.MessageAttachment, error) {
	return nil, nil
}

func (f *chatFakeStore) SearchMessages(ctx context.Context, workspaceID, query string, limit int) ([]chat.MessageSearchResult, error) {
	return nil, nil
}

func (f *chatFakeStore) CreateMessageShareWithOptions(ctx context.Context, conversationID, organizationID, messageID string, expiresAt *time.Time) (chat.MessageShare, error) {
	f.lastConversationID = conversationID
	f.lastOrganizationID = organizationID
	f.lastMessageID = messageID
	return chat.MessageShare{ID: "msgshare_1", ConversationID: conversationID, OrganizationID: organizationID, MessageID: messageID, ExpiresAt: expiresAt}, nil
}

func (f *chatFakeStore) GetMessageShare(ctx context.Context, shareID string, now time.Time) (chat.MessageShareDetail, error) {
	return chat.MessageShareDetail{MessageShare: chat.MessageShare{ID: shareID}}, nil
}

func (f *chatFakeStore) CreateConversationShare(ctx context.Context, conversationID, organizationID string, options chat.ConversationShareStoreOptions) (chat.ConversationShare, error) {
	f.lastConversationID = conversationID
	f.lastOrganizationID = organizationID
	return chat.ConversationShare{ID: "convshare_1", ConversationID: conversationID, OrganizationID: organizationID, StartMessageID: options.StartMessageID, EndMessageID: options.EndMessageID, ExpiresAt: options.ExpiresAt}, nil
}

func (f *chatFakeStore) GetConversationShare(ctx context.Context, shareID string, now time.Time) (chat.ConversationShareDetail, error) {
	return chat.ConversationShareDetail{ConversationShare: chat.ConversationShare{ID: shareID}}, nil
}

type noopReplyGenerator struct{}

func (noopReplyGenerator) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	return "", nil
}

type streamingReplyGenerator struct {
	chunks []string
}

func (g streamingReplyGenerator) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	return strings.Join(g.chunks, ""), nil
}

func (g streamingReplyGenerator) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	for _, chunk := range g.chunks {
		if err := onChunk(chunk); err != nil {
			return err
		}
	}
	return nil
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
		strings.NewReader(`{"modelId":"quality-chat","personaId":"persona_1","systemPromptOverride":"Use docs","temperature":0.7,"maxOutputTokens":1024,"toolsEnabled":true,"knowledgeBaseIds":["kb_2","kb_4"]}`),
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
	if store.lastPersonaID != "persona_1" {
		t.Fatalf("expected persona id persona_1, got %s", store.lastPersonaID)
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
	if response.Data.PersonaID != "persona_1" {
		t.Fatalf("expected response persona id persona_1, got %s", response.Data.PersonaID)
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

func TestChatHandlerCreatePersonaReturnsPersona(t *testing.T) {
	store := &chatFakeStore{}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/app/personas",
		strings.NewReader(`{"name":"Helpful Assistant","role":"Tutor","style":"Friendly","tone":"Warm","constraints":"No code","openingMessage":"Hi!","suggestedQuestions":["What?","How?"]}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createPersona(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data chat.Persona `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Name != "Helpful Assistant" {
		t.Fatalf("expected persona name 'Helpful Assistant', got %s", response.Data.Name)
	}
	if response.Data.Role != "Tutor" {
		t.Fatalf("expected persona role 'Tutor', got %s", response.Data.Role)
	}
	if len(response.Data.SuggestedQuestions) != 2 {
		t.Fatalf("expected 2 suggested questions, got %d", len(response.Data.SuggestedQuestions))
	}
}

func TestChatHandlerCreatePersonaRequiresName(t *testing.T) {
	store := &chatFakeStore{}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/app/personas",
		strings.NewReader(`{"role":"Tutor"}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createPersona(recorder, request)

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestChatHandlerUpdatePersonaReturnsNotFound(t *testing.T) {
	store := &chatFakeStore{personaUpdateErr: sql.ErrNoRows}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(
		stdhttp.MethodPut,
		"/api/v1/app/personas/persona_missing",
		strings.NewReader(`{"name":"Helpful Assistant"}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.updatePersona(recorder, request, "persona_missing")

	if recorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("expected 404, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestChatHandlerDeletePersonaReturnsNotFound(t *testing.T) {
	store := &chatFakeStore{personaDeleteErr: sql.ErrNoRows}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(
		stdhttp.MethodDelete,
		"/api/v1/app/personas/persona_missing",
		nil,
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.deletePersona(recorder, request, "persona_missing")

	if recorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("expected 404, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestChatHandlerForkConversationReturnsFork(t *testing.T) {
	store := &chatFakeStore{}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/app/conversations/fork",
		strings.NewReader(`{"sourceConversationId":"conv_1","branchFromMessageId":"msg_3","title":"My branch"}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.forkConversation(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data chat.Conversation `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ID != "forked_1" {
		t.Fatalf("expected forked_1, got %s", response.Data.ID)
	}
	if response.Data.ParentID != "conv_1" {
		t.Fatalf("expected parent conv_1, got %s", response.Data.ParentID)
	}
	if store.lastBranchMessageID != "msg_3" {
		t.Fatalf("expected branch message msg_3, got %s", store.lastBranchMessageID)
	}
}

func TestChatHandlerForkConversationAcceptsLegacyMessageID(t *testing.T) {
	store := &chatFakeStore{}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/app/conversations/conv_1/fork",
		strings.NewReader(`{"messageId":"msg_legacy","title":"Legacy branch"}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.forkConversationFromSource(recorder, request, "conv_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.lastConversationID != "conv_1" || store.lastBranchMessageID != "msg_legacy" {
		t.Fatalf("expected source conv_1 and legacy branch message msg_legacy, got source=%s branch=%s", store.lastConversationID, store.lastBranchMessageID)
	}
}

func TestChatHandlerForkConversationRequiresSourceID(t *testing.T) {
	store := &chatFakeStore{}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/app/conversations/fork",
		strings.NewReader(`{"branchFromMessageId":"msg_3"}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.forkConversation(recorder, request)

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestChatHandlerSearchMessagesReturnsResults(t *testing.T) {
	store := &chatFakeStore{}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(
		stdhttp.MethodGet,
		"/api/v1/app/conversations/search?q=hello&limit=5",
		nil,
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.searchMessages(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestChatHandlerSearchMessagesRequiresQuery(t *testing.T) {
	store := &chatFakeStore{}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	request := httptest.NewRequest(
		stdhttp.MethodGet,
		"/api/v1/app/conversations/search",
		nil,
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.searchMessages(recorder, request)

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestChatHandlerSendMessagePublishesRealtimeSync(t *testing.T) {
	expectedMessages := []chat.Message{
		{ID: "message_1", Role: "user", Content: "Summarize the rollout."},
		{ID: "message_2", Role: "assistant", Content: "Rollout summarized."},
	}
	store := &chatFakeStore{messages: expectedMessages}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	var notifications []chatRealtimeNotification
	handler.notifyConversation = func(conversationID, eventType string, payload any) {
		notifications = append(notifications, chatRealtimeNotification{conversationID: conversationID, eventType: eventType, payload: payload})
	}
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/app/conversations/conversation_1/messages",
		strings.NewReader(`{"content":"Summarize the rollout."}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		User:        auth.User{ID: "user_1"},
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.sendMessage(recorder, request, "conversation_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(notifications) != 1 {
		t.Fatalf("expected one realtime notification, got %+v", notifications)
	}
	notification := notifications[0]
	if notification.conversationID != "conversation_1" || notification.eventType != "chat_messages_synced" {
		t.Fatalf("unexpected notification target/type: %+v", notification)
	}
	payload, ok := notification.payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %#v", notification.payload)
	}
	if payload["conversationId"] != "conversation_1" || payload["userId"] != "user_1" {
		t.Fatalf("unexpected sync payload identity fields: %+v", payload)
	}
	messages, ok := payload["messages"].([]chat.Message)
	if !ok || len(messages) != len(expectedMessages) || messages[1].ID != "message_2" {
		t.Fatalf("unexpected sync messages payload: %#v", payload["messages"])
	}
}

func TestChatHandlerListMessagesDoesNotPublishRealtimeSync(t *testing.T) {
	store := &chatFakeStore{messages: []chat.Message{{ID: "message_1", Role: "assistant", Content: "Ready."}}}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	var notifications []chatRealtimeNotification
	handler.notifyConversation = func(conversationID, eventType string, payload any) {
		notifications = append(notifications, chatRealtimeNotification{conversationID: conversationID, eventType: eventType, payload: payload})
	}
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/conversations/conversation_1/messages", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		User:        auth.User{ID: "user_1"},
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.listMessages(recorder, request, "conversation_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(notifications) != 0 {
		t.Fatalf("expected listMessages to be read-only for realtime, got %+v", notifications)
	}
}

func TestChatHandlerMessageActionsPublishRealtimeEvents(t *testing.T) {
	store := &chatFakeStore{}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	var notifications []chatRealtimeNotification
	handler.notifyConversation = func(conversationID, eventType string, payload any) {
		notifications = append(notifications, chatRealtimeNotification{conversationID: conversationID, eventType: eventType, payload: payload})
	}
	session := auth.Session{User: auth.User{ID: "user_1"}, WorkspaceID: "workspace_1"}

	updateRequest := httptest.NewRequest(
		stdhttp.MethodPut,
		"/api/v1/app/conversations/conversation_1/messages/message_1",
		strings.NewReader(`{"content":"Edited message"}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, session))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRecorder := httptest.NewRecorder()
	handler.updateMessage(updateRecorder, updateRequest, "conversation_1", "message_1")
	if updateRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected update 200, got %d with body %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	deleteRequest := httptest.NewRequest(
		stdhttp.MethodDelete,
		"/api/v1/app/conversations/conversation_1/messages/message_1",
		nil,
	).WithContext(context.WithValue(context.Background(), sessionContextKey, session))
	deleteRecorder := httptest.NewRecorder()
	handler.deleteMessage(deleteRecorder, deleteRequest, "conversation_1", "message_1")
	if deleteRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected delete 200, got %d with body %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	if len(notifications) != 2 {
		t.Fatalf("expected two realtime notifications, got %+v", notifications)
	}
	if notifications[0].conversationID != "conversation_1" || notifications[0].eventType != "chat_message_updated" {
		t.Fatalf("unexpected update notification: %+v", notifications[0])
	}
	updatePayload, ok := notifications[0].payload.(map[string]any)
	if !ok {
		t.Fatalf("expected update map payload, got %#v", notifications[0].payload)
	}
	if updatePayload["messageId"] != "message_1" || updatePayload["userId"] != "user_1" {
		t.Fatalf("unexpected update payload: %+v", updatePayload)
	}
	if message, ok := updatePayload["message"].(chat.Message); !ok || message.Content != "Edited message" {
		t.Fatalf("unexpected updated message payload: %#v", updatePayload["message"])
	}

	if notifications[1].conversationID != "conversation_1" || notifications[1].eventType != "chat_message_deleted" {
		t.Fatalf("unexpected delete notification: %+v", notifications[1])
	}
	deletePayload, ok := notifications[1].payload.(map[string]any)
	if !ok {
		t.Fatalf("expected delete map payload, got %#v", notifications[1].payload)
	}
	if deletePayload["messageId"] != "message_1" || deletePayload["userId"] != "user_1" {
		t.Fatalf("unexpected delete payload: %+v", deletePayload)
	}
}

func TestChatHandlerStreamMessagePublishesRealtimeSyncAfterCompletion(t *testing.T) {
	expectedMessages := []chat.Message{
		{ID: "message_1", Role: "user", Content: "Draft a summary."},
		{ID: "message_2", Role: "assistant", Content: "Summary drafted."},
	}
	store := &chatFakeStore{messages: expectedMessages}
	handler := newChatHandler(chat.NewService(store, streamingReplyGenerator{chunks: []string{"Summary ", "drafted."}}, "demo-reply", nil))
	var notifications []chatRealtimeNotification
	handler.notifyConversation = func(conversationID, eventType string, payload any) {
		notifications = append(notifications, chatRealtimeNotification{conversationID: conversationID, eventType: eventType, payload: payload})
	}
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/app/conversations/conversation_1/messages/stream",
		strings.NewReader(`{"content":"Draft a summary."}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		User:        auth.User{ID: "user_1"},
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.streamMessage(recorder, request, "conversation_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected stream 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if len(notifications) != 1 {
		t.Fatalf("expected one realtime notification, got %+v", notifications)
	}
	if notifications[0].conversationID != "conversation_1" || notifications[0].eventType != "chat_messages_synced" {
		t.Fatalf("unexpected stream notification: %+v", notifications[0])
	}
	payload, ok := notifications[0].payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %#v", notifications[0].payload)
	}
	if payload["userId"] != "user_1" {
		t.Fatalf("unexpected stream payload: %+v", payload)
	}
	if messages, ok := payload["messages"].([]chat.Message); !ok || len(messages) != len(expectedMessages) {
		t.Fatalf("unexpected stream messages payload: %#v", payload["messages"])
	}
}
