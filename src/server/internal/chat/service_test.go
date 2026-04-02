package chat

import (
	"context"
	"database/sql"
	"testing"

	"oblivious/server/internal/auth"
)

type fakeStore struct {
	config ConversationConfig
}

func (f fakeStore) CreateConversation(ctx context.Context, workspaceID, title, defaultModelID string) (Conversation, error) {
	return Conversation{}, nil
}
func (f fakeStore) CreateMessage(ctx context.Context, conversationID, role, content string) (Message, error) {
	return Message{}, nil
}
func (f fakeStore) GetConversationConfig(ctx context.Context, conversationID, workspaceID, defaultModelID string) (ConversationConfig, error) {
	return f.config, nil
}
func (f fakeStore) ListConversations(ctx context.Context, workspaceID string) ([]Conversation, error) {
	return nil, nil
}
func (f fakeStore) ListMessages(ctx context.Context, conversationID, workspaceID string) ([]Message, error) {
	return nil, nil
}
func (f fakeStore) UpdateConversationConfig(
	ctx context.Context,
	conversationID,
	workspaceID,
	modelID,
	systemPromptOverride string,
	temperature float64,
	maxOutputTokens int,
	toolsEnabled bool,
	knowledgeBaseIDs []string,
) (ConversationConfig, error) {
	return ConversationConfig{}, sql.ErrNoRows
}

type fakeGenerator struct {
	reply string
}

func (f fakeGenerator) GenerateReply(ctx context.Context, messages []Message, config ConversationConfig) (string, error) {
	return f.reply, nil
}

type fakeUsageRecorder struct {
	records []UsageRecord
}

func (f *fakeUsageRecorder) RecordChatUsage(ctx context.Context, record UsageRecord) error {
	f.records = append(f.records, record)
	return nil
}

type recordingStore struct {
	config                   ConversationConfig
	lastConversationID       string
	lastWorkspaceID          string
	lastModelID              string
	lastSystemPromptOverride string
	lastTemperature          float64
	lastMaxOutputTokens      int
	lastToolsEnabled         bool
	lastKnowledgeBaseIDs     []string
	messages                 []Message
}

func (s *recordingStore) CreateConversation(ctx context.Context, workspaceID, title, defaultModelID string) (Conversation, error) {
	return Conversation{}, nil
}

func (s *recordingStore) CreateMessage(ctx context.Context, conversationID, role, content string) (Message, error) {
	message := Message{
		Content: content,
		ID:      role + "-message",
		Role:    role,
	}
	s.messages = append(s.messages, message)
	return message, nil
}

func (s *recordingStore) GetConversationConfig(ctx context.Context, conversationID, workspaceID, defaultModelID string) (ConversationConfig, error) {
	return s.config, nil
}

func (s *recordingStore) ListConversations(ctx context.Context, workspaceID string) ([]Conversation, error) {
	return nil, nil
}

func (s *recordingStore) ListMessages(ctx context.Context, conversationID, workspaceID string) ([]Message, error) {
	return append([]Message(nil), s.messages...), nil
}

func (s *recordingStore) UpdateConversationConfig(
	ctx context.Context,
	conversationID,
	workspaceID,
	modelID,
	systemPromptOverride string,
	temperature float64,
	maxOutputTokens int,
	toolsEnabled bool,
	knowledgeBaseIDs []string,
) (ConversationConfig, error) {
	s.lastConversationID = conversationID
	s.lastWorkspaceID = workspaceID
	s.lastModelID = modelID
	s.lastSystemPromptOverride = systemPromptOverride
	s.lastTemperature = temperature
	s.lastMaxOutputTokens = maxOutputTokens
	s.lastToolsEnabled = toolsEnabled
	s.lastKnowledgeBaseIDs = append([]string(nil), knowledgeBaseIDs...)

	return ConversationConfig{
		ConversationID:       conversationID,
		ModelID:              modelID,
		SystemPromptOverride: systemPromptOverride,
		Temperature:          temperature,
		MaxOutputTokens:      maxOutputTokens,
		ToolsEnabled:         toolsEnabled,
		KnowledgeBaseIDs:     append([]string(nil), knowledgeBaseIDs...),
	}, nil
}

func TestMergeConversationConfigAppliesMessageOverrides(t *testing.T) {
	modelID := "quality-chat"
	systemPrompt := "Be concise"
	temperature := 0.2
	maxTokens := 2048
	toolsEnabled := true

	base := ConversationConfig{
		ConversationID:       "c1",
		ModelID:              "balanced-chat",
		SystemPromptOverride: "Default prompt",
		Temperature:          1,
		MaxOutputTokens:      1024,
		KnowledgeBaseIDs:     []string{"kb_1"},
		ToolsEnabled:         false,
	}

	effective := mergeConversationConfig(base, &MessageOverrides{
		ModelID:              &modelID,
		SystemPromptOverride: &systemPrompt,
		Temperature:          &temperature,
		MaxOutputTokens:      &maxTokens,
		ToolsEnabled:         &toolsEnabled,
	}, "demo-reply")

	if effective.ModelID != modelID || effective.SystemPromptOverride != systemPrompt || effective.Temperature != temperature || effective.MaxOutputTokens != maxTokens || !effective.ToolsEnabled {
		t.Fatalf("unexpected effective config: %+v", effective)
	}
	if len(effective.KnowledgeBaseIDs) != 1 || effective.KnowledgeBaseIDs[0] != "kb_1" {
		t.Fatalf("expected knowledge bindings to be preserved, got %+v", effective.KnowledgeBaseIDs)
	}
}

func TestSendMessageRecordsUsage(t *testing.T) {
	store := &recordingStore{
		config: ConversationConfig{
			ConversationID:   "conversation_1",
			ModelID:          "quality-chat",
			Temperature:      1,
			MaxOutputTokens:  1024,
			KnowledgeBaseIDs: []string{"kb_7"},
		},
	}
	recorder := &fakeUsageRecorder{}
	service := NewService(store, fakeGenerator{reply: "assistant reply"}, "demo-reply", recorder)

	_, err := service.SendMessage(
		context.Background(),
		auth.Session{
			WorkspaceID: "workspace_1",
			User: auth.User{
				ID: "user_1",
			},
		},
		"conversation_1",
		"track usage",
		nil,
	)
	if err != nil {
		t.Fatalf("send message: %v", err)
	}

	if len(recorder.records) != 1 {
		t.Fatalf("expected 1 usage record, got %d", len(recorder.records))
	}

	record := recorder.records[0]
	if record.UserID != "user_1" {
		t.Fatalf("expected user id user_1, got %s", record.UserID)
	}
	if record.WorkspaceID != "workspace_1" {
		t.Fatalf("expected workspace id workspace_1, got %s", record.WorkspaceID)
	}
	if record.ConversationID != "conversation_1" {
		t.Fatalf("expected conversation id conversation_1, got %s", record.ConversationID)
	}
	if record.ModelID != "quality-chat" {
		t.Fatalf("expected model id quality-chat, got %s", record.ModelID)
	}
	if record.RequestCount != 1 {
		t.Fatalf("expected request count 1, got %d", record.RequestCount)
	}
}

func TestUpdateConversationConfigPersistsKnowledgeBaseIDs(t *testing.T) {
	store := &recordingStore{}
	service := NewService(store, fakeGenerator{}, "demo-reply", nil)

	config, err := service.UpdateConversationConfig(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1"},
		"conversation_1",
		"quality-chat",
		"Use docs first",
		0.4,
		2048,
		true,
		[]string{"kb_2", "kb_5"},
	)
	if err != nil {
		t.Fatalf("update config: %v", err)
	}

	if store.lastConversationID != "conversation_1" || store.lastWorkspaceID != "workspace_1" {
		t.Fatalf("unexpected store target: conversation=%s workspace=%s", store.lastConversationID, store.lastWorkspaceID)
	}
	if len(store.lastKnowledgeBaseIDs) != 2 || store.lastKnowledgeBaseIDs[0] != "kb_2" || store.lastKnowledgeBaseIDs[1] != "kb_5" {
		t.Fatalf("expected knowledge ids [kb_2 kb_5], got %+v", store.lastKnowledgeBaseIDs)
	}
	if len(config.KnowledgeBaseIDs) != 2 || config.KnowledgeBaseIDs[0] != "kb_2" || config.KnowledgeBaseIDs[1] != "kb_5" {
		t.Fatalf("expected config knowledge ids [kb_2 kb_5], got %+v", config.KnowledgeBaseIDs)
	}
}
