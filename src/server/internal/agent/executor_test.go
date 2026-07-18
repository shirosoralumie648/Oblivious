package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"oblivious/server/internal/mcp"
	"oblivious/server/internal/releasecontract"
)

func TestLiveAgentToolExecutorReadinessContract(t *testing.T) {
	guard := &liveExecutorGuard{}
	guard.allow.Store(true)
	contract, profile := loadLiveExecutorAuthority(t)
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("build runtime authorities: %v", err)
	}
	registrar := &liveExecutorRegistrar{descriptors: make(map[string]releasecontract.EffectDescriptor)}
	var httpCalls atomic.Int32
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpCalls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer httpServer.Close()
	var processCalls atomic.Int32
	var sandboxCalls atomic.Int32
	executor, err := NewAuthorizedToolExecutor(nil, ToolRuntimeOptions{
		Authorities: authorities,
		Guard:       guard,
		Effects:     registrar,
		HTTPClient:  http.DefaultClient,
		PythonProcessRunner: func(context.Context, string, io.Reader) ([]byte, error) {
			processCalls.Add(1)
			return []byte(`{"ok":true}`), nil
		},
		CustomPythonSandbox: liveSandboxRunner{calls: &sandboxCalls},
	})
	if err != nil {
		t.Fatalf("construct authorized ToolExecutor: %v", err)
	}
	if got := registrar.count(); got != 5 {
		t.Fatalf("constructor descriptor count = %d, want 5", got)
	}
	definitions, err := executor.GetToolDefinitions(t.Context(), &Agent{Tools: []Tool{{Name: "calculator", Type: "builtin", Enabled: true}}})
	if err != nil || len(definitions) != 1 || definitions[0].CapabilityID != "mcp.tool_execution" {
		t.Fatalf("server-derived tool capability definitions = %+v, %v", definitions, err)
	}
	if encoded, _ := json.Marshal(definitions[0].ToOpenAITool()); strings.Contains(string(encoded), "capabilityId") {
		t.Fatalf("provider tool conversion leaked capabilityId: %s", encoded)
	}
	var injected Tool
	if err := json.Unmarshal([]byte(`{"name":"calculator","type":"builtin","enabled":true,"capabilityId":"caller"}`), &injected); err == nil {
		t.Fatal("caller capabilityId mutation unexpectedly decoded")
	}

	t.Run("builtin", func(t *testing.T) {
		a := &Agent{OrganizationID: "org_1", Tools: []Tool{{Name: "calculator", Type: "builtin", Enabled: true}}}
		if _, err := executor.Execute(t.Context(), a, &ToolCall{Name: "calculator", Arguments: map[string]any{"expression": "1+1"}}); err != nil {
			t.Fatalf("builtin execute: %v", err)
		}
		before := guard.count()
		guard.allow.Store(false)
		if _, err := executor.Execute(t.Context(), a, &ToolCall{Name: "calculator", Arguments: map[string]any{"expression": "2+2"}}); err == nil {
			t.Fatal("denied builtin execute unexpectedly succeeded")
		}
		if guard.count() != before+1 {
			t.Fatalf("builtin denial guard delta = %d, want 1", guard.count()-before)
		}
		guard.allow.Store(true)
	})

	t.Run("builtin_web_search", func(t *testing.T) {
		webRegistrar := &liveExecutorRegistrar{descriptors: make(map[string]releasecontract.EffectDescriptor)}
		provider := &liveWebSearchProvider{}
		webExecutor, err := NewAuthorizedToolExecutor(nil, ToolRuntimeOptions{
			Authorities: authorities, Guard: guard, Effects: webRegistrar, HTTPClient: http.DefaultClient, WebSearchProvider: provider,
		})
		if err != nil {
			t.Fatalf("construct web-search executor: %v", err)
		}
		if webRegistrar.count() != 6 {
			t.Fatalf("web-search executor descriptor count = %d, want five executor descriptors plus one WebSearchTool descriptor", webRegistrar.count())
		}
		a := &Agent{OrganizationID: "org_1", Tools: []Tool{{Name: "web_search", Type: "builtin", Enabled: true}}}
		for _, query := range []string{"first", "second"} {
			if _, err := webExecutor.Execute(t.Context(), a, &ToolCall{Name: "web_search", Arguments: map[string]any{"query": query}}); err != nil {
				t.Fatalf("web search %q: %v", query, err)
			}
		}
		if provider.calls.Load() != 2 {
			t.Fatalf("web-search provider calls = %d, want 2", provider.calls.Load())
		}
		guard.allow.Store(false)
		if _, err := webExecutor.Execute(t.Context(), a, &ToolCall{Name: "web_search", Arguments: map[string]any{"query": "expired"}}); err == nil {
			t.Fatal("denied web search unexpectedly succeeded")
		}
		if provider.calls.Load() != 2 {
			t.Fatalf("denied web-search provider calls = %d, want unchanged 2", provider.calls.Load())
		}
		guard.allow.Store(true)
	})

	t.Run("custom_api_http", func(t *testing.T) {
		a := &Agent{OrganizationID: "org_1", Tools: []Tool{{Name: "api", Type: "custom", Runtime: "api", ServerID: httpServer.URL, Enabled: true}}}
		if _, err := executor.Execute(t.Context(), a, &ToolCall{Name: "api", Arguments: map[string]any{"x": 1}}); err != nil {
			t.Fatalf("custom API execute: %v", err)
		}
		before := httpCalls.Load()
		guard.allow.Store(false)
		if _, err := executor.Execute(t.Context(), a, &ToolCall{Name: "api", Arguments: map[string]any{"x": 2}}); err == nil {
			t.Fatal("denied custom API unexpectedly succeeded")
		}
		if httpCalls.Load() != before {
			t.Fatalf("denied custom API calls = %d, want unchanged %d", httpCalls.Load(), before)
		}
		guard.allow.Store(true)
	})

	t.Run("custom_python_process", func(t *testing.T) {
		t.Setenv("APP_ENV", "development")
		t.Setenv("OBLIVIOUS_CUSTOM_PYTHON_BIN", "/bin/python3")
		a := &Agent{OrganizationID: "org_1", Tools: []Tool{{Name: "python", Type: "custom", Runtime: "python", SourceCode: "def main(args): return 1", Enabled: true}}}
		if _, err := executor.Execute(t.Context(), a, &ToolCall{Name: "python", Arguments: map[string]any{}}); err != nil {
			t.Fatalf("local Python execute: %v", err)
		}
		before := processCalls.Load()
		guard.allow.Store(false)
		if _, err := executor.Execute(t.Context(), a, &ToolCall{Name: "python", Arguments: map[string]any{}}); err == nil {
			t.Fatal("denied local Python unexpectedly succeeded")
		}
		if processCalls.Load() != before {
			t.Fatalf("denied local Python calls = %d, want unchanged %d", processCalls.Load(), before)
		}
		guard.allow.Store(true)
	})

	t.Run("custom_python_sandbox", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		a := &Agent{OrganizationID: "org_1", Tools: []Tool{{Name: "sandbox", Type: "custom", Runtime: "python", SourceCode: "def main(args): return 1", Enabled: true}}}
		if _, err := executor.Execute(t.Context(), a, &ToolCall{Name: "sandbox", Arguments: map[string]any{}}); err != nil {
			t.Fatalf("sandbox execute: %v", err)
		}
		before := sandboxCalls.Load()
		guard.allow.Store(false)
		if _, err := executor.Execute(t.Context(), a, &ToolCall{Name: "sandbox", Arguments: map[string]any{}}); err == nil {
			t.Fatal("denied sandbox unexpectedly succeeded")
		}
		if sandboxCalls.Load() != before {
			t.Fatalf("denied sandbox calls = %d, want unchanged %d", sandboxCalls.Load(), before)
		}
		guard.allow.Store(true)
	})

	t.Run("mcp", func(t *testing.T) {
		a := &Agent{OrganizationID: "org_1", Tools: []Tool{{Name: "remote", Type: "mcp", ServerID: "server_1", Enabled: true}}}
		guard.allow.Store(false)
		if _, err := executor.Execute(t.Context(), a, &ToolCall{Name: "remote", Arguments: map[string]any{}}); err == nil {
			t.Fatal("denied MCP unexpectedly succeeded")
		}
		guard.allow.Store(true)
	})
	if _, err := NewAuthorizedToolExecutor(nil, ToolRuntimeOptions{Authorities: authorities, Guard: guard, Effects: registrar, HTTPClient: http.DefaultClient}); err == nil {
		t.Fatal("duplicate ToolExecutor construction unexpectedly succeeded")
	}
}

type liveExecutorGuard struct {
	allow atomic.Bool
	calls atomic.Int32
}

func (g *liveExecutorGuard) Require(context.Context, string, releasecontract.Boundary) error {
	g.calls.Add(1)
	if !g.allow.Load() {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityBlocked, Field: "test"}
	}
	return nil
}

func (g *liveExecutorGuard) count() int { return int(g.calls.Load()) }

type liveExecutorRegistrar struct {
	mu          sync.Mutex
	descriptors map[string]releasecontract.EffectDescriptor
}

func (r *liveExecutorRegistrar) Register(d releasecontract.EffectDescriptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.descriptors[d.ID]; exists {
		return fmt.Errorf("duplicate descriptor %s", d.ID)
	}
	r.descriptors[d.ID] = d
	return nil
}

func (r *liveExecutorRegistrar) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.descriptors)
}

type liveSandboxRunner struct{ calls *atomic.Int32 }

func (r liveSandboxRunner) RunCustomPython(context.Context, CustomPythonSandboxRequest) (*CustomPythonSandboxResult, error) {
	r.calls.Add(1)
	return &CustomPythonSandboxResult{Stdout: `{"ok":true}`, ExitCode: 0}, nil
}

type liveWebSearchProvider struct{ calls atomic.Int32 }

func (p *liveWebSearchProvider) Search(context.Context, string) ([]mcp.WebSearchResult, error) {
	p.calls.Add(1)
	return []mcp.WebSearchResult{{Title: "result"}}, nil
}

func loadLiveExecutorAuthority(t *testing.T) (releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve executor test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../.."))
	contract, err := releasecontract.Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("load release contract: %v", err)
	}
	for i := range contract.Capabilities {
		if contract.Capabilities[i].ID == "sandbox.code_execution" {
			contract.Capabilities[i].Commitment = releasecontract.CommitmentConditional
		}
	}
	for i := range contract.Profiles {
		if contract.Profiles[i].ID != "monolith" {
			continue
		}
		profile := contract.Profiles[i]
		contract.Profiles[i] = profile
		return contract, profile
	}
	t.Fatal("monolith profile missing")
	return releasecontract.AuthoredContractV1{}, releasecontract.DeploymentProfile{}
}

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
