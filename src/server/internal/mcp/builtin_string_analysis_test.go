package mcp

import (
	"context"
	"strings"
	"testing"
)

func TestStringSimilarityTool(t *testing.T) {
	tool := &StringSimilarityTool{}
	ctx := context.Background()

	tests := []struct {
		name   string
		args   map[string]any
		expect string
	}{
		{"empty", map[string]any{}, "0"},
		{"identical", map[string]any{"str1": "hello", "str2": "hello"}, "0"},
		{"different", map[string]any{"str1": "kitten", "str2": "sitting"}, "3"},
		{"empty_vs_text", map[string]any{"str1": "", "str2": "abc"}, "3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.expect {
				t.Errorf("got %q, want %q", result.Content, tt.expect)
			}
		})
	}
}

func TestStringContainsTool(t *testing.T) {
	tool := &StringContainsTool{}
	ctx := context.Background()

	tests := []struct {
		name   string
		args   map[string]any
		expect string
	}{
		{"empty", map[string]any{}, "true"},
		{"contains", map[string]any{"text": "hello world", "substring": "world"}, "true"},
		{"not_contains", map[string]any{"text": "hello", "substring": "xyz"}, "false"},
		{"case_insensitive", map[string]any{"text": "Hello", "substring": "hello", "case_sensitive": false}, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.expect {
				t.Errorf("got %q, want %q", result.Content, tt.expect)
			}
		})
	}
}

func TestStringStartsWithTool(t *testing.T) {
	tool := &StringStartsWithTool{}
	ctx := context.Background()

	tests := []struct {
		name   string
		args   map[string]any
		expect string
	}{
		{"empty", map[string]any{}, "true"},
		{"starts", map[string]any{"text": "hello world", "prefix": "hello"}, "true"},
		{"not_starts", map[string]any{"text": "hello", "prefix": "world"}, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.expect {
				t.Errorf("got %q, want %q", result.Content, tt.expect)
			}
		})
	}
}

func TestStringEndsWithTool(t *testing.T) {
	tool := &StringEndsWithTool{}
	ctx := context.Background()

	tests := []struct {
		name   string
		args   map[string]any
		expect string
	}{
		{"empty", map[string]any{}, "true"},
		{"ends", map[string]any{"text": "hello world", "suffix": "world"}, "true"},
		{"not_ends", map[string]any{"text": "hello", "suffix": "abc"}, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.expect {
				t.Errorf("got %q, want %q", result.Content, tt.expect)
			}
		})
	}
}

func TestStringIndexOfTool(t *testing.T) {
	tool := &StringIndexOfTool{}
	ctx := context.Background()

	tests := []struct {
		name   string
		args   map[string]any
		expect string
	}{
		{"empty", map[string]any{}, "-1"},
		{"found", map[string]any{"text": "hello world", "substring": "world"}, "6"},
		{"not_found", map[string]any{"text": "hello", "substring": "xyz"}, "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.expect {
				t.Errorf("got %q, want %q", result.Content, tt.expect)
			}
		})
	}
}

func TestStringLastIndexOfTool(t *testing.T) {
	tool := &StringLastIndexOfTool{}
	ctx := context.Background()

	tests := []struct {
		name   string
		args   map[string]any
		expect string
	}{
		{"empty", map[string]any{}, "-1"},
		{"found", map[string]any{"text": "hello world world", "substring": "world"}, "12"},
		{"not_found", map[string]any{"text": "hello", "substring": "xyz"}, "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.expect {
				t.Errorf("got %q, want %q", result.Content, tt.expect)
			}
		})
	}
}

func TestStringCountTool(t *testing.T) {
	tool := &StringCountTool{}
	ctx := context.Background()

	tests := []struct {
		name   string
		args   map[string]any
		expect string
	}{
		{"empty", map[string]any{}, "1"},
		{"count", map[string]any{"text": "hello world world", "substring": "world"}, "2"},
		{"not_found", map[string]any{"text": "hello", "substring": "xyz"}, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.expect {
				t.Errorf("got %q, want %q", result.Content, tt.expect)
			}
		})
	}
}

func TestSlugGenerateTool(t *testing.T) {
	tool := &SlugGenerateTool{}
	ctx := context.Background()

	tests := []struct {
		name   string
		args   map[string]any
		expect string
	}{
		{"empty", map[string]any{}, "example-text"},
		{"basic", map[string]any{"text": "Hello World"}, "hello-world"},
		{"special_chars", map[string]any{"text": "Hello, World!"}, "hello-world"},
		{"custom_separator", map[string]any{"text": "Hello World", "separator": "_"}, "hello_world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.expect {
				t.Errorf("got %q, want %q", result.Content, tt.expect)
			}
		})
	}
}

func TestLoremIpsumTool(t *testing.T) {
	tool := &LoremIpsumTool{}
	ctx := context.Background()

	tests := []struct {
		name       string
		args       map[string]any
		checkWords bool
		wantWords  int
	}{
		{"empty", map[string]any{}, true, 50},
		{"one_para", map[string]any{"paragraphs": float64(1), "words_per_para": float64(10)}, true, 10},
		{"two_paras", map[string]any{"paragraphs": float64(2), "words_per_para": float64(5)}, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if strings.Contains(result.Content, "placeholder") {
				t.Errorf("output contains placeholder")
			}
			if tt.checkWords {
				words := strings.Fields(result.Content)
				if len(words) != tt.wantWords {
					t.Errorf("got %d words, want %d", len(words), tt.wantWords)
				}
			}
		})
	}
}

func TestStringDeduplicateTool(t *testing.T) {
	tool := &StringDeduplicateTool{}
	ctx := context.Background()

	tests := []struct {
		name   string
		args   map[string]any
		expect string
	}{
		{"empty", map[string]any{}, ""},
		{"no_duplicates", map[string]any{"text": "a\nb\nc"}, "a\nb\nc"},
		{"duplicates", map[string]any{"text": "a\nb\na\nc"}, "a\nb\nc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.expect {
				t.Errorf("got %q, want %q", result.Content, tt.expect)
			}
		})
	}
}

func TestStringSortLinesTool(t *testing.T) {
	tool := &StringSortLinesTool{}
	ctx := context.Background()

	tests := []struct {
		name   string
		args   map[string]any
		expect string
	}{
		{"empty", map[string]any{}, ""},
		{"asc", map[string]any{"text": "c\nb\na"}, "a\nb\nc"},
		{"desc", map[string]any{"text": "a\nb\nc", "order": "desc"}, "c\nb\na"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.expect {
				t.Errorf("got %q, want %q", result.Content, tt.expect)
			}
		})
	}
}

func TestStringUniqueCharsTool(t *testing.T) {
	tool := &StringUniqueCharsTool{}
	ctx := context.Background()

	tests := []struct {
		name        string
		args        map[string]any
		expectChars int
	}{
		{"empty", map[string]any{}, 0},
		{"unique", map[string]any{"text": "aabbcc"}, 3},
		{"preserve_order", map[string]any{"text": "abc", "preserve_order": true}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if len([]rune(result.Content)) != tt.expectChars {
				t.Errorf("got %d chars, want %d", len([]rune(result.Content)), tt.expectChars)
			}
		})
	}
}
