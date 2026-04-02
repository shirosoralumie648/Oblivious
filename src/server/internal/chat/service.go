package chat

import (
	"context"
	"database/sql"
	"strings"
	"time"
	"unicode/utf8"

	"oblivious/server/internal/auth"
)

type Conversation struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Message struct {
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
	Role      string    `json:"role"`
}

type Store interface {
	CreateConversation(ctx context.Context, workspaceID, title, defaultModelID string) (Conversation, error)
	CreateMessage(ctx context.Context, conversationID, role, content string) (Message, error)
	GetConversationConfig(ctx context.Context, conversationID, workspaceID, defaultModelID string) (ConversationConfig, error)
	ListConversations(ctx context.Context, workspaceID string) ([]Conversation, error)
	ListMessages(ctx context.Context, conversationID, workspaceID string) ([]Message, error)
	UpdateConversationConfig(
		ctx context.Context,
		conversationID,
		workspaceID,
		modelID,
		systemPromptOverride string,
		temperature float64,
		maxOutputTokens int,
		toolsEnabled bool,
	) (ConversationConfig, error)
}

type UsageRecord struct {
	ConversationID string
	InputTokens    int
	ModelID        string
	OutputTokens   int
	RequestCount   int
	UserID         string
	WorkspaceID    string
}

type UsageRecorder interface {
	RecordChatUsage(ctx context.Context, record UsageRecord) error
}

type Service struct {
	defaultModelID string
	replyGenerator ReplyGenerator
	store          Store
	usageRecorder  UsageRecorder
}

func NewService(store Store, replyGenerator ReplyGenerator, defaultModelID string, usageRecorder UsageRecorder) *Service {
	return &Service{
		defaultModelID: defaultModelID,
		replyGenerator: replyGenerator,
		store:          store,
		usageRecorder:  usageRecorder,
	}
}

func (s *Service) CreateConversation(ctx context.Context, session auth.Session, title string) (Conversation, error) {
	if title == "" {
		title = "New conversation"
	}

	return s.store.CreateConversation(ctx, session.WorkspaceID, title, s.defaultModelID)
}

func (s *Service) GetConversationConfig(ctx context.Context, session auth.Session, conversationID string) (ConversationConfig, error) {
	return s.store.GetConversationConfig(ctx, conversationID, session.WorkspaceID, s.defaultModelID)
}

func (s *Service) ListConversations(ctx context.Context, session auth.Session) ([]Conversation, error) {
	return s.store.ListConversations(ctx, session.WorkspaceID)
}

func (s *Service) ListMessages(ctx context.Context, session auth.Session, conversationID string) ([]Message, error) {
	return s.store.ListMessages(ctx, conversationID, session.WorkspaceID)
}

func (s *Service) UpdateConversationConfig(
	ctx context.Context,
	session auth.Session,
	conversationID,
	modelID,
	systemPromptOverride string,
	temperature float64,
	maxOutputTokens int,
	toolsEnabled bool,
) (ConversationConfig, error) {
	if modelID == "" {
		modelID = s.defaultModelID
	}
	if temperature <= 0 {
		temperature = 1
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = 1024
	}

	return s.store.UpdateConversationConfig(
		ctx,
		conversationID,
		session.WorkspaceID,
		modelID,
		systemPromptOverride,
		temperature,
		maxOutputTokens,
		toolsEnabled,
	)
}

func (s *Service) ListModels() []ModelOption {
	defaultModel := s.defaultModelID
	if defaultModel == "" {
		defaultModel = "demo-reply"
	}

	return []ModelOption{
		{ID: defaultModel, Label: defaultModel},
		{ID: "balanced-chat", Label: "balanced-chat"},
		{ID: "quality-chat", Label: "quality-chat"},
	}
}

func mergeConversationConfig(base ConversationConfig, overrides *MessageOverrides, defaultModelID string) ConversationConfig {
	effective := base
	if effective.ModelID == "" {
		effective.ModelID = defaultModelID
	}
	if effective.Temperature <= 0 {
		effective.Temperature = 1
	}
	if effective.MaxOutputTokens <= 0 {
		effective.MaxOutputTokens = 1024
	}
	if overrides == nil {
		return effective
	}
	if overrides.ModelID != nil && *overrides.ModelID != "" {
		effective.ModelID = *overrides.ModelID
	}
	if overrides.SystemPromptOverride != nil {
		effective.SystemPromptOverride = *overrides.SystemPromptOverride
	}
	if overrides.Temperature != nil && *overrides.Temperature > 0 {
		effective.Temperature = *overrides.Temperature
	}
	if overrides.MaxOutputTokens != nil && *overrides.MaxOutputTokens > 0 {
		effective.MaxOutputTokens = *overrides.MaxOutputTokens
	}
	if overrides.ToolsEnabled != nil {
		effective.ToolsEnabled = *overrides.ToolsEnabled
	}
	return effective
}

func (s *Service) SendMessage(ctx context.Context, session auth.Session, conversationID, content string, overrides *MessageOverrides) ([]Message, error) {
	if _, err := s.store.CreateMessage(ctx, conversationID, "user", content); err != nil {
		return nil, err
	}

	messages, err := s.store.ListMessages(ctx, conversationID, session.WorkspaceID)
	if err != nil {
		return nil, err
	}

	conversationConfig, err := s.store.GetConversationConfig(ctx, conversationID, session.WorkspaceID, s.defaultModelID)
	if err != nil {
		return nil, err
	}

	effectiveConfig := mergeConversationConfig(conversationConfig, overrides, s.defaultModelID)

	reply, err := s.replyGenerator.GenerateReply(ctx, messages, effectiveConfig)
	if err != nil {
		return nil, err
	}

	if _, err := s.store.CreateMessage(ctx, conversationID, "assistant", reply); err != nil {
		return nil, err
	}

	if s.usageRecorder != nil {
		if err := s.usageRecorder.RecordChatUsage(ctx, UsageRecord{
			ConversationID: conversationID,
			InputTokens:    estimateTokens(content),
			ModelID:        effectiveConfig.ModelID,
			OutputTokens:   estimateTokens(reply),
			RequestCount:   1,
			UserID:         session.User.ID,
			WorkspaceID:    session.WorkspaceID,
		}); err != nil {
			return nil, err
		}
	}

	return s.store.ListMessages(ctx, conversationID, session.WorkspaceID)
}

func estimateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}

	runeCount := utf8.RuneCountInString(trimmed)
	tokenCount := runeCount / 4
	if runeCount%4 != 0 {
		tokenCount++
	}
	if tokenCount < 1 {
		return 1
	}

	return tokenCount
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
