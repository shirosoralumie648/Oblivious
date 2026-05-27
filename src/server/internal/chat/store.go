package chat

import (
	"context"
	"database/sql"
	"time"

	"oblivious/server/internal/auth"
)

func (s *SQLStore) CreateConversation(ctx context.Context, workspaceID, organizationID, title, defaultModelID string) (Conversation, error) {
	conversationID, err := auth.NewID("conversation")
	if err != nil {
		return Conversation{}, err
	}

	createdAt := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO conversations (id, workspace_id, organization_id, title, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, conversationID, workspaceID, organizationID, title, createdAt); err != nil {
		return Conversation{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO conversation_configs (conversation_id, organization_id, model_id, system_prompt_override, temperature, max_output_tokens, tools_enabled, updated_at)
		VALUES ($1, $2, $3, '', 1, 1024, FALSE, $4)
	`, conversationID, organizationID, defaultModelID, createdAt); err != nil {
		return Conversation{}, err
	}

	return Conversation{
		CreatedAt: createdAt,
		ID:        conversationID,
		Title:     title,
		UpdatedAt: createdAt,
	}, nil
}

func (s *SQLStore) CreateMessage(ctx context.Context, conversationID, organizationID, role, content string) (Message, error) {
	messageID, err := auth.NewID("message")
	if err != nil {
		return Message{}, err
	}

	createdAt := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO messages (id, conversation_id, organization_id, role, content, created_at)
		SELECT $1, c.id, c.organization_id, $3, $4, $5
		FROM conversations c
		WHERE c.id = $2 AND c.organization_id = $6
	`, messageID, conversationID, role, content, createdAt, organizationID)
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

	if _, err := s.db.ExecContext(ctx, `
		UPDATE conversations SET updated_at = $3 WHERE id = $1 AND organization_id = $2
	`, conversationID, organizationID, createdAt); err != nil {
		return Message{}, err
	}

	return Message{
		Content:   content,
		CreatedAt: createdAt,
		ID:        messageID,
		Role:      role,
	}, nil
}

func (s *SQLStore) GetConversationConfig(ctx context.Context, conversationID, organizationID, defaultModelID string) (ConversationConfig, error) {
	config := ConversationConfig{
		ConversationID:       conversationID,
		KnowledgeBaseIDs:     []string{},
		ModelID:              defaultModelID,
		SystemPromptOverride: "",
		Temperature:          1,
		MaxOutputTokens:      1024,
		ToolsEnabled:         false,
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT cc.model_id, cc.system_prompt_override, cc.temperature, cc.max_output_tokens, cc.tools_enabled, cc.updated_at
		FROM conversation_configs cc
		JOIN conversations c ON c.id = cc.conversation_id
		WHERE cc.conversation_id = $1 AND c.organization_id = $2 AND cc.organization_id = $2
	`, conversationID, organizationID).Scan(
		&config.ModelID,
		&config.SystemPromptOverride,
		&config.Temperature,
		&config.MaxOutputTokens,
		&config.ToolsEnabled,
		&config.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			now := time.Now().UTC()
			result, insertErr := s.db.ExecContext(ctx, `
				INSERT INTO conversation_configs (conversation_id, organization_id, model_id, system_prompt_override, temperature, max_output_tokens, tools_enabled, updated_at)
				SELECT c.id, c.organization_id, $3, '', 1, 1024, FALSE, $4
				FROM conversations c
				WHERE c.id = $1 AND c.organization_id = $2
				ON CONFLICT (conversation_id) DO NOTHING
			`, conversationID, organizationID, defaultModelID, now)
			if insertErr != nil {
				return ConversationConfig{}, insertErr
			}
			rowsAffected, rowsErr := result.RowsAffected()
			if rowsErr != nil {
				return ConversationConfig{}, rowsErr
			}
			if rowsAffected == 0 {
				return ConversationConfig{}, sql.ErrNoRows
			}
			config.UpdatedAt = now
			knowledgeBaseIDs, knowledgeErr := s.listConversationKnowledgeBaseIDs(ctx, conversationID, organizationID)
			if knowledgeErr != nil {
				return ConversationConfig{}, knowledgeErr
			}
			config.KnowledgeBaseIDs = knowledgeBaseIDs
			return config, nil
		}
		return ConversationConfig{}, err
	}

	knowledgeBaseIDs, err := s.listConversationKnowledgeBaseIDs(ctx, conversationID, organizationID)
	if err != nil {
		return ConversationConfig{}, err
	}
	config.KnowledgeBaseIDs = knowledgeBaseIDs

	return config, nil
}

func (s *SQLStore) ListConversations(ctx context.Context, organizationID string) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, created_at, updated_at
		FROM conversations
		WHERE organization_id = $1
		ORDER BY updated_at DESC, created_at DESC
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conversations := []Conversation{}
	for rows.Next() {
		var conversation Conversation
		if err := rows.Scan(&conversation.ID, &conversation.Title, &conversation.CreatedAt, &conversation.UpdatedAt); err != nil {
			return nil, err
		}
		conversations = append(conversations, conversation)
	}

	return conversations, rows.Err()
}

func (s *SQLStore) ListMessages(ctx context.Context, conversationID, organizationID string) ([]Message, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `
		SELECT 1
		FROM conversations
		WHERE id = $1 AND organization_id = $2
	`, conversationID, organizationID).Scan(&exists); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.role, m.content, m.created_at
		FROM messages m
		WHERE m.conversation_id = $1 AND m.organization_id = $2
		ORDER BY m.created_at ASC
	`, conversationID, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []Message{}
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message.ID, &message.Role, &message.Content, &message.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	return messages, rows.Err()
}

func (s *SQLStore) UpdateConversationConfig(
	ctx context.Context,
	conversationID,
	organizationID,
	modelID,
	systemPromptOverride string,
	temperature float64,
	maxOutputTokens int,
	toolsEnabled bool,
	knowledgeBaseIDs []string,
) (ConversationConfig, error) {
	updatedAt := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ConversationConfig{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO conversation_configs (
			conversation_id,
			organization_id,
			model_id,
			system_prompt_override,
			temperature,
			max_output_tokens,
			tools_enabled,
			updated_at
		)
		SELECT c.id, c.organization_id, $3, $4, $5, $6, $7, $8
		FROM conversations c
		WHERE c.id = $1 AND c.organization_id = $2
		ON CONFLICT (conversation_id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			model_id = EXCLUDED.model_id,
			system_prompt_override = EXCLUDED.system_prompt_override,
			temperature = EXCLUDED.temperature,
			max_output_tokens = EXCLUDED.max_output_tokens,
			tools_enabled = EXCLUDED.tools_enabled,
			updated_at = EXCLUDED.updated_at
	`, conversationID, organizationID, modelID, systemPromptOverride, temperature, maxOutputTokens, toolsEnabled, updatedAt)
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

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM conversation_knowledge_bindings
		WHERE conversation_id = $1 AND organization_id = $2
	`, conversationID, organizationID); err != nil {
		return ConversationConfig{}, err
	}

	for _, knowledgeBaseID := range knowledgeBaseIDs {
		bindingID, err := auth.NewID("ckb")
		if err != nil {
			return ConversationConfig{}, err
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO conversation_knowledge_bindings (id, conversation_id, knowledge_base_id, organization_id, created_at)
			SELECT $1, c.id, kb.id, c.organization_id, $4
			FROM conversations c
			JOIN knowledge_bases kb ON kb.organization_id = c.organization_id
			WHERE c.id = $2 AND c.organization_id = $3 AND kb.id = $5
		`, bindingID, conversationID, organizationID, updatedAt, knowledgeBaseID)
		if err != nil {
			return ConversationConfig{}, err
		}

		bindingRowsAffected, err := result.RowsAffected()
		if err != nil {
			return ConversationConfig{}, err
		}
		if bindingRowsAffected == 0 {
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
		SystemPromptOverride: systemPromptOverride,
		Temperature:          temperature,
		MaxOutputTokens:      maxOutputTokens,
		ToolsEnabled:         toolsEnabled,
		UpdatedAt:            updatedAt,
	}, nil
}

func (s *SQLStore) listConversationKnowledgeBaseIDs(ctx context.Context, conversationID, organizationID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ckb.knowledge_base_id
		FROM conversation_knowledge_bindings ckb
		JOIN conversations c ON c.id = ckb.conversation_id
		JOIN knowledge_bases kb ON kb.id = ckb.knowledge_base_id
		WHERE ckb.conversation_id = $1
		  AND c.organization_id = $2
		  AND ckb.organization_id = $2
		  AND kb.organization_id = $2
		ORDER BY ckb.created_at ASC, ckb.knowledge_base_id ASC
	`, conversationID, organizationID)
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
