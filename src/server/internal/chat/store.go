package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"oblivious/server/internal/auth"
)

type sqlQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type sqlStoreExecutor interface {
	sqlQueryer
	sqlExecer
}

type conversationDetails struct {
	CreatedAt      time.Time
	ID             string
	OrganizationID string
	ParentID       string
	Title          string
	UpdatedAt      time.Time
	WorkspaceID    string
}

func (s *SQLStore) CreateConversation(ctx context.Context, workspaceID string, args ...string) (Conversation, error) {
	organizationID, title, defaultModelID := parseCreateConversationArgs(args)
	if strings.TrimSpace(title) == "" {
		title = "New conversation"
	}
	if strings.TrimSpace(defaultModelID) == "" {
		defaultModelID = "demo-reply"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Conversation{}, err
	}
	defer tx.Rollback()

	organizationID, err = resolveOrganizationForWorkspace(ctx, tx, workspaceID, organizationID)
	if err != nil {
		return Conversation{}, err
	}

	conversationID, err := auth.NewID("conversation")
	if err != nil {
		return Conversation{}, err
	}
	createdAt := time.Now().UTC()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO conversations (id, workspace_id, organization_id, title, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, conversationID, workspaceID, organizationID, title, createdAt, createdAt); err != nil {
		return Conversation{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO conversation_configs (
			conversation_id,
			organization_id,
			model_id,
			persona_id,
			system_prompt_override,
			temperature,
			max_output_tokens,
			tools_enabled,
			updated_at
		)
		VALUES ($1, $2, $3, '', '', 1, 1024, FALSE, $4)
	`, conversationID, organizationID, defaultModelID, createdAt); err != nil {
		return Conversation{}, err
	}
	if err := tx.Commit(); err != nil {
		return Conversation{}, err
	}

	return Conversation{
		CreatedAt: createdAt,
		ID:        conversationID,
		Title:     title,
		UpdatedAt: createdAt,
	}, nil
}

func parseCreateConversationArgs(args []string) (organizationID, title, defaultModelID string) {
	switch {
	case len(args) >= 3:
		return args[0], args[1], args[2]
	case len(args) == 2:
		return "", args[0], args[1]
	case len(args) == 1:
		return "", args[0], "demo-reply"
	default:
		return "", "New conversation", "demo-reply"
	}
}

func (s *SQLStore) CreateMessage(ctx context.Context, conversationID string, args ...string) (Message, error) {
	organizationID, role, content := parseCreateMessageArgs(args)
	return s.CreateMessageWithOptions(ctx, conversationID, organizationID, role, content, CreateMessageOptions{})
}

func parseCreateMessageArgs(args []string) (organizationID, role, content string) {
	if len(args) >= 3 {
		return args[0], args[1], args[2]
	}
	if len(args) >= 2 {
		return "", args[0], args[1]
	}
	return "", "", ""
}

func (s *SQLStore) CreateMessageWithAttachments(ctx context.Context, conversationID, organizationID, role, content string, attachments []MessageAttachment) (Message, error) {
	return s.CreateMessageWithOptions(ctx, conversationID, organizationID, role, content, CreateMessageOptions{
		Attachments: attachments,
	})
}

func (s *SQLStore) CreateMessageWithOptions(ctx context.Context, conversationID, organizationID, role, content string, options CreateMessageOptions) (Message, error) {
	details, err := s.getConversationDetailsForMessage(ctx, conversationID, organizationID)
	if err != nil {
		return Message{}, err
	}
	organizationID = details.OrganizationID

	messageID, err := auth.NewID("message")
	if err != nil {
		return Message{}, err
	}
	createdAt := time.Now().UTC()
	metadata := MessageMetadata{
		Attachments:        cloneAttachments(options.Attachments),
		KnowledgeCitations: cloneKnowledgeCitations(options.KnowledgeCitations),
	}
	metadataJSON, err := marshalMessageMetadata(metadata)
	if err != nil {
		return Message{}, err
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO messages (id, conversation_id, organization_id, role, content, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, messageID, conversationID, organizationID, role, content, metadataJSON, createdAt); err != nil {
		return Message{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE conversations SET updated_at = $2 WHERE id = $1 AND organization_id = $3
	`, conversationID, createdAt, organizationID); err != nil {
		return Message{}, err
	}

	message := Message{
		Attachments:        cloneAttachments(options.Attachments),
		Content:            content,
		CreatedAt:          createdAt,
		ID:                 messageID,
		KnowledgeCitations: cloneKnowledgeCitations(options.KnowledgeCitations),
		Role:               role,
	}
	if messageMetadataHasValues(metadata) {
		message.Metadata = &metadata
	}
	return message, nil
}

func (s *SQLStore) GetConversation(ctx context.Context, conversationID, scopeID string) (Conversation, error) {
	details, err := s.getConversationDetailsByScope(ctx, s.db, conversationID, scopeID)
	if err != nil {
		return Conversation{}, err
	}
	return conversationFromDetails(details, false), nil
}

func (s *SQLStore) UpdateConversation(ctx context.Context, conversationID, scopeID, title string) (Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New conversation"
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE conversations
		SET title = $3, updated_at = $4
		WHERE id = $1
			AND (organization_id = $2 OR workspace_id = $2)
	`, conversationID, scopeID, title, time.Now().UTC())
	if err != nil {
		return Conversation{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Conversation{}, err
	}
	if rowsAffected == 0 {
		return Conversation{}, sql.ErrNoRows
	}
	return s.GetConversation(ctx, conversationID, scopeID)
}

func (s *SQLStore) DeleteConversation(ctx context.Context, conversationID, scopeID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM conversations
		WHERE id = $1
			AND (organization_id = $2 OR workspace_id = $2)
	`, conversationID, scopeID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *SQLStore) GetConversationConfig(ctx context.Context, conversationID, scopeID, defaultModelID string) (ConversationConfig, error) {
	if strings.TrimSpace(defaultModelID) == "" {
		defaultModelID = "demo-reply"
	}

	config := ConversationConfig{
		ConversationID:       conversationID,
		KnowledgeBaseIDs:     []string{},
		ModelID:              defaultModelID,
		SystemPromptOverride: "",
		Temperature:          1,
		MaxOutputTokens:      1024,
		ToolsEnabled:         false,
	}

	var personaRole, personaStyle, personaTone, personaConstraints sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			cc.model_id,
			cc.system_prompt_override,
			cc.temperature,
			cc.max_output_tokens,
			cc.tools_enabled,
			COALESCE(cc.persona_id, ''),
			p.role,
			p.style,
			p.tone,
			p.constraints,
			cc.updated_at
		FROM conversation_configs cc
		JOIN conversations c ON c.id = cc.conversation_id
		LEFT JOIN personas p ON p.id = cc.persona_id AND p.organization_id = cc.organization_id
		WHERE cc.conversation_id = $1
			AND (c.organization_id = $2 OR c.workspace_id = $2)
	`, conversationID, scopeID).Scan(
		&config.ModelID,
		&config.SystemPromptOverride,
		&config.Temperature,
		&config.MaxOutputTokens,
		&config.ToolsEnabled,
		&config.PersonaID,
		&personaRole,
		&personaStyle,
		&personaTone,
		&personaConstraints,
		&config.UpdatedAt,
	); err != nil {
		if err != sql.ErrNoRows {
			return ConversationConfig{}, err
		}
		details, detailsErr := s.getConversationDetailsByScope(ctx, s.db, conversationID, scopeID)
		if detailsErr != nil {
			return ConversationConfig{}, detailsErr
		}
		now := time.Now().UTC()
		if _, insertErr := s.db.ExecContext(ctx, `
			INSERT INTO conversation_configs (
				conversation_id,
				organization_id,
				model_id,
				persona_id,
				system_prompt_override,
				temperature,
				max_output_tokens,
				tools_enabled,
				updated_at
			)
			VALUES ($1, $2, $3, '', '', 1, 1024, FALSE, $4)
			ON CONFLICT (conversation_id) DO NOTHING
		`, conversationID, details.OrganizationID, defaultModelID, now); insertErr != nil {
			return ConversationConfig{}, insertErr
		}
		config.UpdatedAt = now
	} else {
		config.PersonaRole = personaRole.String
		config.PersonaStyle = personaStyle.String
		config.PersonaTone = personaTone.String
		config.PersonaConstraints = personaConstraints.String
	}

	knowledgeBaseIDs, err := s.listConversationKnowledgeBaseIDs(ctx, conversationID, scopeID)
	if err != nil {
		return ConversationConfig{}, err
	}
	config.KnowledgeBaseIDs = knowledgeBaseIDs
	return config, nil
}

func (s *SQLStore) ListConversations(ctx context.Context, scopeID string) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			c.id,
			c.title,
			COALESCE(c.parent_id, ''),
			c.created_at,
			c.updated_at,
			EXISTS (
				SELECT 1
				FROM messages m
				WHERE m.conversation_id = c.id AND m.bookmarked = TRUE
			) AS has_bookmarked_messages
		FROM conversations c
		WHERE c.organization_id = $1 OR c.workspace_id = $1
		ORDER BY c.updated_at DESC, c.created_at DESC
	`, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conversations := []Conversation{}
	for rows.Next() {
		var conversation Conversation
		if err := rows.Scan(
			&conversation.ID,
			&conversation.Title,
			&conversation.ParentID,
			&conversation.CreatedAt,
			&conversation.UpdatedAt,
			&conversation.HasBookmarkedMessages,
		); err != nil {
			return nil, err
		}
		conversations = append(conversations, conversation)
	}
	return conversations, rows.Err()
}

func (s *SQLStore) ListMessages(ctx context.Context, conversationID, scopeID string) ([]Message, error) {
	if _, err := s.getConversationDetailsByScope(ctx, s.db, conversationID, scopeID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.role, m.content, m.metadata, m.bookmarked, m.created_at
		FROM messages m
		WHERE m.conversation_id = $1
		ORDER BY m.created_at ASC, m.id ASC
	`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessages(rows)
}

func (s *SQLStore) UpdateConversationConfig(
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
) (ConversationConfig, error) {
	if strings.TrimSpace(modelID) == "" {
		modelID = "demo-reply"
	}
	personaID := ""
	if len(personaIDs) > 0 {
		personaID = strings.TrimSpace(personaIDs[0])
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ConversationConfig{}, err
	}
	defer tx.Rollback()

	details, err := s.getConversationDetailsByScope(ctx, tx, conversationID, scopeID)
	if err != nil {
		return ConversationConfig{}, err
	}
	updatedAt := time.Now().UTC()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO conversation_configs (
			conversation_id,
			organization_id,
			model_id,
			persona_id,
			system_prompt_override,
			temperature,
			max_output_tokens,
			tools_enabled,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (conversation_id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			model_id = EXCLUDED.model_id,
			persona_id = EXCLUDED.persona_id,
			system_prompt_override = EXCLUDED.system_prompt_override,
			temperature = EXCLUDED.temperature,
			max_output_tokens = EXCLUDED.max_output_tokens,
			tools_enabled = EXCLUDED.tools_enabled,
			updated_at = EXCLUDED.updated_at
	`, conversationID, details.OrganizationID, modelID, personaID, systemPromptOverride, temperature, maxOutputTokens, toolsEnabled, updatedAt); err != nil {
		return ConversationConfig{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM conversation_knowledge_bindings
		WHERE conversation_id = $1 AND organization_id = $2
	`, conversationID, details.OrganizationID); err != nil {
		return ConversationConfig{}, err
	}

	for _, knowledgeBaseID := range knowledgeBaseIDs {
		bindingID, err := auth.NewID("ckb")
		if err != nil {
			return ConversationConfig{}, err
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO conversation_knowledge_bindings (
				id,
				conversation_id,
				knowledge_base_id,
				organization_id,
				created_at
			)
			SELECT $1, $2, kb.id, $4, $5
			FROM knowledge_bases kb
			WHERE kb.id = $3
				AND (
					($4 <> '' AND kb.organization_id = $4)
					OR ($4 = '' AND kb.workspace_id = $6)
				)
			ON CONFLICT (conversation_id, knowledge_base_id) DO NOTHING
		`, bindingID, conversationID, knowledgeBaseID, details.OrganizationID, updatedAt, details.WorkspaceID)
		if err != nil {
			return ConversationConfig{}, err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return ConversationConfig{}, err
		}
		if rowsAffected == 0 {
			return ConversationConfig{}, sql.ErrNoRows
		}
	}

	if err := tx.Commit(); err != nil {
		return ConversationConfig{}, err
	}

	return ConversationConfig{
		ConversationID:       conversationID,
		KnowledgeBaseIDs:     append([]string(nil), knowledgeBaseIDs...),
		ModelID:              modelID,
		PersonaID:            personaID,
		SystemPromptOverride: systemPromptOverride,
		Temperature:          temperature,
		MaxOutputTokens:      maxOutputTokens,
		ToolsEnabled:         toolsEnabled,
		UpdatedAt:            updatedAt,
	}, nil
}

func (s *SQLStore) ForkConversation(ctx context.Context, organizationID, workspaceID, sourceConversationID, title, branchFromMessageID string) (Conversation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Conversation{}, err
	}
	defer tx.Rollback()

	scopeID := strings.TrimSpace(organizationID)
	if scopeID == "" {
		scopeID = workspaceID
	}
	source, err := s.getConversationDetailsByScope(ctx, tx, sourceConversationID, scopeID)
	if err != nil {
		return Conversation{}, err
	}
	if strings.TrimSpace(workspaceID) != "" && source.WorkspaceID != workspaceID {
		return Conversation{}, sql.ErrNoRows
	}
	if strings.TrimSpace(title) == "" {
		title = source.Title + " (fork)"
	}

	newConversationID, err := auth.NewID("conversation")
	if err != nil {
		return Conversation{}, err
	}
	createdAt := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO conversations (id, workspace_id, organization_id, parent_id, title, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, newConversationID, source.WorkspaceID, source.OrganizationID, source.ID, title, createdAt, createdAt); err != nil {
		return Conversation{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO conversation_configs (
			conversation_id,
			organization_id,
			model_id,
			persona_id,
			system_prompt_override,
			temperature,
			max_output_tokens,
			tools_enabled,
			updated_at
		)
		SELECT
			$1,
			cc.organization_id,
			cc.model_id,
			COALESCE(cc.persona_id, ''),
			cc.system_prompt_override,
			cc.temperature,
			cc.max_output_tokens,
			cc.tools_enabled,
			$3
		FROM conversation_configs cc
		WHERE cc.conversation_id = $2 AND cc.organization_id = $4
	`, newConversationID, source.ID, createdAt, source.OrganizationID); err != nil {
		return Conversation{}, err
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT m.id, m.role, m.content, m.metadata, m.bookmarked, m.created_at
		FROM messages m
		WHERE m.conversation_id = $1
			AND m.organization_id = $2
			AND (
				$3 = ''
				OR m.created_at <= (
					SELECT boundary.created_at
					FROM messages boundary
					WHERE boundary.id = $3
						AND boundary.conversation_id = $1
						AND boundary.organization_id = $2
				)
			)
		ORDER BY m.created_at ASC, m.id ASC
	`, source.ID, source.OrganizationID, branchFromMessageID)
	if err != nil {
		return Conversation{}, err
	}
	sourceMessages, err := scanMessages(rows)
	rows.Close()
	if err != nil {
		return Conversation{}, err
	}
	for _, sourceMessage := range sourceMessages {
		newMessageID, err := auth.NewID("message")
		if err != nil {
			return Conversation{}, err
		}
		metadataJSON, err := marshalMessageMetadataFromMessage(sourceMessage)
		if err != nil {
			return Conversation{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO messages (
				id,
				conversation_id,
				organization_id,
				role,
				content,
				metadata,
				bookmarked,
				created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, newMessageID, newConversationID, source.OrganizationID, sourceMessage.Role, sourceMessage.Content, metadataJSON, sourceMessage.Bookmarked, sourceMessage.CreatedAt); err != nil {
			return Conversation{}, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO conversation_knowledge_bindings (
			id,
			conversation_id,
			knowledge_base_id,
			organization_id,
			created_at
		)
		SELECT $1 || '_' || ckb.id, $2, ckb.knowledge_base_id, ckb.organization_id, $4
		FROM conversation_knowledge_bindings ckb
		WHERE ckb.conversation_id = $3 AND ckb.organization_id = $5
		ON CONFLICT (conversation_id, knowledge_base_id) DO NOTHING
	`, newConversationID, newConversationID, source.ID, createdAt, source.OrganizationID); err != nil {
		return Conversation{}, err
	}

	if err := tx.Commit(); err != nil {
		return Conversation{}, err
	}

	return Conversation{
		CreatedAt: createdAt,
		ID:        newConversationID,
		ParentID:  source.ID,
		Title:     title,
		UpdatedAt: createdAt,
	}, nil
}

func (s *SQLStore) ListConversationBranches(ctx context.Context, conversationID, scopeID string) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workspace_id, organization_id, title, COALESCE(parent_id, ''), created_at, updated_at
		FROM conversations
		WHERE parent_id = $1
			AND (organization_id = $2 OR workspace_id = $2)
		ORDER BY created_at ASC
	`, conversationID, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	branches := []Conversation{}
	for rows.Next() {
		details, err := scanConversationDetails(rows)
		if err != nil {
			return nil, err
		}
		branches = append(branches, conversationFromDetails(details, false))
	}
	return branches, rows.Err()
}

func (s *SQLStore) CreatePersona(ctx context.Context, scopeID string, persona Persona) (Persona, error) {
	organizationID, err := s.resolveOrganizationFromScope(ctx, scopeID)
	if err != nil {
		return Persona{}, err
	}
	personaID, err := auth.NewID("persona")
	if err != nil {
		return Persona{}, err
	}
	createdAt := time.Now().UTC()
	suggestedQuestionsJSON, err := marshalStringSlice(persona.SuggestedQuestions)
	if err != nil {
		return Persona{}, err
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO personas (
			id,
			organization_id,
			name,
			role,
			style,
			tone,
			constraints,
			opening_message,
			suggested_questions,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
	`, personaID, organizationID, persona.Name, persona.Role, persona.Style, persona.Tone, persona.Constraints, persona.OpeningMessage, suggestedQuestionsJSON, createdAt); err != nil {
		return Persona{}, err
	}

	persona.ID = personaID
	persona.WorkspaceID = scopeID
	persona.CreatedAt = createdAt
	return persona, nil
}

func (s *SQLStore) GetPersona(ctx context.Context, personaID, scopeID string) (Persona, error) {
	organizationID, err := s.resolveOrganizationFromScope(ctx, scopeID)
	if err != nil {
		return Persona{}, err
	}

	var persona Persona
	var suggestedQuestionsJSON string
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, name, role, style, tone, constraints, opening_message, suggested_questions, created_at
		FROM personas
		WHERE id = $1 AND organization_id = $2
	`, personaID, organizationID).Scan(
		&persona.ID,
		&persona.Name,
		&persona.Role,
		&persona.Style,
		&persona.Tone,
		&persona.Constraints,
		&persona.OpeningMessage,
		&suggestedQuestionsJSON,
		&persona.CreatedAt,
	); err != nil {
		return Persona{}, err
	}
	persona.WorkspaceID = scopeID
	persona.SuggestedQuestions, _ = unmarshalStringSlice(suggestedQuestionsJSON)
	return persona, nil
}

func (s *SQLStore) ListPersonas(ctx context.Context, scopeID string) ([]Persona, error) {
	organizationID, err := s.resolveOrganizationFromScope(ctx, scopeID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, role, style, tone, constraints, opening_message, suggested_questions, created_at
		FROM personas
		WHERE organization_id = $1
		ORDER BY name ASC, created_at DESC
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	personas := []Persona{}
	for rows.Next() {
		var persona Persona
		var suggestedQuestionsJSON string
		if err := rows.Scan(
			&persona.ID,
			&persona.Name,
			&persona.Role,
			&persona.Style,
			&persona.Tone,
			&persona.Constraints,
			&persona.OpeningMessage,
			&suggestedQuestionsJSON,
			&persona.CreatedAt,
		); err != nil {
			return nil, err
		}
		persona.WorkspaceID = scopeID
		persona.SuggestedQuestions, _ = unmarshalStringSlice(suggestedQuestionsJSON)
		personas = append(personas, persona)
	}
	return personas, rows.Err()
}

func (s *SQLStore) UpdatePersona(ctx context.Context, personaID, scopeID string, persona Persona) (Persona, error) {
	organizationID, err := s.resolveOrganizationFromScope(ctx, scopeID)
	if err != nil {
		return Persona{}, err
	}
	suggestedQuestionsJSON, err := marshalStringSlice(persona.SuggestedQuestions)
	if err != nil {
		return Persona{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE personas
		SET name = $3,
			role = $4,
			style = $5,
			tone = $6,
			constraints = $7,
			opening_message = $8,
			suggested_questions = $9,
			updated_at = $10
		WHERE id = $1 AND organization_id = $2
	`, personaID, organizationID, persona.Name, persona.Role, persona.Style, persona.Tone, persona.Constraints, persona.OpeningMessage, suggestedQuestionsJSON, time.Now().UTC())
	if err != nil {
		return Persona{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Persona{}, err
	}
	if rowsAffected == 0 {
		return Persona{}, sql.ErrNoRows
	}
	return s.GetPersona(ctx, personaID, scopeID)
}

func (s *SQLStore) DeletePersona(ctx context.Context, personaID, scopeID string) error {
	organizationID, err := s.resolveOrganizationFromScope(ctx, scopeID)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM personas WHERE id = $1 AND organization_id = $2
	`, personaID, organizationID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *SQLStore) CreateMessageWithMetadata(ctx context.Context, conversationID, role, content string, metadata *MessageMetadata) (Message, error) {
	options := CreateMessageOptions{}
	if metadata != nil {
		options.Attachments = cloneAttachments(metadata.Attachments)
		options.KnowledgeCitations = cloneKnowledgeCitations(metadata.KnowledgeCitations)
	}
	return s.CreateMessageWithOptions(ctx, conversationID, "", role, content, options)
}

func (s *SQLStore) AddMessageAttachment(ctx context.Context, attachment MessageAttachment) (MessageAttachment, error) {
	if attachment.ID == "" {
		id, err := auth.NewID("attach")
		if err != nil {
			return MessageAttachment{}, err
		}
		attachment.ID = id
	}
	if attachment.CreatedAt.IsZero() {
		attachment.CreatedAt = time.Now().UTC()
	}

	var raw []byte
	if err := s.db.QueryRowContext(ctx, `
		SELECT metadata FROM messages WHERE id = $1
	`, attachment.MessageID).Scan(&raw); err != nil {
		return MessageAttachment{}, err
	}
	metadata, err := decodeMessageMetadata(raw)
	if err != nil {
		return MessageAttachment{}, err
	}
	metadata.Attachments = append(metadata.Attachments, attachment)
	metadataJSON, err := marshalMessageMetadata(metadata)
	if err != nil {
		return MessageAttachment{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE messages SET metadata = $2 WHERE id = $1
	`, attachment.MessageID, metadataJSON); err != nil {
		return MessageAttachment{}, err
	}
	return attachment, nil
}

func (s *SQLStore) ListMessageAttachments(ctx context.Context, messageID string) ([]MessageAttachment, error) {
	var raw []byte
	if err := s.db.QueryRowContext(ctx, `
		SELECT metadata FROM messages WHERE id = $1
	`, messageID).Scan(&raw); err != nil {
		return nil, err
	}
	metadata, err := decodeMessageMetadata(raw)
	if err != nil {
		return nil, err
	}
	return cloneAttachments(metadata.Attachments), nil
}

func (s *SQLStore) SearchMessages(ctx context.Context, scopeID, query string, limit int) ([]MessageSearchResult, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.conversation_id, m.role, m.content, m.created_at
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE (c.organization_id = $1 OR c.workspace_id = $1)
			AND m.content ILIKE '%' || $2 || '%'
		ORDER BY m.created_at DESC
		LIMIT $3
	`, scopeID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []MessageSearchResult{}
	for rows.Next() {
		var result MessageSearchResult
		if err := rows.Scan(&result.MessageID, &result.ConversationID, &result.Role, &result.Content, &result.CreatedAt); err != nil {
			return nil, err
		}
		result.Snippet = extractSnippet(result.Content, query)
		results = append(results, result)
	}
	return results, rows.Err()
}

func (s *SQLStore) CreateMessageShareWithOptions(ctx context.Context, conversationID, organizationID, messageID string, expiresAt *time.Time) (MessageShare, error) {
	shareID, err := auth.NewID("msgshare")
	if err != nil {
		return MessageShare{}, err
	}
	createdAt := time.Now().UTC()
	url := "/share/messages/" + shareID

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO message_shares (
			id,
			conversation_id,
			organization_id,
			message_id,
			url,
			expires_at,
			created_at
		)
		SELECT $1, c.id, c.organization_id, m.id, $5, $6, $7
		FROM conversations c
		JOIN messages m ON m.conversation_id = c.id
		WHERE c.id = $2
			AND c.organization_id = $3
			AND m.id = $4
			AND m.organization_id = c.organization_id
		ON CONFLICT (message_id) DO UPDATE SET
			expires_at = EXCLUDED.expires_at
	`, shareID, conversationID, organizationID, messageID, url, nullableTime(expiresAt), createdAt)
	if err != nil {
		return MessageShare{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return MessageShare{}, err
	}
	if rowsAffected == 0 {
		return MessageShare{}, sql.ErrNoRows
	}
	return MessageShare{
		ConversationID: conversationID,
		CreatedAt:      createdAt,
		ExpiresAt:      cloneTimePtr(expiresAt),
		ID:             shareID,
		MessageID:      messageID,
		OrganizationID: organizationID,
		URL:            url,
	}, nil
}

func (s *SQLStore) GetMessageShare(ctx context.Context, shareID string, now time.Time) (MessageShareDetail, error) {
	var detail MessageShareDetail
	var expiresAt sql.NullTime
	var metadataRaw []byte
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			ms.id,
			ms.conversation_id,
			ms.organization_id,
			ms.message_id,
			ms.url,
			ms.expires_at,
			ms.created_at,
			m.id,
			m.role,
			m.content,
			m.metadata,
			m.bookmarked,
			m.created_at
		FROM message_shares ms
		JOIN messages m ON m.id = ms.message_id AND m.organization_id = ms.organization_id
		WHERE ms.id = $1
	`, shareID).Scan(
		&detail.ID,
		&detail.ConversationID,
		&detail.OrganizationID,
		&detail.MessageID,
		&detail.URL,
		&expiresAt,
		&detail.CreatedAt,
		&detail.Message.ID,
		&detail.Message.Role,
		&detail.Message.Content,
		&metadataRaw,
		&detail.Message.Bookmarked,
		&detail.Message.CreatedAt,
	); err != nil {
		return MessageShareDetail{}, err
	}
	metadata, err := decodeMessageMetadata(metadataRaw)
	if err != nil {
		return MessageShareDetail{}, err
	}
	if messageMetadataHasValues(metadata) {
		detail.Message.Metadata = &metadata
		detail.Message.Attachments = cloneAttachments(metadata.Attachments)
		detail.Message.KnowledgeCitations = cloneKnowledgeCitations(metadata.KnowledgeCitations)
	}
	if expiresAt.Valid {
		expires := expiresAt.Time.UTC()
		detail.ExpiresAt = &expires
		if !now.IsZero() && now.After(expires) {
			return MessageShareDetail{}, ErrMessageShareExpired
		}
	}
	return detail, nil
}

func (s *SQLStore) CreateConversationShare(ctx context.Context, conversationID, organizationID string, options ConversationShareStoreOptions) (ConversationShare, error) {
	shareID, err := auth.NewID("convshare")
	if err != nil {
		return ConversationShare{}, err
	}
	createdAt := time.Now().UTC()
	url := "/share/conversations/" + shareID

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO conversation_shares (
			id,
			conversation_id,
			organization_id,
			start_message_id,
			end_message_id,
			url,
			expires_at,
			created_at
		)
		SELECT $1, c.id, c.organization_id, NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8
		FROM conversations c
		WHERE c.id = $2 AND c.organization_id = $3
	`, shareID, conversationID, organizationID, options.StartMessageID, options.EndMessageID, url, nullableTime(options.ExpiresAt), createdAt)
	if err != nil {
		return ConversationShare{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return ConversationShare{}, err
	}
	if rowsAffected == 0 {
		return ConversationShare{}, sql.ErrNoRows
	}

	return ConversationShare{
		ConversationID: conversationID,
		CreatedAt:      createdAt,
		EndMessageID:   options.EndMessageID,
		ExpiresAt:      cloneTimePtr(options.ExpiresAt),
		ID:             shareID,
		OrganizationID: organizationID,
		StartMessageID: options.StartMessageID,
		URL:            url,
	}, nil
}

func (s *SQLStore) GetConversationShare(ctx context.Context, shareID string, now time.Time) (ConversationShareDetail, error) {
	var detail ConversationShareDetail
	var startMessageID, endMessageID sql.NullString
	var expiresAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			cs.id,
			cs.conversation_id,
			cs.organization_id,
			cs.start_message_id,
			cs.end_message_id,
			cs.url,
			cs.expires_at,
			cs.created_at,
			c.id,
			c.title,
			COALESCE(c.parent_id, ''),
			c.created_at,
			c.updated_at
		FROM conversation_shares cs
		JOIN conversations c ON c.id = cs.conversation_id AND c.organization_id = cs.organization_id
		WHERE cs.id = $1
	`, shareID).Scan(
		&detail.ID,
		&detail.ConversationID,
		&detail.OrganizationID,
		&startMessageID,
		&endMessageID,
		&detail.URL,
		&expiresAt,
		&detail.CreatedAt,
		&detail.Conversation.ID,
		&detail.Conversation.Title,
		&detail.Conversation.ParentID,
		&detail.Conversation.CreatedAt,
		&detail.Conversation.UpdatedAt,
	); err != nil {
		return ConversationShareDetail{}, err
	}
	if startMessageID.Valid {
		detail.StartMessageID = startMessageID.String
	}
	if endMessageID.Valid {
		detail.EndMessageID = endMessageID.String
	}
	if expiresAt.Valid {
		expires := expiresAt.Time.UTC()
		detail.ExpiresAt = &expires
		if !now.IsZero() && now.After(expires) {
			return ConversationShareDetail{}, ErrMessageShareExpired
		}
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.role, m.content, m.metadata, m.bookmarked, m.created_at
		FROM messages m
		WHERE m.conversation_id = $1
			AND m.organization_id = $2
			AND (
				$3 = ''
				OR m.created_at >= (
					SELECT start_message.created_at
					FROM messages start_message
					WHERE start_message.id = $3
						AND start_message.conversation_id = $1
						AND start_message.organization_id = $2
				)
			)
			AND (
				$4 = ''
				OR m.created_at <= (
					SELECT end_message.created_at
					FROM messages end_message
					WHERE end_message.id = $4
						AND end_message.conversation_id = $1
						AND end_message.organization_id = $2
				)
			)
		ORDER BY m.created_at ASC, m.id ASC
	`, detail.ConversationID, detail.OrganizationID, detail.StartMessageID, detail.EndMessageID)
	if err != nil {
		return ConversationShareDetail{}, err
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return ConversationShareDetail{}, err
	}
	detail.Messages = messages
	return detail, nil
}

func (s *SQLStore) BookmarkMessage(ctx context.Context, conversationID, organizationID, messageID string, bookmarked bool) (Message, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE messages
		SET bookmarked = $4
		WHERE id = $3
			AND conversation_id = $1
			AND organization_id = $2
	`, conversationID, organizationID, messageID, bookmarked)
	if err != nil {
		return Message{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Message{}, err
	}
	if rowsAffected == 0 {
		return Message{}, sql.ErrNoRows
	}
	return s.getMessage(ctx, conversationID, organizationID, messageID)
}

func (s *SQLStore) UpdateMessage(ctx context.Context, conversationID, scopeID, messageID, content string) (Message, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return Message{}, errors.New("content is required")
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE messages
		SET content = $4
		WHERE id = $3
			AND conversation_id = $1
			AND organization_id = (
				SELECT organization_id
				FROM conversations
				WHERE id = $1 AND (organization_id = $2 OR workspace_id = $2)
			)
	`, conversationID, scopeID, messageID, content)
	if err != nil {
		return Message{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Message{}, err
	}
	if rowsAffected == 0 {
		return Message{}, sql.ErrNoRows
	}
	return s.getMessage(ctx, conversationID, scopeID, messageID)
}

func (s *SQLStore) DeleteMessage(ctx context.Context, conversationID, scopeID, messageID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM messages
		WHERE id = $3
			AND conversation_id = $1
			AND organization_id = (
				SELECT organization_id
				FROM conversations
				WHERE id = $1 AND (organization_id = $2 OR workspace_id = $2)
			)
	`, conversationID, scopeID, messageID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *SQLStore) getMessage(ctx context.Context, conversationID, scopeID, messageID string) (Message, error) {
	var message Message
	var metadataRaw []byte
	if err := s.db.QueryRowContext(ctx, `
		SELECT m.id, m.role, m.content, m.metadata, m.bookmarked, m.created_at
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE m.id = $1
			AND m.conversation_id = $2
			AND (c.organization_id = $3 OR c.workspace_id = $3)
	`, messageID, conversationID, scopeID).Scan(
		&message.ID,
		&message.Role,
		&message.Content,
		&metadataRaw,
		&message.Bookmarked,
		&message.CreatedAt,
	); err != nil {
		return Message{}, err
	}
	metadata, err := decodeMessageMetadata(metadataRaw)
	if err != nil {
		return Message{}, err
	}
	if messageMetadataHasValues(metadata) {
		message.Metadata = &metadata
		message.Attachments = cloneAttachments(metadata.Attachments)
		message.KnowledgeCitations = cloneKnowledgeCitations(metadata.KnowledgeCitations)
	}
	return message, nil
}

func (s *SQLStore) listConversationKnowledgeBaseIDs(ctx context.Context, conversationID, scopeID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ckb.knowledge_base_id
		FROM conversation_knowledge_bindings ckb
		JOIN conversations c ON c.id = ckb.conversation_id
		JOIN knowledge_bases kb ON kb.id = ckb.knowledge_base_id
		WHERE ckb.conversation_id = $1
			AND (c.organization_id = $2 OR c.workspace_id = $2)
			AND (kb.organization_id = c.organization_id OR kb.workspace_id = c.workspace_id)
		ORDER BY ckb.created_at ASC, ckb.knowledge_base_id ASC
	`, conversationID, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	knowledgeBaseIDs := []string{}
	for rows.Next() {
		var knowledgeBaseID string
		if err := rows.Scan(&knowledgeBaseID); err != nil {
			return nil, err
		}
		knowledgeBaseIDs = append(knowledgeBaseIDs, knowledgeBaseID)
	}
	return knowledgeBaseIDs, rows.Err()
}

func (s *SQLStore) getConversationDetailsForMessage(ctx context.Context, conversationID, organizationID string) (conversationDetails, error) {
	if strings.TrimSpace(organizationID) == "" {
		return s.getConversationDetailsByID(ctx, s.db, conversationID)
	}
	return s.getConversationDetailsByScope(ctx, s.db, conversationID, organizationID)
}

func (s *SQLStore) getConversationDetailsByID(ctx context.Context, queryer sqlQueryer, conversationID string) (conversationDetails, error) {
	var details conversationDetails
	if err := queryer.QueryRowContext(ctx, `
		SELECT id, workspace_id, organization_id, title, COALESCE(parent_id, ''), created_at, updated_at
		FROM conversations
		WHERE id = $1
	`, conversationID).Scan(
		&details.ID,
		&details.WorkspaceID,
		&details.OrganizationID,
		&details.Title,
		&details.ParentID,
		&details.CreatedAt,
		&details.UpdatedAt,
	); err != nil {
		return conversationDetails{}, err
	}
	return details, nil
}

func (s *SQLStore) getConversationDetailsByScope(ctx context.Context, queryer sqlQueryer, conversationID, scopeID string) (conversationDetails, error) {
	var details conversationDetails
	if err := queryer.QueryRowContext(ctx, `
		SELECT id, workspace_id, organization_id, title, COALESCE(parent_id, ''), created_at, updated_at
		FROM conversations
		WHERE id = $1 AND (organization_id = $2 OR workspace_id = $2)
	`, conversationID, scopeID).Scan(
		&details.ID,
		&details.WorkspaceID,
		&details.OrganizationID,
		&details.Title,
		&details.ParentID,
		&details.CreatedAt,
		&details.UpdatedAt,
	); err != nil {
		return conversationDetails{}, err
	}
	return details, nil
}

func scanConversationDetails(scanner interface{ Scan(dest ...any) error }) (conversationDetails, error) {
	var details conversationDetails
	if err := scanner.Scan(
		&details.ID,
		&details.WorkspaceID,
		&details.OrganizationID,
		&details.Title,
		&details.ParentID,
		&details.CreatedAt,
		&details.UpdatedAt,
	); err != nil {
		return conversationDetails{}, err
	}
	return details, nil
}

func conversationFromDetails(details conversationDetails, hasBookmarkedMessages bool) Conversation {
	return Conversation{
		CreatedAt:             details.CreatedAt,
		HasBookmarkedMessages: hasBookmarkedMessages,
		ID:                    details.ID,
		ParentID:              details.ParentID,
		Title:                 details.Title,
		UpdatedAt:             details.UpdatedAt,
	}
}

func resolveOrganizationForWorkspace(ctx context.Context, queryer sqlQueryer, workspaceID, providedOrganizationID string) (string, error) {
	if strings.TrimSpace(providedOrganizationID) != "" {
		return strings.TrimSpace(providedOrganizationID), nil
	}
	var organizationID string
	if err := queryer.QueryRowContext(ctx, `
		SELECT organization_id FROM workspaces WHERE id = $1
	`, workspaceID).Scan(&organizationID); err != nil {
		return "", err
	}
	return organizationID, nil
}

func (s *SQLStore) resolveOrganizationFromScope(ctx context.Context, scopeID string) (string, error) {
	var organizationID string
	if err := s.db.QueryRowContext(ctx, `
		SELECT id FROM organizations WHERE id = $1
	`, scopeID).Scan(&organizationID); err == nil {
		return organizationID, nil
	}
	return resolveOrganizationForWorkspace(ctx, s.db, scopeID, "")
}

func scanMessages(rows *sql.Rows) ([]Message, error) {
	messages := []Message{}
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func scanMessage(scanner interface{ Scan(dest ...any) error }) (Message, error) {
	var message Message
	var metadataRaw []byte
	if err := scanner.Scan(
		&message.ID,
		&message.Role,
		&message.Content,
		&metadataRaw,
		&message.Bookmarked,
		&message.CreatedAt,
	); err != nil {
		return Message{}, err
	}
	metadata, err := decodeMessageMetadata(metadataRaw)
	if err != nil {
		return Message{}, err
	}
	if messageMetadataHasValues(metadata) {
		message.Metadata = &metadata
		message.Attachments = cloneAttachments(metadata.Attachments)
		message.KnowledgeCitations = cloneKnowledgeCitations(metadata.KnowledgeCitations)
	}
	return message, nil
}

func marshalMessageMetadataFromMessage(message Message) (string, error) {
	metadata := MessageMetadata{}
	if message.Metadata != nil {
		metadata = *message.Metadata
	}
	if len(message.Attachments) > 0 {
		metadata.Attachments = cloneAttachments(message.Attachments)
	}
	if len(message.KnowledgeCitations) > 0 {
		metadata.KnowledgeCitations = cloneKnowledgeCitations(message.KnowledgeCitations)
	}
	return marshalMessageMetadata(metadata)
}

func marshalMessageMetadata(metadata MessageMetadata) (string, error) {
	if !messageMetadataHasValues(metadata) {
		return "{}", nil
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return "{}", err
	}
	return string(data), nil
}

func decodeMessageMetadata(raw []byte) (MessageMetadata, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return MessageMetadata{}, nil
	}
	var metadata MessageMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return MessageMetadata{}, err
	}
	return metadata, nil
}

func messageMetadataHasValues(metadata MessageMetadata) bool {
	return len(metadata.Attachments) > 0 || len(metadata.KnowledgeCitations) > 0
}

func cloneAttachments(attachments []MessageAttachment) []MessageAttachment {
	if len(attachments) == 0 {
		return nil
	}
	return append([]MessageAttachment(nil), attachments...)
}

func cloneKnowledgeCitations(citations []KnowledgeCitation) []KnowledgeCitation {
	if len(citations) == 0 {
		return nil
	}
	cloned := append([]KnowledgeCitation(nil), citations...)
	for i := range cloned {
		if len(cloned[i].HighlightPositions) > 0 {
			cloned[i].HighlightPositions = append([]KnowledgeHighlightPosition(nil), cloned[i].HighlightPositions...)
		}
	}
	return cloned
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func extractSnippet(content, query string) string {
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)
	idx := strings.Index(lowerContent, lowerQuery)
	if idx < 0 {
		if len(content) > 200 {
			return content[:200] + "..."
		}
		return content
	}

	start := idx - 60
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 60
	if end > len(content) {
		end = len(content)
	}

	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return snippet
}

func marshalStringSlice(items []string) (string, error) {
	if len(items) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "[]", err
	}
	return string(data), nil
}

func unmarshalStringSlice(data string) ([]string, error) {
	if data == "" || data == "null" {
		return []string{}, nil
	}
	var items []string
	if err := json.Unmarshal([]byte(data), &items); err != nil {
		return []string{}, err
	}
	return items, nil
}

func unsupportedChatStoreCall(method string, args []string) error {
	return fmt.Errorf("%s received unsupported arguments: %v", method, args)
}
