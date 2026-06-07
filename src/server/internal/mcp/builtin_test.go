package mcp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestBuiltinToolsCommercialDefaults(t *testing.T) {
	tools := ListDefaultCommercialBuiltinTools()
	names := make(map[string]bool, len(tools))
	for _, tool := range tools {
		names[tool.Name] = true
	}

	for _, name := range []string{"calculator", "datetime", "json_formatter", "text_transform"} {
		if !names[name] {
			t.Fatalf("expected %s to be default commercial enabled, got names=%v", name, names)
		}
	}
	for _, name := range []string{"web_search", "http_request"} {
		if names[name] {
			t.Fatalf("expected %s to be disabled by default commercial policy, got names=%v", name, names)
		}
	}
}

func TestCalculatorEvaluatesArithmetic(t *testing.T) {
	tool, ok := GetBuiltinTool("calculator")
	if !ok {
		t.Fatal("calculator builtin not found")
	}

	cases := map[string]string{
		"1 + 2 * 3":    "Result: 7",
		"(10 - 4) / 3": "Result: 2",
		"-2 * (3 + 4)": "Result: -14",
	}

	for expression, want := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"expression": expression})
		if err != nil {
			t.Fatalf("calculator returned error for %q: %v", expression, err)
		}
		if result.Content != want {
			t.Fatalf("calculator(%q) content = %q, want %q", expression, result.Content, want)
		}
	}
}

func TestCalculatorRejectsInvalidExpression(t *testing.T) {
	tool, ok := GetBuiltinTool("calculator")
	if !ok {
		t.Fatal("calculator builtin not found")
	}

	for _, expression := range []string{"", "2 + * 3", "10 / 0", "1 + Math.random()"} {
		result, err := tool.Execute(context.Background(), map[string]any{"expression": expression})
		if err == nil && (result == nil || !result.IsError) {
			t.Fatalf("calculator(%q) succeeded with result=%+v, want explicit error", expression, result)
		}

		message := ""
		if err != nil {
			message = err.Error()
		} else {
			message = result.Content
		}
		if strings.Contains(strings.ToLower(message), "placeholder") {
			t.Fatalf("calculator(%q) returned placeholder error text: %q", expression, message)
		}
	}
}

func TestDatetimeReturnsRFC3339(t *testing.T) {
	tool, ok := GetBuiltinTool("datetime")
	if !ok {
		t.Fatal("datetime builtin not found")
	}

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("datetime returned error: %v", err)
	}

	const prefix = "Current date and time: "
	if !strings.HasPrefix(result.Content, prefix) {
		t.Fatalf("datetime content = %q, want prefix %q", result.Content, prefix)
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimPrefix(result.Content, prefix)); err != nil {
		t.Fatalf("datetime returned non-RFC3339 content %q: %v", result.Content, err)
	}
}

func TestJsonFormatterPrettyPrintsAndCompactsJSON(t *testing.T) {
	tool, ok := GetBuiltinTool("json_formatter")
	if !ok {
		t.Fatal("json_formatter builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{
		"json":   `{"name":"Oblivious","features":["mcp","safe"],"enabled":true}`,
		"format": "pretty",
	})
	if err != nil {
		t.Fatalf("json_formatter pretty returned error: %v", err)
	}
	if want := "{\n  \"enabled\": true,\n  \"features\": [\n    \"mcp\",\n    \"safe\"\n  ],\n  \"name\": \"Oblivious\"\n}"; result.Content != want {
		t.Fatalf("json_formatter pretty content = %q, want %q", result.Content, want)
	}

	result, err = tool.Execute(context.Background(), map[string]any{
		"json":   "{\n  \"name\": \"Oblivious\",\n  \"enabled\": true\n}",
		"format": "compact",
	})
	if err != nil {
		t.Fatalf("json_formatter compact returned error: %v", err)
	}
	if want := `{"enabled":true,"name":"Oblivious"}`; result.Content != want {
		t.Fatalf("json_formatter compact content = %q, want %q", result.Content, want)
	}
}

func TestJsonFormatterRejectsInvalidJSON(t *testing.T) {
	tool, ok := GetBuiltinTool("json_formatter")
	if !ok {
		t.Fatal("json_formatter builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"json": `{"missing":`, "format": "pretty"})
	if err == nil && (result == nil || !result.IsError) {
		t.Fatalf("json_formatter accepted invalid JSON with result=%+v", result)
	}
}

func TestTextTransformAppliesDeterministicTransforms(t *testing.T) {
	tool, ok := GetBuiltinTool("text_transform")
	if !ok {
		t.Fatal("text_transform builtin not found")
	}

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "uppercase",
			args: map[string]any{"text": "Hello mcp", "operation": "uppercase"},
			want: "HELLO MCP",
		},
		{
			name: "lowercase",
			args: map[string]any{"text": "Hello MCP", "operation": "lowercase"},
			want: "hello mcp",
		},
		{
			name: "trim",
			args: map[string]any{"text": "  Hello MCP  \n", "operation": "trim"},
			want: "Hello MCP",
		},
		{
			name: "collapse_whitespace",
			args: map[string]any{"text": "Hello\t  MCP\nsafe tools", "operation": "collapse_whitespace"},
			want: "Hello MCP safe tools",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), tt.args)
			if err != nil {
				t.Fatalf("text_transform returned error: %v", err)
			}
			if result.Content != tt.want {
				t.Fatalf("text_transform content = %q, want %q", result.Content, tt.want)
			}
		})
	}
}

func TestDisabledBuiltinsReturnExplicitErrorWithoutPlaceholder(t *testing.T) {
	webSearch, ok := GetBuiltinTool("web_search")
	if !ok {
		t.Fatal("web_search builtin not found")
	}
	assertDisabledBuiltin(t, "web_search", webSearch, map[string]any{"query": "commercial readiness"})

	var outboundAttempted bool
	httpTool := &HTTPRequestTool{
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				outboundAttempted = true
				return nil, errors.New("unexpected outbound request")
			}),
		},
	}
	assertDisabledBuiltin(t, "http_request", httpTool, map[string]any{"method": "GET", "url": "https://example.invalid"})
	if outboundAttempted {
		t.Fatal("http_request attempted outbound network I/O while disabled by default")
	}
}

func TestWebSearchUsesConfiguredProvider(t *testing.T) {
	restore := SetWebSearchProviderForTest(fakeWebSearchProvider{
		results: []WebSearchResult{{
			Title:   "Commercial readiness",
			URL:     "https://docs.example.test/readiness",
			Snippet: "Search provider result",
		}},
	})
	defer restore()

	tool, ok := GetBuiltinTool("web_search")
	if !ok {
		t.Fatal("web_search builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{"query": "commercial readiness"})
	if err != nil {
		t.Fatalf("web_search returned error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("web_search result = %+v, want successful provider content", result)
	}
	if !strings.Contains(result.Content, "Commercial readiness") || !strings.Contains(result.Content, "https://docs.example.test/readiness") {
		t.Fatalf("web_search content = %q, want provider result", result.Content)
	}
}

func TestDefaultEnabledBuiltinsDoNotReturnPlaceholderOutput(t *testing.T) {
	inputs := map[string]map[string]any{
		"calculator":     {"expression": "40 + 2"},
		"datetime":       {},
		"json_formatter": {"json": `{"answer":42}`, "format": "compact"},
		"text_transform": {"text": "safe tools", "operation": "uppercase"},
	}

	for _, definition := range ListDefaultCommercialBuiltinTools() {
		tool, ok := GetBuiltinTool(definition.Name)
		if !ok {
			t.Fatalf("default commercial builtin %s missing from registry", definition.Name)
		}
		result, err := tool.Execute(context.Background(), inputs[definition.Name])
		if err != nil {
			t.Fatalf("%s returned error: %v", definition.Name, err)
		}
		if strings.Contains(strings.ToLower(result.Content), "placeholder") {
			t.Fatalf("%s returned placeholder output: %q", definition.Name, result.Content)
		}
	}
}

type fakeWebSearchProvider struct {
	results []WebSearchResult
}

func (p fakeWebSearchProvider) Search(ctx context.Context, query string) ([]WebSearchResult, error) {
	return p.results, nil
}

func assertDisabledBuiltin(t *testing.T, name string, tool BuiltinTool, args map[string]any) {
	t.Helper()

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "disabled") {
			t.Fatalf("%s returned error %q, want disabled error", name, err.Error())
		}
		return
	}
	if result == nil {
		t.Fatalf("%s returned nil result and nil error", name)
	}
	if !result.IsError {
		t.Fatalf("%s result IsError = false, want true", name)
	}
	lowerContent := strings.ToLower(result.Content)
	if !strings.Contains(lowerContent, "disabled") {
		t.Fatalf("%s content = %q, want disabled message", name, result.Content)
	}
	if strings.Contains(lowerContent, "placeholder") {
		t.Fatalf("%s returned placeholder text while disabled: %q", name, result.Content)
	}
}
