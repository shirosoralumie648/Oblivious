package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"oblivious/server/internal/mcp"
	"oblivious/server/internal/releasecontract"
)

// UnmarshalJSON rejects caller-supplied catalog authority while preserving
// the existing business-field request shape. Capability identity is response
// metadata only and is never persisted.
func (t *Tool) UnmarshalJSON(data []byte) error {
	type alias Tool
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if _, ok := fields["capabilityId"]; ok {
		return fmt.Errorf("capabilityId is server-managed")
	}
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*t = Tool(decoded)
	return nil
}

// ToolExecutor 工具执行器
type ToolExecutor struct {
	mcpClient                 *mcp.Client
	builtinTools              map[string]mcp.BuiltinTool
	webSearchProvider         mcp.WebSearchProvider
	customPythonSandboxRunner CustomPythonSandboxRunner
	authorities               releasecontract.RuntimeAuthorities
	httpClient                *http.Client
	capabilities              map[string]releasecontract.CapabilityID
	pythonProcessRunner       func(context.Context, string, io.Reader) ([]byte, error)
	strict                    bool
}

// ToolRuntimeOptions carries the single startup authority into the live
// Agent executor. Additional effect runners are injectable test seams; they
// never provide capability authority.
type ToolRuntimeOptions struct {
	Authorities         releasecontract.RuntimeAuthorities
	Guard               releasecontract.Guard
	Effects             releasecontract.EffectRegistrar
	HTTPClient          *http.Client
	BuiltinTools        map[string]mcp.BuiltinTool
	WebSearchProvider   mcp.WebSearchProvider
	CustomPythonSandbox CustomPythonSandboxRunner
	PythonProcessRunner func(context.Context, string, io.Reader) ([]byte, error)
}

// NewToolExecutor 创建工具执行器
func NewToolExecutor(mcpClient *mcp.Client) *ToolExecutor {
	return &ToolExecutor{
		mcpClient:    mcpClient,
		builtinTools: mcp.BuiltinTools,
		httpClient:   http.DefaultClient,
	}
}

// NewAuthorizedToolExecutor is the production-capable constructor. It
// resolves and registers every Agent effect descriptor exactly once before
// returning and retains no generation or verdict state.
func NewAuthorizedToolExecutor(mcpClient *mcp.Client, options ToolRuntimeOptions) (*ToolExecutor, error) {
	if options.Guard == nil || options.Effects == nil || !options.Authorities.Valid() || options.HTTPClient == nil {
		return nil, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "agent.toolExecutor"}
	}
	effects := []struct {
		key   string
		id    releasecontract.EffectID
		owner string
	}{
		{"builtin", releasecontract.EffectAgentToolBuiltin, "agent.ToolExecutor.builtin"},
		{"custom_api_http", releasecontract.EffectAgentToolCustomAPIHTTP, "agent.ToolExecutor.custom_api_http"},
		{"custom_python_process", releasecontract.EffectAgentToolLocalPython, "agent.ToolExecutor.local_python"},
		{"custom_python_sandbox", releasecontract.EffectAgentToolPythonSandbox, "agent.ToolExecutor.python_sandbox"},
		{"mcp", releasecontract.EffectAgentToolMCP, "agent.ToolExecutor.mcp"},
	}
	capabilities := make(map[string]releasecontract.CapabilityID, len(effects))
	descriptors := make([]releasecontract.EffectDescriptor, 0, len(effects))
	for _, effect := range effects {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		if err != nil {
			if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
				continue
			}
			return nil, err
		}
		capabilities[effect.key] = capability
		descriptors = append(descriptors, releasecontract.EffectDescriptor{
			ID: string(effect.id), CapabilityID: string(capability), Boundary: releasecontract.BoundaryOutbound, Owner: effect.owner,
		})
	}
	for _, descriptor := range descriptors {
		if err := options.Effects.Register(descriptor); err != nil {
			return nil, err
		}
	}
	builtinTools := options.BuiltinTools
	if builtinTools == nil {
		builtinTools = mcp.BuiltinTools
	}
	clone := make(map[string]mcp.BuiltinTool, len(builtinTools))
	for name, tool := range builtinTools {
		clone[name] = tool
	}
	e := &ToolExecutor{
		mcpClient:                 mcpClient,
		builtinTools:              clone,
		webSearchProvider:         options.WebSearchProvider,
		customPythonSandboxRunner: options.CustomPythonSandbox,
		authorities:               options.Authorities,
		httpClient:                options.HTTPClient,
		capabilities:              capabilities,
		pythonProcessRunner:       options.PythonProcessRunner,
		strict:                    true,
	}
	if options.WebSearchProvider != nil {
		webSearchCapability, err := options.Authorities.CapabilityBindings.Resolve(releasecontract.EffectToolWebSearch)
		if err != nil {
			return nil, err
		}
		e.capabilities["web_search"] = webSearchCapability
		tool, err := mcp.NewWebSearchTool(options.WebSearchProvider, mcp.WebSearchRuntimeOptions{
			Authorities: options.Authorities, Guard: options.Guard, Effects: options.Effects,
		})
		if err != nil {
			return nil, err
		}
		e.builtinTools["web_search"] = tool
	}
	return e, nil
}

func (e *ToolExecutor) SetWebSearchProvider(provider mcp.WebSearchProvider) {
	e.webSearchProvider = provider
}

func (e *ToolExecutor) SetCustomPythonSandboxRunner(runner CustomPythonSandboxRunner) {
	e.customPythonSandboxRunner = runner
}

// ExecuteResult 工具执行结果
type ExecuteResult struct {
	Content string `json:"content"`
	IsError bool   `json:"isError,omitempty"`
}

type CustomPythonSandboxRequest struct {
	OrganizationID string
	UserID         string
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

type CustomPythonSandboxResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Logs     []string
	Raw      map[string]any
}

type CustomPythonSandboxRunner interface {
	RunCustomPython(ctx context.Context, req CustomPythonSandboxRequest) (*CustomPythonSandboxResult, error)
}

// Execute 执行工具调用
func (e *ToolExecutor) Execute(ctx context.Context, agent *Agent, toolCall *ToolCall) (*ExecuteResult, error) {
	if agent == nil || toolCall == nil {
		return nil, fmt.Errorf("agent and tool call are required")
	}
	// 查找工具配置
	var targetTool *Tool
	for _, t := range agent.Tools {
		if t.Name == toolCall.Name && t.Enabled {
			targetTool = &t
			break
		}
	}
	if targetTool == nil {
		return nil, fmt.Errorf("tool not found or disabled: %s", toolCall.Name)
	}

	// 根据工具类型执行
	switch targetTool.Type {
	case "builtin":
		return e.executeBuiltin(ctx, targetTool, toolCall)
	case "mcp":
		return e.executeMCP(ctx, agent.OrganizationID, targetTool.ServerID, toolCall)
	case "custom":
		if normalizeCustomToolRuntime(targetTool.Runtime) == "python" {
			return e.executeCustomPython(ctx, agent, targetTool, toolCall)
		}
		return e.executeCustomAPI(ctx, targetTool, toolCall)
	default:
		return nil, fmt.Errorf("unknown tool type: %s", targetTool.Type)
	}
}

func (e *ToolExecutor) authorizeTool(ctx context.Context, tool *Tool) error {
	if !e.strict {
		return nil
	}
	if !e.authorities.Valid() {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "agent.toolExecutor.authority"}
	}
	if tool == nil {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "agent.tool"}
	}
	var subject releasecontract.CatalogSubject
	var key string
	switch strings.ToLower(strings.TrimSpace(tool.Type)) {
	case "builtin":
		runtime := releasecontract.CatalogRuntimeBuiltin
		if tool.Name == "web_search" {
			runtime = releasecontract.CatalogRuntimeNetwork
		}
		subject = releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectTool, ID: tool.Name, Runtime: runtime}
		key = "builtin"
		if tool.Name == "web_search" {
			key = "web_search"
		}
	case "custom":
		if normalizeCustomToolRuntime(tool.Runtime) == "python" && customPythonDisabledInProduction() {
			subject = releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectRuntime, ID: "sandbox", Runtime: releasecontract.CatalogRuntimeSandbox}
			key = "custom_python_sandbox"
		} else if normalizeCustomToolRuntime(tool.Runtime) == "python" {
			subject = releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectRuntime, ID: "custom", Runtime: releasecontract.CatalogRuntimeCustom}
			key = "custom_python_process"
		} else {
			subject = releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectRuntime, ID: "custom", Runtime: releasecontract.CatalogRuntimeCustom}
			key = "custom_api_http"
		}
	case "mcp":
		subject = releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectRuntime, ID: "mcp", Runtime: releasecontract.CatalogRuntimeMCP}
		key = "mcp"
	default:
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "agent.tool.type"}
	}
	capability, err := e.authorities.CatalogAuthorizer.ResolveAndRequire(ctx, subject, releasecontract.BoundaryOutbound)
	if err != nil {
		return err
	}
	if expected, ok := e.capabilities[key]; !ok || capability != expected {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "agent.tool.capability"}
	}
	return nil
}

// executeBuiltin 执行内置工具
func (e *ToolExecutor) executeBuiltin(ctx context.Context, persistedTool *Tool, toolCall *ToolCall) (*ExecuteResult, error) {
	if err := e.authorizeTool(ctx, persistedTool); err != nil {
		return nil, err
	}
	tool, ok := e.builtinTools[toolCall.Name]
	if !ok {
		return nil, fmt.Errorf("builtin tool not found: %s", toolCall.Name)
	}
	if !e.isCommercialBuiltinEnabled(toolCall.Name) {
		return &ExecuteResult{
			Content: fmt.Sprintf("builtin tool %s is disabled for default commercial use", toolCall.Name),
			IsError: true,
		}, nil
	}
	if !e.strict && toolCall.Name == "web_search" && e.webSearchProvider != nil {
		webSearch, err := mcp.NewWebSearchTool(e.webSearchProvider)
		if err != nil {
			return nil, err
		}
		result, err := webSearch.Execute(ctx, toolCall.Arguments)
		if err != nil {
			return &ExecuteResult{Content: err.Error(), IsError: true}, nil
		}
		return &ExecuteResult{Content: result.Content, IsError: result.IsError}, nil
	}
	result, err := tool.Execute(ctx, toolCall.Arguments)
	if err != nil {
		return &ExecuteResult{
			Content: err.Error(),
			IsError: true,
		}, nil
	}

	return &ExecuteResult{
		Content: result.Content,
		IsError: result.IsError,
	}, nil
}

// executeMCP 执行 MCP 工具
func (e *ToolExecutor) executeMCP(ctx context.Context, organizationID, serverID string, toolCall *ToolCall) (*ExecuteResult, error) {
	if e.strict {
		if err := e.authorizeTool(ctx, &Tool{Name: toolCall.Name, Type: "mcp", ServerID: serverID, Enabled: true}); err != nil {
			return nil, err
		}
	}
	if e.mcpClient == nil {
		return nil, fmt.Errorf("MCP client not configured")
	}
	if serverID == "" {
		return nil, fmt.Errorf("MCP server not specified")
	}

	result, err := e.mcpClient.CallTool(ctx, serverID, organizationID, toolCall.Name, toolCall.Arguments)
	if err != nil {
		return &ExecuteResult{
			Content: err.Error(),
			IsError: true,
		}, nil
	}

	return &ExecuteResult{
		Content: result.Content,
		IsError: result.IsError,
	}, nil
}

// executeCustomAPI 执行自定义 API 工具
func (e *ToolExecutor) executeCustomAPI(ctx context.Context, tool *Tool, toolCall *ToolCall) (*ExecuteResult, error) {
	if err := e.authorizeTool(ctx, tool); err != nil {
		return nil, err
	}
	if tool.ServerID == "" {
		return nil, fmt.Errorf("custom API endpoint not specified")
	}
	arguments := toolCall.Arguments
	if arguments == nil {
		arguments = map[string]any{}
	}
	body, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("marshal custom API arguments: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tool.ServerID, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create custom API request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := e.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return &ExecuteResult{
			Content: err.Error(),
			IsError: true,
		}, nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read custom API response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &ExecuteResult{
			Content: fmt.Sprintf("custom API error: status=%d body=%s", resp.StatusCode, string(respBody)),
			IsError: true,
		}, nil
	}

	return &ExecuteResult{
		Content: string(respBody),
	}, nil
}

func (e *ToolExecutor) executeCustomPython(ctx context.Context, agent *Agent, tool *Tool, toolCall *ToolCall) (*ExecuteResult, error) {
	sourceCode := strings.TrimSpace(tool.SourceCode)
	if sourceCode == "" {
		return nil, fmt.Errorf("custom Python source code not specified")
	}
	if len(sourceCode) > customPythonMaxSourceBytes {
		return &ExecuteResult{
			Content: fmt.Sprintf("custom Python source exceeded %d bytes", customPythonMaxSourceBytes),
			IsError: true,
		}, nil
	}
	arguments := toolCall.Arguments
	if arguments == nil {
		arguments = map[string]any{}
	}
	argumentPayload, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("marshal custom Python arguments: %w", err)
	}
	if len(argumentPayload) > customPythonMaxArgumentBytes {
		return &ExecuteResult{
			Content: fmt.Sprintf("custom Python arguments exceeded %d bytes", customPythonMaxArgumentBytes),
			IsError: true,
		}, nil
	}
	timeout := customPythonTimeout(tool.TimeoutSeconds)

	if customPythonDisabledInProduction() {
		if e.customPythonSandboxRunner == nil {
			return &ExecuteResult{
				Content: "custom Python tools are disabled in production until a container sandbox is configured",
				IsError: true,
			}, nil
		}
		return e.executeCustomPythonSandbox(ctx, agent, tool, toolCall, sourceCode, arguments, timeout)
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload, err := json.Marshal(map[string]any{
		"args": arguments,
		"code": sourceCode,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal custom Python payload: %w", err)
	}

	pythonBin, err := customPythonBinary()
	if err != nil {
		return &ExecuteResult{
			Content: err.Error(),
			IsError: true,
		}, nil
	}
	if err := e.authorizeTool(ctx, tool); err != nil {
		return nil, err
	}
	var output []byte
	if e.pythonProcessRunner != nil {
		output, err = e.pythonProcessRunner(runCtx, pythonBin, bytes.NewReader(payload))
	} else {
		cmd := exec.CommandContext(runCtx, pythonBin, "-I", "-S", "-c", customPythonWrapper)
		cmd.Stdin = bytes.NewReader(payload)
		output, err = cmd.CombinedOutput()
	}
	if runCtx.Err() == context.DeadlineExceeded {
		return &ExecuteResult{
			Content: fmt.Sprintf("custom Python timed out after %s", timeout),
			IsError: true,
		}, nil
	}
	if len(output) > customPythonMaxOutputBytes {
		return &ExecuteResult{
			Content: fmt.Sprintf("custom Python output exceeded %d bytes", customPythonMaxOutputBytes),
			IsError: true,
		}, nil
	}
	if err != nil {
		return &ExecuteResult{
			Content: fmt.Sprintf("custom Python error: %s", strings.TrimSpace(string(output))),
			IsError: true,
		}, nil
	}

	return &ExecuteResult{
		Content: strings.TrimSpace(string(output)),
	}, nil
}

func (e *ToolExecutor) executeCustomPythonSandbox(ctx context.Context, agent *Agent, tool *Tool, toolCall *ToolCall, sourceCode string, arguments map[string]any, timeout time.Duration) (*ExecuteResult, error) {
	if err := e.authorizeTool(ctx, tool); err != nil {
		return nil, err
	}
	organizationID := ""
	userID := ""
	agentID := ""
	if agent != nil {
		organizationID = agent.OrganizationID
		userID = agent.UserID
		agentID = agent.ID
	}
	toolCallID := ""
	toolName := ""
	runID := ""
	toolRunID := ""
	requestID := ""
	if toolCall != nil {
		toolCallID = toolCall.ID
		toolName = toolCall.Name
		runID = toolCall.RunID
		toolRunID = toolCall.ToolRunID
		requestID = toolCall.RequestID
	}
	result, err := e.customPythonSandboxRunner.RunCustomPython(ctx, CustomPythonSandboxRequest{
		OrganizationID: organizationID,
		UserID:         userID,
		AgentID:        agentID,
		RunID:          runID,
		ToolRunID:      toolRunID,
		ToolCallID:     toolCallID,
		ToolName:       toolName,
		RequestID:      requestID,
		Language:       "python",
		Code:           sourceCode,
		Inputs:         arguments,
		TimeoutMS:      int(timeout / time.Millisecond),
	})
	if err != nil {
		return &ExecuteResult{
			Content: fmt.Sprintf("custom Python sandbox error: %s", err.Error()),
			IsError: true,
		}, nil
	}
	if result == nil {
		return &ExecuteResult{
			Content: "custom Python sandbox returned no result",
			IsError: true,
		}, nil
	}
	content := strings.TrimSpace(result.Stdout)
	if result.ExitCode != 0 {
		if content == "" {
			content = strings.TrimSpace(result.Stderr)
		}
		if content == "" && len(result.Logs) > 0 {
			content = strings.Join(result.Logs, "\n")
		}
		if content == "" {
			content = fmt.Sprintf("sandbox exited with code %d", result.ExitCode)
		}
		return &ExecuteResult{
			Content: "custom Python sandbox error: " + content,
			IsError: true,
		}, nil
	}
	return &ExecuteResult{Content: content}, nil
}

func customPythonBinary() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("OBLIVIOUS_CUSTOM_PYTHON_BIN")); configured != "" {
		return configured, nil
	}
	for _, candidate := range []string{"python3", "python"} {
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		if isWindowsPythonAppExecutionAlias(path) {
			continue
		}
		return path, nil
	}
	return "", fmt.Errorf("custom Python runtime not found; set OBLIVIOUS_CUSTOM_PYTHON_BIN")
}

func isWindowsPythonAppExecutionAlias(path string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	return strings.Contains(normalized, "/windowsapps/python")
}

func customPythonTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = 5
	}
	if seconds > 30 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func normalizeCustomToolRuntime(runtime string) string {
	switch strings.ToLower(strings.TrimSpace(runtime)) {
	case "python":
		return "python"
	default:
		return "api"
	}
}

func customPythonDisabledInProduction() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

const (
	customPythonMaxSourceBytes   = 64 * 1024
	customPythonMaxArgumentBytes = 64 * 1024
	customPythonMaxOutputBytes   = 64 * 1024
)

const customPythonWrapper = `
import json
import math
import re
import sys

payload = json.loads(sys.stdin.read() or "{}")
args = payload.get("args") or {}
code = payload.get("code") or ""
safe_builtins = {
    "abs": abs,
    "all": all,
    "any": any,
    "bool": bool,
    "dict": dict,
    "enumerate": enumerate,
    "float": float,
    "int": int,
    "len": len,
    "list": list,
    "max": max,
    "min": min,
    "range": range,
    "round": round,
    "set": set,
    "sorted": sorted,
    "str": str,
    "sum": sum,
    "tuple": tuple,
}
globals_dict = {"__builtins__": safe_builtins, "json": json, "math": math, "re": re}
locals_dict = {}
try:
    exec(code, globals_dict, locals_dict)
    main = locals_dict.get("main") or globals_dict.get("main")
    if callable(main):
        result = main(args)
    else:
        result = locals_dict.get("result")
    if isinstance(result, str):
        sys.stdout.write(result)
    else:
        sys.stdout.write(json.dumps(result, ensure_ascii=False))
except Exception as exc:
    print(f"{exc.__class__.__name__}: {exc}", file=sys.stderr)
    sys.exit(1)
`

// GetToolDefinitions 获取 Agent 可用的工具定义
func (e *ToolExecutor) GetToolDefinitions(ctx context.Context, agent *Agent) ([]ToolDefinition, error) {
	var definitions []ToolDefinition

	for _, t := range agent.Tools {
		if !t.Enabled {
			continue
		}

		def := ToolDefinition{
			Name:             t.Name,
			Description:      t.Description,
			ToolType:         normalizeToolType(t.Type),
			RequiresApproval: t.RequiresApproval,
			RiskLevel:        toolDefinitionRiskLevel(t),
		}
		if e.strict {
			if capability, err := e.resolveToolDefinitionCapability(ctx, &t); err == nil {
				def.CapabilityID = string(capability)
			} else {
				continue
			}
		}

		// 获取 InputSchema
		switch t.Type {
		case "builtin":
			if !e.isCommercialBuiltinEnabled(t.Name) {
				continue
			}
			if builtin, ok := e.builtinTools[t.Name]; ok {
				def.InputSchema = builtin.InputSchema()
				if def.Description == "" {
					def.Description = builtin.Description()
				}
			}
		case "mcp":
			if e.mcpClient != nil && t.ServerID != "" {
				mcpTools, err := e.mcpClient.ListTools(t.ServerID, agent.OrganizationID)
				if err == nil {
					for _, mt := range mcpTools {
						if mt.Name == t.Name {
							def.InputSchema = mt.InputSchema
							if def.Description == "" {
								def.Description = mt.Description
							}
							break
						}
					}
				}
			}
		case "custom":
			if normalizeCustomToolRuntime(t.Runtime) == "python" && customPythonDisabledInProduction() && e.customPythonSandboxRunner == nil {
				continue
			}
			def.InputSchema = t.InputSchema
		}

		definitions = append(definitions, def)
	}

	return definitions, nil
}

func (e *ToolExecutor) isCommercialBuiltinEnabled(name string) bool {
	if mcp.IsDefaultCommercialBuiltin(name) {
		return true
	}
	return name == "web_search" && e.webSearchProvider != nil
}

// ToolDefinition 工具定义（用于 OpenAI function calling）
type ToolDefinition struct {
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	InputSchema      any    `json:"inputSchema,omitempty"`
	ToolType         string `json:"toolType"`
	RequiresApproval bool   `json:"requiresApproval"`
	RiskLevel        string `json:"riskLevel"`
	CapabilityID     string `json:"capabilityId"`
}

func (e *ToolExecutor) resolveToolDefinitionCapability(ctx context.Context, tool *Tool) (releasecontract.CapabilityID, error) {
	if !e.strict || tool == nil || !e.authorities.Valid() {
		return "", &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "agent.toolDefinition"}
	}
	subject := releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectTool, ID: tool.Name, Runtime: releasecontract.CatalogRuntimeBuiltin}
	switch strings.ToLower(strings.TrimSpace(tool.Type)) {
	case "builtin":
		if tool.Name == "web_search" {
			subject.Runtime = releasecontract.CatalogRuntimeNetwork
		}
	case "custom":
		subject = releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectRuntime, ID: "custom", Runtime: releasecontract.CatalogRuntimeCustom}
		if normalizeCustomToolRuntime(tool.Runtime) == "python" && customPythonDisabledInProduction() {
			subject = releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectRuntime, ID: "sandbox", Runtime: releasecontract.CatalogRuntimeSandbox}
		}
	case "mcp":
		subject = releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectRuntime, ID: "mcp", Runtime: releasecontract.CatalogRuntimeMCP}
	default:
		return "", &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "agent.toolDefinition.type"}
	}
	return e.authorities.CatalogAuthorizer.ResolveAndRequire(ctx, subject, releasecontract.BoundaryOutbound)
}

func normalizeToolType(value string) string {
	switch value {
	case "builtin", "mcp", "custom":
		return value
	default:
		return "builtin"
	}
}

func toolDefinitionRiskLevel(tool Tool) string {
	if normalized := normalizeToolRiskLevel(tool.RiskLevel); normalized != "" {
		return normalized
	}
	return inferToolRiskLevel(tool.Name)
}

// ToOpenAITool 转换为 OpenAI 工具格式
func (d ToolDefinition) ToOpenAITool() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        d.Name,
			"description": d.Description,
			"parameters":  d.InputSchema,
		},
	}
}

// ParseToolCallsFromResponse 从 LLM 响应解析工具调用
func ParseToolCallsFromResponse(response map[string]any) ([]ToolCall, error) {
	toolCallsRaw, ok := response["tool_calls"]
	if !ok {
		return nil, nil
	}

	toolCallsArray, ok := toolCallsRaw.([]any)
	if !ok {
		return nil, nil
	}

	var toolCalls []ToolCall
	for _, tcRaw := range toolCallsArray {
		tcMap, ok := tcRaw.(map[string]any)
		if !ok {
			continue
		}

		id, _ := tcMap["id"].(string)
		function, ok := tcMap["function"].(map[string]any)
		if !ok {
			continue
		}

		name, _ := function["name"].(string)
		argsRaw, _ := function["arguments"].(string)

		var arguments map[string]any
		if argsRaw != "" {
			json.Unmarshal([]byte(argsRaw), &arguments)
		}

		toolCalls = append(toolCalls, ToolCall{
			ID:        id,
			Name:      name,
			Arguments: arguments,
		})
	}

	return toolCalls, nil
}
