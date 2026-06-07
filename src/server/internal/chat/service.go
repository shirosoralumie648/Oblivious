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
	CreatedAt             time.Time `json:"createdAt"`
	HasBookmarkedMessages bool      `json:"hasBookmarkedMessages,omitempty"`
	ID                    string    `json:"id"`
	ParentID              string    `json:"parentId,omitempty"`
	PersonaID             string    `json:"personaId,omitempty"`
	KnowledgeBaseIDs      []string  `json:"knowledgeBaseIds,omitempty"`
	Title                 string    `json:"title"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type Message struct {
	Attachments        []MessageAttachment `json:"attachments,omitempty"`
	Bookmarked         bool                `json:"bookmarked,omitempty"`
	Content            string              `json:"content"`
	CreatedAt          time.Time           `json:"createdAt"`
	ID                 string              `json:"id"`
	KnowledgeCitations []KnowledgeCitation `json:"knowledgeCitations,omitempty"`
	Role               string              `json:"role"`
	ToolCallID         string              `json:"toolCallId,omitempty"`
	ToolCalls          []ToolCall          `json:"toolCalls,omitempty"`
	Metadata           *MessageMetadata    `json:"metadata,omitempty"`
}

type TaskDraft struct {
	DraftTaskGoal           string   `json:"draftTaskGoal"`
	RelatedKnowledgeBaseIDs []string `json:"relatedKnowledgeBaseIds"`
	SuggestedBudget         int      `json:"suggestedBudget"`
	SuggestedExecutionMode  string   `json:"suggestedExecutionMode"`
}

type Store interface {
	CreateConversation(ctx context.Context, workspaceID string, args ...string) (Conversation, error)
	CreateMessage(ctx context.Context, conversationID string, args ...string) (Message, error)
	GetConversationConfig(ctx context.Context, conversationID, scopeID, defaultModelID string) (ConversationConfig, error)
	ListConversations(ctx context.Context, scopeID string) ([]Conversation, error)
	ListMessages(ctx context.Context, conversationID, scopeID string) ([]Message, error)
	UpdateConversationConfig(
		ctx context.Context,
		conversationID,
		scopeID,
		modelID,
		systemPromptOverride string,
		temperature float64,
		maxOutputTokens int,
		toolsEnabled bool,
		knowledgeBaseIDs []string,
		personaIDs ...string,
	) (ConversationConfig, error)

	// Conversation forking
	ForkConversation(ctx context.Context, organizationID, workspaceID, sourceConversationID, title, branchFromMessageID string) (Conversation, error)
	ListConversationBranches(ctx context.Context, conversationID, scopeID string) ([]Conversation, error)

	// Persona CRUD
	CreatePersona(ctx context.Context, workspaceID string, persona Persona) (Persona, error)
	GetPersona(ctx context.Context, personaID, workspaceID string) (Persona, error)
	ListPersonas(ctx context.Context, workspaceID string) ([]Persona, error)
	UpdatePersona(ctx context.Context, personaID, workspaceID string, persona Persona) (Persona, error)
	DeletePersona(ctx context.Context, personaID, workspaceID string) error

	// Message metadata & search
	CreateMessageWithMetadata(ctx context.Context, conversationID, role, content string, metadata *MessageMetadata) (Message, error)
	AddMessageAttachment(ctx context.Context, attachment MessageAttachment) (MessageAttachment, error)
	ListMessageAttachments(ctx context.Context, messageID string) ([]MessageAttachment, error)
	SearchMessages(ctx context.Context, workspaceID, query string, limit int) ([]MessageSearchResult, error)
}

type ConversationActionStore interface {
	GetConversation(ctx context.Context, conversationID, scopeID string) (Conversation, error)
	UpdateConversation(ctx context.Context, conversationID, scopeID, title string) (Conversation, error)
	DeleteConversation(ctx context.Context, conversationID, scopeID string) error
}

type MessageActionStore interface {
	UpdateMessage(ctx context.Context, conversationID, scopeID, messageID, content string) (Message, error)
	DeleteMessage(ctx context.Context, conversationID, scopeID, messageID string) error
	BookmarkMessage(ctx context.Context, conversationID, scopeID, messageID string, bookmarked bool) (Message, error)
}

type ShareStore interface {
	CreateMessageShareWithOptions(ctx context.Context, conversationID, organizationID, messageID string, expiresAt *time.Time) (MessageShare, error)
	GetMessageShare(ctx context.Context, shareID string, now time.Time) (MessageShareDetail, error)
	CreateConversationShare(ctx context.Context, conversationID, organizationID string, options ConversationShareStoreOptions) (ConversationShare, error)
	GetConversationShare(ctx context.Context, shareID string, now time.Time) (ConversationShareDetail, error)
}

type UsageRecord struct {
	ConversationID string
	InputTokens    int
	ModelID        string
	OrganizationID string
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

	if organizationID := strings.TrimSpace(session.OrganizationID); organizationID != "" {
		return s.store.CreateConversation(ctx, session.WorkspaceID, organizationID, title, s.defaultModelID)
	}
	return s.store.CreateConversation(ctx, session.WorkspaceID, title, s.defaultModelID)
}

func (s *Service) GetConversation(ctx context.Context, session auth.Session, conversationID string) (Conversation, error) {
	store, ok := s.store.(ConversationActionStore)
	if !ok {
		return Conversation{}, ErrUnsupportedChatAction
	}
	return store.GetConversation(ctx, conversationID, chatSessionScopeID(session))
}

func (s *Service) UpdateConversation(ctx context.Context, session auth.Session, conversationID, title string) (Conversation, error) {
	store, ok := s.store.(ConversationActionStore)
	if !ok {
		return Conversation{}, ErrUnsupportedChatAction
	}
	return store.UpdateConversation(ctx, conversationID, chatSessionScopeID(session), title)
}

func (s *Service) DeleteConversation(ctx context.Context, session auth.Session, conversationID string) error {
	store, ok := s.store.(ConversationActionStore)
	if !ok {
		return ErrUnsupportedChatAction
	}
	return store.DeleteConversation(ctx, conversationID, chatSessionScopeID(session))
}

func (s *Service) ForkConversation(ctx context.Context, session auth.Session, sourceConversationID, branchFromMessageID, title string) (Conversation, error) {
	if title == "" {
		title = "Branched conversation"
	}

	return s.store.ForkConversation(ctx, chatSessionScopeID(session), session.WorkspaceID, sourceConversationID, title, branchFromMessageID)
}

func (s *Service) ListConversationBranches(ctx context.Context, session auth.Session, conversationID string) ([]Conversation, error) {
	return s.store.ListConversationBranches(ctx, conversationID, chatSessionScopeID(session))
}

func (s *Service) CreatePersona(ctx context.Context, session auth.Session, persona Persona) (Persona, error) {
	persona.WorkspaceID = session.WorkspaceID
	return s.store.CreatePersona(ctx, chatSessionScopeID(session), persona)
}

func (s *Service) GetPersona(ctx context.Context, session auth.Session, personaID string) (Persona, error) {
	return s.store.GetPersona(ctx, personaID, chatSessionScopeID(session))
}

func (s *Service) ListPersonas(ctx context.Context, session auth.Session) ([]Persona, error) {
	return s.store.ListPersonas(ctx, chatSessionScopeID(session))
}

func (s *Service) UpdatePersona(ctx context.Context, session auth.Session, personaID string, persona Persona) (Persona, error) {
	return s.store.UpdatePersona(ctx, personaID, chatSessionScopeID(session), persona)
}

func (s *Service) DeletePersona(ctx context.Context, session auth.Session, personaID string) error {
	return s.store.DeletePersona(ctx, personaID, chatSessionScopeID(session))
}

func (s *Service) SearchMessages(ctx context.Context, session auth.Session, query string, limit int) ([]MessageSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.store.SearchMessages(ctx, chatSessionScopeID(session), query, limit)
}

func (s *Service) AddMessageAttachment(ctx context.Context, session auth.Session, attachment MessageAttachment) (MessageAttachment, error) {
	return s.store.AddMessageAttachment(ctx, attachment)
}

func (s *Service) ListMessageAttachments(ctx context.Context, session auth.Session, messageID string) ([]MessageAttachment, error) {
	return s.store.ListMessageAttachments(ctx, messageID)
}

func (s *Service) GetConversationConfig(ctx context.Context, session auth.Session, conversationID string) (ConversationConfig, error) {
	return s.store.GetConversationConfig(ctx, conversationID, chatSessionScopeID(session), s.defaultModelID)
}

func (s *Service) ListConversations(ctx context.Context, session auth.Session) ([]Conversation, error) {
	return s.store.ListConversations(ctx, chatSessionScopeID(session))
}

func (s *Service) ListMessages(ctx context.Context, session auth.Session, conversationID string) ([]Message, error) {
	return s.store.ListMessages(ctx, conversationID, chatSessionScopeID(session))
}

func (s *Service) UpdateMessage(ctx context.Context, session auth.Session, conversationID, messageID, content string) (Message, error) {
	store, ok := s.store.(MessageActionStore)
	if !ok {
		return Message{}, ErrUnsupportedChatAction
	}
	return store.UpdateMessage(ctx, conversationID, chatSessionScopeID(session), messageID, content)
}

func (s *Service) DeleteMessage(ctx context.Context, session auth.Session, conversationID, messageID string) error {
	store, ok := s.store.(MessageActionStore)
	if !ok {
		return ErrUnsupportedChatAction
	}
	return store.DeleteMessage(ctx, conversationID, chatSessionScopeID(session), messageID)
}

func (s *Service) BookmarkMessage(ctx context.Context, session auth.Session, conversationID, messageID string, bookmarked bool) (Message, error) {
	store, ok := s.store.(MessageActionStore)
	if !ok {
		return Message{}, ErrUnsupportedChatAction
	}
	return store.BookmarkMessage(ctx, conversationID, chatSessionScopeID(session), messageID, bookmarked)
}

func (s *Service) CreateMessageShare(ctx context.Context, session auth.Session, conversationID, messageID string, expiresAt *time.Time) (MessageShare, error) {
	store, ok := s.store.(ShareStore)
	if !ok {
		return MessageShare{}, ErrUnsupportedChatAction
	}
	return store.CreateMessageShareWithOptions(ctx, conversationID, chatSessionScopeID(session), messageID, expiresAt)
}

func (s *Service) GetMessageShare(ctx context.Context, shareID string, now time.Time) (MessageShareDetail, error) {
	store, ok := s.store.(ShareStore)
	if !ok {
		return MessageShareDetail{}, ErrUnsupportedChatAction
	}
	return store.GetMessageShare(ctx, shareID, now)
}

func (s *Service) CreateConversationShare(ctx context.Context, session auth.Session, conversationID string, options ConversationShareStoreOptions) (ConversationShare, error) {
	store, ok := s.store.(ShareStore)
	if !ok {
		return ConversationShare{}, ErrUnsupportedChatAction
	}
	return store.CreateConversationShare(ctx, conversationID, chatSessionScopeID(session), options)
}

func (s *Service) GetConversationShare(ctx context.Context, shareID string, now time.Time) (ConversationShareDetail, error) {
	store, ok := s.store.(ShareStore)
	if !ok {
		return ConversationShareDetail{}, ErrUnsupportedChatAction
	}
	return store.GetConversationShare(ctx, shareID, now)
}

func (s *Service) ConvertConversationToTask(ctx context.Context, session auth.Session, conversationID string) (TaskDraft, error) {
	config, err := s.store.GetConversationConfig(ctx, conversationID, chatSessionScopeID(session), s.defaultModelID)
	if err != nil {
		return TaskDraft{}, err
	}

	messages, err := s.store.ListMessages(ctx, conversationID, chatSessionScopeID(session))
	if err != nil {
		return TaskDraft{}, err
	}

	return TaskDraft{
		DraftTaskGoal:           draftTaskGoalFromMessages(messages),
		RelatedKnowledgeBaseIDs: normalizeKnowledgeBaseIDs(config.KnowledgeBaseIDs),
		SuggestedBudget:         20,
		SuggestedExecutionMode:  "standard",
	}, nil
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
	knowledgeBaseIDs []string,
	personaIDs ...string,
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
	knowledgeBaseIDs = normalizeKnowledgeBaseIDs(knowledgeBaseIDs)
	personaID := ""
	if len(personaIDs) > 0 {
		personaID = strings.TrimSpace(personaIDs[0])
	}

	return s.store.UpdateConversationConfig(
		ctx,
		conversationID,
		chatSessionScopeID(session),
		modelID,
		systemPromptOverride,
		temperature,
		maxOutputTokens,
		toolsEnabled,
		knowledgeBaseIDs,
		personaID,
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
	if effective.KnowledgeBaseIDs == nil {
		effective.KnowledgeBaseIDs = []string{}
	}
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
	if overrides.PersonaID != nil && *overrides.PersonaID != "" {
		effective.PersonaID = *overrides.PersonaID
	}
	return effective
}

func normalizeKnowledgeBaseIDs(ids []string) []string {
	if len(ids) == 0 {
		return []string{}
	}

	normalized := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	if len(normalized) == 0 {
		return []string{}
	}

	return normalized
}

func draftTaskGoalFromMessages(messages []Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		if message.Role != "user" {
			continue
		}

		trimmedContent := strings.TrimSpace(message.Content)
		if trimmedContent != "" {
			return trimmedContent
		}
	}

	return "Continue this conversation in SOLO."
}

func (s *Service) SendMessage(ctx context.Context, session auth.Session, conversationID, content string, overrides *MessageOverrides) ([]Message, error) {
	scopeID := chatSessionScopeID(session)
	if strings.TrimSpace(session.OrganizationID) != "" {
		if _, err := s.store.CreateMessage(ctx, conversationID, session.OrganizationID, "user", content); err != nil {
			return nil, err
		}
	} else if _, err := s.store.CreateMessage(ctx, conversationID, "user", content); err != nil {
		return nil, err
	}

	messages, err := s.store.ListMessages(ctx, conversationID, scopeID)
	if err != nil {
		return nil, err
	}

	conversationConfig, err := s.store.GetConversationConfig(ctx, conversationID, scopeID, s.defaultModelID)
	if err != nil {
		return nil, err
	}

	effectiveConfig := mergeConversationConfig(conversationConfig, overrides, s.defaultModelID)

	// Resolve persona system prompt if a persona is configured
	if effectiveConfig.PersonaID != "" {
		persona, personaErr := s.store.GetPersona(ctx, effectiveConfig.PersonaID, scopeID)
		if personaErr == nil && persona.ID != "" {
			personaPrompt := buildPersonaSystemPrompt(persona)
			if effectiveConfig.SystemPromptOverride != "" {
				effectiveConfig.SystemPromptOverride = personaPrompt + "\n\n" + effectiveConfig.SystemPromptOverride
			} else {
				effectiveConfig.SystemPromptOverride = personaPrompt
			}
		}
	}

	reply, err := s.replyGenerator.GenerateReply(ctx, messages, effectiveConfig)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(session.OrganizationID) != "" {
		if _, err := s.store.CreateMessage(ctx, conversationID, session.OrganizationID, "assistant", reply); err != nil {
			return nil, err
		}
	} else if _, err := s.store.CreateMessage(ctx, conversationID, "assistant", reply); err != nil {
		return nil, err
	}

	if s.usageRecorder != nil {
		if err := s.usageRecorder.RecordChatUsage(ctx, UsageRecord{
			ConversationID: conversationID,
			InputTokens:    estimateTokens(content),
			ModelID:        effectiveConfig.ModelID,
			OrganizationID: session.OrganizationID,
			OutputTokens:   estimateTokens(reply),
			RequestCount:   1,
			UserID:         session.User.ID,
			WorkspaceID:    session.WorkspaceID,
		}); err != nil {
			return nil, err
		}
	}

	return s.store.ListMessages(ctx, conversationID, scopeID)
}

func chatSessionScopeID(session auth.Session) string {
	if organizationID := strings.TrimSpace(session.OrganizationID); organizationID != "" {
		return organizationID
	}
	return session.WorkspaceID
}

func buildPersonaSystemPrompt(persona Persona) string {
	parts := []string{}
	if persona.Name != "" {
		parts = append(parts, "Name: "+persona.Name)
	}
	if persona.Role != "" {
		parts = append(parts, "Role: "+persona.Role)
	}
	if persona.Style != "" {
		parts = append(parts, "Style: "+persona.Style)
	}
	if persona.Tone != "" {
		parts = append(parts, "Tone: "+persona.Tone)
	}
	if persona.Constraints != "" {
		parts = append(parts, "Constraints: "+persona.Constraints)
	}
	if persona.OpeningMessage != "" {
		parts = append(parts, "Opening message: "+persona.OpeningMessage)
	}
	return "You are the following persona. Adhere to these traits:\n" + joinLines(parts)
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
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
