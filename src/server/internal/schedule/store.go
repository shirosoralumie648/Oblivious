package schedule

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	"oblivious/server/internal/auth"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

type scheduledTaskScanner interface {
	Scan(dest ...any) error
}

func scanScheduledTask(scanner scheduledTaskScanner) (ScheduledTask, error) {
	var task ScheduledTask
	err := scanner.Scan(
		&task.ID,
		&task.OrganizationID,
		&task.Name,
		&task.TargetType,
		&task.TargetID,
		&task.WorkflowTriggerID,
		&task.CronExpression,
		&task.Enabled,
		&task.LastRunAt,
		&task.NextRunAt,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	return task, err
}

func (s *SQLStore) CreateScheduledTask(ctx context.Context, input CreateScheduledTaskInput) (ScheduledTask, error) {
	id, err := auth.NewID("sched")
	if err != nil {
		return ScheduledTask{}, err
	}

	now := time.Now().UTC()
	task, err := scanScheduledTask(s.db.QueryRowContext(ctx, `
		INSERT INTO scheduled_tasks (
			id,
			organization_id,
			name,
			target_type,
			target_id,
			cron_expression,
			enabled,
			next_run_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		RETURNING id, organization_id, name, target_type, target_id, COALESCE(workflow_trigger_id, ''), cron_expression, enabled, last_run_at, next_run_at, created_at, updated_at
	`, id, input.OrganizationID, input.Name, input.TargetType, input.TargetID, input.CronExpression, input.Enabled, input.NextRunAt, now))
	if err != nil {
		return ScheduledTask{}, err
	}
	return task, nil
}

func (s *SQLStore) SyncWorkflowScheduledTasks(ctx context.Context, input SyncWorkflowScheduledTasksInput) ([]ScheduledTask, error) {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

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

	triggerIDs := make([]string, 0, len(input.Triggers))
	tasks := make([]ScheduledTask, 0, len(input.Triggers))
	for _, trigger := range input.Triggers {
		triggerIDs = append(triggerIDs, trigger.TriggerID)
		id, err := auth.NewID("sched")
		if err != nil {
			return nil, err
		}
		task, err := scanScheduledTask(tx.QueryRowContext(ctx, `
			INSERT INTO scheduled_tasks (
				id,
				organization_id,
				name,
				target_type,
				target_id,
				workflow_trigger_id,
				cron_expression,
				enabled,
				next_run_at,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
			ON CONFLICT (organization_id, target_type, target_id, workflow_trigger_id)
			    WHERE target_type = 'workflow' AND workflow_trigger_id IS NOT NULL
			DO UPDATE SET
				name = EXCLUDED.name,
				cron_expression = EXCLUDED.cron_expression,
				enabled = EXCLUDED.enabled,
				next_run_at = EXCLUDED.next_run_at,
				updated_at = EXCLUDED.updated_at
			RETURNING id, organization_id, name, target_type, target_id, COALESCE(workflow_trigger_id, ''), cron_expression, enabled, last_run_at, next_run_at, created_at, updated_at
		`, id, input.OrganizationID, trigger.Name, TargetTypeWorkflow, input.WorkflowID, trigger.TriggerID, trigger.CronExpression, trigger.Enabled, trigger.NextRunAt, now))
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	if len(triggerIDs) == 0 {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM scheduled_tasks
			WHERE organization_id = $1
			  AND target_type = $2
			  AND target_id = $3
			  AND workflow_trigger_id IS NOT NULL
		`, input.OrganizationID, TargetTypeWorkflow, input.WorkflowID); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM scheduled_tasks
			WHERE organization_id = $1
			  AND target_type = $2
			  AND target_id = $3
			  AND workflow_trigger_id IS NOT NULL
			  AND workflow_trigger_id <> ALL($4)
		`, input.OrganizationID, TargetTypeWorkflow, input.WorkflowID, pq.Array(triggerIDs)); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return tasks, nil
}

func (s *SQLStore) ListScheduledTasks(ctx context.Context, organizationID string) ([]ScheduledTask, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, name, target_type, target_id, COALESCE(workflow_trigger_id, ''), cron_expression, enabled, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_tasks
		WHERE organization_id = $1
		ORDER BY next_run_at ASC NULLS LAST, created_at DESC, id DESC
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []ScheduledTask{}
	for rows.Next() {
		task, err := scanScheduledTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *SQLStore) GetScheduledTask(ctx context.Context, organizationID string, scheduledTaskID string) (ScheduledTask, error) {
	task, err := scanScheduledTask(s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, name, target_type, target_id, COALESCE(workflow_trigger_id, ''), cron_expression, enabled, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_tasks
		WHERE organization_id = $1 AND id = $2
	`, organizationID, scheduledTaskID))
	if err != nil {
		return ScheduledTask{}, err
	}
	return task, nil
}

func (s *SQLStore) UpdateScheduledTaskEnabled(ctx context.Context, organizationID string, scheduledTaskID string, input UpdateScheduledTaskEnabledInput) (ScheduledTask, error) {
	now := time.Now().UTC()
	task, err := scanScheduledTask(s.db.QueryRowContext(ctx, `
		UPDATE scheduled_tasks
		SET enabled = $3,
		    next_run_at = $4,
		    updated_at = $5
		WHERE organization_id = $1 AND id = $2
		RETURNING id, organization_id, name, target_type, target_id, COALESCE(workflow_trigger_id, ''), cron_expression, enabled, last_run_at, next_run_at, created_at, updated_at
	`, organizationID, scheduledTaskID, input.Enabled, input.NextRunAt, now))
	if err != nil {
		return ScheduledTask{}, err
	}
	return task, nil
}

func (s *SQLStore) ClaimDueScheduledTaskRuns(ctx context.Context, input ClaimDueScheduledTaskRunsInput) ([]ClaimedScheduledTaskRun, error) {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	limit := input.Limit
	if limit <= 0 {
		limit = defaultDueTaskClaimLimit
	}

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
		SELECT task.id, task.organization_id, task.name, task.target_type, task.target_id, COALESCE(task.workflow_trigger_id, ''), task.cron_expression, task.enabled, task.last_run_at, task.next_run_at, task.created_at, task.updated_at
		FROM scheduled_tasks task
		WHERE task.enabled = TRUE
		  AND task.next_run_at IS NOT NULL
		  AND task.next_run_at <= $1
		  AND NOT EXISTS (
		      SELECT 1
		      FROM scheduled_task_runs run
		      WHERE run.organization_id = task.organization_id
		        AND run.scheduled_task_id = task.id
		        AND run.status = $2
		  )
		ORDER BY task.next_run_at ASC, task.created_at ASC, task.id ASC
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	`, now, RunStatusRunning, limit)
	if err != nil {
		return nil, err
	}

	tasks := []ScheduledTask{}
	for rows.Next() {
		task, err := scanScheduledTask(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	claimed := make([]ClaimedScheduledTaskRun, 0, len(tasks))
	for _, task := range tasks {
		id, err := auth.NewID("schedrun")
		if err != nil {
			return nil, err
		}

		var run ScheduledTaskRun
		err = tx.QueryRowContext(ctx, `
			INSERT INTO scheduled_task_runs (
				id,
				organization_id,
				scheduled_task_id,
				status,
				started_at,
				finished_at,
				error,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $5, NULL, '', $5, $5)
			RETURNING id, organization_id, scheduled_task_id, status, started_at, finished_at, error, created_at, updated_at
		`, id, task.OrganizationID, task.ID, RunStatusRunning, now).Scan(
			&run.ID,
			&run.OrganizationID,
			&run.ScheduledTaskID,
			&run.Status,
			&run.StartedAt,
			&run.FinishedAt,
			&run.Error,
			&run.CreatedAt,
			&run.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		claimed = append(claimed, ClaimedScheduledTaskRun{Task: task, Run: run})
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return claimed, nil
}

func (s *SQLStore) RecordScheduledTaskRun(ctx context.Context, input RecordScheduledTaskRunInput) (ScheduledTaskRun, error) {
	id, err := auth.NewID("schedrun")
	if err != nil {
		return ScheduledTaskRun{}, err
	}

	now := time.Now().UTC()
	var run ScheduledTaskRun
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO scheduled_task_runs (
			id,
			organization_id,
			scheduled_task_id,
			status,
			started_at,
			finished_at,
			error,
			created_at,
			updated_at
		)
		SELECT $1, task.organization_id, task.id, $4, $5, $6, $7, $8, $8
		FROM scheduled_tasks task
		WHERE task.organization_id = $2 AND task.id = $3
		RETURNING id, organization_id, scheduled_task_id, status, started_at, finished_at, error, created_at, updated_at
	`, id, input.OrganizationID, input.ScheduledTaskID, input.Status, input.StartedAt, input.FinishedAt, input.Error, now).Scan(
		&run.ID,
		&run.OrganizationID,
		&run.ScheduledTaskID,
		&run.Status,
		&run.StartedAt,
		&run.FinishedAt,
		&run.Error,
		&run.CreatedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		return ScheduledTaskRun{}, err
	}
	return run, nil
}

func (s *SQLStore) CompleteScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskID string, scheduledTaskRunID string, input CompleteScheduledTaskRunInput) (ScheduledTaskRun, error) {
	finishedAt := input.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	} else {
		finishedAt = finishedAt.UTC()
	}
	nextRunAt := input.NextRunAt
	if nextRunAt.IsZero() {
		nextRunAt = finishedAt
	} else {
		nextRunAt = nextRunAt.UTC()
	}
	updatedAt := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ScheduledTaskRun{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var run ScheduledTaskRun
	err = tx.QueryRowContext(ctx, `
		UPDATE scheduled_task_runs
		SET status = $4,
		    finished_at = $5,
		    error = '',
		    updated_at = $6
		WHERE organization_id = $1 AND scheduled_task_id = $2 AND id = $3 AND status = $7
		RETURNING id, organization_id, scheduled_task_id, status, started_at, finished_at, error, created_at, updated_at
	`, organizationID, scheduledTaskID, scheduledTaskRunID, RunStatusCompleted, finishedAt, updatedAt, RunStatusRunning).Scan(
		&run.ID,
		&run.OrganizationID,
		&run.ScheduledTaskID,
		&run.Status,
		&run.StartedAt,
		&run.FinishedAt,
		&run.Error,
		&run.CreatedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		return ScheduledTaskRun{}, err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE scheduled_tasks
		SET last_run_at = $3,
		    next_run_at = $4,
		    updated_at = $5
		WHERE organization_id = $1 AND id = $2
	`, organizationID, scheduledTaskID, finishedAt, nextRunAt, updatedAt)
	if err != nil {
		return ScheduledTaskRun{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ScheduledTaskRun{}, err
	}
	if affected == 0 {
		return ScheduledTaskRun{}, sql.ErrNoRows
	}

	if err := tx.Commit(); err != nil {
		return ScheduledTaskRun{}, err
	}
	committed = true
	return run, nil
}

func (s *SQLStore) CompleteManualScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskID string, scheduledTaskRunID string, finishedAt time.Time) (ScheduledTaskRun, error) {
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	} else {
		finishedAt = finishedAt.UTC()
	}
	updatedAt := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ScheduledTaskRun{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var run ScheduledTaskRun
	err = tx.QueryRowContext(ctx, `
		UPDATE scheduled_task_runs
		SET status = $4,
		    finished_at = $5,
		    error = '',
		    updated_at = $6
		WHERE organization_id = $1 AND scheduled_task_id = $2 AND id = $3 AND status = $7
		RETURNING id, organization_id, scheduled_task_id, status, started_at, finished_at, error, created_at, updated_at
	`, organizationID, scheduledTaskID, scheduledTaskRunID, RunStatusCompleted, finishedAt, updatedAt, RunStatusRunning).Scan(
		&run.ID,
		&run.OrganizationID,
		&run.ScheduledTaskID,
		&run.Status,
		&run.StartedAt,
		&run.FinishedAt,
		&run.Error,
		&run.CreatedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		return ScheduledTaskRun{}, err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE scheduled_tasks
		SET last_run_at = $3,
		    updated_at = $4
		WHERE organization_id = $1 AND id = $2
	`, organizationID, scheduledTaskID, finishedAt, updatedAt)
	if err != nil {
		return ScheduledTaskRun{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ScheduledTaskRun{}, err
	}
	if affected == 0 {
		return ScheduledTaskRun{}, sql.ErrNoRows
	}

	if err := tx.Commit(); err != nil {
		return ScheduledTaskRun{}, err
	}
	committed = true
	return run, nil
}

func (s *SQLStore) UpdateScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskRunID string, input UpdateScheduledTaskRunInput) (ScheduledTaskRun, error) {
	now := time.Now().UTC()
	var run ScheduledTaskRun
	err := s.db.QueryRowContext(ctx, `
		UPDATE scheduled_task_runs
		SET status = $3,
		    finished_at = $4,
		    error = $5,
		    updated_at = $6
		WHERE organization_id = $1 AND id = $2
		RETURNING id, organization_id, scheduled_task_id, status, started_at, finished_at, error, created_at, updated_at
	`, organizationID, scheduledTaskRunID, input.Status, input.FinishedAt, input.Error, now).Scan(
		&run.ID,
		&run.OrganizationID,
		&run.ScheduledTaskID,
		&run.Status,
		&run.StartedAt,
		&run.FinishedAt,
		&run.Error,
		&run.CreatedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		return ScheduledTaskRun{}, err
	}
	return run, nil
}

func (s *SQLStore) ListScheduledTaskRuns(ctx context.Context, organizationID string, scheduledTaskID string) ([]ScheduledTaskRun, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, scheduled_task_id, status, started_at, finished_at, error, created_at, updated_at
		FROM scheduled_task_runs
		WHERE organization_id = $1 AND scheduled_task_id = $2
		ORDER BY created_at DESC, id DESC
	`, organizationID, scheduledTaskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []ScheduledTaskRun{}
	for rows.Next() {
		var run ScheduledTaskRun
		if err := rows.Scan(
			&run.ID,
			&run.OrganizationID,
			&run.ScheduledTaskID,
			&run.Status,
			&run.StartedAt,
			&run.FinishedAt,
			&run.Error,
			&run.CreatedAt,
			&run.UpdatedAt,
		); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *SQLStore) CountRunningScheduledTaskRuns(ctx context.Context, organizationID string, scheduledTaskID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM scheduled_task_runs
		WHERE organization_id = $1 AND scheduled_task_id = $2 AND status = $3
	`, organizationID, scheduledTaskID, RunStatusRunning).Scan(&count)
	return count, err
}
