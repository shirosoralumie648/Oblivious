package http

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/knowledge"
	"oblivious/server/internal/workflow"
)

func TestWorkflowLLMGatewayAdapterUsesStructuredChatGateway(t *testing.T) {
	gateway := &recordingWorkflowChatGateway{
		response: &chat.CompletionResponse{
			Content:      "Workflow answer",
			FinishReason: "stop",
			Usage:        &chat.CompletionUsage{PromptTokens: 3, CompletionTokens: 5, TotalTokens: 8},
		},
	}
	adapter := workflowLLMGatewayAdapter{gateway: gateway}

	response, err := adapter.Chat(context.Background(), workflow.LLMChatRequest{
		Model:  "gpt-4o-mini",
		Prompt: "Summarize the incident",
		Options: map[string]any{
			"temperature":     0.25,
			"maxOutputTokens": 128,
		},
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	if len(gateway.messages) != 1 || gateway.messages[0].Role != "user" || gateway.messages[0].Content != "Summarize the incident" {
		t.Fatalf("unexpected gateway messages: %+v", gateway.messages)
	}
	if gateway.config.ModelID != "gpt-4o-mini" || gateway.config.Temperature != 0.25 || gateway.config.MaxOutputTokens != 128 {
		t.Fatalf("unexpected gateway config: %+v", gateway.config)
	}
	if response.Text != "Workflow answer" || response.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected workflow llm response: %+v", response)
	}
	if response.Usage["promptTokens"] != 3 || response.Usage["completionTokens"] != 5 || response.Usage["totalTokens"] != 8 {
		t.Fatalf("unexpected workflow llm usage: %+v", response.Usage)
	}
	if response.Raw["finishReason"] != "stop" {
		t.Fatalf("expected finish reason in raw response, got %+v", response.Raw)
	}
}

func TestWorkflowLLMGatewayAdapterInjectsRelayAttributionMetadata(t *testing.T) {
	gateway := &recordingWorkflowChatGateway{
		response: &chat.CompletionResponse{Content: "Workflow answer"},
	}
	adapter := workflowLLMGatewayAdapter{gateway: gateway}

	_, err := adapter.Chat(context.Background(), workflow.LLMChatRequest{
		Model:          "gpt-4o-mini",
		Prompt:         "Summarize the incident",
		OrganizationID: "org_1",
		UserID:         "user_1",
		WorkspaceID:    "workspace_1",
		RequestID:      "req_workflow_1",
		FeatureType:    "workflow",
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	metadata, ok := chat.RelayRequestMetadataFromContext(gateway.ctx)
	if !ok {
		t.Fatal("expected workflow adapter to inject relay request metadata")
	}
	if metadata.OrganizationID != "org_1" || metadata.UserID != "user_1" || metadata.WorkspaceID != "workspace_1" || metadata.RequestID != "req_workflow_1" {
		t.Fatalf("unexpected relay request metadata: %+v", metadata)
	}
	if metadata.FeatureType != "workflow" {
		t.Fatalf("expected workflow feature attribution, got %+v", metadata)
	}
}

func TestWorkflowNodeExecutorRegistryIncludesRealLLMKnowledgeAndAgentExecutors(t *testing.T) {
	registry := workflowNodeExecutorRegistry(&recordingWorkflowChatGateway{}, knowledge.NewService(nil), &recordingWorkflowAgentStarter{}, workflowToolExecutorAdapter{executor: agent.NewToolExecutor(nil)}, workflowDatabaseSQLRunner{db: openWorkflowDatabaseSQLRunnerTestDB(t, &workflowDatabaseSQLRunnerCapture{})}, workflowJavaScriptCodeRunner{defaultTimeout: time.Second, maxTimeout: time.Second})

	if executor, ok := registry.Get("llm"); !ok || executor.Type() != "llm" {
		t.Fatalf("expected llm executor in workflow registry, got %T ok=%v", executor, ok)
	}
	if executor, ok := registry.Get("knowledge"); !ok || executor.Type() != "knowledge" {
		t.Fatalf("expected knowledge executor in workflow registry, got %T ok=%v", executor, ok)
	}
	if executor, ok := registry.Get("http"); !ok || executor.Type() != "http" {
		t.Fatalf("expected default http executor to remain registered, got %T ok=%v", executor, ok)
	}
	codeExecutor, ok := registry.Get("code")
	if !ok || codeExecutor.Type() != "code" {
		t.Fatalf("expected default code executor contract to remain registered, got %T ok=%v", codeExecutor, ok)
	}
	output, err := codeExecutor.Execute(context.Background(), workflow.NodeExecutorInput{
		Execution: &workflow.WorkflowExecution{OrganizationID: "org_1"},
		Input: map[string]any{
			"language": "javascript",
			"code":     "return { ok: inputs.ok };",
			"inputs":   map[string]any{"ok": true},
		},
	})
	if err != nil {
		t.Fatalf("expected registered code executor to run javascript: %v", err)
	}
	if output["ok"] != true {
		t.Fatalf("expected registered code executor output, got %+v", output)
	}
	if executor, ok := registry.Get("database"); !ok || executor.Type() != "database" {
		t.Fatalf("expected default database executor contract to remain registered, got %T ok=%v", executor, ok)
	}
	if executor, ok := registry.Get("tool"); !ok || executor.Type() != "tool" {
		t.Fatalf("expected default tool executor contract to remain registered, got %T ok=%v", executor, ok)
	}
	if executor, ok := registry.Get("rpa"); !ok || executor.Type() != "rpa" {
		t.Fatalf("expected default rpa executor contract to remain registered, got %T ok=%v", executor, ok)
	}
	if executor, ok := registry.Get("agent"); !ok || executor.Type() != "agent" {
		t.Fatalf("expected agent executor in workflow registry, got %T ok=%v", executor, ok)
	}
}

func TestWorkflowNodeExecutorRegistryKeepsDefaultsWithoutOptionalDependencies(t *testing.T) {
	registry := workflowNodeExecutorRegistry(nil, nil, nil, nil, nil, nil)

	llmExecutor, ok := registry.Get("llm")
	if !ok || llmExecutor.Type() != "llm" {
		t.Fatalf("expected default llm executor to remain registered, got %T ok=%v", llmExecutor, ok)
	}
	knowledgeExecutor, ok := registry.Get("knowledge")
	if !ok || knowledgeExecutor.Type() != "knowledge" {
		t.Fatalf("expected default knowledge executor to remain registered, got %T ok=%v", knowledgeExecutor, ok)
	}
	codeExecutor, ok := registry.Get("code")
	if !ok || codeExecutor.Type() != "code" {
		t.Fatalf("expected default code executor contract to remain registered, got %T ok=%v", codeExecutor, ok)
	}
	databaseExecutor, ok := registry.Get("database")
	if !ok || databaseExecutor.Type() != "database" {
		t.Fatalf("expected default database executor contract to remain registered, got %T ok=%v", databaseExecutor, ok)
	}
	toolExecutor, ok := registry.Get("tool")
	if !ok || toolExecutor.Type() != "tool" {
		t.Fatalf("expected default tool executor contract to remain registered, got %T ok=%v", toolExecutor, ok)
	}
	rpaExecutor, ok := registry.Get("rpa")
	if !ok || rpaExecutor.Type() != "rpa" {
		t.Fatalf("expected default rpa executor contract to remain registered, got %T ok=%v", rpaExecutor, ok)
	}
}

func TestWorkflowAgentServiceAdapterStartsRuns(t *testing.T) {
	starter := &recordingWorkflowAgentStarter{
		result: &agentRunWithMessagesFixture,
	}
	adapter := workflowAgentServiceAdapter{starter: starter}

	result, err := adapter.StartAgentRun(context.Background(), workflow.WorkflowAgentRunRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		WorkspaceID:    "workspace_1",
		RequestID:      "req_1",
		AgentID:        "agent_1",
		ConversationID: "conv_1",
		Input:          "hello",
		Mode:           "planning",
	})
	if err != nil {
		t.Fatalf("StartAgentRun returned error: %v", err)
	}
	if starter.request.AgentID != "agent_1" || starter.request.ConversationID != "conv_1" || starter.request.Input != "hello" {
		t.Fatalf("unexpected agent start request: %+v", starter.request)
	}
	if starter.session.OrganizationID != "org_1" || starter.session.User.ID != "user_1" || starter.session.WorkspaceID != "workspace_1" {
		t.Fatalf("unexpected agent session: %+v", starter.session)
	}
	if result.RunID != "run_1" || result.Status != "completed" || result.FinalMessage != "done" {
		t.Fatalf("unexpected workflow agent run result: %+v", result)
	}
}

func TestWorkflowAgentServiceAdapterApprovesToolRuns(t *testing.T) {
	starter := &recordingWorkflowAgentStarter{
		result: &agentRunWithMessagesFixture,
	}
	adapter := workflowAgentServiceAdapter{starter: starter}

	result, err := adapter.ApproveAgentToolRun(context.Background(), workflow.WorkflowAgentApprovalRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		WorkspaceID:    "workspace_1",
		RunID:          "run_1",
		ToolRunID:      "tool_run_pending",
		Reason:         "operator approved",
	})
	if err != nil {
		t.Fatalf("ApproveAgentToolRun returned error: %v", err)
	}
	if starter.approvedToolRunID != "tool_run_pending" || starter.approvalReason != "operator approved" {
		t.Fatalf("unexpected approval call toolRun=%q reason=%q", starter.approvedToolRunID, starter.approvalReason)
	}
	if starter.fetchedRunID != "run_1" {
		t.Fatalf("expected approved run to be fetched, got %q", starter.fetchedRunID)
	}
	if result.RunID != "run_1" || result.Status != agent.RunStatusCompleted || result.FinalMessage != "done" {
		t.Fatalf("unexpected approved workflow agent result: %+v", result)
	}
}

func TestWorkflowToolExecutorAdapterRunsBuiltinTool(t *testing.T) {
	adapter := workflowToolExecutorAdapter{executor: agent.NewToolExecutor(nil)}

	result, err := adapter.RunWorkflowTool(context.Background(), workflow.WorkflowToolRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		WorkspaceID:    "workspace_1",
		ToolName:       "calculator",
		ToolType:       "builtin",
		Arguments:      map[string]any{"expression": "2 + 3"},
	})
	if err != nil {
		t.Fatalf("RunWorkflowTool returned error: %v", err)
	}
	if result == nil || result.IsError || result.Content != "Result: 5" {
		t.Fatalf("unexpected workflow tool result: %+v", result)
	}
	if result.Raw["toolName"] != "calculator" || result.Raw["toolType"] != "builtin" {
		t.Fatalf("expected tool identity in raw result, got %+v", result.Raw)
	}
}

func TestWorkflowJavaScriptCodeRunnerExecutesScopedCode(t *testing.T) {
	runner := workflowJavaScriptCodeRunner{
		defaultTimeout: time.Second,
		maxTimeout:     time.Second,
	}

	result, err := runner.RunWorkflowCode(context.Background(), workflow.WorkflowCodeRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		WorkspaceID:    "workspace_1",
		Language:       "javascript",
		Code: `
			console.log("calculating");
			return {
				total: inputs.price * inputs.count,
				label: inputs.ticket + ":" + inputs.priority
			};
		`,
		Inputs: map[string]any{
			"count":    float64(7),
			"price":    float64(6),
			"priority": "high",
			"ticket":   "INC-42",
		},
		TimeoutMS: 1000,
	})
	if err != nil {
		t.Fatalf("RunWorkflowCode returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected workflow code result")
	}
	if result.Output["total"] != int64(42) && result.Output["total"] != float64(42) {
		t.Fatalf("expected calculated total, got %+v", result.Output)
	}
	if result.Output["label"] != "INC-42:high" {
		t.Fatalf("expected calculated label, got %+v", result.Output)
	}
	if len(result.Logs) != 1 || result.Logs[0] != "calculating" {
		t.Fatalf("expected console logs, got %+v", result.Logs)
	}
	if result.Raw["language"] != "javascript" {
		t.Fatalf("expected raw language metadata, got %+v", result.Raw)
	}
}

func TestWorkflowJavaScriptCodeRunnerRejectsUnsupportedLanguageAndTimesOut(t *testing.T) {
	runner := workflowJavaScriptCodeRunner{
		defaultTimeout: 10 * time.Millisecond,
		maxTimeout:     10 * time.Millisecond,
	}

	if _, err := runner.RunWorkflowCode(context.Background(), workflow.WorkflowCodeRequest{
		OrganizationID: "org_1",
		Language:       "python",
		Code:           "return {'ok': True}",
	}); err == nil {
		t.Fatal("expected unsupported language error")
	}

	if _, err := runner.RunWorkflowCode(context.Background(), workflow.WorkflowCodeRequest{
		OrganizationID: "org_1",
		Language:       "javascript",
		Code:           "while (true) {}",
		TimeoutMS:      1,
	}); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestWorkflowDatabaseSQLRunnerExecutesReadOnlyQuery(t *testing.T) {
	capture := &workflowDatabaseSQLRunnerCapture{
		columns: []string{"id", "count"},
		rows:    [][]driver.Value{{"ticket_1", int64(7)}},
	}
	database := openWorkflowDatabaseSQLRunnerTestDB(t, capture)
	runner := workflowDatabaseSQLRunner{db: database}

	result, err := runner.RunDatabaseQuery(context.Background(), workflow.WorkflowDatabaseRequest{
		OrganizationID: "org_1",
		Query:          "select $1::text as id, $2::int as count",
		Parameters:     []any{"ticket_1", 7},
		Limit:          5,
		ReadOnly:       true,
	})
	if err != nil {
		t.Fatalf("RunDatabaseQuery returned error: %v", err)
	}
	if result == nil || len(result.Rows) != 1 {
		t.Fatalf("expected one row result, got %+v", result)
	}
	if result.Rows[0]["id"] != "ticket_1" || result.Rows[0]["count"] != int64(7) {
		t.Fatalf("unexpected database rows: %+v", result.Rows)
	}
	if !capture.readOnly {
		t.Fatal("expected database runner to start a read-only transaction")
	}
	if !strings.Contains(capture.query, "select $1::text as id, $2::int as count") || !strings.Contains(capture.query, "LIMIT 5") {
		t.Fatalf("unexpected captured query: %q", capture.query)
	}
	if len(capture.args) != 2 || capture.args[0].Value != "ticket_1" || capture.args[1].Value != 7 {
		t.Fatalf("unexpected captured query args: %+v", capture.args)
	}
}

func TestWorkflowDatabaseSQLRunnerRejectsUnsafeQueries(t *testing.T) {
	database := openWorkflowDatabaseSQLRunnerTestDB(t, &workflowDatabaseSQLRunnerCapture{})
	runner := workflowDatabaseSQLRunner{db: database}

	tests := []struct {
		name string
		req  workflow.WorkflowDatabaseRequest
	}{
		{
			name: "non default connection",
			req: workflow.WorkflowDatabaseRequest{
				OrganizationID: "org_1",
				ConnectionID:   "external_reporting",
				Query:          "select 1",
				ReadOnly:       true,
			},
		},
		{
			name: "tenant query bypass with escaped string literal",
			req: workflow.WorkflowDatabaseRequest{
				OrganizationID: "org_1",
				Query:          "select id from workflows where col = 'some ''escaped'' string organization_id'",
				ReadOnly:       true,
			},
		},
		{
			name: "tenant query bypass with double quotes",
			req: workflow.WorkflowDatabaseRequest{
				OrganizationID: "org_1",
				Query:          "select id from workflows where col = \"organization_id\"",
				ReadOnly:       true,
			},
		},
		{
			name: "tenant query bypass with comments",
			req: workflow.WorkflowDatabaseRequest{
				OrganizationID: "org_1",
				Query:          "select id from workflows -- organization_id",
				ReadOnly:       true,
			},
		},
		{
			name: "tenant query bypass with block comments",
			req: workflow.WorkflowDatabaseRequest{
				OrganizationID: "org_1",
				Query:          "select id from workflows /* organization_id */",
				ReadOnly:       true,
			},
		},
		{
			name: "tenant query bypass with string literal",
			req: workflow.WorkflowDatabaseRequest{
				OrganizationID: "org_1",
				Query:          "select id from workflows where col = 'organization_id'",
				ReadOnly:       true,
			},
		},
		{
			name: "write query",
			req: workflow.WorkflowDatabaseRequest{
				OrganizationID: "org_1",
				Query:          "update workflows set name = name",
				ReadOnly:       true,
			},
		},
		{
			name: "multiple statements",
			req: workflow.WorkflowDatabaseRequest{
				OrganizationID: "org_1",
				Query:          "select 1; select 2",
				ReadOnly:       true,
			},
		},
		{
			name: "unscoped tenant query",
			req: workflow.WorkflowDatabaseRequest{
				OrganizationID: "org_1",
				Query:          "select id from workflows",
				ReadOnly:       true,
			},
		},
		{
			name: "read only false",
			req: workflow.WorkflowDatabaseRequest{
				OrganizationID: "org_1",
				Query:          "select 1",
				ReadOnly:       false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runner.RunDatabaseQuery(context.Background(), tt.req)
			if !errors.Is(err, workflow.ErrInvalidInput) {
				t.Fatalf("RunDatabaseQuery err=%v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestWorkflowDatabaseSQLRunnerAllowsTenantScopedQuery(t *testing.T) {
	capture := &workflowDatabaseSQLRunnerCapture{
		columns: []string{"id"},
		rows:    [][]driver.Value{{"workflow_1"}},
	}
	database := openWorkflowDatabaseSQLRunnerTestDB(t, capture)
	runner := workflowDatabaseSQLRunner{db: database}

	result, err := runner.RunDatabaseQuery(context.Background(), workflow.WorkflowDatabaseRequest{
		OrganizationID: "org_1",
		Query:          "select id from workflows where organization_id = $1",
		Parameters:     []any{"org_1"},
		ReadOnly:       true,
	})
	if err != nil {
		t.Fatalf("RunDatabaseQuery returned error: %v", err)
	}
	if result == nil || len(result.Rows) != 1 || result.Rows[0]["id"] != "workflow_1" {
		t.Fatalf("unexpected tenant-scoped result: %+v", result)
	}
}

var workflowDatabaseSQLRunnerDrivers sync.Map

func openWorkflowDatabaseSQLRunnerTestDB(t *testing.T, capture *workflowDatabaseSQLRunnerCapture) *sql.DB {
	t.Helper()
	driverName := "workflow_database_sql_runner_test_" + strings.ReplaceAll(t.Name(), "/", "_")
	if _, loaded := workflowDatabaseSQLRunnerDrivers.LoadOrStore(driverName, capture); !loaded {
		sql.Register(driverName, workflowDatabaseSQLRunnerDriver{name: driverName})
	}
	database, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open workflow database runner test db: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	return database
}

type workflowDatabaseSQLRunnerCapture struct {
	mu       sync.Mutex
	readOnly bool
	query    string
	args     []driver.NamedValue
	columns  []string
	rows     [][]driver.Value
}

type workflowDatabaseSQLRunnerDriver struct {
	name string
}

func (d workflowDatabaseSQLRunnerDriver) Open(_ string) (driver.Conn, error) {
	capture, _ := workflowDatabaseSQLRunnerDrivers.Load(d.name)
	return workflowDatabaseSQLRunnerConn{capture: capture.(*workflowDatabaseSQLRunnerCapture)}, nil
}

type workflowDatabaseSQLRunnerConn struct {
	capture *workflowDatabaseSQLRunnerCapture
}

func (c workflowDatabaseSQLRunnerConn) Prepare(query string) (driver.Stmt, error) {
	return workflowDatabaseSQLRunnerStmt{capture: c.capture, query: query}, nil
}

func (c workflowDatabaseSQLRunnerConn) Close() error {
	return nil
}

func (c workflowDatabaseSQLRunnerConn) Begin() (driver.Tx, error) {
	return workflowDatabaseSQLRunnerTx{capture: c.capture}, nil
}

func (c workflowDatabaseSQLRunnerConn) BeginTx(_ context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.capture.mu.Lock()
	c.capture.readOnly = opts.ReadOnly
	c.capture.mu.Unlock()
	return workflowDatabaseSQLRunnerTx{capture: c.capture}, nil
}

func (c workflowDatabaseSQLRunnerConn) CheckNamedValue(_ *driver.NamedValue) error {
	return nil
}

type workflowDatabaseSQLRunnerTx struct {
	capture *workflowDatabaseSQLRunnerCapture
}

func (tx workflowDatabaseSQLRunnerTx) Commit() error {
	return nil
}

func (tx workflowDatabaseSQLRunnerTx) Rollback() error {
	return nil
}

func (tx workflowDatabaseSQLRunnerTx) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	tx.capture.mu.Lock()
	tx.capture.query = query
	tx.capture.args = append([]driver.NamedValue(nil), args...)
	columns := append([]string(nil), tx.capture.columns...)
	rows := append([][]driver.Value(nil), tx.capture.rows...)
	tx.capture.mu.Unlock()
	return &workflowDatabaseSQLRunnerRows{columns: columns, rows: rows}, nil
}

type workflowDatabaseSQLRunnerStmt struct {
	capture *workflowDatabaseSQLRunnerCapture
	query   string
}

func (s workflowDatabaseSQLRunnerStmt) Close() error {
	return nil
}

func (s workflowDatabaseSQLRunnerStmt) NumInput() int {
	return -1
}

func (s workflowDatabaseSQLRunnerStmt) Exec(_ []driver.Value) (driver.Result, error) {
	return nil, driver.ErrSkip
}

func (s workflowDatabaseSQLRunnerStmt) Query(args []driver.Value) (driver.Rows, error) {
	namedArgs := make([]driver.NamedValue, 0, len(args))
	for i, arg := range args {
		namedArgs = append(namedArgs, driver.NamedValue{Ordinal: i + 1, Value: arg})
	}
	return workflowDatabaseSQLRunnerTx{capture: s.capture}.QueryContext(context.Background(), s.query, namedArgs)
}

func (s workflowDatabaseSQLRunnerStmt) QueryContext(_ context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return workflowDatabaseSQLRunnerTx{capture: s.capture}.QueryContext(context.Background(), s.query, args)
}

type workflowDatabaseSQLRunnerRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *workflowDatabaseSQLRunnerRows) Columns() []string {
	return r.columns
}

func (r *workflowDatabaseSQLRunnerRows) Close() error {
	return nil
}

func (r *workflowDatabaseSQLRunnerRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

type recordingWorkflowChatGateway struct {
	ctx      context.Context
	response *chat.CompletionResponse
	messages []chat.Message
	config   chat.ConversationConfig
	tools    []map[string]any
}

var agentRunWithMessagesFixture = agent.RunWithMessages{
	Run: &agent.Run{
		ID:             "run_1",
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		Status:         agent.RunStatusCompleted,
		FinalMessageID: "msg_final",
		StartedAt:      time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	},
	Messages: []*agent.Message{{
		ID:             "msg_final",
		ConversationID: "conv_1",
		OrganizationID: "org_1",
		Role:           "assistant",
		Content:        "done",
		CreatedAt:      time.Now().UTC(),
	}},
	ToolRuns: []*agent.ToolRun{{
		ID:             "tool_run_1",
		OrganizationID: "org_1",
		RunID:          "run_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolName:       "datetime",
		Status:         agent.ToolRunStatusCompleted,
		ApprovalStatus: agent.ApprovalStatusNotRequired,
	}},
	PlanSteps: []*agent.PlanStep{{
		ID:     "plan_step_1",
		RunID:  "run_1",
		Title:  "Check status",
		Status: agent.PlanStepStatusCompleted,
	}},
}

type recordingWorkflowAgentStarter struct {
	session           auth.Session
	request           agent.StartRunRequest
	approvedToolRunID string
	approvalReason    string
	fetchedRunID      string
	result            *agent.RunWithMessages
}

func (s *recordingWorkflowAgentStarter) StartRun(ctx context.Context, session auth.Session, req agent.StartRunRequest) (*agent.RunWithMessages, error) {
	_ = ctx
	s.session = session
	s.request = req
	return s.result, nil
}

func (s *recordingWorkflowAgentStarter) StartPlanningRun(ctx context.Context, session auth.Session, req agent.StartRunRequest) (*agent.RunWithMessages, error) {
	_ = ctx
	s.session = session
	s.request = req
	return s.result, nil
}

func (s *recordingWorkflowAgentStarter) ApproveToolRun(ctx context.Context, session auth.Session, toolRunID, reason string) (*agent.ToolRun, error) {
	_ = ctx
	s.session = session
	s.approvedToolRunID = toolRunID
	s.approvalReason = reason
	return &agent.ToolRun{ID: toolRunID, Status: agent.ToolRunStatusCompleted}, nil
}

func (s *recordingWorkflowAgentStarter) GetRunWithMessages(ctx context.Context, session auth.Session, runID string) (*agent.RunWithMessages, error) {
	_ = ctx
	s.session = session
	s.fetchedRunID = runID
	return s.result, nil
}

func (g *recordingWorkflowChatGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	g.ctx = ctx
	g.messages = append([]chat.Message(nil), messages...)
	g.config = config
	return g.response.Content, nil
}

func (g *recordingWorkflowChatGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	g.ctx = ctx
	g.messages = append([]chat.Message(nil), messages...)
	g.config = config
	return onChunk(g.response.Content)
}

func (g *recordingWorkflowChatGateway) GenerateStructuredReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, tools []map[string]any) (*chat.CompletionResponse, error) {
	g.ctx = ctx
	g.messages = append([]chat.Message(nil), messages...)
	g.config = config
	g.tools = append([]map[string]any(nil), tools...)
	return g.response, nil
}
