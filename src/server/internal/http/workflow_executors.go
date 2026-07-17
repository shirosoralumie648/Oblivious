package http

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dop251/goja"
	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/knowledge"
	"oblivious/server/internal/workflow"
)

type workflowLLMGatewayAdapter struct {
	gateway chat.ChatGateway
}

func (a workflowLLMGatewayAdapter) Chat(ctx context.Context, req workflow.LLMChatRequest) (*workflow.LLMChatResponse, error) {
	messages := workflowChatMessages(req)
	config := chat.ConversationConfig{
		ModelID: req.Model,
	}
	if temperature, ok := workflowFloatOption(req.Options, "temperature"); ok {
		config.Temperature = temperature
	}
	if maxTokens, ok := workflowIntOption(req.Options, "maxOutputTokens", "maxTokens"); ok {
		config.MaxOutputTokens = maxTokens
	}
	ctx = chat.WithRelayRequestMetadata(ctx, chat.RelayRequestMetadata{
		OrganizationID: req.OrganizationID,
		UserID:         req.UserID,
		WorkspaceID:    req.WorkspaceID,
		RequestID:      req.RequestID,
		FeatureType:    "workflow",
	})

	structured, ok := a.gateway.(chat.StructuredReplyGenerator)
	if !ok {
		reply, err := a.gateway.GenerateReply(ctx, messages, config)
		if err != nil {
			return nil, err
		}
		return &workflow.LLMChatResponse{
			Text:    reply,
			Content: reply,
			Model:   config.ModelID,
		}, nil
	}
	response, err := structured.GenerateStructuredReply(ctx, messages, config, nil)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return &workflow.LLMChatResponse{Model: config.ModelID}, nil
	}
	result := &workflow.LLMChatResponse{
		Text:    response.Content,
		Content: response.Content,
		Model:   config.ModelID,
		Raw: map[string]any{
			"finishReason": response.FinishReason,
		},
	}
	if response.Usage != nil {
		result.Usage = map[string]any{
			"promptTokens":     response.Usage.PromptTokens,
			"completionTokens": response.Usage.CompletionTokens,
			"totalTokens":      response.Usage.TotalTokens,
		}
	}
	return result, nil
}

type workflowAgentStarter interface {
	StartRun(ctx context.Context, session auth.Session, req agent.StartRunRequest) (*agent.RunWithMessages, error)
	StartPlanningRun(ctx context.Context, session auth.Session, req agent.StartRunRequest) (*agent.RunWithMessages, error)
	ContinuePlanningRun(ctx context.Context, session auth.Session, runID string) (*agent.RunWithMessages, error)
	AdjustPlanSteps(ctx context.Context, session auth.Session, runID, reason string) (*agent.RunWithMessages, error)
	ContinueRunWithTokenBudget(ctx context.Context, session auth.Session, runID string, tokenBudget int) (*agent.RunResult, error)
	ApproveToolRun(ctx context.Context, session auth.Session, toolRunID, reason string) (*agent.ToolRun, error)
	ApprovePlanStep(ctx context.Context, session auth.Session, planStepID, reason string) (*agent.PlanStep, error)
	ExecutePlanStep(ctx context.Context, session auth.Session, planStepID string) (*agent.PlanStep, error)
	SkipPlanStep(ctx context.Context, session auth.Session, planStepID, reason string) (*agent.PlanStep, error)
	RetryPlanStep(ctx context.Context, session auth.Session, planStepID string) (*agent.PlanStep, error)
	GetRunWithMessages(ctx context.Context, session auth.Session, runID string) (*agent.RunWithMessages, error)
}

type workflowAgentServiceAdapter struct {
	starter workflowAgentStarter
}

type workflowToolExecutorAdapter struct {
	executor *agent.ToolExecutor
}

type workflowJavaScriptCodeRunner struct {
	defaultTimeout time.Duration
	maxTimeout     time.Duration
}

type workflowDatabaseSQLRunner struct {
	db *sql.DB
}

var workflowDatabaseWriteKeywordPattern = regexp.MustCompile(`(?i)\b(insert|update|delete|merge|create|alter|drop|truncate|grant|revoke|copy|call|do|vacuum|analyze|reindex|refresh|lock|set|reset|listen|notify|unlisten|comment|cluster)\b`)

func registerWorkflowAgentExecutor(service *workflow.Service, starter workflowAgentStarter) {
	if service == nil || starter == nil {
		return
	}
	service.RegisterNodeExecutors(workflow.NewAgentNodeExecutor(workflowAgentServiceAdapter{starter: starter}))
}

func (r workflowJavaScriptCodeRunner) RunWorkflowCode(ctx context.Context, req workflow.WorkflowCodeRequest) (*workflow.WorkflowCodeResult, error) {
	language := strings.ToLower(strings.TrimSpace(req.Language))
	if language == "" {
		language = "javascript"
	}
	if language != "javascript" && language != "js" {
		return nil, fmt.Errorf("%w: workflow code runner only supports javascript", workflow.ErrInvalidInput)
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, fmt.Errorf("%w: workflow code is required", workflow.ErrInvalidInput)
	}

	timeout := workflowCodeRunnerTimeout(req.TimeoutMS, r.defaultTimeout, r.maxTimeout)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	vm := goja.New()
	logs := []string{}
	if err := vm.Set("inputs", req.Inputs); err != nil {
		return nil, err
	}
	if err := vm.Set("console", map[string]any{
		"log": func(call goja.FunctionCall) goja.Value {
			parts := make([]string, 0, len(call.Arguments))
			for _, argument := range call.Arguments {
				parts = append(parts, fmt.Sprint(argument.Export()))
			}
			logs = append(logs, strings.Join(parts, " "))
			return goja.Undefined()
		},
	}); err != nil {
		return nil, err
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-runCtx.Done():
			vm.Interrupt("workflow code timed out")
		case <-done:
		}
	}()
	value, err := vm.RunScript("workflow-code.js", "(function() {\n"+code+"\n})()")
	close(done)
	if err != nil {
		if _, ok := err.(*goja.InterruptedError); ok && runCtx.Err() != nil {
			return nil, fmt.Errorf("%w: workflow code timed out", workflow.ErrInvalidInput)
		}
		return nil, err
	}

	output, err := workflowJavaScriptCodeOutput(value.Export())
	if err != nil {
		return nil, err
	}
	return &workflow.WorkflowCodeResult{
		Output: output,
		Logs:   logs,
		Raw: map[string]any{
			"language":  "javascript",
			"timeoutMs": timeout.Milliseconds(),
		},
	}, nil
}

func workflowCodeRunnerTimeout(timeoutMS int, defaultTimeout time.Duration, maxTimeout time.Duration) time.Duration {
	if defaultTimeout <= 0 {
		defaultTimeout = 5 * time.Second
	}
	if maxTimeout <= 0 {
		maxTimeout = 30 * time.Second
	}
	timeout := defaultTimeout
	if timeoutMS > 0 {
		timeout = time.Duration(timeoutMS) * time.Millisecond
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if timeout > maxTimeout {
		return maxTimeout
	}
	return timeout
}

func workflowJavaScriptCodeOutput(value any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	output, ok := value.(map[string]any)
	if !ok {
		return map[string]any{"result": value}, nil
	}
	return output, nil
}

func (r workflowDatabaseSQLRunner) RunDatabaseQuery(ctx context.Context, req workflow.WorkflowDatabaseRequest) (*workflow.WorkflowDatabaseResult, error) {
	if r.db == nil {
		return nil, fmt.Errorf("%w: workflow database runner is not configured", workflow.ErrInvalidInput)
	}
	if !workflowDatabaseConnectionAllowed(req.ConnectionID) {
		return nil, fmt.Errorf("%w: workflow database connection is not available", workflow.ErrInvalidInput)
	}
	if !req.ReadOnly {
		return nil, fmt.Errorf("%w: workflow database runner only supports read-only queries", workflow.ErrInvalidInput)
	}
	query := strings.TrimSpace(req.Query)
	if !workflowDatabaseQueryIsReadOnly(query) {
		return nil, fmt.Errorf("%w: workflow database query must be a single read-only select", workflow.ErrInvalidInput)
	}
	if !workflowDatabaseQueryIsTenantScoped(query) {
		return nil, fmt.Errorf("%w: workflow database query must include organization_id scoping", workflow.ErrInvalidInput)
	}
	if req.Limit > 0 {
		query = fmt.Sprintf("SELECT * FROM (%s) AS workflow_database_result LIMIT %d", query, req.Limit)
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, query, req.Parameters...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resultRows, err := workflowDatabaseRows(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &workflow.WorkflowDatabaseResult{
		Rows:         resultRows,
		RowsAffected: int64(len(resultRows)),
	}, nil
}

func workflowDatabaseConnectionAllowed(connectionID string) bool {
	switch strings.ToLower(strings.TrimSpace(connectionID)) {
	case "", "default", "internal", "platform", "platform_postgres", "primary":
		return true
	default:
		return false
	}
}

func workflowDatabaseQueryIsReadOnly(query string) bool {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" || strings.Contains(trimmed, ";") {
		return false
	}
	lowered := strings.ToLower(trimmed)
	if !strings.HasPrefix(lowered, "select ") && !strings.HasPrefix(lowered, "with ") {
		return false
	}
	return !workflowDatabaseWriteKeywordPattern.MatchString(trimmed)
}

func workflowDatabaseQueryIsTenantScoped(query string) bool {
	lowered := strings.ToLower(query)
	if !strings.Contains(lowered, " from ") && !strings.Contains(lowered, "\nfrom ") && !strings.Contains(lowered, "\tfrom ") && !strings.Contains(lowered, " join ") {
		return true
	}
	return strings.Contains(lowered, "organization_id")
}

func workflowDatabaseRows(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			row[column] = workflowDatabaseValue(values[i])
		}
		out = append(out, row)
	}
	return out, nil
}

func workflowDatabaseValue(value any) any {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return value
}

func (a workflowToolExecutorAdapter) RunWorkflowTool(ctx context.Context, req workflow.WorkflowToolRequest) (*workflow.WorkflowToolResult, error) {
	executor := a.executor
	if executor == nil {
		return nil, fmt.Errorf("%w: workflow tool executor is not configured", workflow.ErrInvalidInput)
	}
	toolType := req.ToolType
	if toolType == "" {
		toolType = "builtin"
	}
	result, err := executor.Execute(ctx, &agent.Agent{
		OrganizationID: req.OrganizationID,
		UserID:         req.UserID,
		Tools: []agent.Tool{{
			Name:     req.ToolName,
			Type:     toolType,
			ServerID: req.ServerID,
			Enabled:  true,
		}},
	}, &agent.ToolCall{
		ID:        workflowToolCallID(req),
		Name:      req.ToolName,
		Arguments: req.Arguments,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &workflow.WorkflowToolResult{
			Raw: workflowToolRaw(req),
		}, nil
	}
	return &workflow.WorkflowToolResult{
		Content: result.Content,
		IsError: result.IsError,
		Raw:     workflowToolRaw(req),
	}, nil
}

func (a workflowAgentServiceAdapter) StartAgentRun(ctx context.Context, req workflow.WorkflowAgentRunRequest) (*workflow.WorkflowAgentRunResult, error) {
	session := workflowAgentSession(req.OrganizationID, req.WorkspaceID, req.UserID)
	startReq := agent.StartRunRequest{
		AgentID:        req.AgentID,
		ConversationID: req.ConversationID,
		Input:          req.Input,
		MaxIterations:  req.MaxIterations,
		TokenBudget:    req.TokenBudget,
	}
	var result *agent.RunWithMessages
	var err error
	if req.Mode == agent.ExecutionModePlanning {
		result, err = a.starter.StartPlanningRun(ctx, session, startReq)
	} else {
		result, err = a.starter.StartRun(ctx, session, startReq)
	}
	if err != nil {
		return nil, err
	}
	return workflowAgentRunResult(result), nil
}

func (a workflowAgentServiceAdapter) ApproveAgentToolRun(ctx context.Context, req workflow.WorkflowAgentApprovalRequest) (*workflow.WorkflowAgentRunResult, error) {
	session := workflowAgentSession(req.OrganizationID, req.WorkspaceID, req.UserID)
	if _, err := a.starter.ApproveToolRun(ctx, session, req.ToolRunID, req.Reason); err != nil {
		return nil, err
	}
	result, err := a.starter.GetRunWithMessages(ctx, session, req.RunID)
	if err != nil {
		return nil, err
	}
	return workflowAgentRunResult(result), nil
}

func (a workflowAgentServiceAdapter) ContinueAgentPlan(ctx context.Context, req workflow.WorkflowAgentControlRequest) (*workflow.WorkflowAgentRunResult, error) {
	session := workflowAgentSession(req.OrganizationID, req.WorkspaceID, req.UserID)
	result, err := a.starter.ContinuePlanningRun(ctx, session, req.RunID)
	if err != nil {
		return nil, err
	}
	return workflowAgentRunResult(result), nil
}

func (a workflowAgentServiceAdapter) AdjustAgentPlan(ctx context.Context, req workflow.WorkflowAgentControlRequest) (*workflow.WorkflowAgentRunResult, error) {
	session := workflowAgentSession(req.OrganizationID, req.WorkspaceID, req.UserID)
	result, err := a.starter.AdjustPlanSteps(ctx, session, req.RunID, req.Reason)
	if err != nil {
		return nil, err
	}
	return workflowAgentRunResult(result), nil
}

func (a workflowAgentServiceAdapter) ContinueAgentRunWithTokenBudget(ctx context.Context, req workflow.WorkflowAgentControlRequest) (*workflow.WorkflowAgentRunResult, error) {
	session := workflowAgentSession(req.OrganizationID, req.WorkspaceID, req.UserID)
	if _, err := a.starter.ContinueRunWithTokenBudget(ctx, session, req.RunID, req.TokenBudget); err != nil {
		return nil, err
	}
	return a.reloadAgentRun(ctx, session, req.RunID)
}

func (a workflowAgentServiceAdapter) ApproveAgentPlanStep(ctx context.Context, req workflow.WorkflowAgentControlRequest) (*workflow.WorkflowAgentRunResult, error) {
	session := workflowAgentSession(req.OrganizationID, req.WorkspaceID, req.UserID)
	if err := a.applyAgentPlanStepAction(req, func() (*agent.PlanStep, error) {
		return a.starter.ApprovePlanStep(ctx, session, req.PlanStepID, req.Reason)
	}); err != nil {
		return nil, err
	}
	return a.reloadAgentRun(ctx, session, req.RunID)
}

func (a workflowAgentServiceAdapter) ExecuteAgentPlanStep(ctx context.Context, req workflow.WorkflowAgentControlRequest) (*workflow.WorkflowAgentRunResult, error) {
	session := workflowAgentSession(req.OrganizationID, req.WorkspaceID, req.UserID)
	if err := a.applyAgentPlanStepAction(req, func() (*agent.PlanStep, error) {
		return a.starter.ExecutePlanStep(ctx, session, req.PlanStepID)
	}); err != nil {
		return nil, err
	}
	return a.reloadAgentRun(ctx, session, req.RunID)
}

func (a workflowAgentServiceAdapter) SkipAgentPlanStep(ctx context.Context, req workflow.WorkflowAgentControlRequest) (*workflow.WorkflowAgentRunResult, error) {
	session := workflowAgentSession(req.OrganizationID, req.WorkspaceID, req.UserID)
	if err := a.applyAgentPlanStepAction(req, func() (*agent.PlanStep, error) {
		return a.starter.SkipPlanStep(ctx, session, req.PlanStepID, req.Reason)
	}); err != nil {
		return nil, err
	}
	return a.reloadAgentRun(ctx, session, req.RunID)
}

func (a workflowAgentServiceAdapter) RetryAgentPlanStep(ctx context.Context, req workflow.WorkflowAgentControlRequest) (*workflow.WorkflowAgentRunResult, error) {
	session := workflowAgentSession(req.OrganizationID, req.WorkspaceID, req.UserID)
	if err := a.applyAgentPlanStepAction(req, func() (*agent.PlanStep, error) {
		return a.starter.RetryPlanStep(ctx, session, req.PlanStepID)
	}); err != nil {
		return nil, err
	}
	return a.reloadAgentRun(ctx, session, req.RunID)
}

func (a workflowAgentServiceAdapter) applyAgentPlanStepAction(req workflow.WorkflowAgentControlRequest, action func() (*agent.PlanStep, error)) error {
	step, err := action()
	if err != nil {
		return err
	}
	if step != nil && step.RunID != "" && req.RunID != "" && step.RunID != req.RunID {
		return fmt.Errorf("%w: agent plan step does not belong to run", workflow.ErrInvalidInput)
	}
	return nil
}

func (a workflowAgentServiceAdapter) reloadAgentRun(ctx context.Context, session auth.Session, runID string) (*workflow.WorkflowAgentRunResult, error) {
	result, err := a.starter.GetRunWithMessages(ctx, session, runID)
	if err != nil {
		return nil, err
	}
	return workflowAgentRunResult(result), nil
}

func workflowAgentSession(organizationID, workspaceID, userID string) auth.Session {
	return auth.Session{
		OrganizationID: organizationID,
		WorkspaceID:    workspaceID,
		User:           auth.User{ID: userID},
	}
}

func workflowAgentRunResult(result *agent.RunWithMessages) *workflow.WorkflowAgentRunResult {
	if result == nil {
		return &workflow.WorkflowAgentRunResult{}
	}
	out := &workflow.WorkflowAgentRunResult{
		Messages:  workflowAgentMessages(result.Messages),
		ToolRuns:  workflowAgentToolRuns(result.ToolRuns),
		PlanSteps: workflowAgentPlanSteps(result.PlanSteps),
	}
	if result.Run != nil {
		out.RunID = result.Run.ID
		out.Status = result.Run.Status
		out.FinalMessageID = result.Run.FinalMessageID
	}
	out.FinalMessage = workflowAgentFinalMessage(result.Messages, out.FinalMessageID)
	return out
}

func workflowAgentFinalMessage(messages []*agent.Message, finalMessageID string) string {
	if finalMessageID != "" {
		for _, message := range messages {
			if message != nil && message.ID == finalMessageID {
				return message.Content
			}
		}
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i] != nil && messages[i].Role == "assistant" {
			return messages[i].Content
		}
	}
	return ""
}

func workflowAgentMessages(messages []*agent.Message) []workflow.WorkflowAgentMessage {
	out := make([]workflow.WorkflowAgentMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		out = append(out, workflow.WorkflowAgentMessage{
			ID:      message.ID,
			Role:    message.Role,
			Content: message.Content,
		})
	}
	return out
}

func workflowAgentToolRuns(toolRuns []*agent.ToolRun) []workflow.WorkflowAgentToolRun {
	out := make([]workflow.WorkflowAgentToolRun, 0, len(toolRuns))
	for _, toolRun := range toolRuns {
		if toolRun == nil {
			continue
		}
		out = append(out, workflow.WorkflowAgentToolRun{
			ID:             toolRun.ID,
			ToolName:       toolRun.ToolName,
			Status:         toolRun.Status,
			ApprovalStatus: toolRun.ApprovalStatus,
		})
	}
	return out
}

func workflowAgentPlanSteps(planSteps []*agent.PlanStep) []workflow.WorkflowAgentPlanStep {
	out := make([]workflow.WorkflowAgentPlanStep, 0, len(planSteps))
	for _, step := range planSteps {
		if step == nil {
			continue
		}
		out = append(out, workflow.WorkflowAgentPlanStep{
			ID:             step.ID,
			Title:          step.Title,
			Status:         step.Status,
			ApprovalStatus: step.ApprovalStatus,
			ToolName:       step.ToolName,
			ResultContent:  step.ResultContent,
			Error:          step.Error,
		})
	}
	return out
}

func workflowToolCallID(req workflow.WorkflowToolRequest) string {
	if value := workflowToolMetadataString(req.Metadata, "toolCallId"); value != "" {
		return value
	}
	if value := workflowToolMetadataString(req.Metadata, "id"); value != "" {
		return value
	}
	return "workflow_tool_" + strings.TrimSpace(req.ToolName)
}

func workflowToolMetadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value := metadata[key]
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func workflowToolRaw(req workflow.WorkflowToolRequest) map[string]any {
	return map[string]any{
		"toolName": req.ToolName,
		"toolType": req.ToolType,
		"serverId": req.ServerID,
	}
}

func workflowNodeExecutorRegistry(gateway chat.ChatGateway, knowledgeService *knowledge.Service, agentStarter workflowAgentStarter, toolRunner workflow.ToolRunner, databaseRunner workflow.DatabaseRunner, codeRunner workflow.CodeRunner) *workflow.NodeExecutorRegistry {
	registry := workflow.DefaultNodeExecutorRegistry()
	if gateway != nil {
		registry.Register(workflow.NewLLMNodeExecutor(workflowLLMGatewayAdapter{gateway: gateway}))
	}
	if knowledgeService != nil {
		registry.Register(workflow.NewKnowledgeNodeExecutor(knowledgeService))
	}
	if toolRunner != nil {
		registry.Register(workflow.NewToolNodeExecutor(toolRunner))
	}
	if databaseRunner != nil {
		registry.Register(workflow.NewDatabaseNodeExecutor(databaseRunner))
	}
	if codeRunner != nil {
		registry.Register(workflow.NewCodeNodeExecutor(workflow.WithCodeRunner(codeRunner)))
	}
	if agentStarter != nil {
		registry.Register(workflow.NewAgentNodeExecutor(workflowAgentServiceAdapter{starter: agentStarter}))
	}
	return registry
}

func workflowChatMessages(req workflow.LLMChatRequest) []chat.Message {
	messages := make([]chat.Message, 0, len(req.Messages)+1)
	for _, message := range req.Messages {
		messages = append(messages, chat.Message{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	if len(messages) == 0 && req.Prompt != "" {
		messages = append(messages, chat.Message{Role: "user", Content: req.Prompt})
	}
	return messages
}

func workflowFloatOption(options map[string]any, key string) (float64, bool) {
	value, ok := options[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func workflowIntOption(options map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := options[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed, true
		case int64:
			return int(typed), true
		case float64:
			return int(typed), true
		case float32:
			return int(typed), true
		}
	}
	return 0, false
}
