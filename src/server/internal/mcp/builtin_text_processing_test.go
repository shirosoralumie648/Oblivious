package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestTextUppercase(t *testing.T) {
	tool, ok := GetBuiltinTool("text_uppercase")
	if !ok {
		t.Fatal("text_uppercase builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "hello world"})
	if err != nil {
		t.Fatalf("text_uppercase returned error: %v", err)
	}
	if result.Content != "HELLO WORLD" {
		t.Fatalf("text_uppercase content = %q, want %q", result.Content, "HELLO WORLD")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_uppercase with empty args returned error: %v", err)
	}
	if result.Content != "" {
		t.Fatalf("text_uppercase empty args content = %q, want empty string", result.Content)
	}
}

func TestTextLowercase(t *testing.T) {
	tool, ok := GetBuiltinTool("text_lowercase")
	if !ok {
		t.Fatal("text_lowercase builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "HELLO WORLD"})
	if err != nil {
		t.Fatalf("text_lowercase returned error: %v", err)
	}
	if result.Content != "hello world" {
		t.Fatalf("text_lowercase content = %q, want %q", result.Content, "hello world")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_lowercase with empty args returned error: %v", err)
	}
}

func TestTextTitlecase(t *testing.T) {
	tool, ok := GetBuiltinTool("text_titlecase")
	if !ok {
		t.Fatal("text_titlecase builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "hello world"})
	if err != nil {
		t.Fatalf("text_titlecase returned error: %v", err)
	}
	if result.Content != "Hello World" {
		t.Fatalf("text_titlecase content = %q, want %q", result.Content, "Hello World")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_titlecase with empty args returned error: %v", err)
	}
}

func TestTextTrim(t *testing.T) {
	tool, ok := GetBuiltinTool("text_trim")
	if !ok {
		t.Fatal("text_trim builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "  hello world  \n"})
	if err != nil {
		t.Fatalf("text_trim returned error: %v", err)
	}
	if result.Content != "hello world" {
		t.Fatalf("text_trim content = %q, want %q", result.Content, "hello world")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_trim with empty args returned error: %v", err)
	}
}

func TestTextTrimPrefix(t *testing.T) {
	tool, ok := GetBuiltinTool("text_trim_prefix")
	if !ok {
		t.Fatal("text_trim_prefix builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "hello world", "prefix": "hello "})
	if err != nil {
		t.Fatalf("text_trim_prefix returned error: %v", err)
	}
	if result.Content != "world" {
		t.Fatalf("text_trim_prefix content = %q, want %q", result.Content, "world")
	}

	result, err = tool.Execute(context.Background(), map[string]any{"text": "hello world", "prefix": "goodbye"})
	if err != nil {
		t.Fatalf("text_trim_prefix returned error: %v", err)
	}
	if result.Content != "hello world" {
		t.Fatalf("text_trim_prefix no match content = %q, want %q", result.Content, "hello world")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_trim_prefix with empty args returned error: %v", err)
	}
}

func TestTextTrimSuffix(t *testing.T) {
	tool, ok := GetBuiltinTool("text_trim_suffix")
	if !ok {
		t.Fatal("text_trim_suffix builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "hello world", "suffix": " world"})
	if err != nil {
		t.Fatalf("text_trim_suffix returned error: %v", err)
	}
	if result.Content != "hello" {
		t.Fatalf("text_trim_suffix content = %q, want %q", result.Content, "hello")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_trim_suffix with empty args returned error: %v", err)
	}
}

func TestTextReplace(t *testing.T) {
	tool, ok := GetBuiltinTool("text_replace")
	if !ok {
		t.Fatal("text_replace builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "hello world hello", "old": "hello", "new": "hi"})
	if err != nil {
		t.Fatalf("text_replace returned error: %v", err)
	}
	if result.Content != "hi world hi" {
		t.Fatalf("text_replace content = %q, want %q", result.Content, "hi world hi")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_replace with empty args returned error: %v", err)
	}
}

func TestTextSplit(t *testing.T) {
	tool, ok := GetBuiltinTool("text_split")
	if !ok {
		t.Fatal("text_split builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "a,b,c", "delimiter": ","})
	if err != nil {
		t.Fatalf("text_split returned error: %v", err)
	}
	var parts []string
	if err := json.Unmarshal([]byte(result.Content), &parts); err != nil {
		t.Fatalf("text_split returned non-JSON: %v", err)
	}
	if len(parts) != 3 || parts[0] != "a" || parts[1] != "b" || parts[2] != "c" {
		t.Fatalf("text_split parts = %v, want [a b c]", parts)
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_split with empty args returned error: %v", err)
	}
}

func TestTextJoin(t *testing.T) {
	tool, ok := GetBuiltinTool("text_join")
	if !ok {
		t.Fatal("text_join builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"parts": []any{"a", "b", "c"}, "delimiter": ","})
	if err != nil {
		t.Fatalf("text_join returned error: %v", err)
	}
	if result.Content != "a,b,c" {
		t.Fatalf("text_join content = %q, want %q", result.Content, "a,b,c")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_join with empty args returned error: %v", err)
	}
}

func TestTextReverse(t *testing.T) {
	tool, ok := GetBuiltinTool("text_reverse")
	if !ok {
		t.Fatal("text_reverse builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("text_reverse returned error: %v", err)
	}
	if result.Content != "olleh" {
		t.Fatalf("text_reverse content = %q, want %q", result.Content, "olleh")
	}

	result, err = tool.Execute(context.Background(), map[string]any{"text": "你好"})
	if err != nil {
		t.Fatalf("text_reverse unicode returned error: %v", err)
	}
	if result.Content != "好你" {
		t.Fatalf("text_reverse unicode content = %q, want %q", result.Content, "好你")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_reverse with empty args returned error: %v", err)
	}
}

func TestTextTruncate(t *testing.T) {
	tool, ok := GetBuiltinTool("text_truncate")
	if !ok {
		t.Fatal("text_truncate builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "hello world", "max_length": 5.0, "suffix": "..."})
	if err != nil {
		t.Fatalf("text_truncate returned error: %v", err)
	}
	if result.Content != "hello..." {
		t.Fatalf("text_truncate content = %q, want %q", result.Content, "hello...")
	}

	result, err = tool.Execute(context.Background(), map[string]any{"text": "hi", "max_length": 5.0})
	if err != nil {
		t.Fatalf("text_truncate no truncate returned error: %v", err)
	}
	if result.Content != "hi" {
		t.Fatalf("text_truncate no truncate content = %q, want %q", result.Content, "hi")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_truncate with empty args returned error: %v", err)
	}
}

func TestTextPadLeft(t *testing.T) {
	tool, ok := GetBuiltinTool("text_pad_left")
	if !ok {
		t.Fatal("text_pad_left builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "42", "length": 5.0, "pad_char": "0"})
	if err != nil {
		t.Fatalf("text_pad_left returned error: %v", err)
	}
	if result.Content != "00042" {
		t.Fatalf("text_pad_left content = %q, want %q", result.Content, "00042")
	}

	result, err = tool.Execute(context.Background(), map[string]any{"text": "hello", "length": 3.0})
	if err != nil {
		t.Fatalf("text_pad_left no pad returned error: %v", err)
	}
	if result.Content != "hello" {
		t.Fatalf("text_pad_left no pad content = %q, want %q", result.Content, "hello")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_pad_left with empty args returned error: %v", err)
	}
}

func TestTextPadRight(t *testing.T) {
	tool, ok := GetBuiltinTool("text_pad_right")
	if !ok {
		t.Fatal("text_pad_right builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "42", "length": 5.0, "pad_char": "0"})
	if err != nil {
		t.Fatalf("text_pad_right returned error: %v", err)
	}
	if result.Content != "42000" {
		t.Fatalf("text_pad_right content = %q, want %q", result.Content, "42000")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_pad_right with empty args returned error: %v", err)
	}
}

func TestTextWordCount(t *testing.T) {
	tool, ok := GetBuiltinTool("text_word_count")
	if !ok {
		t.Fatal("text_word_count builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "hello world test"})
	if err != nil {
		t.Fatalf("text_word_count returned error: %v", err)
	}
	if result.Content != "3" {
		t.Fatalf("text_word_count content = %q, want %q", result.Content, "3")
	}

	result, err = tool.Execute(context.Background(), map[string]any{"text": ""})
	if err != nil {
		t.Fatalf("text_word_count empty returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("text_word_count empty content = %q, want %q", result.Content, "0")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_word_count with empty args returned error: %v", err)
	}
}

func TestTextLineCount(t *testing.T) {
	tool, ok := GetBuiltinTool("text_line_count")
	if !ok {
		t.Fatal("text_line_count builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "line1\nline2\nline3"})
	if err != nil {
		t.Fatalf("text_line_count returned error: %v", err)
	}
	if result.Content != "3" {
		t.Fatalf("text_line_count content = %q, want %q", result.Content, "3")
	}

	result, err = tool.Execute(context.Background(), map[string]any{"text": ""})
	if err != nil {
		t.Fatalf("text_line_count empty returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("text_line_count empty content = %q, want %q", result.Content, "0")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_line_count with empty args returned error: %v", err)
	}
}

func TestTextCharCount(t *testing.T) {
	tool, ok := GetBuiltinTool("text_char_count")
	if !ok {
		t.Fatal("text_char_count builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("text_char_count returned error: %v", err)
	}
	if result.Content != "5" {
		t.Fatalf("text_char_count content = %q, want %q", result.Content, "5")
	}

	result, err = tool.Execute(context.Background(), map[string]any{"text": "你好世界"})
	if err != nil {
		t.Fatalf("text_char_count unicode returned error: %v", err)
	}
	if result.Content != "4" {
		t.Fatalf("text_char_count unicode content = %q, want %q", result.Content, "4")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("text_char_count with empty args returned error: %v", err)
	}
}

func TestTextProcessingToolsDefaultCommercialEnabled(t *testing.T) {
	expectedTools := []string{
		"text_uppercase", "text_lowercase", "text_titlecase", "text_trim",
		"text_trim_prefix", "text_trim_suffix", "text_replace", "text_split",
		"text_join", "text_reverse", "text_truncate", "text_pad_left",
		"text_pad_right", "text_word_count", "text_line_count", "text_char_count",
	}

	commercialTools := ListDefaultCommercialBuiltinTools()
	toolNames := make(map[string]bool)
	for _, tool := range commercialTools {
		toolNames[tool.Name] = true
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Fatalf("expected %s to be default commercial enabled", name)
		}
	}
}

func TestTextProcessingToolsNoPlaceholder(t *testing.T) {
	tools := []string{
		"text_uppercase", "text_lowercase", "text_titlecase", "text_trim",
		"text_trim_prefix", "text_trim_suffix", "text_replace", "text_split",
		"text_join", "text_reverse", "text_truncate", "text_pad_left",
		"text_pad_right", "text_word_count", "text_line_count", "text_char_count",
	}

	for _, name := range tools {
		tool, ok := GetBuiltinTool(name)
		if !ok {
			t.Fatalf("%s builtin not found", name)
		}
		result, err := tool.Execute(context.Background(), map[string]any{})
		if err != nil {
			t.Fatalf("%s with empty args returned error: %v", name, err)
		}
		if result == nil {
			t.Fatalf("%s returned nil result", name)
		}
		if strings.Contains(strings.ToLower(result.Content), "placeholder") {
			t.Fatalf("%s returned placeholder output: %q", name, result.Content)
		}
	}
}
