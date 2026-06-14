package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/secretbox"

	"github.com/lib/pq"
)

type Store interface {
	CreateWorkflow(ctx context.Context, req CreateWorkflowRequest) (*WorkflowDefinition, error)
	GetWorkflow(ctx context.Context, organizationID, id string) (*WorkflowDefinition, error)
	ListWorkflows(ctx context.Context, organizationID string) ([]*WorkflowDefinition, error)
	ListWorkflowVersions(ctx context.Context, organizationID, workflowID string) ([]*WorkflowDefinition, error)
	GetWorkflowVersion(ctx context.Context, organizationID, workflowID string, version int) (*WorkflowDefinition, error)
	UpdateWorkflow(ctx context.Context, req UpdateWorkflowStoreRequest) (*WorkflowDefinition, error)
	CreateExecution(ctx context.Context, req CreateExecutionRequest) (*WorkflowExecution, error)
	ListExecutions(ctx context.Context, organizationID, workflowID string) ([]*WorkflowExecution, error)
	GetExecution(ctx context.Context, organizationID, id string) (*WorkflowExecution, error)
	ListActiveExecutionHealth(ctx context.Context, organizationID string, statuses []ExecutionStatus) ([]WorkflowExecutionHealthSummary, error)
	CountRunningExecutions(ctx context.Context, organizationID, workflowID string) (int, error)
	CountRunningExecutionsForOrganization(ctx context.Context, organizationID string) (int, error)
	UpdateExecutionStatus(ctx context.Context, organizationID, id string, status ExecutionStatus, completedAt *time.Time) (*WorkflowExecution, error)
	CreateNodeExecution(ctx context.Context, organizationID, executionID string, req CreateNodeExecutionRequest) (*WorkflowNodeExecution, error)
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

type CreateWorkflowRequest struct {
	OrganizationID string
	Name           string
	Description    string
	Status         WorkflowStatus
	Version        int
	Definition     map[string]any
	Variables      map[string]any
}

type UpdateWorkflowStoreRequest struct {
	OrganizationID string
	WorkflowID     string
	Name           string
	Description    string
	Status         WorkflowStatus
	Definition     map[string]any
	Variables      map[string]any
}

type CreateExecutionRequest struct {
	OrganizationID   string
	WorkflowID       string
	WorkflowVersion  int
	Status           ExecutionStatus
	Input            map[string]any
	Output           map[string]any
	Error            map[string]any
	Context          map[string]any
	WorkflowSnapshot map[string]any
	StartedAt        time.Time
	CompletedAt      *time.Time
	DurationMS       int
	NodeExecutions   []CreateNodeExecutionRequest
}

type CreateNodeExecutionRequest struct {
	NodeID      string
	NodeType    string
	Status      NodeStatus
	Attempt     int
	Input       map[string]any
	Output      map[string]any
	Error       map[string]any
	Context     map[string]any
	StartedAt   time.Time
	CompletedAt *time.Time
	DurationMS  int
}

func (s *SQLStore) CreateWorkflow(ctx context.Context, req CreateWorkflowRequest) (*WorkflowDefinition, error) {
	id, err := auth.NewID("workflow")
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	status := req.Status
	if status == "" {
		status = WorkflowStatusDraft
	}
	version := req.Version
	if version <= 0 {
		version = 1
	}

	definitionJSON, err := marshalWorkflowDefinitionForSQL(req.Definition)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow definition: %w", err)
	}
	variablesJSON, err := marshalJSONObject(req.Variables)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow variables: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflows (
			id, organization_id, name, description, status, version, definition, variables, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
	`, id, req.OrganizationID, req.Name, req.Description, string(status), version, definitionJSON, variablesJSON, now); err != nil {
		return nil, fmt.Errorf("insert workflow: %w", err)
	}

	if err := insertWorkflowVersion(ctx, tx, workflowVersionInsert{
		WorkflowID:     id,
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Description:    req.Description,
		Status:         status,
		Version:        version,
		DefinitionJSON: definitionJSON,
		VariablesJSON:  variablesJSON,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetWorkflow(ctx, req.OrganizationID, id)
}

func (s *SQLStore) GetWorkflow(ctx context.Context, organizationID, id string) (*WorkflowDefinition, error) {
	workflow, err := scanWorkflow(s.db.QueryRowContext(ctx, workflowSelectSQL+`
		WHERE id = $1 AND organization_id = $2
	`, id, organizationID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workflow: %w", err)
	}
	return workflow, nil
}

func (s *SQLStore) ListWorkflows(ctx context.Context, organizationID string) ([]*WorkflowDefinition, error) {
	rows, err := s.db.QueryContext(ctx, workflowSelectSQL+`
		WHERE organization_id = $1
		ORDER BY updated_at DESC, name ASC
	`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	defer rows.Close()

	workflows := []*WorkflowDefinition{}
	for rows.Next() {
		workflow, err := scanWorkflow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow: %w", err)
		}
		workflows = append(workflows, workflow)
	}
	return workflows, rows.Err()
}

func (s *SQLStore) ListWorkflowVersions(ctx context.Context, organizationID, workflowID string) ([]*WorkflowDefinition, error) {
	rows, err := s.db.QueryContext(ctx, workflowVersionSelectSQL+`
		WHERE workflow_id = $1 AND organization_id = $2
		ORDER BY version ASC
	`, workflowID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list workflow versions: %w", err)
	}
	defer rows.Close()

	versions := []*WorkflowDefinition{}
	for rows.Next() {
		version, err := scanWorkflowVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow version: %w", err)
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

func (s *SQLStore) GetWorkflowVersion(ctx context.Context, organizationID, workflowID string, version int) (*WorkflowDefinition, error) {
	workflowVersion, err := scanWorkflowVersion(s.db.QueryRowContext(ctx, workflowVersionSelectSQL+`
		WHERE workflow_id = $1 AND organization_id = $2 AND version = $3
	`, workflowID, organizationID, version))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workflow version: %w", err)
	}
	return workflowVersion, nil
}

func (s *SQLStore) UpdateWorkflow(ctx context.Context, req UpdateWorkflowStoreRequest) (*WorkflowDefinition, error) {
	definitionJSON, err := marshalWorkflowDefinitionForSQL(req.Definition)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow definition: %w", err)
	}
	variablesJSON, err := marshalJSONObject(req.Variables)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow variables: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var nextVersion int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM workflow_versions
		WHERE workflow_id = $1 AND organization_id = $2
	`, req.WorkflowID, req.OrganizationID).Scan(&nextVersion); err != nil {
		return nil, fmt.Errorf("next workflow version: %w", err)
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE workflows
		SET name = $1, description = $2, status = $3, version = $4, definition = $5, variables = $6, updated_at = $7
		WHERE id = $8 AND organization_id = $9
	`, req.Name, req.Description, string(req.Status), nextVersion, definitionJSON, variablesJSON, now, req.WorkflowID, req.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("update workflow: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("update workflow rows: %w", err)
	}
	if rowsAffected == 0 {
		return nil, nil
	}
	if err := insertWorkflowVersion(ctx, tx, workflowVersionInsert{
		WorkflowID:     req.WorkflowID,
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Description:    req.Description,
		Status:         req.Status,
		Version:        nextVersion,
		DefinitionJSON: definitionJSON,
		VariablesJSON:  variablesJSON,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetWorkflow(ctx, req.OrganizationID, req.WorkflowID)
}

func (s *SQLStore) CreateExecution(ctx context.Context, req CreateExecutionRequest) (*WorkflowExecution, error) {
	id, err := auth.NewID("wexec")
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	startedAt := req.StartedAt
	if startedAt.IsZero() {
		startedAt = now
	}
	status := req.Status
	if status == "" {
		status = ExecutionStatusRunning
	}

	inputJSON, err := marshalJSONObject(req.Input)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow execution input: %w", err)
	}
	outputJSON, err := marshalJSONObject(req.Output)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow execution output: %w", err)
	}
	errorJSON, err := marshalJSONObject(req.Error)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow execution error: %w", err)
	}
	contextJSON, err := marshalJSONObject(req.Context)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow execution context: %w", err)
	}
	workflowSnapshotJSON, err := marshalWorkflowDefinitionForSQL(req.WorkflowSnapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow execution snapshot: %w", err)
	}
	workflowVersion := req.WorkflowVersion
	if workflowVersion <= 0 {
		workflowVersion = 1
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_executions (
			id, workflow_id, workflow_version, organization_id, status, input, output, error, context, workflow_snapshot,
			started_at, completed_at, duration_ms, created_at, updated_at
		)
		SELECT $1, w.id, $4, w.organization_id, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14
		FROM workflows w
		WHERE w.id = $2 AND w.organization_id = $3
	`, id, req.WorkflowID, req.OrganizationID, workflowVersion, string(status), inputJSON, outputJSON, errorJSON, contextJSON,
		workflowSnapshotJSON, startedAt, req.CompletedAt, req.DurationMS, now)
	if err != nil {
		return nil, fmt.Errorf("insert workflow execution: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("insert workflow execution rows: %w", err)
	}
	if rowsAffected == 0 {
		return nil, sql.ErrNoRows
	}

	for _, node := range req.NodeExecutions {
		if err := insertNodeExecution(ctx, tx, req.OrganizationID, id, node, now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetExecution(ctx, req.OrganizationID, id)
}

func (s *SQLStore) GetExecution(ctx context.Context, organizationID, id string) (*WorkflowExecution, error) {
	execution, err := scanExecution(s.db.QueryRowContext(ctx, executionSelectSQL+`
		WHERE id = $1 AND organization_id = $2
	`, id, organizationID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workflow execution: %w", err)
	}
	nodes, err := s.listNodeExecutions(ctx, organizationID, execution.ID)
	if err != nil {
		return nil, err
	}
	execution.NodeExecutions = nodes
	return execution, nil
}

func (s *SQLStore) UpdateExecutionStatus(ctx context.Context, organizationID, id string, status ExecutionStatus, completedAt *time.Time) (*WorkflowExecution, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE workflow_executions
		SET status = $1, completed_at = $2, updated_at = $3
		WHERE id = $4 AND organization_id = $5
	`, string(status), completedAt, time.Now().UTC(), id, organizationID)
	if err != nil {
		return nil, fmt.Errorf("update workflow execution status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("update workflow execution status rows: %w", err)
	}
	if rowsAffected == 0 {
		return nil, nil
	}
	return s.GetExecution(ctx, organizationID, id)
}

func (s *SQLStore) ListExecutions(ctx context.Context, organizationID, workflowID string) ([]*WorkflowExecution, error) {
	rows, err := s.db.QueryContext(ctx, executionSelectSQL+`
		WHERE organization_id = $1 AND workflow_id = $2
		ORDER BY started_at DESC, created_at DESC
	`, organizationID, workflowID)
	if err != nil {
		return nil, fmt.Errorf("list workflow executions: %w", err)
	}
	defer rows.Close()

	executions := []*WorkflowExecution{}
	for rows.Next() {
		execution, err := scanExecution(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow execution: %w", err)
		}
		executions = append(executions, execution)
	}
	return executions, rows.Err()
}

func (s *SQLStore) ListActiveExecutionHealth(ctx context.Context, organizationID string, statuses []ExecutionStatus) ([]WorkflowExecutionHealthSummary, error) {
	if len(statuses) == 0 {
		return []WorkflowExecutionHealthSummary{}, nil
	}
	statusValues := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if status == "" {
			continue
		}
		statusValues = append(statusValues, string(status))
	}
	if len(statusValues) == 0 {
		return []WorkflowExecutionHealthSummary{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT status, COUNT(*), MIN(started_at)
		FROM workflow_executions
		WHERE ($1 = '' OR organization_id = $1) AND status = ANY($2)
		GROUP BY status
	`, organizationID, pq.Array(statusValues))
	if err != nil {
		return nil, fmt.Errorf("list active workflow execution health: %w", err)
	}
	defer rows.Close()

	summaries := []WorkflowExecutionHealthSummary{}
	for rows.Next() {
		var summary WorkflowExecutionHealthSummary
		var status string
		if err := rows.Scan(&status, &summary.Count, &summary.OldestStartedAt); err != nil {
			return nil, fmt.Errorf("scan active workflow execution health: %w", err)
		}
		summary.Status = ExecutionStatus(status)
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (s *SQLStore) CountRunningExecutions(ctx context.Context, organizationID, workflowID string) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM workflow_executions
		WHERE organization_id = $1 AND workflow_id = $2 AND status = $3
	`, organizationID, workflowID, string(ExecutionStatusRunning)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count running workflow executions: %w", err)
	}
	return count, nil
}

func (s *SQLStore) CountRunningExecutionsForOrganization(ctx context.Context, organizationID string) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM workflow_executions
		WHERE organization_id = $1 AND status = $2
	`, organizationID, string(ExecutionStatusRunning)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count running organization workflow executions: %w", err)
	}
	return count, nil
}

func insertNodeExecution(ctx context.Context, tx *sql.Tx, organizationID, executionID string, req CreateNodeExecutionRequest, now time.Time) error {
	id, err := auth.NewID("wnode")
	if err != nil {
		return err
	}
	status := req.Status
	if status == "" {
		status = NodeStatusPending
	}
	startedAt := req.StartedAt
	if startedAt.IsZero() {
		startedAt = now
	}

	inputJSON, err := marshalWorkflowNodePayloadForSQL(req.Input)
	if err != nil {
		return fmt.Errorf("marshal workflow node input: %w", err)
	}
	outputJSON, err := marshalWorkflowNodePayloadForSQL(req.Output)
	if err != nil {
		return fmt.Errorf("marshal workflow node output: %w", err)
	}
	errorJSON, err := marshalWorkflowNodePayloadForSQL(req.Error)
	if err != nil {
		return fmt.Errorf("marshal workflow node error: %w", err)
	}
	contextJSON, err := marshalWorkflowNodePayloadForSQL(req.Context)
	if err != nil {
		return fmt.Errorf("marshal workflow node context: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_node_executions (
			id, execution_id, organization_id, node_id, node_type, status, attempt,
			input, output, error, context, started_at, completed_at, duration_ms, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $15)
	`, id, executionID, organizationID, req.NodeID, req.NodeType, string(status), req.Attempt,
		inputJSON, outputJSON, errorJSON, contextJSON, startedAt, req.CompletedAt, req.DurationMS, now); err != nil {
		return fmt.Errorf("insert workflow node execution: %w", err)
	}
	return nil
}

func (s *SQLStore) CreateNodeExecution(ctx context.Context, organizationID, executionID string, req CreateNodeExecutionRequest) (*WorkflowNodeExecution, error) {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var executionIDValue string
	if err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM workflow_executions
		WHERE id = $1 AND organization_id = $2
	`, executionID, organizationID).Scan(&executionIDValue); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("verify workflow execution: %w", err)
	}

	if err := insertNodeExecution(ctx, tx, organizationID, executionID, req, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	nodes, err := s.listNodeExecutions(ctx, organizationID, executionID)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, nil
	}
	return &nodes[len(nodes)-1], nil
}

func (s *SQLStore) listNodeExecutions(ctx context.Context, organizationID, executionID string) ([]WorkflowNodeExecution, error) {
	rows, err := s.db.QueryContext(ctx, nodeExecutionSelectSQL+`
		WHERE organization_id = $1 AND execution_id = $2
		ORDER BY created_at ASC
	`, organizationID, executionID)
	if err != nil {
		return nil, fmt.Errorf("list workflow node executions: %w", err)
	}
	defer rows.Close()

	nodes := []WorkflowNodeExecution{}
	for rows.Next() {
		node, err := scanNodeExecution(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow node execution: %w", err)
		}
		nodes = append(nodes, *node)
	}
	return nodes, rows.Err()
}

const workflowSelectSQL = `
	SELECT id, organization_id, name, description, status, version, definition, variables, created_at, updated_at
	FROM workflows
`

const workflowVersionSelectSQL = `
	SELECT workflow_id, organization_id, name, description, status, version, definition, variables, created_at, updated_at
	FROM workflow_versions
`

const executionSelectSQL = `
	SELECT id, workflow_id, workflow_version, organization_id, status, input, output, error, context, workflow_snapshot,
		started_at, completed_at, duration_ms, created_at, updated_at
	FROM workflow_executions
`

const nodeExecutionSelectSQL = `
	SELECT id, execution_id, organization_id, node_id, node_type, status, attempt,
		input, output, error, context, started_at, completed_at, duration_ms, created_at, updated_at
	FROM workflow_node_executions
`

type scanner interface {
	Scan(dest ...any) error
}

func scanWorkflow(row scanner) (*WorkflowDefinition, error) {
	var workflow WorkflowDefinition
	var status string
	var definitionJSON, variablesJSON []byte
	if err := row.Scan(
		&workflow.ID, &workflow.OrganizationID, &workflow.Name, &workflow.Description, &status,
		&workflow.Version, &definitionJSON, &variablesJSON, &workflow.CreatedAt, &workflow.UpdatedAt,
	); err != nil {
		return nil, err
	}
	workflow.Status = WorkflowStatus(status)
	if err := unmarshalJSONObject(definitionJSON, &workflow.Definition); err != nil {
		return nil, err
	}
	openedDefinition, err := openWorkflowDefinitionSecrets(workflow.Definition)
	if err != nil {
		return nil, fmt.Errorf("open workflow definition %s: %w", workflow.ID, err)
	}
	workflow.Definition = openedDefinition
	if err := unmarshalJSONObject(variablesJSON, &workflow.Variables); err != nil {
		return nil, err
	}
	return &workflow, nil
}

type workflowVersionInsert struct {
	WorkflowID     string
	OrganizationID string
	Name           string
	Description    string
	Status         WorkflowStatus
	Version        int
	DefinitionJSON []byte
	VariablesJSON  []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func insertWorkflowVersion(ctx context.Context, tx *sql.Tx, req workflowVersionInsert) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_versions (
			workflow_id, organization_id, version, name, description, status, definition, variables, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, req.WorkflowID, req.OrganizationID, req.Version, req.Name, req.Description, string(req.Status),
		req.DefinitionJSON, req.VariablesJSON, req.CreatedAt, req.UpdatedAt); err != nil {
		return fmt.Errorf("insert workflow version: %w", err)
	}
	return nil
}

func scanWorkflowVersion(row scanner) (*WorkflowDefinition, error) {
	var workflow WorkflowDefinition
	var status string
	var definitionJSON, variablesJSON []byte
	if err := row.Scan(
		&workflow.ID, &workflow.OrganizationID, &workflow.Name, &workflow.Description, &status,
		&workflow.Version, &definitionJSON, &variablesJSON, &workflow.CreatedAt, &workflow.UpdatedAt,
	); err != nil {
		return nil, err
	}
	workflow.Status = WorkflowStatus(status)
	if err := unmarshalJSONObject(definitionJSON, &workflow.Definition); err != nil {
		return nil, err
	}
	openedDefinition, err := openWorkflowDefinitionSecrets(workflow.Definition)
	if err != nil {
		return nil, fmt.Errorf("open workflow version %s:%d: %w", workflow.ID, workflow.Version, err)
	}
	workflow.Definition = openedDefinition
	if err := unmarshalJSONObject(variablesJSON, &workflow.Variables); err != nil {
		return nil, err
	}
	return &workflow, nil
}

func scanExecution(row scanner) (*WorkflowExecution, error) {
	var execution WorkflowExecution
	var status string
	var inputJSON, outputJSON, errorJSON, contextJSON, workflowSnapshotJSON []byte
	if err := row.Scan(
		&execution.ID, &execution.WorkflowID, &execution.WorkflowVersion, &execution.OrganizationID, &status,
		&inputJSON, &outputJSON, &errorJSON, &contextJSON, &workflowSnapshotJSON, &execution.StartedAt,
		&execution.CompletedAt, &execution.DurationMS, &execution.CreatedAt, &execution.UpdatedAt,
	); err != nil {
		return nil, err
	}
	execution.Status = ExecutionStatus(status)
	if err := unmarshalJSONObject(inputJSON, &execution.Input); err != nil {
		return nil, err
	}
	if err := unmarshalJSONObject(outputJSON, &execution.Output); err != nil {
		return nil, err
	}
	if err := unmarshalJSONObject(errorJSON, &execution.Error); err != nil {
		return nil, err
	}
	if err := unmarshalJSONObject(contextJSON, &execution.Context); err != nil {
		return nil, err
	}
	if err := unmarshalJSONObject(workflowSnapshotJSON, &execution.WorkflowSnapshot); err != nil {
		return nil, err
	}
	openedSnapshot, err := openWorkflowDefinitionSecrets(execution.WorkflowSnapshot)
	if err != nil {
		return nil, fmt.Errorf("open workflow execution snapshot %s: %w", execution.ID, err)
	}
	execution.WorkflowSnapshot = openedSnapshot
	return &execution, nil
}

func scanNodeExecution(row scanner) (*WorkflowNodeExecution, error) {
	var node WorkflowNodeExecution
	var status string
	var inputJSON, outputJSON, errorJSON, contextJSON []byte
	if err := row.Scan(
		&node.ID, &node.ExecutionID, &node.OrganizationID, &node.NodeID, &node.NodeType,
		&status, &node.Attempt, &inputJSON, &outputJSON, &errorJSON, &contextJSON,
		&node.StartedAt, &node.CompletedAt, &node.DurationMS, &node.CreatedAt, &node.UpdatedAt,
	); err != nil {
		return nil, err
	}
	node.Status = NodeStatus(status)
	if err := unmarshalJSONObject(inputJSON, &node.Input); err != nil {
		return nil, err
	}
	openedInput, err := openWorkflowDefinitionSecrets(node.Input)
	if err != nil {
		return nil, fmt.Errorf("open workflow node input %s: %w", node.ID, err)
	}
	node.Input = openedInput
	if err := unmarshalJSONObject(outputJSON, &node.Output); err != nil {
		return nil, err
	}
	openedOutput, err := openWorkflowDefinitionSecrets(node.Output)
	if err != nil {
		return nil, fmt.Errorf("open workflow node output %s: %w", node.ID, err)
	}
	node.Output = openedOutput
	if err := unmarshalJSONObject(errorJSON, &node.Error); err != nil {
		return nil, err
	}
	openedError, err := openWorkflowDefinitionSecrets(node.Error)
	if err != nil {
		return nil, fmt.Errorf("open workflow node error %s: %w", node.ID, err)
	}
	node.Error = openedError
	if err := unmarshalJSONObject(contextJSON, &node.Context); err != nil {
		return nil, err
	}
	openedContext, err := openWorkflowDefinitionSecrets(node.Context)
	if err != nil {
		return nil, fmt.Errorf("open workflow node context %s: %w", node.ID, err)
	}
	node.Context = openedContext
	return &node, nil
}

func marshalJSONObject(value map[string]any) ([]byte, error) {
	if value == nil {
		value = map[string]any{}
	}
	return json.Marshal(value)
}

func marshalWorkflowDefinitionForSQL(definition map[string]any) ([]byte, error) {
	protected, err := protectWorkflowDefinitionSecrets(definition)
	if err != nil {
		return nil, err
	}
	return marshalJSONObject(protected)
}

func marshalWorkflowNodePayloadForSQL(payload map[string]any) ([]byte, error) {
	protected, err := protectWorkflowDefinitionSecrets(payload)
	if err != nil {
		return nil, err
	}
	return marshalJSONObject(protected)
}

func protectWorkflowDefinitionSecrets(definition map[string]any) (map[string]any, error) {
	protected, err := protectWorkflowDefinitionSecretMap(definition)
	if err != nil {
		return nil, err
	}
	return protected, nil
}

func protectWorkflowDefinitionSecretMap(definition map[string]any) (map[string]any, error) {
	if definition == nil {
		return map[string]any{}, nil
	}
	protected := make(map[string]any, len(definition))
	for key, value := range definition {
		if IsWorkflowSecretDefinitionKey(key) {
			if plaintext, ok := value.(string); ok && plaintext != "" {
				stored, err := secretbox.Protect(secretbox.DomainWorkflowDefinitionSecretValue, plaintext)
				if err != nil {
					return nil, fmt.Errorf("protect %s: %w", key, err)
				}
				protected[key] = stored
				continue
			}
		}
		protectedValue, err := protectWorkflowDefinitionSecretValue(value)
		if err != nil {
			return nil, err
		}
		protected[key] = protectedValue
	}
	return protected, nil
}

func protectWorkflowDefinitionSecretValue(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		return protectWorkflowDefinitionSecretMap(typed)
	case []any:
		protected := make([]any, 0, len(typed))
		for _, item := range typed {
			protectedItem, err := protectWorkflowDefinitionSecretValue(item)
			if err != nil {
				return nil, err
			}
			protected = append(protected, protectedItem)
		}
		return protected, nil
	case []map[string]any:
		protected := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			protectedItem, err := protectWorkflowDefinitionSecretMap(item)
			if err != nil {
				return nil, err
			}
			protected = append(protected, protectedItem)
		}
		return protected, nil
	default:
		return value, nil
	}
}

func openWorkflowDefinitionSecrets(definition map[string]any) (map[string]any, error) {
	return openWorkflowDefinitionSecretMap(definition)
}

func openWorkflowDefinitionSecretMap(definition map[string]any) (map[string]any, error) {
	if definition == nil {
		return map[string]any{}, nil
	}
	opened := make(map[string]any, len(definition))
	for key, value := range definition {
		if IsWorkflowSecretDefinitionKey(key) {
			if stored, ok := value.(string); ok && stored != "" {
				plaintext, err := secretbox.Open(secretbox.DomainWorkflowDefinitionSecretValue, stored)
				if err != nil {
					return nil, fmt.Errorf("open %s: %w", key, err)
				}
				opened[key] = plaintext
				continue
			}
		}
		openedValue, err := openWorkflowDefinitionSecretValue(value)
		if err != nil {
			return nil, err
		}
		opened[key] = openedValue
	}
	return opened, nil
}

func openWorkflowDefinitionSecretValue(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		return openWorkflowDefinitionSecretMap(typed)
	case []any:
		opened := make([]any, 0, len(typed))
		for _, item := range typed {
			openedItem, err := openWorkflowDefinitionSecretValue(item)
			if err != nil {
				return nil, err
			}
			opened = append(opened, openedItem)
		}
		return opened, nil
	case []map[string]any:
		opened := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			openedItem, err := openWorkflowDefinitionSecretMap(item)
			if err != nil {
				return nil, err
			}
			opened = append(opened, openedItem)
		}
		return opened, nil
	default:
		return value, nil
	}
}

func unmarshalJSONObject(data []byte, target *map[string]any) error {
	if len(data) == 0 {
		*target = map[string]any{}
		return nil
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	if *target == nil {
		*target = map[string]any{}
	}
	return nil
}
