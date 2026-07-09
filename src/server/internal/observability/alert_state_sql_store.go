package observability

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SQLAlertStateStore struct {
	db                *sql.DB
	escalationWindow  time.Duration
	escalationLimit   int
	warningOpenWindow time.Duration
}

func NewSQLAlertStateStore(db *sql.DB) *SQLAlertStateStore {
	return &SQLAlertStateStore{
		db:                db,
		escalationWindow:  5 * time.Minute,
		escalationLimit:   3,
		warningOpenWindow: 30 * time.Minute,
	}
}

func (s *SQLAlertStateStore) RecordAlertOpen(ctx context.Context, event AlertEvent) (AlertState, error) {
	if s == nil || s.db == nil {
		return AlertState{}, errors.New("alert state store database is required")
	}
	key := alertKey(event)
	if key == "" {
		return AlertState{}, errors.New("alert key is required")
	}
	if event.Severity == "" {
		event.Severity = AlertSeverityInfo
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AlertState{}, fmt.Errorf("begin alert state update: %w", err)
	}
	defer tx.Rollback()

	state, found, err := s.getAlertStateForUpdate(ctx, tx, key)
	if err != nil {
		return AlertState{}, err
	}
	if !found {
		state = AlertState{
			Key:      key,
			Status:   AlertStatusOpen,
			OpenedAt: event.OccurredAt,
		}
	}

	state.Status = AlertStatusOpen
	state.Severity = event.Severity
	state.Title = event.Title
	state.Component = event.Component
	state.LastOccurredAt = event.OccurredAt
	state.OccurrenceCount++
	state.ResolvedAt = time.Time{}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO observability_alert_states (
	alert_key, status, severity, original_severity, escalated, title, component,
	opened_at, last_occurred_at, occurrence_count, acknowledged_at, resolved_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL, NOW())
ON CONFLICT (alert_key) DO UPDATE SET
	status = EXCLUDED.status,
	severity = EXCLUDED.severity,
	original_severity = EXCLUDED.original_severity,
	escalated = EXCLUDED.escalated,
	title = EXCLUDED.title,
	component = EXCLUDED.component,
	last_occurred_at = EXCLUDED.last_occurred_at,
	occurrence_count = observability_alert_states.occurrence_count + 1,
	resolved_at = NULL,
	updated_at = NOW()
`, state.Key, state.Status, state.Severity, state.OriginalSeverity, state.Escalated, state.Title, state.Component, state.OpenedAt, state.LastOccurredAt, state.OccurrenceCount, nullableTime(state.AcknowledgedAt)); err != nil {
		return AlertState{}, fmt.Errorf("upsert alert state: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO observability_alert_occurrences (alert_key, occurred_at)
VALUES ($1, $2)
`, key, event.OccurredAt); err != nil {
		return AlertState{}, fmt.Errorf("record alert occurrence: %w", err)
	}

	cutoff := event.OccurredAt.Add(-s.escalationWindow)
	var occurrences int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM observability_alert_occurrences
WHERE alert_key = $1 AND occurred_at >= $2
`, key, cutoff).Scan(&occurrences); err != nil {
		return AlertState{}, fmt.Errorf("count alert occurrences: %w", err)
	}
	if occurrences >= s.escalationLimit {
		escalated := escalateSeverity(event.Severity)
		if escalated != event.Severity {
			state.OriginalSeverity = event.Severity
			state.Severity = escalated
			state.Escalated = true
			if _, err := tx.ExecContext(ctx, `
UPDATE observability_alert_states
SET severity = $2, original_severity = $3, escalated = true, updated_at = NOW()
WHERE alert_key = $1
`, key, state.Severity, state.OriginalSeverity); err != nil {
				return AlertState{}, fmt.Errorf("persist alert escalation: %w", err)
			}
		}
	}
	if shouldEscalateSustainedWarning(state, event.OccurredAt, s.warningOpenWindow) {
		state.OriginalSeverity = AlertSeverityWarning
		state.Severity = AlertSeverityCritical
		state.Escalated = true
		if _, err := tx.ExecContext(ctx, `
UPDATE observability_alert_states
SET severity = $2, original_severity = $3, escalated = true, updated_at = NOW()
WHERE alert_key = $1
`, key, state.Severity, state.OriginalSeverity); err != nil {
			return AlertState{}, fmt.Errorf("persist sustained warning escalation: %w", err)
		}
	}

	if err := scanAlertState(tx.QueryRowContext(ctx, selectAlertStateSQL+` WHERE alert_key = $1`, key), &state); err != nil {
		return AlertState{}, fmt.Errorf("reload alert state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return AlertState{}, fmt.Errorf("commit alert state update: %w", err)
	}
	return cloneAlertState(state), nil
}

func (s *SQLAlertStateStore) GetAlertState(ctx context.Context, key string) (AlertState, bool, error) {
	if s == nil || s.db == nil {
		return AlertState{}, false, errors.New("alert state store database is required")
	}
	var state AlertState
	err := scanAlertState(s.db.QueryRowContext(ctx, selectAlertStateSQL+` WHERE alert_key = $1`, key), &state)
	if err == sql.ErrNoRows {
		return AlertState{}, false, nil
	}
	if err != nil {
		return AlertState{}, false, fmt.Errorf("get alert state: %w", err)
	}
	return cloneAlertState(state), true, nil
}

func (s *SQLAlertStateStore) ListAlertStates(ctx context.Context, filter AlertStateFilter) ([]AlertState, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("alert state store database is required")
	}

	query := selectAlertStateSQL
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 6)
	addCondition := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}

	if filter.Status != "" {
		addCondition("status = $%d", filter.Status)
	}
	if filter.Severity != "" {
		addCondition("severity = $%d", filter.Severity)
	}
	if filter.Component != "" {
		addCondition("component = $%d", filter.Component)
	}
	if filter.KeyPrefix != "" {
		addCondition("alert_key LIKE $%d", filter.KeyPrefix+"%")
	}
	if !filter.From.IsZero() {
		addCondition("last_occurred_at >= $%d", filter.From)
	}
	if !filter.To.IsZero() {
		addCondition("last_occurred_at <= $%d", filter.To)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY last_occurred_at DESC, updated_at DESC, alert_key ASC"
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list alert states: %w", err)
	}
	defer rows.Close()

	states := []AlertState{}
	for rows.Next() {
		var state AlertState
		if err := scanAlertState(rows, &state); err != nil {
			return nil, fmt.Errorf("scan alert state: %w", err)
		}
		states = append(states, cloneAlertState(state))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert states: %w", err)
	}
	return states, nil
}

func (s *SQLAlertStateStore) AcknowledgeAlert(ctx context.Context, key string, acknowledgedAt time.Time) (AlertState, error) {
	if s == nil || s.db == nil {
		return AlertState{}, errors.New("alert state store database is required")
	}
	if acknowledgedAt.IsZero() {
		acknowledgedAt = time.Now().UTC()
	}
	var state AlertState
	err := scanAlertState(s.db.QueryRowContext(ctx, `
UPDATE observability_alert_states
SET status = $2, acknowledged_at = $3, updated_at = NOW()
WHERE alert_key = $1
RETURNING alert_key, status, severity, original_severity, escalated, title, component,
	opened_at, last_occurred_at, occurrence_count, acknowledged_at, resolved_at
`, key, AlertStatusAcknowledged, acknowledgedAt), &state)
	if err == sql.ErrNoRows {
		return AlertState{}, errors.New("alert state not found")
	}
	if err != nil {
		return AlertState{}, fmt.Errorf("acknowledge alert: %w", err)
	}
	return cloneAlertState(state), nil
}

func (s *SQLAlertStateStore) ResolveAlert(ctx context.Context, key string, resolvedAt time.Time) (AlertState, error) {
	if s == nil || s.db == nil {
		return AlertState{}, errors.New("alert state store database is required")
	}
	if resolvedAt.IsZero() {
		resolvedAt = time.Now().UTC()
	}
	var state AlertState
	err := scanAlertState(s.db.QueryRowContext(ctx, `
UPDATE observability_alert_states
SET status = $2, resolved_at = $3, updated_at = NOW()
WHERE alert_key = $1
RETURNING alert_key, status, severity, original_severity, escalated, title, component,
	opened_at, last_occurred_at, occurrence_count, acknowledged_at, resolved_at
`, key, AlertStatusResolved, resolvedAt), &state)
	if err == sql.ErrNoRows {
		return AlertState{}, errors.New("alert state not found")
	}
	if err != nil {
		return AlertState{}, fmt.Errorf("resolve alert: %w", err)
	}
	return cloneAlertState(state), nil
}

func (s *SQLAlertStateStore) RecordNotification(ctx context.Context, event AlertEvent, window time.Duration) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("alert state store database is required")
	}
	key := alertKey(event)
	if key == "" {
		return true, nil
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	notifyKey := string(event.Severity) + ":" + key

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin notification state update: %w", err)
	}
	defer tx.Rollback()

	var lastNotifiedAt time.Time
	err = tx.QueryRowContext(ctx, `
SELECT last_notified_at FROM observability_notification_states
WHERE notify_key = $1
FOR UPDATE
`, notifyKey).Scan(&lastNotifiedAt)
	switch {
	case err == nil && window > 0 && event.OccurredAt.Sub(lastNotifiedAt) < window:
		return false, tx.Commit()
	case err == nil:
	case err == sql.ErrNoRows:
	default:
		return false, fmt.Errorf("get notification state: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO observability_notification_states (notify_key, alert_key, severity, last_notified_at, updated_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (notify_key) DO UPDATE SET
	last_notified_at = EXCLUDED.last_notified_at,
	updated_at = NOW()
`, notifyKey, key, event.Severity, event.OccurredAt); err != nil {
		return false, fmt.Errorf("upsert notification state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit notification state update: %w", err)
	}
	return true, nil
}

func (s *SQLAlertStateStore) RecordDeliveryAttempts(ctx context.Context, event AlertEvent, results []AlertDeliveryResult) error {
	if s == nil || s.db == nil {
		return errors.New("alert state store database is required")
	}
	key := alertKey(event)
	if key == "" {
		return errors.New("alert key is required")
	}
	if event.Severity == "" {
		event.Severity = AlertSeverityInfo
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}

	for i, result := range results {
		if _, err := s.db.ExecContext(ctx, `
INSERT INTO observability_alert_delivery_attempts (
	id, alert_key, severity, component, channel, provider_id, provider_kind, delivered, error, attempted_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`, makeAlertDeliveryAttemptID(key, event.OccurredAt, result.Channel, i+1), key, event.Severity, event.Component, result.Channel, result.ProviderID, result.ProviderKind, result.Delivered, alertDeliveryResultError(result), event.OccurredAt); err != nil {
			return fmt.Errorf("insert alert delivery attempt: %w", err)
		}
	}
	return nil
}

func (s *SQLAlertStateStore) ListDeliveryAttempts(ctx context.Context, filter AlertDeliveryHistoryFilter) ([]AlertDeliveryAttempt, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("alert state store database is required")
	}

	query := `
SELECT id, alert_key, severity, component, channel, provider_id, provider_kind, delivered, error, attempted_at
FROM observability_alert_delivery_attempts`
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 6)
	addCondition := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if filter.AlertKey != "" {
		addCondition("alert_key = $%d", filter.AlertKey)
	}
	if filter.KeyPrefix != "" {
		addCondition("alert_key LIKE $%d", filter.KeyPrefix+"%")
	}
	if !filter.From.IsZero() {
		addCondition("attempted_at >= $%d", filter.From)
	}
	if !filter.To.IsZero() {
		addCondition("attempted_at <= $%d", filter.To)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY attempted_at DESC, id ASC"
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list alert delivery attempts: %w", err)
	}
	defer rows.Close()

	attempts := []AlertDeliveryAttempt{}
	for rows.Next() {
		var attempt AlertDeliveryAttempt
		if err := rows.Scan(&attempt.ID, &attempt.AlertKey, &attempt.Severity, &attempt.Component, &attempt.Channel, &attempt.ProviderID, &attempt.ProviderKind, &attempt.Delivered, &attempt.Error, &attempt.AttemptedAt); err != nil {
			return nil, fmt.Errorf("scan alert delivery attempt: %w", err)
		}
		attempts = append(attempts, cloneAlertDeliveryAttempt(attempt))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert delivery attempts: %w", err)
	}
	return attempts, nil
}

func (s *SQLAlertStateStore) RecordRecoveryAction(ctx context.Context, action RecoveryAction, cooldown time.Duration) (bool, RecoveryAction, error) {
	if s == nil || s.db == nil {
		return false, RecoveryAction{}, errors.New("alert state store database is required")
	}
	if action.AlertKey == "" || action.PolicyName == "" {
		return false, RecoveryAction{}, errors.New("recovery action alert key and policy name are required")
	}
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, RecoveryAction{}, fmt.Errorf("begin recovery action update: %w", err)
	}
	defer tx.Rollback()

	last, found, err := s.getLastRecoveryAction(ctx, tx, action.PolicyName, action.AlertKey)
	if err != nil {
		return false, RecoveryAction{}, err
	}
	if found && cooldown > 0 && action.CreatedAt.Sub(last.CreatedAt) < cooldown {
		return false, cloneRecoveryAction(last), tx.Commit()
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO observability_recovery_actions (
	id, policy_name, alert_key, severity, component, action_type, status, reason, attempt, next_attempt_at, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`, action.ID, action.PolicyName, action.AlertKey, action.Severity, action.Component, action.Type, action.Status, action.Reason, action.Attempt, nullableTime(action.NextAttemptAt), action.CreatedAt); err != nil {
		return false, RecoveryAction{}, fmt.Errorf("insert recovery action: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, RecoveryAction{}, fmt.Errorf("commit recovery action update: %w", err)
	}
	return true, cloneRecoveryAction(action), nil
}

func (s *SQLAlertStateStore) ListRecoveryActions(ctx context.Context, filter RecoveryActionFilter) ([]RecoveryAction, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("alert state store database is required")
	}

	query := `
SELECT id, policy_name, alert_key, severity, component, action_type, status, reason, attempt, next_attempt_at, created_at
FROM observability_recovery_actions`
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 6)
	addCondition := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if filter.AlertKey != "" {
		addCondition("alert_key = $%d", filter.AlertKey)
	}
	if filter.KeyPrefix != "" {
		addCondition("alert_key LIKE $%d", filter.KeyPrefix+"%")
	}
	if filter.PolicyName != "" {
		addCondition("policy_name = $%d", filter.PolicyName)
	}
	if filter.Component != "" {
		addCondition("component = $%d", filter.Component)
	}
	if filter.Type != "" {
		addCondition("action_type = $%d", filter.Type)
	}
	if !filter.From.IsZero() {
		addCondition("created_at >= $%d", filter.From)
	}
	if !filter.To.IsZero() {
		addCondition("created_at <= $%d", filter.To)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC, id ASC"
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list recovery actions: %w", err)
	}
	defer rows.Close()

	actions := []RecoveryAction{}
	for rows.Next() {
		var action RecoveryAction
		var nextAttemptAt sql.NullTime
		if err := rows.Scan(&action.ID, &action.PolicyName, &action.AlertKey, &action.Severity, &action.Component, &action.Type, &action.Status, &action.Reason, &action.Attempt, &nextAttemptAt, &action.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recovery action: %w", err)
		}
		if nextAttemptAt.Valid {
			action.NextAttemptAt = nextAttemptAt.Time
		}
		actions = append(actions, cloneRecoveryAction(action))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recovery actions: %w", err)
	}
	return actions, nil
}

func (s *SQLAlertStateStore) getAlertStateForUpdate(ctx context.Context, tx *sql.Tx, key string) (AlertState, bool, error) {
	var state AlertState
	err := scanAlertState(tx.QueryRowContext(ctx, selectAlertStateSQL+` WHERE alert_key = $1 FOR UPDATE`, key), &state)
	if err == sql.ErrNoRows {
		return AlertState{}, false, nil
	}
	if err != nil {
		return AlertState{}, false, fmt.Errorf("get alert state for update: %w", err)
	}
	return state, true, nil
}

func (s *SQLAlertStateStore) getLastRecoveryAction(ctx context.Context, tx *sql.Tx, policyName, alertKey string) (RecoveryAction, bool, error) {
	var action RecoveryAction
	var nextAttemptAt sql.NullTime
	err := tx.QueryRowContext(ctx, `
SELECT id, policy_name, alert_key, severity, component, action_type, status, reason, attempt, next_attempt_at, created_at
FROM observability_recovery_actions
WHERE policy_name = $1 AND alert_key = $2
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE
`, policyName, alertKey).Scan(&action.ID, &action.PolicyName, &action.AlertKey, &action.Severity, &action.Component, &action.Type, &action.Status, &action.Reason, &action.Attempt, &nextAttemptAt, &action.CreatedAt)
	if err == sql.ErrNoRows {
		return RecoveryAction{}, false, nil
	}
	if err != nil {
		return RecoveryAction{}, false, fmt.Errorf("get last recovery action: %w", err)
	}
	if nextAttemptAt.Valid {
		action.NextAttemptAt = nextAttemptAt.Time
	}
	return cloneRecoveryAction(action), true, nil
}

const selectAlertStateSQL = `
SELECT alert_key, status, severity, original_severity, escalated, title, component,
	opened_at, last_occurred_at, occurrence_count, acknowledged_at, resolved_at
FROM observability_alert_states`

type sqlScanner interface {
	Scan(dest ...any) error
}

func scanAlertState(row sqlScanner, state *AlertState) error {
	var acknowledgedAt sql.NullTime
	var resolvedAt sql.NullTime
	if err := row.Scan(
		&state.Key,
		&state.Status,
		&state.Severity,
		&state.OriginalSeverity,
		&state.Escalated,
		&state.Title,
		&state.Component,
		&state.OpenedAt,
		&state.LastOccurredAt,
		&state.OccurrenceCount,
		&acknowledgedAt,
		&resolvedAt,
	); err != nil {
		return err
	}
	if acknowledgedAt.Valid {
		state.AcknowledgedAt = acknowledgedAt.Time
	} else {
		state.AcknowledgedAt = time.Time{}
	}
	if resolvedAt.Valid {
		state.ResolvedAt = resolvedAt.Time
	} else {
		state.ResolvedAt = time.Time{}
	}
	return nil
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
