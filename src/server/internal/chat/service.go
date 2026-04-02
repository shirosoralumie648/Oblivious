package chat

import (
	"context"
	"database/sql"
	"time"

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

type Service struct {
	defaultModelID string
	replyGenerator ReplyGenerator
	store          Store
}

func NewService(store Store, replyGenerator ReplyGenerator, defaultModelID string) *Service {
	return &Service{
		defaultModelID: defaultModelID,
		replyGenerator: replyGenerator,
		store:          store,
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

func (s *Service) SendMessage(ctx context.Context, session auth.Session, conversationID, content string) ([]Message, error) {
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

	reply, err := s.replyGenerator.GenerateReply(ctx, messages, conversationConfig)
	if err != nil {
		return nil, err
	}

	if _, err := s.store.CreateMessage(ctx, conversationID, "assistant", reply); err != nil {
		return nil, err
	}

	return s.store.ListMessages(ctx, conversationID, session.WorkspaceID)
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
