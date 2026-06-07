package usage

import (
	"context"
	"database/sql"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/relay"
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
			organization_id,
			conversation_id,
			model_id,
			request_count,
			input_tokens,
			output_tokens
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, usageID, record.UserID, record.WorkspaceID, record.OrganizationID, record.ConversationID, record.ModelID, record.RequestCount, record.InputTokens, record.OutputTokens)

	return err
}

func (r *SQLRecorder) RecordRelayUsage(ctx context.Context, record relay.RelayUsageLogRecord) error {
	usageID, err := auth.NewID("usage")
	if err != nil {
		return err
	}

	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	featureType := record.FeatureType
	if featureType == "" {
		featureType = record.APIType
	}
	quotaMode := record.QuotaMode
	if quotaMode == "" {
		quotaMode = "none"
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO usage_records (
			id,
			user_id,
			workspace_id,
			organization_id,
			conversation_id,
			model_id,
			request_count,
			input_tokens,
			output_tokens,
			api_type,
			channel_id,
			provider,
			api_token_id,
			feature_type,
			quota_mode,
			status,
			status_code,
			latency_ms,
			cost,
			channel_cost,
			request_id,
			error_code,
			total_tokens,
			created_at
		)
		VALUES (
			$1,
			$2,
			NULL,
			$3,
			NULL,
			$4,
			1,
			$5,
			$6,
			$7,
			$8,
			$9,
			NULLIF($10, ''),
			NULLIF($11, ''),
			NULLIF($12, ''),
			$13,
			$14,
			$15,
			$16,
			$17,
			NULLIF($18, ''),
			NULLIF($19, ''),
			$20,
			$21
		)
	`, usageID,
		record.UserID,
		record.OrganizationID,
		record.Model,
		record.PromptTokens,
		record.CompletionTokens,
		record.APIType,
		record.ChannelID,
		record.Provider,
		record.APITokenID,
		featureType,
		quotaMode,
		string(record.Status),
		record.StatusCode,
		record.LatencyMS,
		record.Cost,
		record.ChannelCost,
		record.RequestID,
		record.ErrorCode,
		record.TotalTokens,
		createdAt,
	)

	return err
}
