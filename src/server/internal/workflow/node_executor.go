package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/knowledge"
	workflowexecutor "oblivious/server/internal/workflow/executor"
)

var (
	ErrNodeExecutorNotFound      = errors.New("workflow node executor not found")
	ErrWorkflowUserInputRequired = errors.New("workflow user input required")
	ErrWorkflowVariableNotFound  = errors.New("workflow variable not found")
)

type NodeExecutor interface {
	Type() string
	Execute(ctx context.Context, input NodeExecutorInput) (map[string]any, error)
}

type NodeExecutorInput struct {
	Workflow  *WorkflowDefinition
	Execution *WorkflowExecution
	Node      Node
	Input     map[string]any
}

type NodeExecutorRegistry struct {
	executors map[string]NodeExecutor
}

func NewNodeExecutorRegistry(executors ...NodeExecutor) *NodeExecutorRegistry {
	registry := &NodeExecutorRegistry{executors: map[string]NodeExecutor{}}
	for _, executor := range executors {
		registry.Register(executor)
	}
	return registry
}

func (r *NodeExecutorRegistry) Register(executor NodeExecutor) {
	if r == nil || executor == nil {
		return
	}
	nodeType := strings.TrimSpace(executor.Type())
	if nodeType == "" {
		return
	}
	r.executors[nodeType] = executor
}

func (r *NodeExecutorRegistry) Get(nodeType string) (NodeExecutor, bool) {
	if r == nil {
		return nil, false
	}
	executor, ok := r.executors[strings.TrimSpace(nodeType)]
	return executor, ok
}

type functionNodeExecutor struct {
	nodeType string
	execute  func(context.Context, NodeExecutorInput) (map[string]any, error)
}

func (e functionNodeExecutor) Type() string { return e.nodeType }

func (e functionNodeExecutor) Execute(ctx context.Context, input NodeExecutorInput) (map[string]any, error) {
	return e.execute(ctx, input)
}

func StaticNodeExecutor(nodeType string, output map[string]any) NodeExecutor {
	return functionNodeExecutor{
		nodeType: strings.TrimSpace(nodeType),
		execute: func(context.Context, NodeExecutorInput) (map[string]any, error) {
			return mergeWorkflowMaps(output, nil), nil
		},
	}
}

func EchoNodeExecutor(nodeType string) NodeExecutor {
	return functionNodeExecutor{
		nodeType: strings.TrimSpace(nodeType),
		execute: func(_ context.Context, input NodeExecutorInput) (map[string]any, error) {
			return mergeWorkflowMaps(input.Input, nil), nil
		},
	}
}

func InputEchoNodeExecutor(nodeType string) NodeExecutor {
	return functionNodeExecutor{
		nodeType: strings.TrimSpace(nodeType),
		execute: func(_ context.Context, input NodeExecutorInput) (map[string]any, error) {
			return map[string]any{"input": mergeWorkflowMaps(input.Input, nil)}, nil
		},
	}
}

func UserInputNodeExecutor() NodeExecutor {
	return userInputNodeExecutor("user_input")
}

func userInputNodeExecutor(nodeType string) NodeExecutor {
	return functionNodeExecutor{
		nodeType: strings.TrimSpace(nodeType),
		execute: func(context.Context, NodeExecutorInput) (map[string]any, error) {
			return nil, ErrWorkflowUserInputRequired
		},
	}
}

func ConditionNodeExecutor() NodeExecutor {
	return functionNodeExecutor{
		nodeType: "condition",
		execute: func(_ context.Context, input NodeExecutorInput) (map[string]any, error) {
			matched, err := evaluateCondition(input)
			if err != nil {
				return nil, err
			}
			branch := "false"
			if matched {
				branch = "true"
			}
			return map[string]any{
				"matched": matched,
				"branch":  branch,
			}, nil
		},
	}
}

func LoopNodeExecutor() NodeExecutor {
	return functionNodeExecutor{
		nodeType: "loop",
		execute: func(_ context.Context, input NodeExecutorInput) (map[string]any, error) {
			items, err := loopItemsFromInput(input.Input)
			if err != nil {
				return nil, err
			}
			if maxIterations, ok := intFromWorkflowValue(input.Input["maxIterations"]); ok && maxIterations >= 0 && len(items) > maxIterations {
				items = items[:maxIterations]
			}
			if maxIterations, ok := intFromWorkflowValue(input.Input["max_iterations"]); ok && maxIterations >= 0 && len(items) > maxIterations {
				items = items[:maxIterations]
			}
			matched := len(items) > 0
			branch := "false"
			if matched {
				branch = "true"
			}
			output := map[string]any{
				"items":          items,
				"iterationCount": float64(len(items)),
				"matched":        matched,
				"branch":         branch,
			}
			if len(items) > 0 {
				output["currentItem"] = items[0]
				output["index"] = float64(0)
			}
			return output, nil
		},
	}
}

type CodeRunner interface {
	RunWorkflowCode(ctx context.Context, req WorkflowCodeRequest) (*WorkflowCodeResult, error)
}

type WorkflowCodeRequest struct {
	OrganizationID string
	UserID         string
	WorkspaceID    string
	AgentID        string
	RunID          string
	ToolRunID      string
	ToolCallID     string
	ToolName       string
	RequestID      string
	Language       string
	Code           string
	Inputs         map[string]any
	TimeoutMS      int
}

type WorkflowCodeResult struct {
	Output map[string]any
	Logs   []string
	Raw    map[string]any
}

type CodeNodeExecutor struct {
	runner CodeRunner
}

type CodeNodeExecutorOption func(*CodeNodeExecutor)

func NewCodeNodeExecutor(options ...CodeNodeExecutorOption) *CodeNodeExecutor {
	executor := &CodeNodeExecutor{}
	for _, option := range options {
		if option != nil {
			option(executor)
		}
	}
	return executor
}

func WithCodeRunner(runner CodeRunner) CodeNodeExecutorOption {
	return func(executor *CodeNodeExecutor) {
		executor.runner = runner
	}
}

func (e *CodeNodeExecutor) Type() string { return "code" }

func (e *CodeNodeExecutor) Execute(ctx context.Context, input NodeExecutorInput) (map[string]any, error) {
	for _, key := range []string{"outputs", "output", "result"} {
		if output, ok := mapStringAnyFromAny(input.Input[key]); ok {
			return mergeWorkflowMaps(output, nil), nil
		}
	}
	if e == nil || e.runner == nil {
		return nil, fmt.Errorf("%w: code runner is required", ErrInvalidInput)
	}
	req, err := workflowCodeRequestFromInput(input)
	if err != nil {
		return nil, err
	}
	result, err := e.runner.RunWorkflowCode(ctx, req)
	if err != nil {
		return nil, err
	}
	return workflowCodeOutput(result), nil
}

type HTTPNodeExecutor struct {
	client *http.Client
}

func NewHTTPNodeExecutor(client *http.Client) *HTTPNodeExecutor {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPNodeExecutor{client: client}
}

func (e *HTTPNodeExecutor) Type() string { return "http" }

func (e *HTTPNodeExecutor) Execute(ctx context.Context, input NodeExecutorInput) (map[string]any, error) {
	method := strings.ToUpper(strings.TrimSpace(stringFromWorkflowValue(input.Input["method"])))
	if method == "" {
		method = http.MethodGet
	}
	rawURL := strings.TrimSpace(stringFromWorkflowValue(input.Input["url"]))
	if rawURL == "" {
		return nil, fmt.Errorf("%w: http node url is required", ErrInvalidInput)
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("%w: http node url is invalid", ErrInvalidInput)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("%w: http node url must use http or https", ErrInvalidInput)
	}

	var body io.Reader
	if value, ok := input.Input["body"]; ok && value != nil {
		payload, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("%w: marshal http node body: %v", ErrInvalidInput, err)
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range workflowStringMap(input.Input["headers"]) {
		if strings.TrimSpace(key) != "" {
			req.Header.Set(key, value)
		}
	}

	client := e.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	output := map[string]any{
		"statusCode": resp.StatusCode,
		"body":       string(responseBody),
		"headers":    workflowHTTPHeaders(resp.Header),
	}
	if resp.StatusCode >= 400 {
		return output, fmt.Errorf("http node returned status %d", resp.StatusCode)
	}
	return output, nil
}

type KnowledgeRetriever interface {
	RetrieveWithOptions(ctx context.Context, session auth.Session, knowledgeBaseID, query string, options knowledge.KnowledgeRetrievalOptions) ([]knowledge.KnowledgeRetrievalResult, error)
}

type KnowledgeNodeExecutor struct {
	retriever KnowledgeRetriever
}

func NewKnowledgeNodeExecutor(retriever KnowledgeRetriever) *KnowledgeNodeExecutor {
	return &KnowledgeNodeExecutor{retriever: retriever}
}

func (e *KnowledgeNodeExecutor) Type() string { return "knowledge" }

func (e *KnowledgeNodeExecutor) Execute(ctx context.Context, input NodeExecutorInput) (map[string]any, error) {
	if e == nil || e.retriever == nil {
		return nil, fmt.Errorf("%w: knowledge retriever is required", ErrInvalidInput)
	}
	knowledgeBaseID := firstWorkflowString(input.Input, "knowledgeBaseId", "knowledgeBaseID", "kbId", "kbID")
	if knowledgeBaseID == "" {
		return nil, fmt.Errorf("%w: knowledge node knowledgeBaseId is required", ErrInvalidInput)
	}
	query := strings.TrimSpace(stringFromWorkflowValue(input.Input["query"]))
	if query == "" {
		return nil, fmt.Errorf("%w: knowledge node query is required", ErrInvalidInput)
	}
	options := knowledgeNodeRetrievalOptions(input.Input)
	results, err := e.retriever.RetrieveWithOptions(ctx, workflowKnowledgeSession(input.Execution), knowledgeBaseID, query, options)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"results":         results,
		"count":           len(results),
		"query":           query,
		"knowledgeBaseId": knowledgeBaseID,
	}, nil
}

type DatabaseRunner interface {
	RunDatabaseQuery(ctx context.Context, req WorkflowDatabaseRequest) (*WorkflowDatabaseResult, error)
}

type WorkflowDatabaseRequest struct {
	OrganizationID string
	UserID         string
	WorkspaceID    string
	ConnectionID   string
	Query          string
	Parameters     []any
	Limit          int
	ReadOnly       bool
}

type WorkflowDatabaseResult struct {
	Rows         []map[string]any
	RowsAffected int64
}

type ToolRunner interface {
	RunWorkflowTool(ctx context.Context, req WorkflowToolRequest) (*WorkflowToolResult, error)
}

type WorkflowToolRequest struct {
	OrganizationID string
	UserID         string
	WorkspaceID    string
	ToolName       string
	ToolType       string
	ServerID       string
	Arguments      map[string]any
	Metadata       map[string]any
}

type WorkflowToolResult struct {
	Content string
	IsError bool
	Output  map[string]any
	Raw     map[string]any
}

type DatabaseNodeExecutor struct {
	runner DatabaseRunner
}

func NewDatabaseNodeExecutor(runner DatabaseRunner) *DatabaseNodeExecutor {
	return &DatabaseNodeExecutor{runner: runner}
}

func (e *DatabaseNodeExecutor) Type() string { return "database" }

func (e *DatabaseNodeExecutor) Execute(ctx context.Context, input NodeExecutorInput) (map[string]any, error) {
	if e == nil || e.runner == nil {
		return nil, fmt.Errorf("%w: database runner is required", ErrInvalidInput)
	}
	req, err := workflowDatabaseRequestFromInput(input)
	if err != nil {
		return nil, err
	}
	result, err := e.runner.RunDatabaseQuery(ctx, req)
	if err != nil {
		return nil, err
	}
	return workflowDatabaseOutput(result), nil
}

type ToolNodeExecutor struct {
	runner ToolRunner
}

func NewToolNodeExecutor(runner ToolRunner) *ToolNodeExecutor {
	return &ToolNodeExecutor{runner: runner}
}

func (e *ToolNodeExecutor) Type() string { return "tool" }

func (e *ToolNodeExecutor) Execute(ctx context.Context, input NodeExecutorInput) (map[string]any, error) {
	if e == nil || e.runner == nil {
		return nil, fmt.Errorf("%w: tool runner is required", ErrInvalidInput)
	}
	req, err := workflowToolRequestFromInput(input)
	if err != nil {
		return nil, err
	}
	result, err := e.runner.RunWorkflowTool(ctx, req)
	if err != nil {
		return nil, err
	}
	return workflowToolOutput(result), nil
}

type RPARunner interface {
	RunRPA(ctx context.Context, req WorkflowRPARequest) (*WorkflowRPAResult, error)
}

type WorkflowRPARequest struct {
	OrganizationID string
	UserID         string
	WorkspaceID    string
	TargetURL      string
	Steps          []WorkflowRPAStep
	TimeoutMS      int
	Screenshot     bool
	BrowserMode    string
}

type WorkflowRPAStep struct {
	Action   string
	Selector string
	Value    string
	Options  map[string]any
}

type WorkflowRPAResult struct {
	FinalURL   string
	Screenshot *WorkflowRPAArtifact
	Steps      []WorkflowRPAStepResult
	Output     map[string]any
}

type WorkflowRPAArtifact struct {
	ContentType string
	URL         string
}

type WorkflowRPAStepResult struct {
	Action  string
	Status  string
	Message string
	Output  map[string]any
}

type RPANodeExecutor struct {
	runner RPARunner
}

func NewRPANodeExecutor(runner RPARunner) *RPANodeExecutor {
	return &RPANodeExecutor{runner: runner}
}

func (e *RPANodeExecutor) Type() string { return "rpa" }

func (e *RPANodeExecutor) Execute(ctx context.Context, input NodeExecutorInput) (map[string]any, error) {
	if e == nil || e.runner == nil {
		return nil, fmt.Errorf("%w: rpa runner is required", ErrInvalidInput)
	}
	req, err := workflowRPARequestFromInput(input)
	if err != nil {
		return nil, err
	}
	result, err := e.runner.RunRPA(ctx, req)
	if err != nil {
		return nil, err
	}
	return workflowRPAOutput(result), nil
}

func knowledgeNodeRetrievalOptions(input map[string]any) knowledge.KnowledgeRetrievalOptions {
	options := knowledge.KnowledgeRetrievalOptions{
		Mode:            strings.TrimSpace(stringFromWorkflowValue(input["mode"])),
		DocumentVersion: strings.TrimSpace(stringFromWorkflowValue(input["documentVersion"])),
	}
	if limit, ok := intFromWorkflowValue(input["limit"]); ok {
		options.Limit = limit
	}
	if minScore, ok := numberFromWorkflowValue(input["minScore"]); ok {
		options.MinScore = minScore
	}
	if vectorWeight, ok := numberFromWorkflowValue(input["vectorWeight"]); ok {
		options.VectorWeight = vectorWeight
	}
	if keywordWeight, ok := numberFromWorkflowValue(input["keywordWeight"]); ok {
		options.KeywordWeight = keywordWeight
	}
	if allVersions, ok := boolFromWorkflowValue(input["allVersions"]); ok {
		options.AllVersions = allVersions
	}
	return options
}

func workflowDatabaseRequestFromInput(input NodeExecutorInput) (WorkflowDatabaseRequest, error) {
	nodeInput := input.Input
	if nodeInput == nil {
		nodeInput = map[string]any{}
	}
	req := WorkflowDatabaseRequest{
		OrganizationID: workflowDatabaseOrganizationID(input),
		UserID:         firstWorkflowString(nodeInput, "userId", "userID", "user_id"),
		WorkspaceID:    firstWorkflowString(nodeInput, "workspaceId", "workspaceID", "workspace_id"),
		ConnectionID:   firstWorkflowString(nodeInput, "connectionId", "connectionID", "connection_id", "datasourceId", "datasourceID", "datasource_id"),
		Query:          strings.TrimSpace(stringFromWorkflowValue(nodeInput["query"])),
		Parameters:     workflowDatabaseParameters(nodeInput["parameters"]),
		ReadOnly:       true,
	}
	if input.Execution != nil {
		if req.UserID == "" {
			req.UserID = firstWorkflowString(input.Execution.Context, "userId", "userID", "user_id")
		}
		if req.WorkspaceID == "" {
			req.WorkspaceID = firstWorkflowString(input.Execution.Context, "workspaceId", "workspaceID", "workspace_id")
		}
	}
	if readOnly, ok := boolFromWorkflowValue(nodeInput["readOnly"]); ok {
		req.ReadOnly = readOnly
	} else if readOnly, ok := boolFromWorkflowValue(nodeInput["read_only"]); ok {
		req.ReadOnly = readOnly
	}
	if limit, ok := intFromWorkflowValue(nodeInput["limit"]); ok && limit > 0 {
		req.Limit = limit
	}
	if req.OrganizationID == "" {
		return WorkflowDatabaseRequest{}, fmt.Errorf("%w: organization ID is required for database node", ErrInvalidInput)
	}
	if req.Query == "" {
		return WorkflowDatabaseRequest{}, fmt.Errorf("%w: database node query is required", ErrInvalidInput)
	}
	return req, nil
}

func workflowDatabaseOrganizationID(input NodeExecutorInput) string {
	if input.Execution != nil && strings.TrimSpace(input.Execution.OrganizationID) != "" {
		return strings.TrimSpace(input.Execution.OrganizationID)
	}
	if input.Workflow != nil {
		return strings.TrimSpace(input.Workflow.OrganizationID)
	}
	return ""
}

func workflowDatabaseParameters(value any) []any {
	switch typed := value.(type) {
	case []any:
		return append([]any(nil), typed...)
	case []string:
		parameters := make([]any, 0, len(typed))
		for _, parameter := range typed {
			parameters = append(parameters, parameter)
		}
		return parameters
	default:
		return nil
	}
}

func workflowDatabaseOutput(result *WorkflowDatabaseResult) map[string]any {
	if result == nil {
		return map[string]any{
			"rows":         []map[string]any{},
			"rowCount":     0,
			"rowsAffected": int64(0),
		}
	}
	rows := make([]map[string]any, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, mergeWorkflowMaps(row, nil))
	}
	return map[string]any{
		"rows":         rows,
		"rowCount":     len(rows),
		"rowsAffected": result.RowsAffected,
	}
}

func workflowCodeRequestFromInput(input NodeExecutorInput) (WorkflowCodeRequest, error) {
	nodeInput := input.Input
	if nodeInput == nil {
		nodeInput = map[string]any{}
	}
	inputs, err := workflowCodeInputs(nodeInput["inputs"])
	if err != nil {
		return WorkflowCodeRequest{}, err
	}
	req := WorkflowCodeRequest{
		OrganizationID: workflowDatabaseOrganizationID(input),
		UserID:         firstWorkflowString(nodeInput, "userId", "userID", "user_id"),
		WorkspaceID:    firstWorkflowString(nodeInput, "workspaceId", "workspaceID", "workspace_id"),
		Language:       firstWorkflowString(nodeInput, "language", "lang"),
		Code:           strings.TrimSpace(stringFromWorkflowValue(nodeInput["code"])),
		Inputs:         inputs,
	}
	if input.Execution != nil {
		if req.UserID == "" {
			req.UserID = firstWorkflowString(input.Execution.Context, "userId", "userID", "user_id")
		}
		if req.WorkspaceID == "" {
			req.WorkspaceID = firstWorkflowString(input.Execution.Context, "workspaceId", "workspaceID", "workspace_id")
		}
	}
	if timeoutMS, ok := intFromWorkflowValue(nodeInput["timeoutMs"]); ok && timeoutMS > 0 {
		req.TimeoutMS = timeoutMS
	} else if timeoutMS, ok := intFromWorkflowValue(nodeInput["timeout_ms"]); ok && timeoutMS > 0 {
		req.TimeoutMS = timeoutMS
	}
	if req.OrganizationID == "" {
		return WorkflowCodeRequest{}, fmt.Errorf("%w: organization ID is required for code node", ErrInvalidInput)
	}
	if req.Code == "" {
		return WorkflowCodeRequest{}, fmt.Errorf("%w: code node code is required", ErrInvalidInput)
	}
	if req.Language == "" {
		req.Language = "javascript"
	}
	return req, nil
}

func workflowCodeInputs(value any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	inputs, ok := mapStringAnyFromAny(value)
	if !ok {
		return nil, fmt.Errorf("%w: code node inputs must be an object", ErrInvalidInput)
	}
	return mergeWorkflowMaps(inputs, nil), nil
}

func workflowCodeOutput(result *WorkflowCodeResult) map[string]any {
	if result == nil {
		return map[string]any{
			"logs": []string{},
			"raw":  map[string]any{},
		}
	}
	output := mergeWorkflowMaps(result.Output, nil)
	if result.Logs != nil {
		output["logs"] = append([]string(nil), result.Logs...)
	}
	if result.Raw != nil {
		output["raw"] = mergeWorkflowMaps(result.Raw, nil)
	}
	return output
}

func workflowToolRequestFromInput(input NodeExecutorInput) (WorkflowToolRequest, error) {
	nodeInput := input.Input
	if nodeInput == nil {
		nodeInput = map[string]any{}
	}
	arguments, err := workflowToolArguments(nodeInput["arguments"])
	if err != nil {
		return WorkflowToolRequest{}, err
	}
	metadata, err := workflowToolMetadata(nodeInput["metadata"])
	if err != nil {
		return WorkflowToolRequest{}, err
	}
	req := WorkflowToolRequest{
		OrganizationID: workflowDatabaseOrganizationID(input),
		UserID:         firstWorkflowString(nodeInput, "userId", "userID", "user_id"),
		WorkspaceID:    firstWorkflowString(nodeInput, "workspaceId", "workspaceID", "workspace_id"),
		ToolName:       firstWorkflowString(nodeInput, "toolName", "tool_name", "name"),
		ToolType:       firstWorkflowString(nodeInput, "toolType", "tool_type", "type"),
		ServerID:       firstWorkflowString(nodeInput, "serverId", "serverID", "server_id"),
		Arguments:      arguments,
		Metadata:       metadata,
	}
	if input.Execution != nil {
		if req.UserID == "" {
			req.UserID = firstWorkflowString(input.Execution.Context, "userId", "userID", "user_id")
		}
		if req.WorkspaceID == "" {
			req.WorkspaceID = firstWorkflowString(input.Execution.Context, "workspaceId", "workspaceID", "workspace_id")
		}
	}
	if req.OrganizationID == "" {
		return WorkflowToolRequest{}, fmt.Errorf("%w: organization ID is required for tool node", ErrInvalidInput)
	}
	if req.ToolName == "" {
		return WorkflowToolRequest{}, fmt.Errorf("%w: tool node toolName is required", ErrInvalidInput)
	}
	if req.ToolType == "" {
		req.ToolType = "builtin"
	}
	return req, nil
}

func workflowToolArguments(value any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	arguments, ok := mapStringAnyFromAny(value)
	if !ok {
		return nil, fmt.Errorf("%w: tool node arguments must be an object", ErrInvalidInput)
	}
	return mergeWorkflowMaps(arguments, nil), nil
}

func workflowToolMetadata(value any) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}
	metadata, ok := mapStringAnyFromAny(value)
	if !ok {
		return nil, fmt.Errorf("%w: tool node metadata must be an object", ErrInvalidInput)
	}
	return mergeWorkflowMaps(metadata, nil), nil
}

func workflowToolOutput(result *WorkflowToolResult) map[string]any {
	if result == nil {
		return map[string]any{
			"content": "",
			"isError": false,
			"output":  map[string]any{},
			"raw":     map[string]any{},
		}
	}
	return map[string]any{
		"content": result.Content,
		"isError": result.IsError,
		"output":  mergeWorkflowMaps(result.Output, nil),
		"raw":     mergeWorkflowMaps(result.Raw, nil),
	}
}

func workflowRPARequestFromInput(input NodeExecutorInput) (WorkflowRPARequest, error) {
	nodeInput := input.Input
	if nodeInput == nil {
		nodeInput = map[string]any{}
	}
	steps, err := workflowRPASteps(nodeInput["steps"])
	if err != nil {
		return WorkflowRPARequest{}, err
	}
	req := WorkflowRPARequest{
		OrganizationID: workflowDatabaseOrganizationID(input),
		UserID:         firstWorkflowString(nodeInput, "userId", "userID", "user_id"),
		WorkspaceID:    firstWorkflowString(nodeInput, "workspaceId", "workspaceID", "workspace_id"),
		TargetURL:      firstWorkflowString(nodeInput, "targetUrl", "targetURL", "target_url", "url"),
		Steps:          steps,
		BrowserMode:    firstWorkflowString(nodeInput, "browserMode", "browser_mode"),
	}
	if input.Execution != nil {
		if req.UserID == "" {
			req.UserID = firstWorkflowString(input.Execution.Context, "userId", "userID", "user_id")
		}
		if req.WorkspaceID == "" {
			req.WorkspaceID = firstWorkflowString(input.Execution.Context, "workspaceId", "workspaceID", "workspace_id")
		}
	}
	if timeoutMS, ok := intFromWorkflowValue(nodeInput["timeoutMs"]); ok && timeoutMS > 0 {
		req.TimeoutMS = timeoutMS
	} else if timeoutMS, ok := intFromWorkflowValue(nodeInput["timeout_ms"]); ok && timeoutMS > 0 {
		req.TimeoutMS = timeoutMS
	}
	if screenshot, ok := boolFromWorkflowValue(nodeInput["screenshot"]); ok {
		req.Screenshot = screenshot
	}
	if req.OrganizationID == "" {
		return WorkflowRPARequest{}, fmt.Errorf("%w: organization ID is required for rpa node", ErrInvalidInput)
	}
	if req.TargetURL == "" {
		return WorkflowRPARequest{}, fmt.Errorf("%w: rpa node targetUrl is required", ErrInvalidInput)
	}
	if !workflowRPAValidTargetURL(req.TargetURL) {
		return WorkflowRPARequest{}, fmt.Errorf("%w: rpa node targetUrl is invalid", ErrInvalidInput)
	}
	return req, nil
}

func workflowRPASteps(value any) ([]WorkflowRPAStep, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case []WorkflowRPAStep:
		return append([]WorkflowRPAStep(nil), typed...), nil
	case []any:
		steps := make([]WorkflowRPAStep, 0, len(typed))
		for _, item := range typed {
			stepInput, ok := mapStringAnyFromAny(item)
			if !ok {
				return nil, fmt.Errorf("%w: rpa node steps must contain objects", ErrInvalidInput)
			}
			action := strings.TrimSpace(stringFromWorkflowValue(stepInput["action"]))
			if action == "" {
				return nil, fmt.Errorf("%w: rpa node step action is required", ErrInvalidInput)
			}
			if !workflowRPAActionSupported(action) {
				return nil, fmt.Errorf("%w: unsupported rpa node step action %s", ErrInvalidInput, action)
			}
			options := map[string]any(nil)
			if rawOptions, exists := stepInput["options"]; exists && rawOptions != nil {
				parsedOptions, ok := mapStringAnyFromAny(rawOptions)
				if !ok {
					return nil, fmt.Errorf("%w: rpa node step options must be an object", ErrInvalidInput)
				}
				options = mergeWorkflowMaps(parsedOptions, nil)
			}
			steps = append(steps, WorkflowRPAStep{
				Action:   action,
				Selector: strings.TrimSpace(stringFromWorkflowValue(stepInput["selector"])),
				Value:    stringFromWorkflowValue(stepInput["value"]),
				Options:  options,
			})
		}
		return steps, nil
	default:
		return nil, fmt.Errorf("%w: rpa node steps must be an array", ErrInvalidInput)
	}
}

func workflowRPAValidTargetURL(rawURL string) bool {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	return (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") && parsedURL.Host != ""
}

func workflowRPAActionSupported(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "goto", "click", "fill", "type", "select", "press", "wait", "wait_for_selector", "waitforselector", "screenshot", "extract", "evaluate":
		return true
	default:
		return false
	}
}

func workflowRPAOutput(result *WorkflowRPAResult) map[string]any {
	if result == nil {
		return map[string]any{
			"finalUrl": "",
			"steps":    []map[string]any{},
			"output":   map[string]any{},
		}
	}
	output := map[string]any{
		"finalUrl": result.FinalURL,
		"steps":    workflowRPAStepResultsOutput(result.Steps),
		"output":   mergeWorkflowMaps(result.Output, nil),
	}
	if result.Screenshot != nil {
		output["screenshot"] = map[string]any{
			"contentType": result.Screenshot.ContentType,
			"url":         result.Screenshot.URL,
		}
	}
	return output
}

func workflowRPAStepResultsOutput(results []WorkflowRPAStepResult) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, result := range results {
		out = append(out, map[string]any{
			"action":  result.Action,
			"status":  result.Status,
			"message": result.Message,
			"output":  mergeWorkflowMaps(result.Output, nil),
		})
	}
	return out
}

func workflowKnowledgeSession(execution *WorkflowExecution) auth.Session {
	session := auth.Session{}
	if execution == nil {
		return session
	}
	session.OrganizationID = strings.TrimSpace(execution.OrganizationID)
	session.WorkspaceID = firstWorkflowString(execution.Context, "workspaceId", "workspaceID", "workspace_id")
	session.ID = firstWorkflowString(execution.Context, "sessionId", "sessionID", "session_id")
	session.User.ID = firstWorkflowString(execution.Context, "userId", "userID", "user_id")
	return session
}

func firstWorkflowString(input map[string]any, keys ...string) string {
	for _, key := range keys {
		if input == nil {
			return ""
		}
		value := strings.TrimSpace(stringFromWorkflowValue(input[key]))
		if value != "" {
			return value
		}
	}
	return ""
}

func intFromWorkflowValue(value any) (int, bool) {
	number, ok := numberFromWorkflowValue(value)
	if !ok {
		return 0, false
	}
	return int(number), true
}

func loopItemsFromInput(input map[string]any) ([]any, error) {
	if input == nil {
		return nil, fmt.Errorf("%w: loop node items are required", ErrInvalidInput)
	}
	value, ok := input["items"]
	if !ok {
		value, ok = input["input"]
	}
	if !ok {
		return nil, fmt.Errorf("%w: loop node items are required", ErrInvalidInput)
	}
	switch typed := value.(type) {
	case []any:
		return append([]any(nil), typed...), nil
	case []string:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("%w: loop node items must be an array", ErrInvalidInput)
	}
}

func defaultNodeExecutorRegistry() *NodeExecutorRegistry {
	return NewNodeExecutorRegistry(
		EchoNodeExecutor("start"),
		EchoNodeExecutor("end"),
		InputEchoNodeExecutor("manual"),
		InputEchoNodeExecutor("trigger"),
		ConditionNodeExecutor(),
		LoopNodeExecutor(),
		NewCodeNodeExecutor(),
		NewHTTPNodeExecutor(nil),
		EchoNodeExecutor("llm"),
		EchoNodeExecutor("knowledge"),
		NewToolNodeExecutor(nil),
		NewDatabaseNodeExecutor(nil),
		NewRPANodeExecutor(nil),
		UserInputNodeExecutor(),
		userInputNodeExecutor("approval"),
	)
}

func DefaultNodeExecutorRegistry() *NodeExecutorRegistry {
	return defaultNodeExecutorRegistry()
}

func (s *Service) RunReadyNode(ctx context.Context, organizationID, executionID, nodeID string) error {
	execution, err := s.getExecutionForTransition(ctx, organizationID, executionID)
	if err != nil {
		return err
	}
	node, err := s.nodeDefinitionForExecution(ctx, organizationID, execution, strings.TrimSpace(nodeID))
	if err != nil {
		return err
	}
	workflow, err := s.GetWorkflow(ctx, organizationID, execution.WorkflowID)
	if err != nil {
		return err
	}
	nodeType := node.Type
	if strings.TrimSpace(nodeType) == "" {
		nodeType = "start"
	}
	executor, ok := s.nodeExecutors.Get(nodeType)
	if !ok {
		err := fmt.Errorf("%w: executor for node type %s", ErrNodeExecutorNotFound, nodeType)
		return s.recordRuntimeNodeFailure(ctx, organizationID, execution, node, err)
	}
	inputSource := node.Input
	if inputSource == nil {
		if pendingInput, ok := latestPendingWorkflowNodeInput(execution.NodeExecutions, node.ID); ok {
			inputSource = pendingInput
		}
	}
	input, err := interpolateNodeInput(inputSource, workflow, execution, node)
	if err != nil {
		return s.recordRuntimeNodeFailure(ctx, organizationID, execution, node, err)
	}
	output, err := executor.Execute(ctx, NodeExecutorInput{
		Workflow:  workflow,
		Execution: execution,
		Node:      node,
		Input:     input,
	})
	if err != nil {
		if errors.Is(err, ErrWorkflowUserInputRequired) {
			return s.recordRuntimeUserInputWait(ctx, organizationID, execution, node, input, output)
		}
		return s.recordRuntimeNodeFailure(ctx, organizationID, execution, node, err)
	}
	now := time.Now().UTC()
	_, err = s.RecordNodeStatus(ctx, organizationID, execution.ID, RecordNodeStatusRequest{
		NodeID:      node.ID,
		NodeType:    nodeType,
		Status:      NodeStatusSucceeded,
		Attempt:     latestNodeAttempt(execution.NodeExecutions, node.ID),
		Input:       input,
		Output:      output,
		StartedAt:   now,
		CompletedAt: &now,
	})
	return err
}

func (s *Service) recordRuntimeUserInputWait(ctx context.Context, organizationID string, execution *WorkflowExecution, node Node, input map[string]any, output map[string]any) error {
	stateMachine := workflowexecutor.NewStateMachineWithStatus(string(execution.Status))
	if _, err := stateMachine.Transition(string(StateEventPause)); err != nil {
		if errors.Is(err, workflowexecutor.ErrInvalidStateTransition) || errors.Is(err, workflowexecutor.ErrStateMachineLocked) {
			return fmt.Errorf("%w: %v", ErrInvalidTransition, err)
		}
		return err
	}

	nodeType := node.Type
	if strings.TrimSpace(nodeType) == "" {
		nodeType = "user_input"
	}
	waitReason := "user_input_required"
	if strings.TrimSpace(nodeType) == "agent" {
		waitReason = "agent_approval_required"
	} else if strings.TrimSpace(nodeType) == "approval" {
		waitReason = "approval_required"
	}
	if _, err := s.store.CreateNodeExecution(ctx, organizationID, execution.ID, CreateNodeExecutionRequest{
		NodeID:   node.ID,
		NodeType: nodeType,
		Status:   NodeStatusPending,
		Attempt:  latestNodeAttempt(execution.NodeExecutions, node.ID),
		Input:    mergeWorkflowMaps(input, nil),
		Output:   mergeWorkflowMaps(output, nil),
		Context: map[string]any{
			"waitReason": waitReason,
		},
	}); err != nil {
		return err
	}
	if _, err := s.transitionExecutionStatus(ctx, organizationID, execution.ID, StateEventPause, nil); err != nil {
		return err
	}
	return ErrWorkflowUserInputRequired
}

func (s *Service) recordRuntimeNodeFailure(ctx context.Context, organizationID string, execution *WorkflowExecution, node Node, runtimeErr error) error {
	nodeType := node.Type
	if strings.TrimSpace(nodeType) == "" {
		nodeType = "start"
	}
	now := time.Now().UTC()
	_, err := s.RecordNodeStatus(ctx, organizationID, execution.ID, RecordNodeStatusRequest{
		NodeID:      node.ID,
		NodeType:    nodeType,
		Status:      NodeStatusFailed,
		Attempt:     latestNodeAttempt(execution.NodeExecutions, node.ID),
		Error:       map[string]any{"message": runtimeErr.Error()},
		StartedAt:   now,
		CompletedAt: &now,
	})
	if err != nil {
		return err
	}
	return runtimeErr
}

var (
	workflowTemplatePattern      = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
	workflowWholeTemplatePattern = regexp.MustCompile(`^\s*\{\{\s*([^{}]+?)\s*\}\}\s*$`)
)

func interpolateNodeInput(input map[string]any, workflow *WorkflowDefinition, execution *WorkflowExecution, currentNode Node) (map[string]any, error) {
	if input == nil {
		return mergeWorkflowMaps(execution.Input, nil), nil
	}
	localVariables, err := resolveCurrentNodeLocalVariables(currentNode, workflow, execution)
	if err != nil {
		return nil, err
	}
	resolved, err := interpolateValue(input, workflow, execution, currentNode, localVariables)
	if err != nil {
		return nil, err
	}
	mapped, ok := resolved.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: node input must resolve to an object", ErrInvalidInput)
	}
	return mapped, nil
}

func interpolateValue(value any, workflow *WorkflowDefinition, execution *WorkflowExecution, currentNode Node, localVariables map[string]any) (any, error) {
	switch typed := value.(type) {
	case string:
		if parts := workflowWholeTemplatePattern.FindStringSubmatch(typed); len(parts) == 2 {
			return resolveWorkflowVariable(strings.TrimSpace(parts[1]), workflow, execution, currentNode, localVariables)
		}
		return interpolateString(typed, workflow, execution, currentNode, localVariables)
	case map[string]any:
		next := make(map[string]any, len(typed))
		for key, item := range typed {
			resolved, err := interpolateValue(item, workflow, execution, currentNode, localVariables)
			if err != nil {
				return nil, err
			}
			next[key] = resolved
		}
		return next, nil
	case []any:
		next := make([]any, 0, len(typed))
		for _, item := range typed {
			resolved, err := interpolateValue(item, workflow, execution, currentNode, localVariables)
			if err != nil {
				return nil, err
			}
			next = append(next, resolved)
		}
		return next, nil
	default:
		return value, nil
	}
}

func interpolateString(template string, workflow *WorkflowDefinition, execution *WorkflowExecution, currentNode Node, localVariables map[string]any) (string, error) {
	var interpolateErr error
	resolved := workflowTemplatePattern.ReplaceAllStringFunc(template, func(match string) string {
		if interpolateErr != nil {
			return match
		}
		parts := workflowTemplatePattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		value, err := resolveWorkflowVariable(strings.TrimSpace(parts[1]), workflow, execution, currentNode, localVariables)
		if err != nil {
			interpolateErr = err
			return match
		}
		return fmt.Sprint(value)
	})
	if interpolateErr != nil {
		return "", interpolateErr
	}
	return resolved, nil
}

func resolveWorkflowVariable(path string, workflow *WorkflowDefinition, execution *WorkflowExecution, currentNode Node, localVariables map[string]any) (any, error) {
	switch {
	case path == "execution.id":
		if execution == nil || strings.TrimSpace(execution.ID) == "" {
			return nil, fmt.Errorf("%w: %s", ErrWorkflowVariableNotFound, path)
		}
		return execution.ID, nil
	case path == "execution.started_at":
		if execution == nil || execution.StartedAt.IsZero() {
			return nil, fmt.Errorf("%w: %s", ErrWorkflowVariableNotFound, path)
		}
		return execution.StartedAt.Format(time.RFC3339Nano), nil
	case path == "workflow.name":
		return workflow.Name, nil
	case path == "org.id":
		if execution == nil || strings.TrimSpace(execution.OrganizationID) == "" {
			return nil, fmt.Errorf("%w: %s", ErrWorkflowVariableNotFound, path)
		}
		return execution.OrganizationID, nil
	case path == "user.id":
		return resolveWorkflowUserID(execution, path)
	case strings.HasPrefix(path, "workflow."):
		return lookupPath(workflow.Variables, strings.TrimPrefix(path, "workflow."), path)
	case strings.HasPrefix(path, "input."):
		return lookupPath(execution.Input, strings.TrimPrefix(path, "input."), path)
	case strings.HasPrefix(path, "node."):
		return resolveCurrentNodeVariable(path, currentNode, localVariables)
	case strings.HasPrefix(path, "nodes."):
		return lookupPath(nodeOutputsByID(execution.NodeExecutions), strings.TrimPrefix(path, "nodes."), path)
	default:
		return nil, fmt.Errorf("%w: %s", ErrWorkflowVariableNotFound, path)
	}
}

func resolveCurrentNodeLocalVariables(currentNode Node, workflow *WorkflowDefinition, execution *WorkflowExecution) (map[string]any, error) {
	if len(currentNode.Variables) == 0 {
		return nil, nil
	}
	resolved := make(map[string]any, len(currentNode.Variables))
	for key, value := range currentNode.Variables {
		next, err := interpolateValue(value, workflow, execution, Node{}, nil)
		if err != nil {
			return nil, err
		}
		resolved[key] = next
	}
	return resolved, nil
}

func resolveCurrentNodeVariable(path string, currentNode Node, localVariables map[string]any) (any, error) {
	prefix := "node." + strings.TrimSpace(currentNode.ID) + "."
	if prefix == "node.." || !strings.HasPrefix(path, prefix) {
		return nil, fmt.Errorf("%w: %s", ErrWorkflowVariableNotFound, path)
	}
	return lookupPath(localVariables, strings.TrimPrefix(path, prefix), path)
}

func resolveWorkflowUserID(execution *WorkflowExecution, originalPath string) (any, error) {
	if execution == nil {
		return nil, fmt.Errorf("%w: %s", ErrWorkflowVariableNotFound, originalPath)
	}
	if value := firstWorkflowString(execution.Context, "userId", "userID", "user_id"); value != "" {
		return value, nil
	}
	if value, err := lookupPath(execution.Context, "user.id", originalPath); err == nil {
		if text := strings.TrimSpace(stringFromWorkflowValue(value)); text != "" {
			return text, nil
		}
	}
	if value, err := lookupPath(execution.Context, "trigger.user.id", originalPath); err == nil {
		if text := strings.TrimSpace(stringFromWorkflowValue(value)); text != "" {
			return text, nil
		}
	}
	if value, err := lookupPath(execution.Context, "trigger.userId", originalPath); err == nil {
		if text := strings.TrimSpace(stringFromWorkflowValue(value)); text != "" {
			return text, nil
		}
	}
	if value, err := lookupPath(execution.Context, "trigger.user_id", originalPath); err == nil {
		if text := strings.TrimSpace(stringFromWorkflowValue(value)); text != "" {
			return text, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrWorkflowVariableNotFound, originalPath)
}

func evaluateCondition(input NodeExecutorInput) (bool, error) {
	config := input.Input
	if config == nil {
		config = map[string]any{}
	}
	if expression := strings.TrimSpace(stringFromWorkflowValue(config["expression"])); expression != "" {
		return evaluateConditionExpression(expression)
	}
	if field := strings.TrimSpace(stringFromWorkflowValue(config["field"])); field != "" {
		left, err := resolveConditionFieldValue(field, input)
		if err != nil {
			left = field
		}
		if right, ok := config["equals"]; ok {
			return compareConditionValues(left, "equals", right)
		}
		if right, ok := config["notEquals"]; ok {
			return compareConditionValues(left, "not_equals", right)
		}
		if right, ok := config["not_equals"]; ok {
			return compareConditionValues(left, "not_equals", right)
		}
		operator := strings.TrimSpace(stringFromWorkflowValue(config["operator"]))
		if operator == "" {
			operator = "exists"
		}
		return compareConditionValues(left, operator, config["right"])
	}
	if left, ok := config["left"]; ok {
		operator := strings.TrimSpace(stringFromWorkflowValue(config["operator"]))
		if operator == "" {
			if _, hasRight := config["right"]; hasRight {
				operator = "equals"
			} else {
				operator = "truthy"
			}
		}
		return compareConditionValues(left, operator, config["right"])
	}
	return false, fmt.Errorf("%w: condition node requires expression, field, or left", ErrInvalidInput)
}

func evaluateConditionExpression(expression string) (bool, error) {
	expression = strings.TrimSpace(expression)
	if matched, ok := boolFromWorkflowValue(expression); ok {
		return matched, nil
	}
	for _, operator := range []string{" not contains ", " contains ", ">=", "<=", "!=", "==", ">", "<", "="} {
		index := strings.Index(strings.ToLower(expression), operator)
		if index < 0 {
			continue
		}
		left := stripConditionOperand(expression[:index])
		right := stripConditionOperand(expression[index+len(operator):])
		return compareConditionValues(left, strings.TrimSpace(operator), right)
	}
	return false, fmt.Errorf("%w: condition expression is invalid", ErrInvalidInput)
}

func stripConditionOperand(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 {
		if (trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') || (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') {
			return trimmed[1 : len(trimmed)-1]
		}
	}
	return trimmed
}

func resolveConditionFieldValue(field string, input NodeExecutorInput) (any, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return nil, fmt.Errorf("%w: condition field is required", ErrInvalidInput)
	}
	if strings.HasPrefix(field, "workflow.") || strings.HasPrefix(field, "input.") || strings.HasPrefix(field, "node.") || strings.HasPrefix(field, "nodes.") {
		localVariables, err := resolveCurrentNodeLocalVariables(input.Node, input.Workflow, input.Execution)
		if err != nil {
			return nil, err
		}
		return resolveWorkflowVariable(field, input.Workflow, input.Execution, input.Node, localVariables)
	}
	if input.Execution != nil {
		if value, err := lookupPath(input.Execution.Input, field, "input."+field); err == nil {
			return value, nil
		}
	}
	if value, err := lookupPath(input.Input, field, field); err == nil {
		return value, nil
	}
	return nil, fmt.Errorf("%w: condition field %s", ErrWorkflowVariableNotFound, field)
}

func compareConditionValues(left any, operator string, right any) (bool, error) {
	switch normalizeConditionOperator(operator) {
	case "equals":
		return workflowValuesEqual(left, right), nil
	case "not_equals":
		return !workflowValuesEqual(left, right), nil
	case "contains":
		return strings.Contains(stringFromWorkflowValue(left), stringFromWorkflowValue(right)), nil
	case "not_contains":
		return !strings.Contains(stringFromWorkflowValue(left), stringFromWorkflowValue(right)), nil
	case "greater_than":
		return compareNumericCondition(left, right, func(leftNumber, rightNumber float64) bool {
			return leftNumber > rightNumber
		})
	case "greater_or_equal":
		return compareNumericCondition(left, right, func(leftNumber, rightNumber float64) bool {
			return leftNumber >= rightNumber
		})
	case "less_than":
		return compareNumericCondition(left, right, func(leftNumber, rightNumber float64) bool {
			return leftNumber < rightNumber
		})
	case "less_or_equal":
		return compareNumericCondition(left, right, func(leftNumber, rightNumber float64) bool {
			return leftNumber <= rightNumber
		})
	case "exists":
		return conditionValueExists(left), nil
	case "not_exists":
		return !conditionValueExists(left), nil
	case "truthy":
		return conditionValueTruthy(left), nil
	default:
		return false, fmt.Errorf("%w: unsupported condition operator %s", ErrInvalidInput, operator)
	}
}

func normalizeConditionOperator(operator string) string {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case "", "equals", "equal", "eq", "==", "=":
		return "equals"
	case "not_equals", "not equals", "not_equal", "ne", "!=", "<>":
		return "not_equals"
	case "contains", "includes":
		return "contains"
	case "not_contains", "not contains", "not_includes":
		return "not_contains"
	case "greater_than", "greater than", "gt", ">":
		return "greater_than"
	case "greater_or_equal", "greater or equal", "greater_than_or_equal", "gte", ">=":
		return "greater_or_equal"
	case "less_than", "less than", "lt", "<":
		return "less_than"
	case "less_or_equal", "less or equal", "less_than_or_equal", "lte", "<=":
		return "less_or_equal"
	case "exists", "present":
		return "exists"
	case "not_exists", "not exists", "missing":
		return "not_exists"
	case "truthy", "true":
		return "truthy"
	default:
		return strings.ToLower(strings.TrimSpace(operator))
	}
}

func workflowValuesEqual(left any, right any) bool {
	if leftNumber, ok := numberFromWorkflowValue(left); ok {
		if rightNumber, ok := numberFromWorkflowValue(right); ok {
			return leftNumber == rightNumber
		}
	}
	if leftBool, ok := boolFromWorkflowValue(left); ok {
		if rightBool, ok := boolFromWorkflowValue(right); ok {
			return leftBool == rightBool
		}
	}
	return stringFromWorkflowValue(left) == stringFromWorkflowValue(right)
}

func compareNumericCondition(left any, right any, compare func(float64, float64) bool) (bool, error) {
	leftNumber, leftOK := numberFromWorkflowValue(left)
	rightNumber, rightOK := numberFromWorkflowValue(right)
	if !leftOK || !rightOK {
		return false, fmt.Errorf("%w: condition operator requires numeric values", ErrInvalidInput)
	}
	return compare(leftNumber, rightNumber), nil
}

func numberFromWorkflowValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		var parsed float64
		if _, err := fmt.Sscanf(trimmed, "%f", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func boolFromWorkflowValue(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true":
			return true, true
		case "false":
			return false, true
		}
	}
	return false, false
}

func conditionValueExists(value any) bool {
	if value == nil {
		return false
	}
	return strings.TrimSpace(stringFromWorkflowValue(value)) != ""
}

func conditionValueTruthy(value any) bool {
	if typed, ok := boolFromWorkflowValue(value); ok {
		return typed
	}
	if number, ok := numberFromWorkflowValue(value); ok {
		return number != 0
	}
	trimmed := strings.ToLower(strings.TrimSpace(stringFromWorkflowValue(value)))
	return trimmed != "" && trimmed != "null" && trimmed != "nil" && trimmed != "0"
}

func nodeOutputsByID(nodes []WorkflowNodeExecution) map[string]any {
	outputs := map[string]any{}
	for _, node := range nodes {
		if !isSuccessfulNodeStatus(node.Status) {
			continue
		}
		outputs[node.NodeID] = map[string]any{"output": node.Output}
	}
	return outputs
}

func stringFromWorkflowValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	}
}

func workflowStringMap(value any) map[string]string {
	out := map[string]string{}
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			out[key] = stringFromWorkflowValue(item)
		}
	case map[string]string:
		for key, item := range typed {
			out[key] = item
		}
	}
	return out
}

func workflowHTTPHeaders(headers http.Header) map[string]any {
	out := make(map[string]any, len(headers))
	for key, values := range headers {
		if len(values) == 1 {
			out[key] = values[0]
			continue
		}
		copied := append([]string(nil), values...)
		out[key] = copied
	}
	return out
}

func lookupPath(root map[string]any, path string, originalPath string) (any, error) {
	var current any = root
	for _, part := range strings.Split(path, ".") {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrWorkflowVariableNotFound, originalPath)
		}
		value, ok := currentMap[part]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrWorkflowVariableNotFound, originalPath)
		}
		current = value
	}
	return current, nil
}
