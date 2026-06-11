package mcp

import (
	"context"
	"strings"
	"testing"
)

func TestBase64EncodeDecodeRoundtrip(t *testing.T) {
	encodeTool, ok := GetBuiltinTool("base64_encode")
	if !ok {
		t.Fatal("base64_encode builtin not found")
	}
	decodeTool, ok := GetBuiltinTool("base64_decode")
	if !ok {
		t.Fatal("base64_decode builtin not found")
	}

	cases := []string{"Hello, World!", "MCP safe tools", "特殊字符测试", ""}
	for _, input := range cases {
		encoded, err := encodeTool.Execute(context.Background(), map[string]any{"text": input})
		if err != nil {
			t.Fatalf("base64_encode(%q) returned error: %v", input, err)
		}
		decoded, err := decodeTool.Execute(context.Background(), map[string]any{"encoded": encoded.Content})
		if err != nil {
			t.Fatalf("base64_decode(%q) returned error: %v", encoded.Content, err)
		}
		if decoded.Content != input {
			t.Fatalf("roundtrip failed: input=%q, decoded=%q", input, decoded.Content)
		}
	}
}

func TestBase64DecodeInvalid(t *testing.T) {
	tool, ok := GetBuiltinTool("base64_decode")
	if !ok {
		t.Fatal("base64_decode builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"encoded": "not valid base64!@#"})
	if err == nil && (result == nil || !result.IsError) {
		t.Fatalf("base64_decode accepted invalid input with result=%+v", result)
	}
}

func TestHexEncodeDecodeRoundtrip(t *testing.T) {
	encodeTool, ok := GetBuiltinTool("hex_encode")
	if !ok {
		t.Fatal("hex_encode builtin not found")
	}
	decodeTool, ok := GetBuiltinTool("hex_decode")
	if !ok {
		t.Fatal("hex_decode builtin not found")
	}

	cases := []string{"Hello", "MCP", ""}
	for _, input := range cases {
		encoded, err := encodeTool.Execute(context.Background(), map[string]any{"text": input})
		if err != nil {
			t.Fatalf("hex_encode(%q) returned error: %v", input, err)
		}
		decoded, err := decodeTool.Execute(context.Background(), map[string]any{"hex": encoded.Content})
		if err != nil {
			t.Fatalf("hex_decode(%q) returned error: %v", encoded.Content, err)
		}
		if decoded.Content != input {
			t.Fatalf("roundtrip failed: input=%q, decoded=%q", input, decoded.Content)
		}
	}
}

func TestHexDecodeInvalid(t *testing.T) {
	tool, ok := GetBuiltinTool("hex_decode")
	if !ok {
		t.Fatal("hex_decode builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"hex": "not hex!"})
	if err == nil && (result == nil || !result.IsError) {
		t.Fatalf("hex_decode accepted invalid input with result=%+v", result)
	}
}

func TestURLEncodeDecodeRoundtrip(t *testing.T) {
	encodeTool, ok := GetBuiltinTool("url_encode")
	if !ok {
		t.Fatal("url_encode builtin not found")
	}
	decodeTool, ok := GetBuiltinTool("url_decode")
	if !ok {
		t.Fatal("url_decode builtin not found")
	}

	cases := []string{"Hello World", "key=value&foo=bar", "特殊字符", ""}
	for _, input := range cases {
		encoded, err := encodeTool.Execute(context.Background(), map[string]any{"text": input})
		if err != nil {
			t.Fatalf("url_encode(%q) returned error: %v", input, err)
		}
		decoded, err := decodeTool.Execute(context.Background(), map[string]any{"encoded": encoded.Content})
		if err != nil {
			t.Fatalf("url_decode(%q) returned error: %v", encoded.Content, err)
		}
		if decoded.Content != input {
			t.Fatalf("roundtrip failed: input=%q, decoded=%q", input, decoded.Content)
		}
	}
}

func TestHTMLEscapeUnescapeRoundtrip(t *testing.T) {
	escapeTool, ok := GetBuiltinTool("html_escape")
	if !ok {
		t.Fatal("html_escape builtin not found")
	}
	unescapeTool, ok := GetBuiltinTool("html_unescape")
	if !ok {
		t.Fatal("html_unescape builtin not found")
	}

	cases := []string{"<script>alert('xss')</script>", "A & B", "\"quoted\"", ""}
	for _, input := range cases {
		escaped, err := escapeTool.Execute(context.Background(), map[string]any{"text": input})
		if err != nil {
			t.Fatalf("html_escape(%q) returned error: %v", input, err)
		}
		unescaped, err := unescapeTool.Execute(context.Background(), map[string]any{"text": escaped.Content})
		if err != nil {
			t.Fatalf("html_unescape(%q) returned error: %v", escaped.Content, err)
		}
		if unescaped.Content != input {
			t.Fatalf("roundtrip failed: input=%q, unescaped=%q", input, unescaped.Content)
		}
	}
}

func TestHTMLEscapeSpecialChars(t *testing.T) {
	tool, ok := GetBuiltinTool("html_escape")
	if !ok {
		t.Fatal("html_escape builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "<>&\""})
	if err != nil {
		t.Fatalf("html_escape returned error: %v", err)
	}
	if result.Content != "&lt;&gt;&amp;&#34;" {
		t.Fatalf("html_escape content = %q, want %q", result.Content, "&lt;&gt;&amp;&#34;")
	}
}

func TestBase64URLEncodeDecodeRoundtrip(t *testing.T) {
	encodeTool, ok := GetBuiltinTool("base64_url_encode")
	if !ok {
		t.Fatal("base64_url_encode builtin not found")
	}
	decodeTool, ok := GetBuiltinTool("base64_url_decode")
	if !ok {
		t.Fatal("base64_url_decode builtin not found")
	}

	cases := []string{"Hello, URL!", "safe+chars", ""}
	for _, input := range cases {
		encoded, err := encodeTool.Execute(context.Background(), map[string]any{"text": input})
		if err != nil {
			t.Fatalf("base64_url_encode(%q) returned error: %v", input, err)
		}
		decoded, err := decodeTool.Execute(context.Background(), map[string]any{"encoded": encoded.Content})
		if err != nil {
			t.Fatalf("base64_url_decode(%q) returned error: %v", encoded.Content, err)
		}
		if decoded.Content != input {
			t.Fatalf("roundtrip failed: input=%q, decoded=%q", input, decoded.Content)
		}
	}
}

func TestJSONEscapeSpecialChars(t *testing.T) {
	tool, ok := GetBuiltinTool("json_escape")
	if !ok {
		t.Fatal("json_escape builtin not found")
	}

	cases := map[string]string{
		"hello\nworld": `hello\nworld`,
		"tab\there":    `tab\there`,
		`"quoted"`:     `\"quoted\"`,
		"":             "",
	}

	for input, want := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"text": input})
		if err != nil {
			t.Fatalf("json_escape(%q) returned error: %v", input, err)
		}
		if result.Content != want {
			t.Fatalf("json_escape(%q) = %q, want %q", input, result.Content, want)
		}
	}
}

func TestCSVEscapeQuoting(t *testing.T) {
	tool, ok := GetBuiltinTool("csv_escape")
	if !ok {
		t.Fatal("csv_escape builtin not found")
	}

	cases := map[string]string{
		"simple":           "simple",
		"has,comma":        `"has,comma"`,
		`has"quote`:        `"has""quote"`,
		"has\nline":        "\"has\nline\"",
		"normal text here": "normal text here",
		"":                 "",
	}

	for input, want := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"text": input})
		if err != nil {
			t.Fatalf("csv_escape(%q) returned error: %v", input, err)
		}
		if result.Content != want {
			t.Fatalf("csv_escape(%q) = %q, want %q", input, result.Content, want)
		}
	}
}

func TestEncodingToolsWithEmptyArgs(t *testing.T) {
	tools := []string{
		"base64_encode", "base64_decode",
		"hex_encode", "hex_decode",
		"url_encode", "url_decode",
		"html_escape", "html_unescape",
		"base64_url_encode", "base64_url_decode",
		"json_escape", "csv_escape",
	}

	for _, name := range tools {
		t.Run(name, func(t *testing.T) {
			tool, ok := GetBuiltinTool(name)
			if !ok {
				t.Fatalf("%s builtin not found", name)
			}
			result, err := tool.Execute(context.Background(), map[string]any{})
			if err != nil {
				t.Fatalf("%s with empty args returned error: %v", name, err)
			}
			if result == nil {
				t.Fatalf("%s with empty args returned nil result", name)
			}
			if strings.Contains(strings.ToLower(result.Content), "placeholder") {
				t.Fatalf("%s returned placeholder output: %q", name, result.Content)
			}
		})
	}
}

func TestEncodingToolsCommercialDefault(t *testing.T) {
	tools := []string{
		"base64_encode", "base64_decode",
		"hex_encode", "hex_decode",
		"url_encode", "url_decode",
		"html_escape", "html_unescape",
		"base64_url_encode", "base64_url_decode",
		"json_escape", "csv_escape",
	}

	commercialTools := ListDefaultCommercialBuiltinTools()
	names := make(map[string]bool, len(commercialTools))
	for _, tool := range commercialTools {
		names[tool.Name] = true
	}

	for _, name := range tools {
		if !names[name] {
			t.Fatalf("expected %s to be default commercial enabled", name)
		}
	}
}
