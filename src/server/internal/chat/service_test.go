package chat

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"oblivious/server/internal/auth"
)

type fakeStore struct {
	config   ConversationConfig
	messages []Message
}

func (f fakeStore) CreateConversation(ctx context.Context, workspaceID string, args ...string) (Conversation, error) {
	return Conversation{}, nil
}
func (f fakeStore) CreateMessage(ctx context.Context, conversationID string, args ...string) (Message, error) {
	return Message{}, nil
}
func (f fakeStore) GetConversationConfig(ctx context.Context, conversationID, workspaceID, defaultModelID string) (ConversationConfig, error) {
	return f.config, nil
}
func (f fakeStore) ListConversations(ctx context.Context, workspaceID string) ([]Conversation, error) {
	return nil, nil
}
func (f fakeStore) ListMessages(ctx context.Context, conversationID, workspaceID string) ([]Message, error) {
	return append([]Message(nil), f.messages...), nil
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
	personaIDs ...string,
) (ConversationConfig, error) {
	return ConversationConfig{}, sql.ErrNoRows
}
func (f fakeStore) ForkConversation(ctx context.Context, organizationID, workspaceID, sourceConversationID, title, branchFromMessageID string) (Conversation, error) {
	return Conversation{}, nil
}
func (f fakeStore) ListConversationBranches(ctx context.Context, conversationID, workspaceID string) ([]Conversation, error) {
	return nil, nil
}
func (f fakeStore) CreatePersona(ctx context.Context, workspaceID string, persona Persona) (Persona, error) {
	return persona, nil
}
func (f fakeStore) GetPersona(ctx context.Context, personaID, workspaceID string) (Persona, error) {
	return Persona{}, nil
}
func (f fakeStore) ListPersonas(ctx context.Context, workspaceID string) ([]Persona, error) {
	return nil, nil
}
func (f fakeStore) UpdatePersona(ctx context.Context, personaID, workspaceID string, persona Persona) (Persona, error) {
	return persona, nil
}
func (f fakeStore) DeletePersona(ctx context.Context, personaID, workspaceID string) error {
	return nil
}
func (f fakeStore) CreateMessageWithMetadata(ctx context.Context, conversationID, role, content string, metadata *MessageMetadata) (Message, error) {
	return Message{}, nil
}
func (f fakeStore) AddMessageAttachment(ctx context.Context, attachment MessageAttachment) (MessageAttachment, error) {
	return attachment, nil
}
func (f fakeStore) ListMessageAttachments(ctx context.Context, messageID string) ([]MessageAttachment, error) {
	return nil, nil
}
func (f fakeStore) SearchMessages(ctx context.Context, workspaceID, query string, limit int) ([]MessageSearchResult, error) {
	return nil, nil
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

type fakeSemanticWorkflowTriggerer struct {
	requests []SemanticWorkflowTriggerRequest
}

func (f *fakeSemanticWorkflowTriggerer) TriggerSemanticWorkflows(ctx context.Context, req SemanticWorkflowTriggerRequest) error {
	f.requests = append(f.requests, req)
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

func (s *recordingStore) CreateConversation(ctx context.Context, workspaceID string, args ...string) (Conversation, error) {
	return Conversation{}, nil
}

func (s *recordingStore) CreateMessage(ctx context.Context, conversationID string, args ...string) (Message, error) {
	_, role, content := parseCreateMessageArgs(args)
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
	personaIDs ...string,
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

func (s *recordingStore) ForkConversation(ctx context.Context, organizationID, workspaceID, sourceConversationID, title, branchFromMessageID string) (Conversation, error) {
	return Conversation{ID: "forked_conversation", ParentID: sourceConversationID, Title: title}, nil
}

func (s *recordingStore) ListConversationBranches(ctx context.Context, conversationID, workspaceID string) ([]Conversation, error) {
	return nil, nil
}

func (s *recordingStore) CreatePersona(ctx context.Context, workspaceID string, persona Persona) (Persona, error) {
	persona.ID = "persona_new"
	return persona, nil
}

func (s *recordingStore) GetPersona(ctx context.Context, personaID, workspaceID string) (Persona, error) {
	return Persona{ID: personaID, WorkspaceID: workspaceID}, nil
}

func (s *recordingStore) ListPersonas(ctx context.Context, workspaceID string) ([]Persona, error) {
	return nil, nil
}

func (s *recordingStore) UpdatePersona(ctx context.Context, personaID, workspaceID string, persona Persona) (Persona, error) {
	persona.ID = personaID
	return persona, nil
}

func (s *recordingStore) DeletePersona(ctx context.Context, personaID, workspaceID string) error {
	return nil
}

func (s *recordingStore) CreateMessageWithMetadata(ctx context.Context, conversationID, role, content string, metadata *MessageMetadata) (Message, error) {
	return s.CreateMessage(ctx, conversationID, role, content)
}

func (s *recordingStore) AddMessageAttachment(ctx context.Context, attachment MessageAttachment) (MessageAttachment, error) {
	return attachment, nil
}

func (s *recordingStore) ListMessageAttachments(ctx context.Context, messageID string) ([]MessageAttachment, error) {
	return nil, nil
}

func (s *recordingStore) SearchMessages(ctx context.Context, workspaceID, query string, limit int) ([]MessageSearchResult, error) {
	return nil, nil
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

func TestSendMessageTriggersSemanticWorkflowsAfterAssistantReply(t *testing.T) {
	store := &recordingStore{
		config: ConversationConfig{
			ConversationID:  "conversation_1",
			ModelID:         "quality-chat",
			Temperature:     1,
			MaxOutputTokens: 1024,
		},
	}
	triggerer := &fakeSemanticWorkflowTriggerer{}
	service := NewService(store, fakeGenerator{reply: "assistant reply"}, "demo-reply", nil)
	service.SetSemanticWorkflowTriggerer(triggerer)

	_, err := service.SendMessage(
		context.Background(),
		auth.Session{
			OrganizationID: "org_1",
			WorkspaceID:    "workspace_1",
			User: auth.User{
				ID: "user_1",
			},
		},
		"conversation_1",
		"please triage this incident",
		nil,
	)
	if err != nil {
		t.Fatalf("send message: %v", err)
	}

	if len(triggerer.requests) != 1 {
		t.Fatalf("expected one semantic workflow trigger request, got %d", len(triggerer.requests))
	}
	request := triggerer.requests[0]
	if request.OrganizationID != "org_1" || request.WorkspaceID != "workspace_1" || request.UserID != "user_1" {
		t.Fatalf("unexpected trigger scope: %+v", request)
	}
	if request.ConversationID != "conversation_1" || request.Message != "please triage this incident" {
		t.Fatalf("unexpected trigger payload: %+v", request)
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

func TestConvertConversationToTaskBuildsTaskDraftFromLatestUserMessage(t *testing.T) {
	store := fakeStore{
		config: ConversationConfig{
			ConversationID:   "conversation_1",
			KnowledgeBaseIDs: []string{"kb_2", "kb_5"},
		},
		messages: []Message{
			{ID: "message_1", Role: "assistant", Content: "How can I help?"},
			{ID: "message_2", Role: "user", Content: "  Draft a launch checklist from our current discussion.  "},
		},
	}
	service := NewService(store, fakeGenerator{}, "demo-reply", nil)

	draft, err := service.ConvertConversationToTask(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "conversation_1")
	if err != nil {
		t.Fatalf("convert conversation to task: %v", err)
	}

	if draft.DraftTaskGoal != "Draft a launch checklist from our current discussion." {
		t.Fatalf("unexpected draft goal: %+v", draft)
	}
	if draft.SuggestedExecutionMode != "standard" {
		t.Fatalf("expected standard execution mode, got %+v", draft)
	}
	if draft.SuggestedBudget != 20 {
		t.Fatalf("expected suggested budget 20, got %+v", draft)
	}
	if len(draft.RelatedKnowledgeBaseIDs) != 2 || draft.RelatedKnowledgeBaseIDs[0] != "kb_2" || draft.RelatedKnowledgeBaseIDs[1] != "kb_5" {
		t.Fatalf("unexpected related knowledge bases: %+v", draft.RelatedKnowledgeBaseIDs)
	}
}

func TestMergeConversationConfigAppliesPersonaIDOverride(t *testing.T) {
	personaID := "persona_custom"
	base := ConversationConfig{
		ConversationID:   "c1",
		ModelID:          "balanced-chat",
		Temperature:      1,
		MaxOutputTokens:  1024,
		KnowledgeBaseIDs: []string{},
	}
	effective := mergeConversationConfig(base, &MessageOverrides{
		PersonaID: &personaID,
	}, "demo-reply")
	if effective.PersonaID != personaID {
		t.Fatalf("expected persona id %s, got %s", personaID, effective.PersonaID)
	}
}

func TestForkConversationReturnsForkedConversation(t *testing.T) {
	store := &recordingStore{}
	service := NewService(store, fakeGenerator{}, "demo-reply", nil)

	conv, err := service.ForkConversation(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1"},
		"source_conv",
		"msg_branch_point",
		"My branch",
	)
	if err != nil {
		t.Fatalf("fork conversation: %v", err)
	}
	if conv.ID != "forked_conversation" {
		t.Fatalf("expected forked_conversation id, got %s", conv.ID)
	}
	if conv.ParentID != "source_conv" {
		t.Fatalf("expected parent source_conv, got %s", conv.ParentID)
	}
	if conv.Title != "My branch" {
		t.Fatalf("expected title 'My branch', got %s", conv.Title)
	}
}

func TestCreatePersonaPersistsPersona(t *testing.T) {
	store := &recordingStore{}
	service := NewService(store, fakeGenerator{}, "demo-reply", nil)

	persona, err := service.CreatePersona(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1"},
		Persona{
			Name:  "Helpful Assistant",
			Role:  "Tutor",
			Style: "Friendly",
			Tone:  "Warm",
		},
	)
	if err != nil {
		t.Fatalf("create persona: %v", err)
	}
	if persona.ID != "persona_new" {
		t.Fatalf("expected persona_new id, got %s", persona.ID)
	}
	if persona.Name != "Helpful Assistant" {
		t.Fatalf("expected name 'Helpful Assistant', got %s", persona.Name)
	}
}

func TestSendMessageWithPersonaInjectsPersonaSystemPrompt(t *testing.T) {
	personaID := "persona_test"
	store := &personaAwareStore{
		config: ConversationConfig{
			ConversationID:   "conversation_1",
			ModelID:          "quality-chat",
			PersonaID:        personaID,
			Temperature:      1,
			MaxOutputTokens:  1024,
			KnowledgeBaseIDs: []string{},
		},
		persona: Persona{
			ID:   personaID,
			Name: "Test Persona",
			Role: "Assistant",
		},
	}
	generator := &capturingGenerator{}
	service := NewService(store, generator, "demo-reply", nil)

	_, err := service.SendMessage(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}},
		"conversation_1",
		"Hello",
		nil,
	)
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if generator.lastConfig.SystemPromptOverride == "" {
		t.Fatalf("expected persona system prompt to be injected, got empty")
	}
	if !strings.Contains(generator.lastConfig.SystemPromptOverride, "Test Persona") {
		t.Fatalf("expected persona name in system prompt, got %s", generator.lastConfig.SystemPromptOverride)
	}
	if !strings.Contains(generator.lastConfig.SystemPromptOverride, "Assistant") {
		t.Fatalf("expected persona role in system prompt, got %s", generator.lastConfig.SystemPromptOverride)
	}
}

func TestSearchMessagesReturnsResults(t *testing.T) {
	store := &searchableStore{
		results: []MessageSearchResult{
			{MessageID: "msg_1", ConversationID: "conv_1", Role: "user", Content: "Hello world", Snippet: "Hello world"},
		},
	}
	service := NewService(store, fakeGenerator{}, "demo-reply", nil)

	results, err := service.SearchMessages(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1"},
		"hello",
		10,
	)
	if err != nil {
		t.Fatalf("search messages: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Snippet != "Hello world" {
		t.Fatalf("expected snippet 'Hello world', got %s", results[0].Snippet)
	}
}

func TestExtractSnippetHighlightsMatch(t *testing.T) {
	content := "This is a very long message that we use for testing purposes and it contains the word important somewhere in the middle of a much longer piece of text that exceeds 120 characters easily"
	snippet := extractSnippet(content, "important")
	if !strings.Contains(snippet, "important") {
		t.Fatalf("expected snippet to contain 'important', got %s", snippet)
	}
	if !strings.HasPrefix(snippet, "...") {
		t.Fatalf("expected snippet to start with '...', got %s", snippet)
	}
	if !strings.HasSuffix(snippet, "...") {
		t.Fatalf("expected snippet to end with '...', got %s", snippet)
	}
}

func TestExtractSnippetReturnsFullContentWhenShort(t *testing.T) {
	content := "short text"
	snippet := extractSnippet(content, "text")
	if snippet != "short text" {
		t.Fatalf("expected full content, got %s", snippet)
	}
}

type capturingGenerator struct {
	lastConfig ConversationConfig
}

func (g *capturingGenerator) GenerateReply(ctx context.Context, messages []Message, config ConversationConfig) (string, error) {
	g.lastConfig = config
	return "reply", nil
}

type searchableStore struct {
	results []MessageSearchResult
}

func (s *searchableStore) CreateConversation(ctx context.Context, workspaceID string, args ...string) (Conversation, error) {
	return Conversation{}, nil
}
func (s *searchableStore) CreateMessage(ctx context.Context, conversationID string, args ...string) (Message, error) {
	return Message{}, nil
}
func (s *searchableStore) GetConversationConfig(ctx context.Context, conversationID, workspaceID, defaultModelID string) (ConversationConfig, error) {
	return ConversationConfig{}, nil
}
func (s *searchableStore) ListConversations(ctx context.Context, workspaceID string) ([]Conversation, error) {
	return nil, nil
}
func (s *searchableStore) ListMessages(ctx context.Context, conversationID, workspaceID string) ([]Message, error) {
	return nil, nil
}
func (s *searchableStore) UpdateConversationConfig(ctx context.Context, conversationID, workspaceID, modelID, systemPromptOverride string, temperature float64, maxOutputTokens int, toolsEnabled bool, knowledgeBaseIDs []string, personaIDs ...string) (ConversationConfig, error) {
	return ConversationConfig{}, nil
}
func (s *searchableStore) ForkConversation(ctx context.Context, organizationID, workspaceID, sourceConversationID, title, branchFromMessageID string) (Conversation, error) {
	return Conversation{}, nil
}
func (s *searchableStore) ListConversationBranches(ctx context.Context, conversationID, workspaceID string) ([]Conversation, error) {
	return nil, nil
}
func (s *searchableStore) CreatePersona(ctx context.Context, workspaceID string, persona Persona) (Persona, error) {
	return persona, nil
}
func (s *searchableStore) GetPersona(ctx context.Context, personaID, workspaceID string) (Persona, error) {
	return Persona{}, nil
}
func (s *searchableStore) ListPersonas(ctx context.Context, workspaceID string) ([]Persona, error) {
	return nil, nil
}
func (s *searchableStore) UpdatePersona(ctx context.Context, personaID, workspaceID string, persona Persona) (Persona, error) {
	return persona, nil
}
func (s *searchableStore) DeletePersona(ctx context.Context, personaID, workspaceID string) error {
	return nil
}
func (s *searchableStore) CreateMessageWithMetadata(ctx context.Context, conversationID, role, content string, metadata *MessageMetadata) (Message, error) {
	return Message{}, nil
}
func (s *searchableStore) AddMessageAttachment(ctx context.Context, attachment MessageAttachment) (MessageAttachment, error) {
	return attachment, nil
}
func (s *searchableStore) ListMessageAttachments(ctx context.Context, messageID string) ([]MessageAttachment, error) {
	return nil, nil
}
func (s *searchableStore) SearchMessages(ctx context.Context, workspaceID, query string, limit int) ([]MessageSearchResult, error) {
	return s.results, nil
}

type personaAwareStore struct {
	config   ConversationConfig
	persona  Persona
	messages []Message
}

func (s *personaAwareStore) CreateConversation(ctx context.Context, workspaceID string, args ...string) (Conversation, error) {
	return Conversation{}, nil
}
func (s *personaAwareStore) CreateMessage(ctx context.Context, conversationID string, args ...string) (Message, error) {
	_, role, content := parseCreateMessageArgs(args)
	msg := Message{Content: content, ID: role + "-message", Role: role}
	s.messages = append(s.messages, msg)
	return msg, nil
}
func (s *personaAwareStore) GetConversationConfig(ctx context.Context, conversationID, workspaceID, defaultModelID string) (ConversationConfig, error) {
	return s.config, nil
}
func (s *personaAwareStore) ListConversations(ctx context.Context, workspaceID string) ([]Conversation, error) {
	return nil, nil
}
func (s *personaAwareStore) ListMessages(ctx context.Context, conversationID, workspaceID string) ([]Message, error) {
	return append([]Message(nil), s.messages...), nil
}
func (s *personaAwareStore) UpdateConversationConfig(ctx context.Context, conversationID, workspaceID, modelID, systemPromptOverride string, temperature float64, maxOutputTokens int, toolsEnabled bool, knowledgeBaseIDs []string, personaIDs ...string) (ConversationConfig, error) {
	return ConversationConfig{}, nil
}
func (s *personaAwareStore) ForkConversation(ctx context.Context, organizationID, workspaceID, sourceConversationID, title, branchFromMessageID string) (Conversation, error) {
	return Conversation{}, nil
}
func (s *personaAwareStore) ListConversationBranches(ctx context.Context, conversationID, workspaceID string) ([]Conversation, error) {
	return nil, nil
}
func (s *personaAwareStore) CreatePersona(ctx context.Context, workspaceID string, persona Persona) (Persona, error) {
	return persona, nil
}
func (s *personaAwareStore) GetPersona(ctx context.Context, personaID, workspaceID string) (Persona, error) {
	return s.persona, nil
}
func (s *personaAwareStore) ListPersonas(ctx context.Context, workspaceID string) ([]Persona, error) {
	return nil, nil
}
func (s *personaAwareStore) UpdatePersona(ctx context.Context, personaID, workspaceID string, persona Persona) (Persona, error) {
	return persona, nil
}
func (s *personaAwareStore) DeletePersona(ctx context.Context, personaID, workspaceID string) error {
	return nil
}
func (s *personaAwareStore) CreateMessageWithMetadata(ctx context.Context, conversationID, role, content string, metadata *MessageMetadata) (Message, error) {
	return s.CreateMessage(ctx, conversationID, role, content)
}
func (s *personaAwareStore) AddMessageAttachment(ctx context.Context, attachment MessageAttachment) (MessageAttachment, error) {
	return attachment, nil
}
func (s *personaAwareStore) ListMessageAttachments(ctx context.Context, messageID string) ([]MessageAttachment, error) {
	return nil, nil
}
func (s *personaAwareStore) SearchMessages(ctx context.Context, workspaceID, query string, limit int) ([]MessageSearchResult, error) {
	return nil, nil
}
