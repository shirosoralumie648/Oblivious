package task

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

func (s *SQLStore) ListTasks(ctx context.Context, workspaceID string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, goal, execution_mode, authorization_scope, status, budget_limit, budget_consumed, result_summary, started_at, finished_at, created_at, updated_at
		FROM tasks
		WHERE workspace_id = $1
		ORDER BY updated_at DESC, created_at DESC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var current Task
		if err := rows.Scan(
			&current.ID,
			&current.Title,
			&current.Goal,
			&current.ExecutionMode,
			&current.AuthorizationScope,
			&current.Status,
			&current.BudgetLimit,
			&current.BudgetConsumed,
			&current.ResultSummary,
			&current.StartedAt,
			&current.FinishedAt,
			&current.CreatedAt,
			&current.UpdatedAt,
		); err != nil {
			return nil, err
		}

		tasks = append(tasks, current)
	}

	return tasks, rows.Err()
}

func (s *SQLStore) GetTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	taskRow, toolAllowList, toolDenyList, err := s.getTaskRow(ctx, workspaceID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}

	knowledgeBaseIDs, err := s.listTaskKnowledgeBaseIDs(ctx, workspaceID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}

	steps, err := s.listTaskSteps(ctx, workspaceID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}

	return TaskDetail{
		Task:             taskRow,
		KnowledgeBaseIDs: knowledgeBaseIDs,
		Steps:            steps,
		ToolAllowList:    toolAllowList,
		ToolDenyList:     toolDenyList,
	}, nil
}

func (s *SQLStore) CreateTask(
	ctx context.Context,
	workspaceID,
	title,
	goal,
	executionMode string,
	authorizationScope string,
	budgetLimit int,
	knowledgeBaseIDs []string,
	toolAllowList []string,
	toolDenyList []string,
) (Task, error) {
	taskID, err := auth.NewID("task")
	if err != nil {
		return Task{}, err
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO tasks (
			id,
			workspace_id,
			user_id,
			title,
			goal,
			mode,
			execution_mode,
			authorization_scope,
			tool_allow_list,
			tool_deny_list,
			status,
			budget_limit,
			budget_consumed,
			result_summary,
			created_at,
			updated_at
		)
		SELECT $1, w.id, w.user_id, $3, $4, 'solo', $5, $6, $7, $8, 'draft', $9, 0, '', $10, $10
		FROM workspaces w
		WHERE w.id = $2
	`, taskID, workspaceID, title, goal, executionMode, authorizationScope, pq.Array(toolAllowList), pq.Array(toolDenyList), budgetLimit, now)
	if err != nil {
		return Task{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Task{}, err
	}
	if rowsAffected == 0 {
		return Task{}, sql.ErrNoRows
	}

	for _, knowledgeBaseID := range knowledgeBaseIDs {
		bindingID, err := auth.NewID("tkb")
		if err != nil {
			return Task{}, err
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO task_knowledge_bindings (id, task_id, knowledge_base_id, created_at)
			SELECT $1, t.id, kb.id, $4
			FROM tasks t
			JOIN knowledge_bases kb ON kb.workspace_id = t.workspace_id
			WHERE t.id = $2 AND t.workspace_id = $3 AND kb.id = $5
		`, bindingID, taskID, workspaceID, now, knowledgeBaseID)
		if err != nil {
			return Task{}, err
		}

		bindingRowsAffected, err := result.RowsAffected()
		if err != nil {
			return Task{}, err
		}
		if bindingRowsAffected == 0 {
			return Task{}, sql.ErrNoRows
		}
	}

	if err := tx.Commit(); err != nil {
		return Task{}, err
	}

	return Task{
		AuthorizationScope: authorizationScope,
		BudgetConsumed:     0,
		BudgetLimit:        budgetLimit,
		CreatedAt:          now,
		ExecutionMode:      executionMode,
		Goal:               goal,
		ID:                 taskID,
		Status:             "draft",
		Title:              title,
		UpdatedAt:          now,
	}, nil
}

func (s *SQLStore) StartTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TaskDetail{}, err
	}
	defer tx.Rollback()

	current, err := s.getRuntimeTaskDetailTx(ctx, tx, workspaceID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}

	switch current.Status {
	case "draft", "completed", "cancelled":
		if err := s.resetRuntimeTaskTx(ctx, tx, workspaceID, taskID, current, now); err != nil {
			return TaskDetail{}, err
		}
	case "running":
		next, err := continueRuntimeTask(current, now)
		if err != nil {
			return TaskDetail{}, err
		}
		if err := s.persistRuntimeTaskTx(ctx, tx, workspaceID, taskID, next); err != nil {
			return TaskDetail{}, err
		}
	default:
		return TaskDetail{}, sql.ErrNoRows
	}

	if err := tx.Commit(); err != nil {
		return TaskDetail{}, err
	}

	return s.GetTask(ctx, workspaceID, taskID)
}

func (s *SQLStore) ApproveTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TaskDetail{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE tasks AS t
		SET status = 'running',
			budget_consumed = CASE
				WHEN t.budget_limit > 0 THEN GREATEST(1, LEAST(t.budget_limit, (t.budget_limit + 3) / 4))
				ELSE 0
			END,
			updated_at = $3
		WHERE t.workspace_id = $1 AND t.id = $2 AND t.status = 'awaiting_confirmation'
	`, workspaceID, taskID, now)
	if err != nil {
		return TaskDetail{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return TaskDetail{}, err
	}
	if rowsAffected == 0 {
		return TaskDetail{}, sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE task_steps
		SET status = 'completed',
			updated_at = $2,
			started_at = COALESCE(started_at, $2),
			finished_at = COALESCE(finished_at, $2)
		WHERE task_id = $1 AND status = 'awaiting_confirmation'
	`, taskID, now); err != nil {
		return TaskDetail{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE task_steps
		SET status = 'running',
			updated_at = $2,
			started_at = COALESCE(started_at, $2)
		WHERE task_id = $1
			AND step_index = (
				SELECT step_index
				FROM task_steps
				WHERE task_id = $1 AND status = 'pending'
				ORDER BY step_index ASC
				LIMIT 1
			)
	`, taskID, now); err != nil {
		return TaskDetail{}, err
	}

	if err := tx.Commit(); err != nil {
		return TaskDetail{}, err
	}

	return s.GetTask(ctx, workspaceID, taskID)
}

func (s *SQLStore) PauseTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TaskDetail{}, err
	}
	defer tx.Rollback()

	current, err := s.getRuntimeTaskDetailTx(ctx, tx, workspaceID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}

	next, err := pauseRuntimeTask(current, now)
	if err != nil {
		if err == ErrInvalidTaskTransition {
			return TaskDetail{}, sql.ErrNoRows
		}
		return TaskDetail{}, err
	}

	if err := s.persistRuntimeTaskTx(ctx, tx, workspaceID, taskID, next); err != nil {
		return TaskDetail{}, err
	}

	if err := tx.Commit(); err != nil {
		return TaskDetail{}, err
	}

	return s.GetTask(ctx, workspaceID, taskID)
}

func (s *SQLStore) ResumeTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TaskDetail{}, err
	}
	defer tx.Rollback()

	current, err := s.getRuntimeTaskDetailTx(ctx, tx, workspaceID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}

	next, err := resumeRuntimeTask(current, now)
	if err != nil {
		if err == ErrInvalidTaskTransition {
			return TaskDetail{}, sql.ErrNoRows
		}
		return TaskDetail{}, err
	}

	if err := s.persistRuntimeTaskTx(ctx, tx, workspaceID, taskID, next); err != nil {
		return TaskDetail{}, err
	}

	if err := tx.Commit(); err != nil {
		return TaskDetail{}, err
	}

	return s.GetTask(ctx, workspaceID, taskID)
}

func (s *SQLStore) CancelTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TaskDetail{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = 'cancelled',
			finished_at = $3,
			updated_at = $3
		WHERE workspace_id = $1 AND id = $2 AND status IN ('running', 'paused', 'draft', 'awaiting_confirmation')
	`, workspaceID, taskID, now)
	if err != nil {
		return TaskDetail{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return TaskDetail{}, err
	}
	if rowsAffected == 0 {
		return TaskDetail{}, sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE task_steps
		SET status = 'cancelled',
			updated_at = $2,
			finished_at = COALESCE(finished_at, $2)
		WHERE task_id = $1 AND status IN ('running', 'paused', 'pending', 'awaiting_confirmation')
	`, taskID, now); err != nil {
		return TaskDetail{}, err
	}

	if err := tx.Commit(); err != nil {
		return TaskDetail{}, err
	}

	return s.GetTask(ctx, workspaceID, taskID)
}

func (s *SQLStore) UpdateTaskBudget(ctx context.Context, workspaceID, taskID string, budgetLimit int) (TaskDetail, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE tasks
		SET budget_limit = $3,
			updated_at = $4
		WHERE workspace_id = $1
			AND id = $2
			AND status IN ('draft', 'running', 'paused', 'awaiting_confirmation')
	`, workspaceID, taskID, budgetLimit, time.Now().UTC())
	if err != nil {
		return TaskDetail{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return TaskDetail{}, err
	}
	if rowsAffected == 0 {
		return TaskDetail{}, sql.ErrNoRows
	}

	return s.GetTask(ctx, workspaceID, taskID)
}

func (s *SQLStore) getRuntimeTaskDetailTx(ctx context.Context, tx *sql.Tx, workspaceID, taskID string) (TaskDetail, error) {
	var taskRow Task
	if err := tx.QueryRowContext(ctx, `
		SELECT id, title, goal, execution_mode, authorization_scope, status, budget_limit, budget_consumed, result_summary, started_at, finished_at, created_at, updated_at
		FROM tasks
		WHERE workspace_id = $1 AND id = $2
	`, workspaceID, taskID).Scan(
		&taskRow.ID,
		&taskRow.Title,
		&taskRow.Goal,
		&taskRow.ExecutionMode,
		&taskRow.AuthorizationScope,
		&taskRow.Status,
		&taskRow.BudgetLimit,
		&taskRow.BudgetConsumed,
		&taskRow.ResultSummary,
		&taskRow.StartedAt,
		&taskRow.FinishedAt,
		&taskRow.CreatedAt,
		&taskRow.UpdatedAt,
	); err != nil {
		return TaskDetail{}, err
	}

	steps, err := s.listTaskStepsTx(ctx, tx, workspaceID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}

	return TaskDetail{
		Task:  taskRow,
		Steps: steps,
	}, nil
}

func (s *SQLStore) listTaskStepsTx(ctx context.Context, tx *sql.Tx, workspaceID, taskID string) ([]TaskStep, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT ts.id, ts.step_index, ts.title, ts.status, ts.created_at, ts.updated_at, ts.started_at, ts.finished_at
		FROM task_steps ts
		JOIN tasks t ON t.id = ts.task_id
		WHERE ts.task_id = $1 AND t.workspace_id = $2
		ORDER BY ts.step_index ASC, ts.created_at ASC
	`, taskID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	steps := []TaskStep{}
	for rows.Next() {
		var step TaskStep
		if err := rows.Scan(
			&step.ID,
			&step.StepIndex,
			&step.Title,
			&step.Status,
			&step.CreatedAt,
			&step.UpdatedAt,
			&step.StartedAt,
			&step.FinishedAt,
		); err != nil {
			return nil, err
		}

		steps = append(steps, step)
	}

	return steps, rows.Err()
}

func (s *SQLStore) resetRuntimeTaskTx(ctx context.Context, tx *sql.Tx, workspaceID, taskID string, current TaskDetail, now time.Time) error {
	initialStatus := "running"
	initialBudget := 0
	if current.ExecutionMode == "safe" {
		initialStatus = "awaiting_confirmation"
	} else {
		initialBudget = nextRuntimeBudget(current.BudgetLimit, 0, false)
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = $3,
			budget_consumed = $4,
			started_at = $5,
			finished_at = NULL,
			result_summary = '',
			updated_at = $5
		WHERE workspace_id = $1 AND id = $2
	`, workspaceID, taskID, initialStatus, initialBudget, now)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM task_steps
		WHERE task_id = $1
	`, taskID); err != nil {
		return err
	}

	stepTitles, stepStatuses := runtimePlanForMode(current.ExecutionMode)
	for index, title := range stepTitles {
		stepID, err := auth.NewID("step")
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO task_steps (
				id,
				task_id,
				step_index,
				title,
				status,
				created_at,
				updated_at,
				started_at,
				finished_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $6, $7, $8)
		`, stepID, taskID, index+1, title, stepStatuses[index], now, startedAtForStatus(stepStatuses[index], now), finishedAtForStatus(stepStatuses[index], now)); err != nil {
			return err
		}
	}

	return nil
}

func (s *SQLStore) persistRuntimeTaskTx(ctx context.Context, tx *sql.Tx, workspaceID, taskID string, detail TaskDetail) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = $3,
			budget_consumed = $4,
			started_at = $5,
			finished_at = $6,
			result_summary = $7,
			updated_at = $8
		WHERE workspace_id = $1 AND id = $2
	`, workspaceID, taskID, detail.Status, detail.BudgetConsumed, detail.StartedAt, detail.FinishedAt, detail.ResultSummary, detail.UpdatedAt)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	for _, step := range detail.Steps {
		result, err := tx.ExecContext(ctx, `
			UPDATE task_steps
			SET title = $2,
				status = $3,
				updated_at = $4,
				started_at = $5,
				finished_at = $6
			WHERE id = $1
		`, step.ID, step.Title, step.Status, step.UpdatedAt, step.StartedAt, step.FinishedAt)
		if err != nil {
			return err
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return sql.ErrNoRows
		}
	}

	return nil
}

func (s *SQLStore) getTaskRow(ctx context.Context, workspaceID, taskID string) (Task, []string, []string, error) {
	var taskRow Task
	toolAllowList := []string{}
	toolDenyList := []string{}
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, title, goal, execution_mode, authorization_scope, tool_allow_list, tool_deny_list, status, budget_limit, budget_consumed, result_summary, started_at, finished_at, created_at, updated_at
		FROM tasks
		WHERE workspace_id = $1 AND id = $2
	`, workspaceID, taskID).Scan(
		&taskRow.ID,
		&taskRow.Title,
		&taskRow.Goal,
		&taskRow.ExecutionMode,
		&taskRow.AuthorizationScope,
		pq.Array(&toolAllowList),
		pq.Array(&toolDenyList),
		&taskRow.Status,
		&taskRow.BudgetLimit,
		&taskRow.BudgetConsumed,
		&taskRow.ResultSummary,
		&taskRow.StartedAt,
		&taskRow.FinishedAt,
		&taskRow.CreatedAt,
		&taskRow.UpdatedAt,
	); err != nil {
		return Task{}, nil, nil, err
	}

	return taskRow, toolAllowList, toolDenyList, nil
}

func (s *SQLStore) listTaskKnowledgeBaseIDs(ctx context.Context, workspaceID, taskID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tkb.knowledge_base_id
		FROM task_knowledge_bindings tkb
		JOIN tasks t ON t.id = tkb.task_id
		JOIN knowledge_bases kb ON kb.id = tkb.knowledge_base_id
		WHERE tkb.task_id = $1 AND t.workspace_id = $2 AND kb.workspace_id = $2
		ORDER BY tkb.created_at ASC, tkb.knowledge_base_id ASC
	`, taskID, workspaceID)
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

func (s *SQLStore) listTaskSteps(ctx context.Context, workspaceID, taskID string) ([]TaskStep, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts.id, ts.step_index, ts.title, ts.status, ts.created_at, ts.updated_at, ts.started_at, ts.finished_at
		FROM task_steps ts
		JOIN tasks t ON t.id = ts.task_id
		WHERE ts.task_id = $1 AND t.workspace_id = $2
		ORDER BY ts.step_index ASC, ts.created_at ASC
	`, taskID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	steps := []TaskStep{}
	for rows.Next() {
		var step TaskStep
		if err := rows.Scan(
			&step.ID,
			&step.StepIndex,
			&step.Title,
			&step.Status,
			&step.CreatedAt,
			&step.UpdatedAt,
			&step.StartedAt,
			&step.FinishedAt,
		); err != nil {
			return nil, err
		}

		steps = append(steps, step)
	}

	return steps, rows.Err()
}

func startedAtForStatus(status string, now time.Time) *time.Time {
	if status == "completed" || status == "running" {
		value := now
		return &value
	}

	return nil
}

func finishedAtForStatus(status string, now time.Time) *time.Time {
	if status == "completed" {
		value := now
		return &value
	}

	return nil
}

func runtimePlanForMode(executionMode string) ([]string, []string) {
	if executionMode == "safe" {
		return []string{
			"Understand the goal",
			"Confirm execution boundary",
			"Deliver runtime result",
		}, []string{"completed", "awaiting_confirmation", "pending"}
	}

	return []string{
		"Understand the goal",
		"Review workspace context",
		"Deliver runtime result",
	}, []string{"completed", "running", "pending"}
}
