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
func (f fakeStore) UpdateConversationConfig(ctx context.Context, conversationID, workspaceID, modelID, systemPromptOverride string, temperature float64, maxOutputTokens int, toolsEnabled bool) (ConversationConfig, error) {
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
	config   ConversationConfig
	messages []Message
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

func (s *recordingStore) UpdateConversationConfig(ctx context.Context, conversationID, workspaceID, modelID, systemPromptOverride string, temperature float64, maxOutputTokens int, toolsEnabled bool) (ConversationConfig, error) {
	return ConversationConfig{}, nil
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
}

func TestSendMessageRecordsUsage(t *testing.T) {
	store := &recordingStore{
		config: ConversationConfig{
			ConversationID:  "conversation_1",
			ModelID:         "quality-chat",
			Temperature:     1,
			MaxOutputTokens: 1024,
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
