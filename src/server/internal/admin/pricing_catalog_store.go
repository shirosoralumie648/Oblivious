package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *SQLStore) ListActiveRelayPricingCatalogEntries(ctx context.Context) ([]RelayPricingCatalogEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, api_type, model, dimension, unit_cost, markup, currency, source, active, effective_from
		FROM relay_pricing_entries
		WHERE active = true
		ORDER BY api_type, model, dimension, effective_from DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list active relay pricing catalog entries: %w", err)
	}
	defer rows.Close()

	entries := []RelayPricingCatalogEntry{}
	for rows.Next() {
		var entry RelayPricingCatalogEntry
		var effectiveFrom time.Time
		if err := rows.Scan(
			&entry.ID,
			&entry.APIType,
			&entry.Model,
			&entry.Dimension,
			&entry.UnitCost,
			&entry.Markup,
			&entry.Currency,
			&entry.Source,
			&entry.Active,
			&effectiveFrom,
		); err != nil {
			return nil, fmt.Errorf("scan relay pricing catalog entry: %w", err)
		}
		entry.EffectiveFrom = &effectiveFrom
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *SQLStore) CreateRelayPricingCatalogImport(ctx context.Context, catalogImport RelayPricingCatalogImport) (*RelayPricingCatalogImport, error) {
	entries, err := json.Marshal(catalogImport.Entries)
	if err != nil {
		return nil, fmt.Errorf("encode relay pricing catalog import entries: %w", err)
	}
	diff, err := json.Marshal(catalogImport.Diff)
	if err != nil {
		return nil, fmt.Errorf("encode relay pricing catalog import diff: %w", err)
	}
	if catalogImport.CreatedAt.IsZero() {
		catalogImport.CreatedAt = time.Now().UTC()
	}
	if catalogImport.Status == "" {
		catalogImport.Status = "pending"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin relay pricing catalog import: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO relay_pricing_catalog_imports (
			id, provider, source, source_hash, status, notes, deactivate_missing,
			imported_by, imported_by_email, entries, diff, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11::jsonb, $12)
	`, catalogImport.ID, catalogImport.Provider, catalogImport.Source, catalogImport.SourceHash, catalogImport.Status, catalogImport.Notes,
		catalogImport.DeactivateMissing, catalogImport.ImportedBy, catalogImport.ImportedByEmail, string(entries), string(diff), catalogImport.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create relay pricing catalog import: %w", err)
	}
	if err := s.insertRelayPricingCatalogEvent(ctx, tx, relayPricingCatalogEvent{
		ImportID:   catalogImport.ID,
		Action:     "import_created",
		ActorID:    catalogImport.ImportedBy,
		ActorEmail: catalogImport.ImportedByEmail,
		After:      catalogImport,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit relay pricing catalog import: %w", err)
	}
	return &catalogImport, nil
}

func (s *SQLStore) GetRelayPricingCatalogImport(ctx context.Context, importID string) (*RelayPricingCatalogImport, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, provider, source, source_hash, status, notes, deactivate_missing,
		       imported_by, imported_by_email, approved_by, approved_by_email,
		       entries, diff, created_at, approved_at
		FROM relay_pricing_catalog_imports
		WHERE id = $1
	`, importID)
	catalogImport, err := scanRelayPricingCatalogImport(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRelayPricingCatalogImportNotFound
	}
	return catalogImport, err
}

func (s *SQLStore) ListRelayPricingCatalogImports(ctx context.Context, filter RelayPricingCatalogImportFilter) ([]*RelayPricingCatalogImport, int, error) {
	filter = normalizeRelayPricingCatalogImportFilter(filter)
	where, args := relayPricingCatalogImportWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM relay_pricing_catalog_imports `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count relay pricing catalog imports: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider, source, source_hash, status, notes, deactivate_missing,
		       imported_by, imported_by_email, approved_by, approved_by_email,
		       entries, diff, created_at, approved_at
		FROM relay_pricing_catalog_imports
		`+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), append(args, filter.Limit, filter.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list relay pricing catalog imports: %w", err)
	}
	defer rows.Close()

	imports := []*RelayPricingCatalogImport{}
	for rows.Next() {
		catalogImport, err := scanRelayPricingCatalogImport(rows)
		if err != nil {
			return nil, 0, err
		}
		imports = append(imports, catalogImport)
	}
	return imports, total, rows.Err()
}

func (s *SQLStore) CreateRelayPricingCatalogSyncRun(ctx context.Context, run RelayPricingCatalogSyncRun) (*RelayPricingCatalogSyncRun, error) {
	run = normalizeRelayPricingCatalogSyncRun(run)
	metadata := "{}"
	if len(run.Metadata) > 0 && json.Valid(run.Metadata) {
		metadata = string(run.Metadata)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO relay_pricing_sync_runs (
			id, job, provider, source, source_ref, source_hash, status, import_id,
			entry_count, skipped_count, checked_records, issue_count, error, metadata,
			started_at, finished_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''),
		        $9, $10, $11, $12, $13, $14::jsonb, $15, $16)
	`, run.ID, run.Job, run.Provider, run.Source, run.SourceRef, run.SourceHash, run.Status, run.ImportID,
		run.EntryCount, run.SkippedCount, run.CheckedRecords, run.IssueCount, run.Error, metadata,
		run.StartedAt, run.FinishedAt); err != nil {
		return nil, fmt.Errorf("create relay pricing sync run: %w", err)
	}
	if len(run.Metadata) == 0 {
		run.Metadata = json.RawMessage(metadata)
	}
	return &run, nil
}

func (s *SQLStore) ListRelayPricingCatalogSyncRuns(ctx context.Context, filter RelayPricingCatalogSyncRunFilter) ([]*RelayPricingCatalogSyncRun, int, error) {
	filter = normalizeRelayPricingCatalogSyncRunFilter(filter)
	where, args := relayPricingCatalogSyncRunWhere(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM relay_pricing_sync_runs `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count relay pricing sync runs: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, job, provider, source, source_ref, source_hash, status,
		       COALESCE(import_id, ''), entry_count, skipped_count, checked_records,
		       issue_count, error, metadata, started_at, finished_at
		FROM relay_pricing_sync_runs
		`+where+`
		ORDER BY finished_at DESC, id DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), append(args, filter.Limit, filter.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list relay pricing sync runs: %w", err)
	}
	defer rows.Close()

	runs := []*RelayPricingCatalogSyncRun{}
	for rows.Next() {
		run, err := scanRelayPricingCatalogSyncRun(rows)
		if err != nil {
			return nil, 0, err
		}
		runs = append(runs, run)
	}
	return runs, total, rows.Err()
}

func (s *SQLStore) ApproveRelayPricingCatalogImport(ctx context.Context, importID, actorID, actorEmail string) (*RelayPricingCatalogImport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin relay pricing catalog approval: %w", err)
	}
	defer tx.Rollback()

	catalogImport, err := s.getRelayPricingCatalogImportForUpdate(ctx, tx, importID)
	if err != nil {
		return nil, err
	}
	if catalogImport.Status != "pending" {
		return nil, ErrRelayPricingCatalogImportNotPending
	}

	for i := range catalogImport.Diff.Entries {
		diffEntry := &catalogImport.Diff.Entries[i]
		switch diffEntry.Action {
		case "add", "update":
			if diffEntry.After == nil {
				return nil, fmt.Errorf("relay pricing catalog diff %s missing after entry", diffEntry.Key)
			}
			if diffEntry.Action == "update" {
				if _, err := tx.ExecContext(ctx, `
					UPDATE relay_pricing_entries
					SET active = false, updated_at = NOW()
					WHERE api_type = $1 AND model = $2 AND dimension = $3 AND active = true
				`, diffEntry.After.APIType, diffEntry.After.Model, diffEntry.After.Dimension); err != nil {
					return nil, fmt.Errorf("deactivate old relay pricing entry: %w", err)
				}
			}
			appliedID, err := upsertRelayPricingEntry(ctx, tx, *diffEntry.After)
			if err != nil {
				return nil, err
			}
			diffEntry.Applied = true
			diffEntry.AppliedID = appliedID
			after := *diffEntry.After
			after.ID = appliedID
			action := "entry_added"
			if diffEntry.Action == "update" {
				action = "entry_updated"
			}
			if err := s.insertRelayPricingCatalogEvent(ctx, tx, relayPricingCatalogEvent{
				ImportID:       catalogImport.ID,
				PricingEntryID: appliedID,
				Action:         action,
				ActorID:        actorID,
				ActorEmail:     actorEmail,
				Before:         diffEntry.Before,
				After:          after,
			}); err != nil {
				return nil, err
			}
		case "deactivate":
			if diffEntry.Before == nil {
				return nil, fmt.Errorf("relay pricing catalog diff %s missing before entry", diffEntry.Key)
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE relay_pricing_entries
				SET active = false, updated_at = NOW()
				WHERE id = $1
			`, diffEntry.Before.ID); err != nil {
				return nil, fmt.Errorf("deactivate relay pricing entry: %w", err)
			}
			diffEntry.Applied = true
			diffEntry.AppliedID = diffEntry.Before.ID
			if err := s.insertRelayPricingCatalogEvent(ctx, tx, relayPricingCatalogEvent{
				ImportID:       catalogImport.ID,
				PricingEntryID: diffEntry.Before.ID,
				Action:         "entry_deactivated",
				ActorID:        actorID,
				ActorEmail:     actorEmail,
				Before:         diffEntry.Before,
			}); err != nil {
				return nil, err
			}
		case "unchanged":
			// No catalog mutation is needed for unchanged rows, but the diff is
			// kept for approval evidence.
		default:
			return nil, fmt.Errorf("unsupported relay pricing catalog diff action: %s", diffEntry.Action)
		}
	}

	now := time.Now().UTC()
	diffJSON, err := json.Marshal(catalogImport.Diff)
	if err != nil {
		return nil, fmt.Errorf("encode approved relay pricing catalog diff: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE relay_pricing_catalog_imports
		SET status = 'approved',
		    approved_by = $2,
		    approved_by_email = $3,
		    approved_at = $4,
		    diff = $5::jsonb
		WHERE id = $1
	`, catalogImport.ID, actorID, actorEmail, now, string(diffJSON)); err != nil {
		return nil, fmt.Errorf("approve relay pricing catalog import: %w", err)
	}
	catalogImport.Status = "approved"
	catalogImport.ApprovedBy = actorID
	catalogImport.ApprovedByEmail = actorEmail
	catalogImport.ApprovedAt = &now
	if err := s.insertRelayPricingCatalogEvent(ctx, tx, relayPricingCatalogEvent{
		ImportID:   catalogImport.ID,
		Action:     "approved",
		ActorID:    actorID,
		ActorEmail: actorEmail,
		After:      catalogImport,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit relay pricing catalog approval: %w", err)
	}
	return catalogImport, nil
}

func (s *SQLStore) RejectRelayPricingCatalogImport(ctx context.Context, importID, actorID, actorEmail, reason string) (*RelayPricingCatalogImport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin relay pricing catalog rejection: %w", err)
	}
	defer tx.Rollback()

	catalogImport, err := s.getRelayPricingCatalogImportForUpdate(ctx, tx, importID)
	if err != nil {
		return nil, err
	}
	if catalogImport.Status != "pending" {
		return nil, ErrRelayPricingCatalogImportNotPending
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE relay_pricing_catalog_imports
		SET status = 'rejected'
		WHERE id = $1
	`, catalogImport.ID); err != nil {
		return nil, fmt.Errorf("reject relay pricing catalog import: %w", err)
	}
	catalogImport.Status = "rejected"
	if err := s.insertRelayPricingCatalogEvent(ctx, tx, relayPricingCatalogEvent{
		ImportID:   catalogImport.ID,
		Action:     "rejected",
		ActorID:    actorID,
		ActorEmail: actorEmail,
		Before:     map[string]any{"status": "pending"},
		After: map[string]any{
			"status": "rejected",
			"reason": strings.TrimSpace(reason),
		},
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit relay pricing catalog rejection: %w", err)
	}
	return catalogImport, nil
}

type relayPricingCatalogEvent struct {
	ImportID       string
	PricingEntryID string
	Action         string
	ActorID        string
	ActorEmail     string
	Before         any
	After          any
}

type relayPricingCatalogImportScanner interface {
	Scan(dest ...any) error
}

func relayPricingCatalogImportWhere(filter RelayPricingCatalogImportFilter) (string, []any) {
	conditions := []string{}
	args := []any{}
	add := func(column, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		args = append(args, strings.TrimSpace(value))
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	add("provider", filter.Provider)
	add("source", filter.Source)
	add("status", filter.Status)
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func relayPricingCatalogSyncRunWhere(filter RelayPricingCatalogSyncRunFilter) (string, []any) {
	conditions := []string{}
	args := []any{}
	add := func(column, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		args = append(args, strings.TrimSpace(value))
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	add("job", filter.Job)
	add("provider", filter.Provider)
	add("source", filter.Source)
	add("status", filter.Status)
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func scanRelayPricingCatalogImport(scanner relayPricingCatalogImportScanner) (*RelayPricingCatalogImport, error) {
	var catalogImport RelayPricingCatalogImport
	var entriesJSON []byte
	var diffJSON []byte
	var approvedAt sql.NullTime
	if err := scanner.Scan(
		&catalogImport.ID,
		&catalogImport.Provider,
		&catalogImport.Source,
		&catalogImport.SourceHash,
		&catalogImport.Status,
		&catalogImport.Notes,
		&catalogImport.DeactivateMissing,
		&catalogImport.ImportedBy,
		&catalogImport.ImportedByEmail,
		&catalogImport.ApprovedBy,
		&catalogImport.ApprovedByEmail,
		&entriesJSON,
		&diffJSON,
		&catalogImport.CreatedAt,
		&approvedAt,
	); err != nil {
		return nil, fmt.Errorf("scan relay pricing catalog import: %w", err)
	}
	if len(entriesJSON) > 0 {
		if err := json.Unmarshal(entriesJSON, &catalogImport.Entries); err != nil {
			return nil, fmt.Errorf("decode relay pricing catalog import entries: %w", err)
		}
	}
	if len(diffJSON) > 0 {
		if err := json.Unmarshal(diffJSON, &catalogImport.Diff); err != nil {
			return nil, fmt.Errorf("decode relay pricing catalog import diff: %w", err)
		}
	}
	if catalogImport.Entries == nil {
		catalogImport.Entries = []RelayPricingCatalogEntry{}
	}
	if catalogImport.Diff.Entries == nil {
		catalogImport.Diff.Entries = []RelayPricingCatalogDiffEntry{}
	}
	if approvedAt.Valid {
		catalogImport.ApprovedAt = &approvedAt.Time
	}
	return &catalogImport, nil
}

func scanRelayPricingCatalogSyncRun(scanner relayPricingCatalogImportScanner) (*RelayPricingCatalogSyncRun, error) {
	var run RelayPricingCatalogSyncRun
	var metadata []byte
	if err := scanner.Scan(
		&run.ID,
		&run.Job,
		&run.Provider,
		&run.Source,
		&run.SourceRef,
		&run.SourceHash,
		&run.Status,
		&run.ImportID,
		&run.EntryCount,
		&run.SkippedCount,
		&run.CheckedRecords,
		&run.IssueCount,
		&run.Error,
		&metadata,
		&run.StartedAt,
		&run.FinishedAt,
	); err != nil {
		return nil, fmt.Errorf("scan relay pricing sync run: %w", err)
	}
	if len(metadata) > 0 && json.Valid(metadata) {
		run.Metadata = json.RawMessage(append([]byte(nil), metadata...))
	}
	return &run, nil
}

func normalizeRelayPricingCatalogSyncRun(run RelayPricingCatalogSyncRun) RelayPricingCatalogSyncRun {
	run.Job = strings.ToLower(strings.TrimSpace(run.Job))
	run.Provider = strings.ToLower(strings.TrimSpace(run.Provider))
	run.Source = strings.TrimSpace(run.Source)
	run.SourceRef = strings.TrimSpace(run.SourceRef)
	run.SourceHash = strings.TrimSpace(run.SourceHash)
	run.Status = strings.ToLower(strings.TrimSpace(run.Status))
	run.ImportID = strings.TrimSpace(run.ImportID)
	run.Error = strings.TrimSpace(run.Error)
	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	if run.Job == "" {
		run.Job = "freshness"
	}
	if run.Status == "" {
		run.Status = "succeeded"
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	if run.FinishedAt.IsZero() {
		run.FinishedAt = time.Now().UTC()
	}
	if run.FinishedAt.Before(run.StartedAt) {
		run.FinishedAt = run.StartedAt
	}
	if run.EntryCount < 0 {
		run.EntryCount = 0
	}
	if run.SkippedCount < 0 {
		run.SkippedCount = 0
	}
	if run.CheckedRecords < 0 {
		run.CheckedRecords = 0
	}
	if run.IssueCount < 0 {
		run.IssueCount = 0
	}
	return run
}

func (s *SQLStore) getRelayPricingCatalogImportForUpdate(ctx context.Context, tx *sql.Tx, importID string) (*RelayPricingCatalogImport, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, provider, source, source_hash, status, notes, deactivate_missing,
		       imported_by, imported_by_email, approved_by, approved_by_email,
		       entries, diff, created_at, approved_at
		FROM relay_pricing_catalog_imports
		WHERE id = $1
		FOR UPDATE
	`, importID)
	catalogImport, err := scanRelayPricingCatalogImport(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRelayPricingCatalogImportNotFound
	}
	return catalogImport, err
}

func upsertRelayPricingEntry(ctx context.Context, tx *sql.Tx, entry RelayPricingCatalogEntry) (string, error) {
	effectiveFrom := time.Now().UTC()
	if entry.EffectiveFrom != nil && !entry.EffectiveFrom.IsZero() {
		effectiveFrom = entry.EffectiveFrom.UTC()
	}
	var id string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO relay_pricing_entries (id, api_type, model, dimension, unit_cost, markup, currency, source, active, effective_from, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true, $9, NOW())
		ON CONFLICT (api_type, model, dimension, effective_from) DO UPDATE SET
			unit_cost = EXCLUDED.unit_cost,
			markup = EXCLUDED.markup,
			currency = EXCLUDED.currency,
			source = EXCLUDED.source,
			active = true,
			updated_at = NOW()
		RETURNING id
	`, entry.ID, entry.APIType, entry.Model, entry.Dimension, entry.UnitCost, entry.Markup, entry.Currency, entry.Source, effectiveFrom).Scan(&id); err != nil {
		return "", fmt.Errorf("upsert relay pricing entry: %w", err)
	}
	return id, nil
}

func (s *SQLStore) insertRelayPricingCatalogEvent(ctx context.Context, tx *sql.Tx, event relayPricingCatalogEvent) error {
	before, err := marshalCatalogEventPayload(event.Before)
	if err != nil {
		return err
	}
	after, err := marshalCatalogEventPayload(event.After)
	if err != nil {
		return err
	}
	exec := func(query string, args ...any) (sql.Result, error) {
		if tx != nil {
			return tx.ExecContext(ctx, query, args...)
		}
		return s.db.ExecContext(ctx, query, args...)
	}
	_, err = exec(`
		INSERT INTO relay_pricing_catalog_events (
			id, import_id, pricing_entry_id, action, actor_id, actor_email, before, after, created_at
		)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, $5, $6, $7::jsonb, $8::jsonb, NOW())
	`, uuid.New().String(), event.ImportID, event.PricingEntryID, event.Action, event.ActorID, event.ActorEmail, before, after)
	if err != nil {
		return fmt.Errorf("insert relay pricing catalog event: %w", err)
	}
	return nil
}

func marshalCatalogEventPayload(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode relay pricing catalog event payload: %w", err)
	}
	return string(data), nil
}
