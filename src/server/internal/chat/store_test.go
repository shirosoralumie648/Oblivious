package chat

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func chatTestDatabase(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for chat SQL store integration tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	if _, err := database.Exec(`SELECT pg_advisory_lock(104211)`); err != nil {
		t.Fatalf("lock integration test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104211)`); err != nil {
			t.Fatalf("unlock integration test database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS conversation_knowledge_bindings CASCADE`,
		`DROP TABLE IF EXISTS knowledge_bases CASCADE`,
		`DROP TABLE IF EXISTS conversation_configs CASCADE`,
		`DROP TABLE IF EXISTS conversation_shares CASCADE`,
		`DROP TABLE IF EXISTS message_shares CASCADE`,
		`DROP TABLE IF EXISTS messages CASCADE`,
		`DROP TABLE IF EXISTS conversations CASCADE`,
		`DROP TABLE IF EXISTS workspaces CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', metadata JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, parent_id TEXT REFERENCES conversations(id) ON DELETE SET NULL, title TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, role TEXT NOT NULL, content TEXT NOT NULL, metadata JSONB NOT NULL DEFAULT '{}', bookmarked BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE message_shares (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE, url TEXT NOT NULL, expires_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE (message_id))`,
		`CREATE TABLE conversation_shares (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, start_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL, end_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL, url TEXT NOT NULL, expires_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE knowledge_bases (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, name TEXT NOT NULL, document_count INTEGER NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE conversation_knowledge_bindings (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE, knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE (conversation_id, knowledge_base_id))`,
		`CREATE TABLE conversation_configs (conversation_id TEXT PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, model_id TEXT NOT NULL DEFAULT 'demo-reply', persona_id TEXT NOT NULL DEFAULT '', system_prompt_override TEXT NOT NULL DEFAULT '', temperature DOUBLE PRECISION NOT NULL DEFAULT 1, max_output_tokens INTEGER NOT NULL DEFAULT 1024, tools_enabled BOOLEAN NOT NULL DEFAULT FALSE, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE personas (id TEXT PRIMARY KEY, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, name TEXT NOT NULL DEFAULT '', role TEXT NOT NULL DEFAULT '', style TEXT NOT NULL DEFAULT '', tone TEXT NOT NULL DEFAULT '', constraints TEXT NOT NULL DEFAULT '', opening_message TEXT NOT NULL DEFAULT '', suggested_questions JSONB NOT NULL DEFAULT '[]'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`INSERT INTO organizations (id, slug, name) VALUES ('org_1', 'org-1', 'Org 1'), ('org_2', 'org-2', 'Org 2')`,
		`INSERT INTO workspaces (id, organization_id, name) VALUES ('workspace_1', 'org_1', 'Workspace 1'), ('workspace_2', 'org_2', 'Workspace 2')`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare database: %v", err)
		}
	}

	return database
}

func TestSQLStoreMessageShareExpiresAndReadsPublicPayload(t *testing.T) {
	database := chatTestDatabase(t)
	store := NewSQLStore(database)
	ctx := context.Background()

	conversation, err := store.CreateConversation(ctx, "workspace_1", "org_1", "Share source", "demo-reply")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message, err := store.CreateMessage(ctx, conversation.ID, "org_1", "assistant", "Shareable answer")
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	expiresAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	share, err := store.CreateMessageShareWithOptions(ctx, conversation.ID, "org_1", message.ID, &expiresAt)
	if err != nil {
		t.Fatalf("create message share: %v", err)
	}
	if share.ExpiresAt == nil || !share.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected share expiration %s, got %+v", expiresAt, share)
	}

	detail, err := store.GetMessageShare(ctx, share.ID, expiresAt.Add(-time.Minute))
	if err != nil {
		t.Fatalf("get message share before expiration: %v", err)
	}
	if detail.ConversationID != conversation.ID || detail.Message.Content != "Shareable answer" {
		t.Fatalf("unexpected share detail: %+v", detail)
	}
	if detail.ExpiresAt == nil || !detail.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected detail expiration %s, got %+v", expiresAt, detail.ExpiresAt)
	}

	if _, err := store.GetMessageShare(ctx, share.ID, expiresAt.Add(time.Second)); !errors.Is(err, ErrMessageShareExpired) {
		t.Fatalf("expected expired share error after expiration, got %v", err)
	}
}

func TestSQLStoreConversationShareReturnsRequestedMessageRange(t *testing.T) {
	database := chatTestDatabase(t)
	store := NewSQLStore(database)
	ctx := context.Background()

	conversation, err := store.CreateConversation(ctx, "workspace_1", "org_1", "Share source", "demo-reply")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	first, err := store.CreateMessage(ctx, conversation.ID, "org_1", "user", "First message")
	if err != nil {
		t.Fatalf("create first message: %v", err)
	}
	second, err := store.CreateMessage(ctx, conversation.ID, "org_1", "assistant", "Second message")
	if err != nil {
		t.Fatalf("create second message: %v", err)
	}
	third, err := store.CreateMessage(ctx, conversation.ID, "org_1", "user", "Third message")
	if err != nil {
		t.Fatalf("create third message: %v", err)
	}
	expiresAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	share, err := store.CreateConversationShare(ctx, conversation.ID, "org_1", ConversationShareStoreOptions{
		StartMessageID: second.ID,
		EndMessageID:   third.ID,
		ExpiresAt:      &expiresAt,
	})
	if err != nil {
		t.Fatalf("create conversation share: %v", err)
	}
	if share.ExpiresAt == nil || !share.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected share expiration %s, got %+v", expiresAt, share)
	}

	detail, err := store.GetConversationShare(ctx, share.ID, expiresAt.Add(-time.Minute))
	if err != nil {
		t.Fatalf("get conversation share before expiration: %v", err)
	}
	if detail.ConversationID != conversation.ID || detail.Conversation.Title != "Share source" {
		t.Fatalf("unexpected conversation share detail: %+v", detail)
	}
	if detail.StartMessageID != second.ID || detail.EndMessageID != third.ID {
		t.Fatalf("unexpected share boundaries: start=%s end=%s", detail.StartMessageID, detail.EndMessageID)
	}
	if len(detail.Messages) != 2 || detail.Messages[0].ID != second.ID || detail.Messages[1].ID != third.ID {
		t.Fatalf("expected second and third messages only, got %+v; first=%s", detail.Messages, first.ID)
	}

	if _, err := store.GetConversationShare(ctx, share.ID, expiresAt.Add(time.Second)); !errors.Is(err, ErrMessageShareExpired) {
		t.Fatalf("expected expired share error after expiration, got %v", err)
	}
}

func TestSQLStoreForkConversationCopiesScopedConversationData(t *testing.T) {
	database := chatTestDatabase(t)
	store := NewSQLStore(database)
	ctx := context.Background()

	source, err := store.CreateConversation(ctx, "workspace_1", "org_1", "Source conversation", "demo-reply")
	if err != nil {
		t.Fatalf("create source conversation: %v", err)
	}
	if _, err := store.CreateMessage(ctx, source.ID, "org_1", "user", "First message"); err != nil {
		t.Fatalf("create first message: %v", err)
	}
	if _, err := store.CreateMessage(ctx, source.ID, "org_1", "assistant", "Second message"); err != nil {
		t.Fatalf("create second message: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO knowledge_bases (id, workspace_id, organization_id, name)
		VALUES ('kb_1', 'workspace_1', 'org_1', 'Knowledge 1')
	`); err != nil {
		t.Fatalf("insert knowledge base: %v", err)
	}
	if _, err := store.UpdateConversationConfig(ctx, source.ID, "org_1", "quality-chat", "Use docs", 0.4, 2048, true, []string{"kb_1"}, "persona_1"); err != nil {
		t.Fatalf("update source config: %v", err)
	}

	fork, err := store.ForkConversation(ctx, "org_1", "workspace_1", source.ID, "", "")
	if err != nil {
		t.Fatalf("fork conversation: %v", err)
	}

	if fork.ID == source.ID {
		t.Fatalf("expected fork to have a new id, got source id %s", fork.ID)
	}
	if fork.Title != "Source conversation (fork)" {
		t.Fatalf("expected default fork title, got %q", fork.Title)
	}
	if fork.ParentID != source.ID {
		t.Fatalf("expected fork parent id %s, got %q", source.ID, fork.ParentID)
	}

	persistedFork, err := store.GetConversation(ctx, fork.ID, "org_1")
	if err != nil {
		t.Fatalf("get fork conversation: %v", err)
	}
	if persistedFork.ParentID != source.ID {
		t.Fatalf("expected persisted fork parent id %s, got %q", source.ID, persistedFork.ParentID)
	}

	conversations, err := store.ListConversations(ctx, "org_1")
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	parentIDByConversationID := map[string]string{}
	for _, conversation := range conversations {
		parentIDByConversationID[conversation.ID] = conversation.ParentID
	}
	if parentIDByConversationID[fork.ID] != source.ID {
		t.Fatalf("expected listed fork parent id %s, got %q", source.ID, parentIDByConversationID[fork.ID])
	}

	messages, err := store.ListMessages(ctx, fork.ID, "org_1")
	if err != nil {
		t.Fatalf("list fork messages: %v", err)
	}
	if len(messages) != 2 || messages[0].Role != "user" || messages[0].Content != "First message" || messages[1].Role != "assistant" || messages[1].Content != "Second message" {
		t.Fatalf("expected copied messages in order, got %+v", messages)
	}

	config, err := store.GetConversationConfig(ctx, fork.ID, "org_1", "demo-reply")
	if err != nil {
		t.Fatalf("get fork config: %v", err)
	}
	if config.ModelID != "quality-chat" || config.SystemPromptOverride != "Use docs" || config.Temperature != 0.4 || config.MaxOutputTokens != 2048 || !config.ToolsEnabled {
		t.Fatalf("expected copied config, got %+v", config)
	}
	if config.PersonaID != "persona_1" {
		t.Fatalf("expected copied persona id persona_1, got %+v", config)
	}
	if len(config.KnowledgeBaseIDs) != 1 || config.KnowledgeBaseIDs[0] != "kb_1" {
		t.Fatalf("expected copied knowledge binding, got %+v", config.KnowledgeBaseIDs)
	}
}

func TestSQLStoreConversationConfigPersistsPersonaID(t *testing.T) {
	database := chatTestDatabase(t)
	store := NewSQLStore(database)
	ctx := context.Background()

	conversation, err := store.CreateConversation(ctx, "workspace_1", "org_1", "Persona thread", "demo-reply")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO personas (id, organization_id, role, style, tone, constraints)
		VALUES ('persona_1', 'org_1', 'Product coach', 'Socratic', 'Calm and direct', 'Ask one question at a time.')
	`); err != nil {
		t.Fatalf("insert persona: %v", err)
	}

	updated, err := store.UpdateConversationConfig(ctx, conversation.ID, "org_1", "quality-chat", "Use docs", 0.4, 2048, true, nil, "persona_1")
	if err != nil {
		t.Fatalf("update config: %v", err)
	}
	if updated.PersonaID != "persona_1" {
		t.Fatalf("expected updated persona id persona_1, got %+v", updated)
	}

	config, err := store.GetConversationConfig(ctx, conversation.ID, "org_1", "demo-reply")
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if config.PersonaID != "persona_1" {
		t.Fatalf("expected stored persona id persona_1, got %+v", config)
	}
	if config.PersonaRole != "Product coach" || config.PersonaStyle != "Socratic" || config.PersonaTone != "Calm and direct" || config.PersonaConstraints != "Ask one question at a time." {
		t.Fatalf("expected stored persona prompt fields, got %+v", config)
	}
}

func TestSQLStoreCreateAndListMessagePreservesAttachments(t *testing.T) {
	database := chatTestDatabase(t)
	store := NewSQLStore(database)
	ctx := context.Background()

	conversation, err := store.CreateConversation(ctx, "workspace_1", "org_1", "Multimodal thread", "demo-reply")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	attachments := []MessageAttachment{
		{
			ID:          "attachment_1",
			Name:        "diagram.png",
			ContentType: "image/png",
			SizeBytes:   48123,
			Type:        "image",
			URL:         "https://cdn.example.com/diagram.png",
		},
		{
			ID:             "attachment_2",
			Name:           "brief.pdf",
			ContentType:    "application/pdf",
			ProviderFileID: "file_provider_1",
			SizeBytes:      128000,
			Type:           "file",
		},
	}

	created, err := store.CreateMessageWithAttachments(ctx, conversation.ID, "org_1", "user", "Use these files", attachments)
	if err != nil {
		t.Fatalf("create message with attachments: %v", err)
	}
	if !reflect.DeepEqual(created.Attachments, attachments) {
		t.Fatalf("expected created message attachments to round-trip, got %+v", created.Attachments)
	}

	messages, err := store.ListMessages(ctx, conversation.ID, "org_1")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %+v", messages)
	}
	if !reflect.DeepEqual(messages[0].Attachments, attachments) {
		t.Fatalf("expected listed attachments to round-trip, got %+v", messages[0].Attachments)
	}
}

func TestSQLStoreCreateAndListMessagePreservesKnowledgeCitations(t *testing.T) {
	database := chatTestDatabase(t)
	store := NewSQLStore(database)
	ctx := context.Background()

	conversation, err := store.CreateConversation(ctx, "workspace_1", "org_1", "Cited answer thread", "demo-reply")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	citations := []KnowledgeCitation{
		{
			ChunkID:            "chunk_7",
			ChunkIndex:         3,
			DocumentID:         "doc_9",
			DocumentTitle:      "Runbook.md",
			DocumentVersion:    "v4",
			HighlightPositions: []KnowledgeHighlightPosition{{Start: 0, End: 8}},
			KnowledgeBaseID:    "kb_2",
			KnowledgeBaseName:  "Operations",
			OriginalText:       "Rollback requires a staged deploy and incident owner.",
			PageNumber:         15,
			Score:              0.91,
			Snippet:            "Rollback requires a staged deploy.",
			SourceURL:          "https://docs.example.com/runbook",
		},
	}

	created, err := store.CreateMessageWithOptions(ctx, conversation.ID, "org_1", "assistant", "Use the runbook.", CreateMessageOptions{
		KnowledgeCitations: citations,
	})
	if err != nil {
		t.Fatalf("create message with knowledge citations: %v", err)
	}
	if !reflect.DeepEqual(created.KnowledgeCitations, citations) {
		t.Fatalf("expected created citations to round-trip, got %+v", created.KnowledgeCitations)
	}

	messages, err := store.ListMessages(ctx, conversation.ID, "org_1")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %+v", messages)
	}
	if !reflect.DeepEqual(messages[0].KnowledgeCitations, citations) {
		t.Fatalf("expected listed citations to round-trip, got %+v", messages[0].KnowledgeCitations)
	}
}

func TestSQLStoreListPersonasScopesOrganizationAndOrdersByName(t *testing.T) {
	database := chatTestDatabase(t)
	store := NewSQLStore(database)
	ctx := context.Background()

	if _, err := database.Exec(`
		INSERT INTO personas (id, organization_id, name, role, style, tone, constraints)
		VALUES
			('persona_b', 'org_1', 'Research coach', 'Research coach', 'Socratic', 'Calm', 'Ask focused questions.'),
			('persona_a', 'org_1', 'Launch reviewer', 'Launch reviewer', 'Direct', 'Precise', 'Call out rollout risk.'),
			('persona_other', 'org_2', 'Other org persona', 'Other', 'Hidden', 'Hidden', 'Hidden')
	`); err != nil {
		t.Fatalf("insert personas: %v", err)
	}

	personas, err := store.ListPersonas(ctx, "org_1")
	if err != nil {
		t.Fatalf("list personas: %v", err)
	}

	if len(personas) != 2 {
		t.Fatalf("expected 2 scoped personas, got %+v", personas)
	}
	if personas[0].ID != "persona_a" || personas[0].Name != "Launch reviewer" {
		t.Fatalf("expected first persona ordered by name, got %+v", personas[0])
	}
	if personas[1].ID != "persona_b" || personas[1].Role != "Research coach" || personas[1].Constraints != "Ask focused questions." {
		t.Fatalf("expected second persona details, got %+v", personas[1])
	}
}

func TestSQLStoreForkConversationCopiesMessagesThroughBoundary(t *testing.T) {
	database := chatTestDatabase(t)
	store := NewSQLStore(database)
	ctx := context.Background()

	source, err := store.CreateConversation(ctx, "workspace_1", "org_1", "Source conversation", "demo-reply")
	if err != nil {
		t.Fatalf("create source conversation: %v", err)
	}
	first, err := store.CreateMessage(ctx, source.ID, "org_1", "user", "First message")
	if err != nil {
		t.Fatalf("create first message: %v", err)
	}
	second, err := store.CreateMessage(ctx, source.ID, "org_1", "assistant", "Second message")
	if err != nil {
		t.Fatalf("create second message: %v", err)
	}
	if _, err := store.CreateMessage(ctx, source.ID, "org_1", "user", "Third message"); err != nil {
		t.Fatalf("create third message: %v", err)
	}

	fork, err := store.ForkConversation(ctx, "org_1", "workspace_1", source.ID, "Branch after answer", second.ID)
	if err != nil {
		t.Fatalf("fork conversation through message: %v", err)
	}

	messages, err := store.ListMessages(ctx, fork.ID, "org_1")
	if err != nil {
		t.Fatalf("list fork messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected fork to copy 2 messages through boundary, got %+v", messages)
	}
	if messages[0].Content != first.Content || messages[1].Content != second.Content {
		t.Fatalf("expected fork to copy through second message, got %+v", messages)
	}
}

func TestSQLStoreForkConversationCopiesMessageAttachments(t *testing.T) {
	database := chatTestDatabase(t)
	store := NewSQLStore(database)
	ctx := context.Background()

	source, err := store.CreateConversation(ctx, "workspace_1", "org_1", "Source conversation", "demo-reply")
	if err != nil {
		t.Fatalf("create source conversation: %v", err)
	}
	attachments := []MessageAttachment{{
		ID:          "attachment_1",
		Name:        "whiteboard.png",
		ContentType: "image/png",
		SizeBytes:   9001,
		Type:        "image",
	}}
	if _, err := store.CreateMessageWithAttachments(ctx, source.ID, "org_1", "user", "Review this sketch", attachments); err != nil {
		t.Fatalf("create message with attachments: %v", err)
	}

	fork, err := store.ForkConversation(ctx, "org_1", "workspace_1", source.ID, "Branch with attachments", "")
	if err != nil {
		t.Fatalf("fork conversation: %v", err)
	}

	messages, err := store.ListMessages(ctx, fork.ID, "org_1")
	if err != nil {
		t.Fatalf("list fork messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected fork to copy 1 message, got %+v", messages)
	}
	if !reflect.DeepEqual(messages[0].Attachments, attachments) {
		t.Fatalf("expected fork to copy attachments, got %+v", messages[0].Attachments)
	}
}

func TestSQLStoreListConversationsMarksThreadsWithBookmarkedMessages(t *testing.T) {
	database := chatTestDatabase(t)
	store := NewSQLStore(database)
	ctx := context.Background()

	bookmarkedConversation, err := store.CreateConversation(ctx, "workspace_1", "org_1", "Bookmarked research", "demo-reply")
	if err != nil {
		t.Fatalf("create bookmarked conversation: %v", err)
	}
	plainConversation, err := store.CreateConversation(ctx, "workspace_1", "org_1", "Plain research", "demo-reply")
	if err != nil {
		t.Fatalf("create plain conversation: %v", err)
	}
	message, err := store.CreateMessage(ctx, bookmarkedConversation.ID, "org_1", "assistant", "Important answer")
	if err != nil {
		t.Fatalf("create bookmarked message: %v", err)
	}
	if _, err := store.BookmarkMessage(ctx, bookmarkedConversation.ID, "org_1", message.ID, true); err != nil {
		t.Fatalf("bookmark message: %v", err)
	}
	if _, err := store.CreateMessage(ctx, plainConversation.ID, "org_1", "assistant", "Ordinary answer"); err != nil {
		t.Fatalf("create plain message: %v", err)
	}

	conversations, err := store.ListConversations(ctx, "org_1")
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}

	hasBookmarkedByID := map[string]bool{}
	for _, conversation := range conversations {
		hasBookmarkedByID[conversation.ID] = conversation.HasBookmarkedMessages
	}
	if !hasBookmarkedByID[bookmarkedConversation.ID] {
		t.Fatalf("expected bookmarked conversation to be marked, got %+v", conversations)
	}
	if hasBookmarkedByID[plainConversation.ID] {
		t.Fatalf("expected plain conversation not to be marked, got %+v", conversations)
	}
}
