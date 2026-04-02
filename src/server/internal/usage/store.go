package usage

import (
	"context"
	"database/sql"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
)

type SQLRecorder struct {
	db *sql.DB
}

func NewSQLRecorder(db *sql.DB) *SQLRecorder {
	return &SQLRecorder{db: db}
}

func (r *SQLRecorder) RecordChatUsage(ctx context.Context, record chat.UsageRecord) error {
	usageID, err := auth.NewID("usage")
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO usage_records (
			id,
			user_id,
			workspace_id,
			conversation_id,
			model_id,
			request_count,
			input_tokens,
			output_tokens
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, usageID, record.UserID, record.WorkspaceID, record.ConversationID, record.ModelID, record.RequestCount, record.InputTokens, record.OutputTokens)

	return err
}
