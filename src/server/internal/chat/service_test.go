package chat

import (
	"context"
	"database/sql"
	"testing"
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

type fakeGenerator struct{}

func (f fakeGenerator) GenerateReply(ctx context.Context, messages []Message, config ConversationConfig) (string, error) {
	return "", nil
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
