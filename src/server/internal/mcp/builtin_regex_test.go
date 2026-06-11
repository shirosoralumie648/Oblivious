package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRegexMatchTool(t *testing.T) {
	tool := &RegexMatchTool{}
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]any
		want    string
		wantErr bool
	}{
		{
			name: "match found",
			args: map[string]any{"text": "hello world", "pattern": "world"},
			want: "true",
		},
		{
			name: "no match",
			args: map[string]any{"text": "hello world", "pattern": "^world"},
			want: "false",
		},
		{
			name: "empty args",
			args: map[string]any{},
			want: "true",
		},
		{
			name:    "invalid pattern",
			args:    map[string]any{"text": "test", "pattern": "["},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if tt.wantErr && !result.IsError {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantErr && result.IsError {
				t.Errorf("Unexpected error: %v", result.Content)
			}
			if !tt.wantErr && result.Content != tt.want {
				t.Errorf("Execute() = %v, want %v", result.Content, tt.want)
			}
		})
	}
}

func TestRegexFindTool(t *testing.T) {
	tool := &RegexFindTool{}
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]any
		want    string
		wantErr bool
	}{
		{
			name: "find match",
			args: map[string]any{"text": "hello world", "pattern": "\\w+"},
			want: "hello",
		},
		{
			name: "no match",
			args: map[string]any{"text": "hello", "pattern": "\\d+"},
			want: "",
		},
		{
			name: "empty args",
			args: map[string]any{},
			want: "",
		},
		{
			name:    "invalid pattern",
			args:    map[string]any{"text": "test", "pattern": "("},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if tt.wantErr && !result.IsError {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantErr && result.IsError {
				t.Errorf("Unexpected error: %v", result.Content)
			}
			if !tt.wantErr && result.Content != tt.want {
				t.Errorf("Execute() = %v, want %v", result.Content, tt.want)
			}
		})
	}
}

func TestRegexFindAllTool(t *testing.T) {
	tool := &RegexFindAllTool{}
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]any
		want    []string
		wantErr bool
	}{
		{
			name: "find all matches",
			args: map[string]any{"text": "hello world foo", "pattern": "\\w+"},
			want: []string{"hello", "world", "foo"},
		},
		{
			name: "with limit",
			args: map[string]any{"text": "a b c d", "pattern": "\\w+", "limit": 2.0},
			want: []string{"a", "b"},
		},
		{
			name: "empty args",
			args: map[string]any{},
			want: []string{""},
		},
		{
			name:    "invalid pattern",
			args:    map[string]any{"text": "test", "pattern": "*"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if tt.wantErr && !result.IsError {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantErr && result.IsError {
				t.Errorf("Unexpected error: %v", result.Content)
			}
			if !tt.wantErr {
				var got []string
				if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}
				if len(got) != len(tt.want) {
					t.Errorf("Execute() = %v, want %v", got, tt.want)
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("Execute()[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestRegexReplaceTool(t *testing.T) {
	tool := &RegexReplaceTool{}
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]any
		want    string
		wantErr bool
	}{
		{
			name: "replace match",
			args: map[string]any{"text": "hello world", "pattern": "world", "replacement": "gopher"},
			want: "hello gopher",
		},
		{
			name: "replace all",
			args: map[string]any{"text": "foo foo foo", "pattern": "foo", "replacement": "bar"},
			want: "bar bar bar",
		},
		{
			name: "empty args",
			args: map[string]any{},
			want: "",
		},
		{
			name:    "invalid pattern",
			args:    map[string]any{"text": "test", "pattern": "?"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if tt.wantErr && !result.IsError {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantErr && result.IsError {
				t.Errorf("Unexpected error: %v", result.Content)
			}
			if !tt.wantErr && result.Content != tt.want {
				t.Errorf("Execute() = %v, want %v", result.Content, tt.want)
			}
		})
	}
}

func TestRegexSplitTool(t *testing.T) {
	tool := &RegexSplitTool{}
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]any
		want    []string
		wantErr bool
	}{
		{
			name: "split by whitespace",
			args: map[string]any{"text": "hello world foo", "pattern": "\\s+"},
			want: []string{"hello", "world", "foo"},
		},
		{
			name: "with limit",
			args: map[string]any{"text": "a,b,c,d", "pattern": ",", "limit": 2.0},
			want: []string{"a", "b,c,d"},
		},
		{
			name: "empty args",
			args: map[string]any{},
			want: []string{""},
		},
		{
			name:    "invalid pattern",
			args:    map[string]any{"text": "test", "pattern": "+"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if tt.wantErr && !result.IsError {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantErr && result.IsError {
				t.Errorf("Unexpected error: %v", result.Content)
			}
			if !tt.wantErr {
				var got []string
				if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}
				if len(got) != len(tt.want) {
					t.Errorf("Execute() = %v, want %v", got, tt.want)
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("Execute()[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestRegexExtractTool(t *testing.T) {
	tool := &RegexExtractTool{}
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]any
		want    map[string]string
		wantErr bool
	}{
		{
			name: "extract named groups",
			args: map[string]any{"text": "hello world", "pattern": "(?P<greeting>\\w+) (?P<target>\\w+)"},
			want: map[string]string{"greeting": "hello", "target": "world"},
		},
		{
			name: "no match",
			args: map[string]any{"text": "hello", "pattern": "(?P<num>\\d+)"},
			want: map[string]string{},
		},
		{
			name: "empty args",
			args: map[string]any{},
			want: map[string]string{"match": ""},
		},
		{
			name:    "invalid pattern",
			args:    map[string]any{"text": "test", "pattern": "(?P<"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if tt.wantErr && !result.IsError {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantErr && result.IsError {
				t.Errorf("Unexpected error: %v", result.Content)
			}
			if !tt.wantErr {
				var got map[string]string
				if err := json.Unmarshal([]byte(result.Content), &got); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}
				if len(got) != len(tt.want) {
					t.Errorf("Execute() = %v, want %v", got, tt.want)
				}
				for k, v := range tt.want {
					if got[k] != v {
						t.Errorf("Execute()[%s] = %v, want %v", k, got[k], v)
					}
				}
			}
		})
	}
}

func TestRegexValidateTool(t *testing.T) {
	tool := &RegexValidateTool{}
	ctx := context.Background()

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "valid pattern",
			args: map[string]any{"pattern": "\\w+"},
			want: "valid",
		},
		{
			name: "invalid pattern",
			args: map[string]any{"pattern": "["},
			want: "error parsing regexp:",
		},
		{
			name: "empty args",
			args: map[string]any{},
			want: "valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if tt.want == "valid" {
				if result.Content != "valid" {
					t.Errorf("Execute() = %v, want %v", result.Content, tt.want)
				}
			} else {
				if len(result.Content) < len(tt.want) || result.Content[:len(tt.want)] != tt.want {
					t.Errorf("Execute() = %v, want prefix %v", result.Content, tt.want)
				}
			}
		})
	}
}

func TestRegexEscapeTool(t *testing.T) {
	tool := &RegexEscapeTool{}
	ctx := context.Background()

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "escape special chars",
			args: map[string]any{"text": "hello.world"},
			want: "hello\\.world",
		},
		{
			name: "escape multiple chars",
			args: map[string]any{"text": "[a-z]+"},
			want: "\\[a-z\\]\\+",
		},
		{
			name: "empty args",
			args: map[string]any{},
			want: "",
		},
		{
			name: "no special chars",
			args: map[string]any{"text": "hello"},
			want: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if result.IsError {
				t.Errorf("Unexpected error: %v", result.Content)
			}
			if result.Content != tt.want {
				t.Errorf("Execute() = %v, want %v", result.Content, tt.want)
			}
		})
	}
}
