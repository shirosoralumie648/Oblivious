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
			TimeoutSeconds: 2,
		}},
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:   "call_sum_order",
		Name: "sum_order",
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
