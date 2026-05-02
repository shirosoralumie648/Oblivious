package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"oblivious/server/internal/auth"
)

// AuditStore defines operations on the audit log.
type AuditStore interface {
	CreateAuditEntry(ctx context.Context, entry *AuditEntry) error
	ListAuditEntries(ctx context.Context, filter AuditFilter) ([]*AuditEntry, int, error)
}

// CreateAuditEntry inserts a new audit log entry.
func (s *SQLStore) CreateAuditEntry(ctx context.Context, entry *AuditEntry) error {
	if entry.ID == "" {
		id, err := auth.NewID("aud")
		if err != nil {
			return fmt.Errorf("generate audit id: %w", err)
		}
		entry.ID = id
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_logs (id, actor_id, actor_email, action, resource_type, resource_id, changes, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, entry.ID, entry.ActorID, entry.ActorEmail, entry.Action,
		entry.ResourceType, nullIfEmpty(entry.ResourceID),
		nullIfEmpty(entry.Changes), nullIfEmpty(entry.IPAddress),
		entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("create audit entry: %w", err)
	}
	return nil
}

// ListAuditEntries returns audit entries with filters and total count.
func (s *SQLStore) ListAuditEntries(ctx context.Context, filter AuditFilter) ([]*AuditEntry, int, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.ActorID != "" {
		conditions = append(conditions, fmt.Sprintf("actor_id = $%d", argIdx))
		args = append(args, filter.ActorID)
		argIdx++
	}
	if filter.Action != "" {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIdx))
		args = append(args, filter.Action)
		argIdx++
	}
	if filter.ResourceType != "" {
		conditions = append(conditions, fmt.Sprintf("resource_type = $%d", argIdx))
		args = append(args, filter.ResourceType)
		argIdx++
	}
	if filter.ResourceID != "" {
		conditions = append(conditions, fmt.Sprintf("resource_id = $%d", argIdx))
		args = append(args, filter.ResourceID)
		argIdx++
	}
	if filter.DateFrom != "" {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d::timestamptz", argIdx))
		args = append(args, filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != "" {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d::timestamptz", argIdx))
		args = append(args, filter.DateTo)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total (without limit/offset)
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM audit_logs %s`, where)
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count audit entries: %w", err)
	}

	// Fetch page
	dataQuery := fmt.Sprintf(`
		SELECT id, actor_id, actor_email, action, resource_type,
		       COALESCE(resource_id, ''), COALESCE(changes, ''),
		       COALESCE(ip_address, ''), created_at
		FROM audit_logs
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit entries: %w", err)
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.ActorEmail, &e.Action,
			&e.ResourceType, &e.ResourceID, &e.Changes, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit entry: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, total, rows.Err()
}

// nullIfEmpty returns a sql.NullString that is NULL if the value is empty.
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return sql.NullString{}
	}
	return s
}
