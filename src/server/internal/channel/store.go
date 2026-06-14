package channel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"

	"oblivious/server/internal/notification"
	"oblivious/server/internal/secretbox"
)

type Store interface {
	CreateConfig(ctx context.Context, config *ChannelConfig) (*ChannelConfig, error)
	GetConfig(ctx context.Context, organizationID, id string) (*ChannelConfig, error)
	GetConfigByID(ctx context.Context, id string) (*ChannelConfig, error)
	ListConfigs(ctx context.Context, organizationID string) ([]*ChannelConfig, error)
	UpdateConfig(ctx context.Context, organizationID, id string, update ConfigUpdate) (*ChannelConfig, error)
	UpdateConfigStatus(ctx context.Context, organizationID, id string, status ChannelStatus) (*ChannelConfig, error)
	RecordMessageLog(ctx context.Context, log *ChannelMessageLog) (*ChannelMessageLog, error)
	CountConsecutiveDeliveryFailures(ctx context.Context, channelID string, limit int) (int, error)
	CountConsecutiveSuccessfulDeliveries(ctx context.Context, channelID string, limit int) (int, error)
}

type MessageLogStore interface {
	ListMessageLogs(ctx context.Context, channelID string, input ListMessageLogsInput) ([]*ChannelMessageLog, error)
	ListFailedMessageLogs(ctx context.Context, channelID string, input ListMessageLogsInput) ([]*ChannelMessageLog, error)
}

type MessageLogArchiveStore interface {
	ListExpiredMessageLogsForArchive(ctx context.Context, input ArchiveExpiredMessageLogsInput) ([]*ChannelMessageLog, error)
	DeleteArchivedMessageLogs(ctx context.Context, ids []string) (ArchiveExpiredMessageLogsResult, error)
}

type RetryMessageStore interface {
	ListDueRetryMessages(ctx context.Context, input ClaimDueRetryMessagesInput) ([]*ChannelMessageLog, error)
	ClaimDueRetryMessages(ctx context.Context, input ClaimDueRetryMessagesInput) ([]*ChannelMessageLog, error)
}

type RetryWorkerStore interface {
	RetryMessageStore
	GetConfigByID(ctx context.Context, id string) (*ChannelConfig, error)
	UpdateRetryMessageLog(ctx context.Context, log *ChannelMessageLog) (*ChannelMessageLog, error)
	UpdateConfigStatus(ctx context.Context, organizationID, id string, status ChannelStatus) (*ChannelConfig, error)
	CountConsecutiveDeliveryFailures(ctx context.Context, channelID string, limit int) (int, error)
	CountConsecutiveSuccessfulDeliveries(ctx context.Context, channelID string, limit int) (int, error)
}

type ConfigUpdate struct {
	Type   ChannelType
	Name   string
	Config map[string]any
	Status ChannelStatus
}

const defaultMessageLogListLimit = 20
const defaultRetryMessageClaimLimit = 100
const defaultArchiveMessageLogLimit = 500
const defaultMessageLogRetention = 30 * 24 * time.Hour

type ListMessageLogsInput struct {
	Limit int
}

type ClaimDueRetryMessagesInput struct {
	ChannelID         string
	FallbackChannelID string
	Now               time.Time
	Limit             int
	Force             bool
}

type ArchiveExpiredMessageLogsInput struct {
	Before time.Time
	Limit  int
}

type ArchiveExpiredMessageLogsResult struct {
	ArchivedIDs []string  `json:"archived_ids"`
	Count       int       `json:"count"`
	Before      time.Time `json:"before"`
	ObjectKey   string    `json:"object_key,omitempty"`
}

type MessageLogRetentionPolicy struct {
	Retention time.Duration
	Limit     int
	Now       func() time.Time
}

func (p MessageLogRetentionPolicy) ArchiveInput() ArchiveExpiredMessageLogsInput {
	retention := p.Retention
	if retention <= 0 {
		retention = defaultMessageLogRetention
	}
	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}
	return ArchiveExpiredMessageLogsInput{
		Before: now.Add(-retention),
		Limit:  p.Limit,
	}
}

type SQLStore struct {
	db *sql.DB
}

const archiveExpiredMessageLogsSQL = `
	WITH expired AS (
		SELECT id
		FROM channel_messages
		WHERE created_at < $1
		  AND status IN ($2, $3)
		ORDER BY created_at ASC, id ASC
		LIMIT $4
	)
	DELETE FROM channel_messages message
	USING expired
	WHERE message.id = expired.id
	RETURNING message.id
`

const listExpiredMessageLogsForArchiveSQL = `
	SELECT id, channel_id, conversation_id, direction, raw_message, transformed_message,
		transform_success, transform_error, status, retry_count, failure_reason, next_retry_at, created_at
	FROM channel_messages
	WHERE created_at < $1
	  AND status IN ($2, $3)
	ORDER BY created_at ASC, id ASC
	LIMIT $4
`

const deleteArchivedMessageLogsSQL = `
	DELETE FROM channel_messages
	WHERE id = ANY($1)
	  AND status IN ($2, $3)
	RETURNING id
`

const updateRetryMessageLogSQL = `
	UPDATE channel_messages
	SET channel_id = $2,
		conversation_id = $3,
		raw_message = COALESCE($4, raw_message),
		transformed_message = $5,
		transform_success = $6,
		transform_error = $7,
		status = $8,
		retry_count = $9,
		failure_reason = $10,
		next_retry_at = $11
	WHERE id = $1
	RETURNING id, channel_id, conversation_id, direction, raw_message, transformed_message,
		transform_success, transform_error, status, retry_count, failure_reason, next_retry_at, created_at
`

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) CreateConfig(ctx context.Context, config *ChannelConfig) (*ChannelConfig, error) {
	if config == nil {
		return nil, fmt.Errorf("channel config is required")
	}
	id := config.ID
	if id == "" {
		id = generateID("channel_config")
	}
	status := config.Status
	if status == "" {
		status = ChannelStatusActive
	}
	protectedConfig, err := protectChannelSQLConfig(config.Config)
	if err != nil {
		return nil, fmt.Errorf("protect channel config: %w", err)
	}
	configJSON, err := json.Marshal(protectedConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal channel config: %w", err)
	}
	now := time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO channel_configs (id, organization_id, type, name, config, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
	`, id, config.OrganizationID, config.Type, config.Name, configJSON, status, now)
	if err != nil {
		return nil, fmt.Errorf("insert channel config: %w", err)
	}

	return s.GetConfig(ctx, config.OrganizationID, id)
}

func (s *SQLStore) GetConfig(ctx context.Context, organizationID, id string) (*ChannelConfig, error) {
	config, err := scanChannelConfig(s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, type, name, config, status, created_at, updated_at
		FROM channel_configs
		WHERE organization_id = $1 AND id = $2
	`, organizationID, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel config: %w", err)
	}
	return config, nil
}

func (s *SQLStore) GetConfigByID(ctx context.Context, id string) (*ChannelConfig, error) {
	config, err := scanChannelConfig(s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, type, name, config, status, created_at, updated_at
		FROM channel_configs
		WHERE id = $1
	`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel config by id: %w", err)
	}
	return config, nil
}

func (s *SQLStore) ListConfigs(ctx context.Context, organizationID string) ([]*ChannelConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, type, name, config, status, created_at, updated_at
		FROM channel_configs
		WHERE organization_id = $1
		ORDER BY created_at DESC, id DESC
	`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list channel configs: %w", err)
	}
	defer rows.Close()

	var configs []*ChannelConfig
	for rows.Next() {
		config, err := scanChannelConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("scan channel config: %w", err)
		}
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

func (s *SQLStore) UpdateConfigStatus(ctx context.Context, organizationID, id string, status ChannelStatus) (*ChannelConfig, error) {
	updated, err := scanChannelConfig(s.db.QueryRowContext(ctx, `
		UPDATE channel_configs
		SET status = $3, updated_at = $4
		WHERE organization_id = $1 AND id = $2
		RETURNING id, organization_id, type, name, config, status, created_at, updated_at
	`, organizationID, id, status, time.Now().UTC()))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update channel config status: %w", err)
	}
	return updated, nil
}

func (s *SQLStore) UpdateConfig(ctx context.Context, organizationID, id string, update ConfigUpdate) (*ChannelConfig, error) {
	protectedConfig, err := protectChannelSQLConfig(update.Config)
	if err != nil {
		return nil, fmt.Errorf("protect channel config update: %w", err)
	}
	configJSON, err := json.Marshal(protectedConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal channel config update: %w", err)
	}

	updated, err := scanChannelConfig(s.db.QueryRowContext(ctx, `
		UPDATE channel_configs
		SET type = $3, name = $4, config = $5, status = $6, updated_at = $7
		WHERE organization_id = $1 AND id = $2
		RETURNING id, organization_id, type, name, config, status, created_at, updated_at
	`, organizationID, id, update.Type, update.Name, configJSON, update.Status, time.Now().UTC()))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update channel config: %w", err)
	}
	return updated, nil
}

func (s *SQLStore) RecordMessageLog(ctx context.Context, log *ChannelMessageLog) (*ChannelMessageLog, error) {
	if log == nil {
		return nil, fmt.Errorf("channel message log is required")
	}
	id := log.ID
	if id == "" {
		id = generateID("channel_message")
	}
	createdAt := log.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	status := log.Status
	if status == "" {
		status = MessageStatusRecorded
	}

	transformedJSON, err := transformedMessageJSON(log)
	if err != nil {
		return nil, fmt.Errorf("marshal transformed channel message: %w", err)
	}
	rawMessage := nonNullableJSON(log.RawMessage)

	var conversationID sql.NullString
	if log.ConversationID != "" {
		conversationID = sql.NullString{String: log.ConversationID, Valid: true}
	}
	var transformError sql.NullString
	if log.TransformError != "" {
		transformError = sql.NullString{String: log.TransformError, Valid: true}
	}
	var failureReason sql.NullString
	if log.FailureReason != "" {
		failureReason = sql.NullString{String: log.FailureReason, Valid: true}
	}
	var nextRetryAt sql.NullTime
	if log.NextRetryAt != nil {
		nextRetryAt = sql.NullTime{Time: *log.NextRetryAt, Valid: true}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO channel_messages (
			id, channel_id, conversation_id, direction, raw_message, transformed_message,
			transform_success, transform_error, status, retry_count, failure_reason, next_retry_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, id, log.ChannelID, conversationID, log.Direction, rawMessage, nullableJSON(transformedJSON),
		log.TransformSuccess, transformError, status, log.RetryCount, failureReason, nextRetryAt, createdAt)
	if err != nil {
		return nil, fmt.Errorf("insert channel message log: %w", err)
	}

	stored := *log
	stored.ID = id
	stored.RawMessage = append(json.RawMessage(nil), rawMessage...)
	stored.Status = status
	stored.CreatedAt = createdAt
	return &stored, nil
}

func (s *SQLStore) UpdateRetryMessageLog(ctx context.Context, log *ChannelMessageLog) (*ChannelMessageLog, error) {
	if log == nil {
		return nil, fmt.Errorf("channel message log is required")
	}
	if log.ID == "" {
		return nil, fmt.Errorf("channel message log id is required")
	}

	status := log.Status
	if status == "" {
		status = MessageStatusRecorded
	}

	transformedJSON, err := transformedMessageJSON(log)
	if err != nil {
		return nil, fmt.Errorf("marshal transformed channel message: %w", err)
	}

	var conversationID sql.NullString
	if log.ConversationID != "" {
		conversationID = sql.NullString{String: log.ConversationID, Valid: true}
	}
	var transformError sql.NullString
	if log.TransformError != "" {
		transformError = sql.NullString{String: log.TransformError, Valid: true}
	}
	var failureReason sql.NullString
	if log.FailureReason != "" {
		failureReason = sql.NullString{String: log.FailureReason, Valid: true}
	}
	var nextRetryAt sql.NullTime
	if log.NextRetryAt != nil {
		nextRetryAt = sql.NullTime{Time: *log.NextRetryAt, Valid: true}
	}

	updated, err := scanChannelMessageLog(s.db.QueryRowContext(ctx, updateRetryMessageLogSQL, log.ID, log.ChannelID, conversationID, nullableJSON(log.RawMessage), nullableJSON(transformedJSON),
		log.TransformSuccess, transformError, status, log.RetryCount, failureReason, nextRetryAt))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update channel retry message log: %w", err)
	}
	return updated, nil
}

func (s *SQLStore) ListMessageLogs(ctx context.Context, channelID string, input ListMessageLogsInput) ([]*ChannelMessageLog, error) {
	limit := normalizeMessageLogListInput(input)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, channel_id, conversation_id, direction, raw_message, transformed_message,
			transform_success, transform_error, status, retry_count, failure_reason, next_retry_at, created_at
		FROM channel_messages
		WHERE channel_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, channelID, limit)
	if err != nil {
		return nil, fmt.Errorf("list channel message logs: %w", err)
	}
	defer rows.Close()

	logs, err := scanChannelMessageLogs(rows)
	if err != nil {
		return nil, fmt.Errorf("scan channel message logs: %w", err)
	}
	return logs, nil
}

func (s *SQLStore) ListFailedMessageLogs(ctx context.Context, channelID string, input ListMessageLogsInput) ([]*ChannelMessageLog, error) {
	limit := normalizeMessageLogListInput(input)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, channel_id, conversation_id, direction, raw_message, transformed_message,
			transform_success, transform_error, status, retry_count, failure_reason, next_retry_at, created_at
		FROM channel_messages
		WHERE channel_id = $1
		  AND direction = 'outbound'
		  AND status IN ($2, $3)
		ORDER BY next_retry_at ASC NULLS LAST, created_at DESC, id DESC
		LIMIT $4
	`, channelID, MessageStatusRetryPending, MessageStatusPermanentFailure, limit)
	if err != nil {
		return nil, fmt.Errorf("list failed channel message logs: %w", err)
	}
	defer rows.Close()

	logs, err := scanChannelMessageLogs(rows)
	if err != nil {
		return nil, fmt.Errorf("scan failed channel message logs: %w", err)
	}
	return logs, nil
}

func (s *SQLStore) ListDueRetryMessages(ctx context.Context, input ClaimDueRetryMessagesInput) ([]*ChannelMessageLog, error) {
	now, limit := normalizeRetryClaimInput(input)
	channelID := input.ChannelID

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, channel_id, conversation_id, direction, raw_message, transformed_message,
			transform_success, transform_error, status, retry_count, failure_reason, next_retry_at, created_at
		FROM channel_messages
		WHERE direction = 'outbound'
		  AND status = $1
		  AND next_retry_at IS NOT NULL
		  AND ($5 = true OR next_retry_at <= $2)
		  AND ($4 = '' OR channel_id = $4)
		ORDER BY next_retry_at ASC, created_at ASC, id ASC
		LIMIT $3
	`, MessageStatusRetryPending, now, limit, channelID, input.Force)
	if err != nil {
		return nil, fmt.Errorf("list due channel retry messages: %w", err)
	}
	defer rows.Close()

	logs, err := scanChannelMessageLogs(rows)
	if err != nil {
		return nil, fmt.Errorf("scan due channel retry messages: %w", err)
	}
	return logs, nil
}

func (s *SQLStore) ClaimDueRetryMessages(ctx context.Context, input ClaimDueRetryMessagesInput) ([]*ChannelMessageLog, error) {
	now, limit := normalizeRetryClaimInput(input)
	channelID := input.ChannelID

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	rows, err := tx.QueryContext(ctx, `
		WITH due AS (
			SELECT id
			FROM channel_messages
			WHERE direction = 'outbound'
			  AND status = $1
			  AND next_retry_at IS NOT NULL
			  AND ($6 = true OR next_retry_at <= $2)
			  AND ($5 = '' OR channel_id = $5)
			ORDER BY next_retry_at ASC, created_at ASC, id ASC
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		),
		updated AS (
			UPDATE channel_messages message
			SET status = $4
			FROM due
			WHERE message.id = due.id
			RETURNING message.id, message.channel_id, message.conversation_id, message.direction, message.raw_message, message.transformed_message,
				message.transform_success, message.transform_error, message.status, message.retry_count, message.failure_reason, message.next_retry_at,
				message.created_at
		)
		SELECT id, channel_id, conversation_id, direction, raw_message, transformed_message,
			transform_success, transform_error, status, retry_count, failure_reason, next_retry_at, created_at
		FROM updated
		ORDER BY next_retry_at ASC, created_at ASC, id ASC
	`, MessageStatusRetryPending, now, limit, MessageStatusSending, channelID, input.Force)
	if err != nil {
		return nil, fmt.Errorf("claim due channel retry messages: %w", err)
	}

	logs, err := scanChannelMessageLogs(rows)
	if closeErr := rows.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, fmt.Errorf("scan claimed channel retry messages: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return logs, nil
}

func (s *SQLStore) CountConsecutiveDeliveryFailures(ctx context.Context, channelID string, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `
		WITH recent AS (
			SELECT
				ROW_NUMBER() OVER (ORDER BY created_at DESC, id DESC) AS row_number,
				(status IN ('retry_pending', 'permanent_failure') OR transform_success = false) AS failed
			FROM channel_messages
			WHERE channel_id = $1 AND direction = 'outbound'
			ORDER BY created_at DESC, id DESC
			LIMIT $2
		),
		first_success AS (
			SELECT COALESCE(MIN(row_number), $2 + 1) AS row_number
			FROM recent
			WHERE failed = false
		)
		SELECT COUNT(*)
		FROM recent, first_success
		WHERE recent.failed = true AND recent.row_number < first_success.row_number
	`, channelID, limit).Scan(&count); err != nil {
		return 0, fmt.Errorf("count consecutive channel delivery failures: %w", err)
	}
	return count, nil
}

func (s *SQLStore) CountConsecutiveSuccessfulDeliveries(ctx context.Context, channelID string, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `
		WITH recent AS (
			SELECT
				ROW_NUMBER() OVER (ORDER BY created_at DESC, id DESC) AS row_number,
				(status = 'recorded' AND transform_success = true) AS succeeded
			FROM channel_messages
			WHERE channel_id = $1 AND direction = 'outbound'
			ORDER BY created_at DESC, id DESC
			LIMIT $2
		),
		first_failure AS (
			SELECT COALESCE(MIN(row_number), $2 + 1) AS row_number
			FROM recent
			WHERE succeeded = false
		)
		SELECT COUNT(*)
		FROM recent, first_failure
		WHERE recent.succeeded = true AND recent.row_number < first_failure.row_number
	`, channelID, limit).Scan(&count); err != nil {
		return 0, fmt.Errorf("count consecutive successful channel deliveries: %w", err)
	}
	return count, nil
}

func (s *SQLStore) ArchiveExpiredMessageLogs(ctx context.Context, input ArchiveExpiredMessageLogsInput) (ArchiveExpiredMessageLogsResult, error) {
	before, limit := normalizeArchiveExpiredMessageLogsInput(input)
	result := ArchiveExpiredMessageLogsResult{Before: before}
	if before.IsZero() {
		return result, fmt.Errorf("archive cutoff is required")
	}

	rows, err := s.db.QueryContext(ctx, archiveExpiredMessageLogsSQL, before, MessageStatusRecorded, MessageStatusPermanentFailure, limit)
	if err != nil {
		return result, fmt.Errorf("archive expired channel message logs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return result, fmt.Errorf("scan archived channel message log id: %w", err)
		}
		result.ArchivedIDs = append(result.ArchivedIDs, id)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("scan archived channel message log ids: %w", err)
	}
	result.Count = len(result.ArchivedIDs)
	return result, nil
}

func (s *SQLStore) ListExpiredMessageLogsForArchive(ctx context.Context, input ArchiveExpiredMessageLogsInput) ([]*ChannelMessageLog, error) {
	before, limit := normalizeArchiveExpiredMessageLogsInput(input)
	if before.IsZero() {
		return nil, fmt.Errorf("archive cutoff is required")
	}
	rows, err := s.db.QueryContext(ctx, listExpiredMessageLogsForArchiveSQL, before, MessageStatusRecorded, MessageStatusPermanentFailure, limit)
	if err != nil {
		return nil, fmt.Errorf("list expired channel message logs for archive: %w", err)
	}
	defer rows.Close()
	logs, err := scanChannelMessageLogs(rows)
	if err != nil {
		return nil, fmt.Errorf("scan expired channel message logs for archive: %w", err)
	}
	return logs, nil
}

func (s *SQLStore) DeleteArchivedMessageLogs(ctx context.Context, ids []string) (ArchiveExpiredMessageLogsResult, error) {
	result := ArchiveExpiredMessageLogsResult{}
	ids = normalizeArchiveMessageLogIDs(ids)
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, deleteArchivedMessageLogsSQL, pq.Array(ids), MessageStatusRecorded, MessageStatusPermanentFailure)
	if err != nil {
		return result, fmt.Errorf("delete archived channel message logs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return result, fmt.Errorf("scan deleted channel message log id: %w", err)
		}
		result.ArchivedIDs = append(result.ArchivedIDs, id)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("scan deleted channel message log ids: %w", err)
	}
	result.Count = len(result.ArchivedIDs)
	return result, nil
}

func (s *SQLStore) CreateEvent(ctx context.Context, event notification.NotificationEvent) (*notification.Notification, error) {
	return notification.NewService(notification.NewSQLStore(s.db)).CreateEvent(ctx, event)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanChannelConfig(row rowScanner) (*ChannelConfig, error) {
	var config ChannelConfig
	var configJSON []byte
	if err := row.Scan(
		&config.ID,
		&config.OrganizationID,
		&config.Type,
		&config.Name,
		&configJSON,
		&config.Status,
		&config.CreatedAt,
		&config.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &config.Config); err != nil {
			return nil, fmt.Errorf("unmarshal channel config: %w", err)
		}
	}
	if config.Config == nil {
		config.Config = map[string]any{}
	}
	openedConfig, err := openChannelSQLConfig(config.Config)
	if err != nil {
		return nil, fmt.Errorf("open channel config %s: %w", config.ID, err)
	}
	config.Config = openedConfig
	return &config, nil
}

func protectChannelSQLConfig(config map[string]any) (map[string]any, error) {
	protected := make(map[string]any, len(config))
	for key, value := range config {
		if IsChannelSecretConfigKey(key) {
			if plaintext, ok := value.(string); ok && plaintext != "" {
				stored, err := secretbox.Protect(secretbox.DomainPublishingChannelConfigKey, plaintext)
				if err != nil {
					return nil, fmt.Errorf("protect %s: %w", key, err)
				}
				protected[key] = stored
				continue
			}
		}
		protected[key] = value
	}
	return protected, nil
}

func openChannelSQLConfig(config map[string]any) (map[string]any, error) {
	opened := make(map[string]any, len(config))
	for key, value := range config {
		if IsChannelSecretConfigKey(key) {
			if stored, ok := value.(string); ok && stored != "" {
				plaintext, err := secretbox.Open(secretbox.DomainPublishingChannelConfigKey, stored)
				if err != nil {
					return nil, fmt.Errorf("open %s: %w", key, err)
				}
				opened[key] = plaintext
				continue
			}
		}
		opened[key] = value
	}
	return opened, nil
}

func normalizeMessageLogListInput(input ListMessageLogsInput) int {
	if input.Limit <= 0 {
		return defaultMessageLogListLimit
	}
	if input.Limit > defaultRetryMessageClaimLimit {
		return defaultRetryMessageClaimLimit
	}
	return input.Limit
}

func normalizeRetryClaimInput(input ClaimDueRetryMessagesInput) (time.Time, int) {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	limit := input.Limit
	if limit <= 0 {
		limit = defaultRetryMessageClaimLimit
	}
	return now, limit
}

func normalizeArchiveExpiredMessageLogsInput(input ArchiveExpiredMessageLogsInput) (time.Time, int) {
	before := input.Before
	if !before.IsZero() {
		before = before.UTC()
	}
	limit := input.Limit
	if limit <= 0 || limit > defaultArchiveMessageLogLimit {
		limit = defaultArchiveMessageLogLimit
	}
	return before, limit
}

func normalizeArchiveMessageLogIDs(ids []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}

func scanChannelMessageLogs(rows *sql.Rows) ([]*ChannelMessageLog, error) {
	var logs []*ChannelMessageLog
	for rows.Next() {
		log, err := scanChannelMessageLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

func scanChannelMessageLog(row rowScanner) (*ChannelMessageLog, error) {
	var log ChannelMessageLog
	var conversationID sql.NullString
	var transformedJSON []byte
	var transformError sql.NullString
	var failureReason sql.NullString
	var nextRetryAt sql.NullTime

	if err := row.Scan(
		&log.ID,
		&log.ChannelID,
		&conversationID,
		&log.Direction,
		&log.RawMessage,
		&transformedJSON,
		&log.TransformSuccess,
		&transformError,
		&log.Status,
		&log.RetryCount,
		&failureReason,
		&nextRetryAt,
		&log.CreatedAt,
	); err != nil {
		return nil, err
	}

	if conversationID.Valid {
		log.ConversationID = conversationID.String
	}
	if len(transformedJSON) > 0 {
		if err := json.Unmarshal(transformedJSON, &log.TransformedMessage); err != nil {
			return nil, fmt.Errorf("unmarshal transformed channel message: %w", err)
		}
	}
	if transformError.Valid {
		log.TransformError = transformError.String
	}
	if failureReason.Valid {
		log.FailureReason = failureReason.String
	}
	if nextRetryAt.Valid {
		nextRetryAtUTC := nextRetryAt.Time.UTC()
		log.NextRetryAt = &nextRetryAtUTC
	}

	return &log, nil
}

func nullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func nonNullableJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func transformedMessageJSON(log *ChannelMessageLog) ([]byte, error) {
	if log == nil {
		return nil, nil
	}
	if !log.TransformSuccess && isZeroInternalMessage(log.TransformedMessage) {
		return nil, nil
	}
	return json.Marshal(log.TransformedMessage)
}

func isZeroInternalMessage(message InternalMessage) bool {
	return message.ID == "" &&
		message.ConversationID == "" &&
		message.Role == "" &&
		len(message.Content) == 0 &&
		len(message.Metadata) == 0 &&
		message.Timestamp.IsZero()
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
