package usage

import (
	"context"
	"database/sql"
	"encoding/json"
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
	priceSnapshot, err := relayPriceSnapshotJSON(record)
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
			created_at,
			price_snapshot,
			price_currency,
			price_source,
			price_effective_from
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
			$21,
			$22,
			NULLIF($23, ''),
			NULLIF($24, ''),
			$25
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
		priceSnapshot,
		record.PriceCurrency,
		record.PriceSource,
		record.PriceEffectiveFrom,
	)

	return err
}

func (r *SQLRecorder) ReplaceRelayUsage(ctx context.Context, record relay.RelayUsageLogRecord) error {
	if record.RequestID == "" {
		return r.RecordRelayUsage(ctx, record)
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
	priceSnapshot, err := relayPriceSnapshotJSON(record)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE usage_records
		SET
			user_id = $1,
			organization_id = $2,
			model_id = $3,
			input_tokens = $4,
			output_tokens = $5,
			api_type = $6,
			channel_id = $7,
			provider = $8,
			api_token_id = NULLIF($9, ''),
			feature_type = NULLIF($10, ''),
			quota_mode = NULLIF($11, ''),
			status = $12,
			status_code = $13,
			latency_ms = $14,
			cost = $15,
			channel_cost = $16,
			error_code = NULLIF($17, ''),
			total_tokens = $18,
			created_at = $19,
			price_snapshot = $20,
			price_currency = NULLIF($21, ''),
			price_source = NULLIF($22, ''),
			price_effective_from = $23
		WHERE request_id = $24
	`, record.UserID,
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
		record.ErrorCode,
		record.TotalTokens,
		createdAt,
		priceSnapshot,
		record.PriceCurrency,
		record.PriceSource,
		record.PriceEffectiveFrom,
		record.RequestID,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected > 0 {
		return nil
	}
	return r.RecordRelayUsage(ctx, record)
}

func relayPriceSnapshotJSON(record relay.RelayUsageLogRecord) (string, error) {
	if record.PriceSnapshot == nil {
		return "{}", nil
	}
	data, err := json.Marshal(record.PriceSnapshot)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
