package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/mcp"
)

func TestToolExecutorExecutesCustomAPITool(t *testing.T) {
	var (
		receivedBody map[string]any
		receivedPath string
		receivedType string
		receivedVerb string
	)
	expectedResponse := `{"forecast":"cloudy","city":"Shanghai"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedType = r.Header.Get("Content-Type")
		receivedVerb = r.Method

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			return
		}
		if err := json.Unmarshal(body, &receivedBody); err != nil {
			t.Errorf("decode request body: %v; body=%s", err, string(body))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(expectedResponse))
	}))
	defer server.Close()

	executor := NewToolExecutor(nil)
	agent := &Agent{
		ID:             "agent_custom_api",
		OrganizationID: "org_1",
		Tools: []Tool{{
			Name:     "lookup_weather",
			Type:     "custom",
			ServerID: server.URL + "/tools/weather",
			Enabled:  true,
		}},
	}
	toolCall := &ToolCall{
		ID:   "call_weather",
		Name: "lookup_weather",
		Arguments: map[string]any{
			"city": "Shanghai",
			"days": 3,
		},
	}

	result, err := executor.Execute(context.Background(), agent, toolCall)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if receivedVerb != http.MethodPost {
		t.Fatalf("custom API method = %q, want %q", receivedVerb, http.MethodPost)
	}
	if receivedPath != "/tools/weather" {
		t.Fatalf("custom API path = %q, want /tools/weather", receivedPath)
	}
	if !strings.HasPrefix(receivedType, "application/json") {
		t.Fatalf("custom API content type = %q, want application/json", receivedType)
	}
	if receivedBody["city"] != "Shanghai" || receivedBody["days"] != float64(3) {
		t.Fatalf("custom API body = %#v, want tool call arguments", receivedBody)
	}
	if result == nil || result.IsError || result.Content != expectedResponse {
		t.Fatalf("custom API result = %+v, want successful response content %s", result, expectedResponse)
	}
}

func TestToolExecutorExecutesCustomPythonTool(t *testing.T) {
	executor := NewToolExecutor(nil)
	agent := &Agent{
		ID:             "agent_custom_python",
		OrganizationID: "org_1",
		Tools: []Tool{{
			Name:           "sum_order",
			Type:           "custom",
			Runtime:        "python",
			SourceCode:     "def main(args):\n    return {\"total\": args[\"subtotal\"] + args[\"shipping\"]}",
			Enabled:        true,
			TimeoutSeconds: 15,
		}},
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:        "call_sum_order",
		Name:      "sum_order",
		RunID:     "run_custom_python",
		ToolRunID: "tool_run_custom_python",
		RequestID: "req_custom_python",
		Arguments: map[string]any{
			"shipping": 2,
			"subtotal": 5,
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.IsError || !strings.Contains(result.Content, `"total": 7`) {
		t.Fatalf("custom Python result = %+v, want JSON total 7", result)
	}
}

func TestToolExecutorRejectsCustomPythonInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	executor := NewToolExecutor(nil)
	agent := &Agent{
		ID:             "agent_custom_python_production",
		OrganizationID: "org_1",
		Tools: []Tool{{
			Name:       "sum_order",
			Type:       "custom",
			Runtime:    "python",
			SourceCode: "def main(args):\n    return 7",
			Enabled:    true,
		}},
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:        "call_sum_order",
		Name:      "sum_order",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || !result.IsError || !strings.Contains(result.Content, "disabled in production") {
		t.Fatalf("custom Python production result = %+v, want fail-closed disabled result", result)
	}
}

func TestToolExecutorUsesCustomPythonSandboxRunnerInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	runner := &recordingCustomPythonSandboxRunner{
		result: &CustomPythonSandboxResult{Stdout: `{"total":7}`, ExitCode: 0},
	}
	executor := NewToolExecutor(nil)
	executor.SetCustomPythonSandboxRunner(runner)
	agent := &Agent{
		ID:             "agent_custom_python_production_sandbox",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Tools: []Tool{{
			Name:           "sum_order",
			Type:           "custom",
			Runtime:        "python",
			SourceCode:     "def main(args):\n    return {\"total\": args[\"subtotal\"] + args[\"shipping\"]}",
			Enabled:        true,
			TimeoutSeconds: 15,
		}},
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:        "call_sum_order",
		Name:      "sum_order",
		RunID:     "run_custom_python",
		ToolRunID: "tool_run_custom_python",
		RequestID: "req_custom_python",
		Arguments: map[string]any{
			"shipping": 2,
			"subtotal": 5,
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.IsError || result.Content != `{"total":7}` {
		t.Fatalf("custom Python sandbox result = %+v, want sandbox stdout", result)
	}
	if runner.calls != 1 {
		t.Fatalf("sandbox calls = %d, want 1", runner.calls)
	}
	if runner.request.OrganizationID != "org_1" || runner.request.UserID != "user_1" {
		t.Fatalf("sandbox identity = %q/%q, want org_1/user_1", runner.request.OrganizationID, runner.request.UserID)
	}
	if runner.request.AgentID != "agent_custom_python_production_sandbox" ||
		runner.request.RunID != "run_custom_python" ||
		runner.request.ToolRunID != "tool_run_custom_python" ||
		runner.request.ToolCallID != "call_sum_order" ||
		runner.request.ToolName != "sum_order" ||
		runner.request.RequestID != "req_custom_python" {
		t.Fatalf("sandbox execution context = agent:%q run:%q toolRun:%q toolCall:%q tool:%q request:%q",
			runner.request.AgentID,
			runner.request.RunID,
			runner.request.ToolRunID,
			runner.request.ToolCallID,
			runner.request.ToolName,
			runner.request.RequestID,
		)
	}
	if runner.request.Language != "python" || runner.request.Code != agent.Tools[0].SourceCode {
		t.Fatalf("sandbox language/code = %q/%q", runner.request.Language, runner.request.Code)
	}
	if runner.request.TimeoutMS != 15_000 {
		t.Fatalf("sandbox timeout = %d, want 15000", runner.request.TimeoutMS)
	}
	if runner.request.Inputs["shipping"] != 2 || runner.request.Inputs["subtotal"] != 5 {
		t.Fatalf("sandbox inputs = %+v, want tool call arguments", runner.request.Inputs)
	}
}

func TestToolExecutorOmitsCustomPythonDefinitionsInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	executor := NewToolExecutor(nil)
	agent := &Agent{
		ID:             "agent_custom_python_definitions",
		OrganizationID: "org_1",
		Tools: []Tool{
			{
				Name:        "python_tool",
				Type:        "custom",
				Runtime:     "python",
				Description: "host python should not be advertised",
				Enabled:     true,
			},
			{
				Name:        "api_tool",
				Type:        "custom",
				Runtime:     "api",
				Description: "custom API remains available",
				Enabled:     true,
			},
		},
	}

	definitions, err := executor.GetToolDefinitions(context.Background(), agent)
	if err != nil {
		t.Fatalf("GetToolDefinitions returned error: %v", err)
	}
	if len(definitions) != 1 || definitions[0].Name != "api_tool" {
		t.Fatalf("expected only custom API definition in production, got %+v", definitions)
	}
}

func TestToolExecutorBlocksCustomPythonImports(t *testing.T) {
	executor := NewToolExecutor(nil)
	agent := &Agent{
		ID:             "agent_custom_python_sandbox",
		OrganizationID: "org_1",
		Tools: []Tool{{
			Name:           "unsafe_python",
			Type:           "custom",
			Runtime:        "python",
			SourceCode:     "def main(args):\n    import os\n    return os.listdir('/')",
			Enabled:        true,
			TimeoutSeconds: 2,
		}},
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:        "call_unsafe_python",
		Name:      "unsafe_python",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("custom Python import should be blocked, got %+v", result)
	}
	if strings.Contains(result.Content, "bin") || strings.Contains(result.Content, "etc") {
		t.Fatalf("custom Python import appears to have listed host files: %+v", result)
	}
}

func TestToolExecutorRejectsCustomPythonOversizedOutput(t *testing.T) {
	executor := NewToolExecutor(nil)
	agent := &Agent{
		ID:             "agent_custom_python_output_limit",
		OrganizationID: "org_1",
		Tools: []Tool{{
			Name:           "noisy_python",
			Type:           "custom",
			Runtime:        "python",
			SourceCode:     "def main(args):\n    return 'x' * args['size']",
			Enabled:        true,
			TimeoutSeconds: 2,
		}},
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:   "call_noisy_python",
		Name: "noisy_python",
		Arguments: map[string]any{
			"size": customPythonMaxOutputBytes + 1,
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || !result.IsError || !strings.Contains(result.Content, "output exceeded") {
		t.Fatalf("custom Python oversized output result = %+v, want output-limit error", result)
	}
	if result != nil && len(result.Content) > 256 {
		t.Fatalf("custom Python output-limit error should not echo oversized output, got %d bytes", len(result.Content))
	}
}

func TestToolExecutorRejectsCustomPythonOversizedSource(t *testing.T) {
	executor := NewToolExecutor(nil)
	agent := &Agent{
		ID:             "agent_custom_python_source_limit",
		OrganizationID: "org_1",
		Tools: []Tool{{
			Name:           "large_python",
			Type:           "custom",
			Runtime:        "python",
			SourceCode:     strings.Repeat("x", customPythonMaxSourceBytes+1),
			Enabled:        true,
			TimeoutSeconds: 2,
		}},
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:        "call_large_python",
		Name:      "large_python",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || !result.IsError || !strings.Contains(result.Content, "source exceeded") {
		t.Fatalf("custom Python oversized source result = %+v, want source-limit error", result)
	}
}

func TestToolExecutorRejectsCustomPythonOversizedArguments(t *testing.T) {
	executor := NewToolExecutor(nil)
	agent := &Agent{
		ID:             "agent_custom_python_argument_limit",
		OrganizationID: "org_1",
		Tools: []Tool{{
			Name:           "large_args_python",
			Type:           "custom",
			Runtime:        "python",
			SourceCode:     "def main(args):\n    return {'ok': True}",
			Enabled:        true,
			TimeoutSeconds: 2,
		}},
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:   "call_large_args_python",
		Name: "large_args_python",
		Arguments: map[string]any{
			"payload": strings.Repeat("x", customPythonMaxArgumentBytes+1),
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || !result.IsError || !strings.Contains(result.Content, "arguments exceeded") {
		t.Fatalf("custom Python oversized arguments result = %+v, want argument-limit error", result)
	}
}

type recordingCustomPythonSandboxRunner struct {
	calls   int
	request CustomPythonSandboxRequest
	result  *CustomPythonSandboxResult
	err     error
}

func (r *recordingCustomPythonSandboxRunner) RunCustomPython(ctx context.Context, req CustomPythonSandboxRequest) (*CustomPythonSandboxResult, error) {
	r.calls++
	r.request = req
	return r.result, r.err
}

func TestToolExecutorAllowsWebSearchWhenProviderConfigured(t *testing.T) {
	executor := NewToolExecutor(nil)
	executor.SetWebSearchProvider(fakeAgentWebSearchProvider{
		results: []mcp.WebSearchResult{{
			Title:   "Agent search result",
			URL:     "https://search.example.test/result",
			Snippet: "provider-backed search",
		}},
	})
	agent := &Agent{
		ID: "agent_search",
		Tools: []Tool{{
			Name:    "web_search",
			Type:    "builtin",
			Enabled: true,
		}},
	}

	definitions, err := executor.GetToolDefinitions(context.Background(), agent)
	if err != nil {
		t.Fatalf("GetToolDefinitions returned error: %v", err)
	}
	if len(definitions) != 1 || definitions[0].Name != "web_search" {
		t.Fatalf("expected web_search definition with provider configured, got %+v", definitions)
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:        "call_search",
		Name:      "web_search",
		Arguments: map[string]any{"query": "agent search"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.IsError || !strings.Contains(result.Content, "Agent search result") {
		t.Fatalf("web_search result = %+v, want provider-backed content", result)
	}
}

type fakeAgentWebSearchProvider struct {
	results []mcp.WebSearchResult
}

func (p fakeAgentWebSearchProvider) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	return p.results, nil
}
