package chat

import (
	"context"
	"database/sql"
	"time"

	"oblivious/server/internal/auth"
)

func (s *SQLStore) CreateConversation(ctx context.Context, workspaceID, title, defaultModelID string) (Conversation, error) {
	conversationID, err := auth.NewID("conversation")
	if err != nil {
		return Conversation{}, err
	}

	createdAt := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO conversations (id, workspace_id, title, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`, conversationID, workspaceID, title, createdAt, createdAt); err != nil {
		return Conversation{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO conversation_configs (conversation_id, model_id, system_prompt_override, temperature, max_output_tokens, tools_enabled, updated_at)
		VALUES ($1, $2, '', 1, 1024, FALSE, $3)
	`, conversationID, defaultModelID, createdAt); err != nil {
		return Conversation{}, err
	}

	return Conversation{
		CreatedAt: createdAt,
		ID:        conversationID,
		Title:     title,
		UpdatedAt: createdAt,
	}, nil
}

func (s *SQLStore) CreateMessage(ctx context.Context, conversationID, role, content string) (Message, error) {
	messageID, err := auth.NewID("message")
	if err != nil {
		return Message{}, err
	}

	createdAt := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO messages (id, conversation_id, role, content, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, messageID, conversationID, role, content, createdAt); err != nil {
		return Message{}, err
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE conversations SET updated_at = $2 WHERE id = $1
	`, conversationID, createdAt); err != nil {
		return Message{}, err
	}

	return Message{
		Content:   content,
		CreatedAt: createdAt,
		ID:        messageID,
		Role:      role,
	}, nil
}

func (s *SQLStore) GetConversationConfig(ctx context.Context, conversationID, workspaceID, defaultModelID string) (ConversationConfig, error) {
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
		WHERE cc.conversation_id = $1 AND c.workspace_id = $2
	`, conversationID, workspaceID).Scan(
		&config.ModelID,
		&config.SystemPromptOverride,
		&config.Temperature,
		&config.MaxOutputTokens,
		&config.ToolsEnabled,
		&config.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			now := time.Now().UTC()
			if _, insertErr := s.db.ExecContext(ctx, `
				INSERT INTO conversation_configs (conversation_id, model_id, system_prompt_override, temperature, max_output_tokens, tools_enabled, updated_at)
				SELECT c.id, $3, '', 1, 1024, FALSE, $4
				FROM conversations c
				WHERE c.id = $1 AND c.workspace_id = $2
				ON CONFLICT (conversation_id) DO NOTHING
			`, conversationID, workspaceID, defaultModelID, now); insertErr != nil {
				return ConversationConfig{}, insertErr
			}
			config.UpdatedAt = now
			knowledgeBaseIDs, knowledgeErr := s.listConversationKnowledgeBaseIDs(ctx, conversationID, workspaceID)
			if knowledgeErr != nil {
				return ConversationConfig{}, knowledgeErr
			}
			config.KnowledgeBaseIDs = knowledgeBaseIDs
			return config, nil
		}
		return ConversationConfig{}, err
	}

	knowledgeBaseIDs, err := s.listConversationKnowledgeBaseIDs(ctx, conversationID, workspaceID)
	if err != nil {
		return ConversationConfig{}, err
	}
	config.KnowledgeBaseIDs = knowledgeBaseIDs

	return config, nil
}

func (s *SQLStore) ListConversations(ctx context.Context, workspaceID string) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, created_at, updated_at
		FROM conversations
		WHERE workspace_id = $1
		ORDER BY updated_at DESC, created_at DESC
	`, workspaceID)
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

func (s *SQLStore) ListMessages(ctx context.Context, conversationID, workspaceID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.role, m.content, m.created_at
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE m.conversation_id = $1 AND c.workspace_id = $2
		ORDER BY m.created_at ASC
	`, conversationID, workspaceID)
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
	workspaceID,
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
			model_id,
			system_prompt_override,
			temperature,
			max_output_tokens,
			tools_enabled,
			updated_at
		)
		SELECT c.id, $3, $4, $5, $6, $7, $8
		FROM conversations c
		WHERE c.id = $1 AND c.workspace_id = $2
		ON CONFLICT (conversation_id) DO UPDATE SET
			model_id = EXCLUDED.model_id,
			system_prompt_override = EXCLUDED.system_prompt_override,
			temperature = EXCLUDED.temperature,
			max_output_tokens = EXCLUDED.max_output_tokens,
			tools_enabled = EXCLUDED.tools_enabled,
			updated_at = EXCLUDED.updated_at
	`, conversationID, workspaceID, modelID, systemPromptOverride, temperature, maxOutputTokens, toolsEnabled, updatedAt)
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
		WHERE conversation_id = $1
	`, conversationID); err != nil {
		return ConversationConfig{}, err
	}

	for _, knowledgeBaseID := range knowledgeBaseIDs {
		bindingID, err := auth.NewID("ckb")
		if err != nil {
			return ConversationConfig{}, err
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO conversation_knowledge_bindings (id, conversation_id, knowledge_base_id, created_at)
			SELECT $1, c.id, kb.id, $4
			FROM conversations c
			JOIN knowledge_bases kb ON kb.workspace_id = c.workspace_id
			WHERE c.id = $2 AND c.workspace_id = $3 AND kb.id = $5
		`, bindingID, conversationID, workspaceID, updatedAt, knowledgeBaseID)
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

func (s *SQLStore) listConversationKnowledgeBaseIDs(ctx context.Context, conversationID, workspaceID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ckb.knowledge_base_id
		FROM conversation_knowledge_bindings ckb
		JOIN conversations c ON c.id = ckb.conversation_id
		JOIN knowledge_bases kb ON kb.id = ckb.knowledge_base_id
		WHERE ckb.conversation_id = $1 AND c.workspace_id = $2 AND kb.workspace_id = $2
		ORDER BY ckb.created_at ASC, ckb.knowledge_base_id ASC
	`, conversationID, workspaceID)
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
