package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMarkdownToTextTool(t *testing.T) {
	tool := &MarkdownToTextTool{}
	ctx := context.Background()

	tests := []struct {
		name     string
		markdown string
		want     string
	}{
		{
			name:     "bold and italic",
			markdown: "This is **bold** and *italic* text",
			want:     "This is bold and italic text",
		},
		{
			name:     "links",
			markdown: "Check [this link](https://example.com)",
			want:     "Check this link",
		},
		{
			name:     "headers",
			markdown: "# Header 1\n## Header 2",
			want:     "Header 1\nHeader 2",
		},
		{
			name:     "empty input",
			markdown: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]any{"markdown": tt.markdown})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.want {
				t.Errorf("got %q, want %q", result.Content, tt.want)
			}
		})
	}
}

func TestMarkdownToHTMLTool(t *testing.T) {
	tool := &MarkdownToHTMLTool{}
	ctx := context.Background()

	tests := []struct {
		name     string
		markdown string
		contains []string
	}{
		{
			name:     "header",
			markdown: "# Hello",
			contains: []string{"<h1>", "Hello", "</h1>"},
		},
		{
			name:     "bold",
			markdown: "**bold**",
			contains: []string{"<strong>", "bold", "</strong>"},
		},
		{
			name:     "link",
			markdown: "[text](url)",
			contains: []string{"<a href=\"url\">", "text", "</a>"},
		},
		{
			name:     "empty input",
			markdown: "",
			contains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]any{"markdown": tt.markdown})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			for _, substr := range tt.contains {
				if !strings.Contains(result.Content, substr) {
					t.Errorf("output missing %q: %s", substr, result.Content)
				}
			}
		})
	}
}

func TestHTMLStripTagsTool(t *testing.T) {
	tool := &HTMLStripTagsTool{}
	ctx := context.Background()

	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "simple tags",
			html: "<p>Hello <b>world</b></p>",
			want: "Hello world",
		},
		{
			name: "entities",
			html: "&lt;script&gt;",
			want: "<script>",
		},
		{
			name: "empty input",
			html: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]any{"html": tt.html})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.want {
				t.Errorf("got %q, want %q", result.Content, tt.want)
			}
		})
	}
}

func TestHTMLExtractLinksTool(t *testing.T) {
	tool := &HTMLExtractLinksTool{}
	ctx := context.Background()

	tests := []struct {
		name  string
		html  string
		count int
	}{
		{
			name:  "single link",
			html:  `<a href="http://example.com">Example</a>`,
			count: 1,
		},
		{
			name:  "multiple links",
			html:  `<a href="url1">Link1</a> <a href="url2">Link2</a>`,
			count: 2,
		},
		{
			name:  "no links",
			html:  "<p>No links here</p>",
			count: 0,
		},
		{
			name:  "empty input",
			html:  "",
			count: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]any{"html": tt.html})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			var links []map[string]string
			if err := json.Unmarshal([]byte(result.Content), &links); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if len(links) != tt.count {
				t.Errorf("got %d links, want %d", len(links), tt.count)
			}
		})
	}
}

func TestHTMLExtractImagesTool(t *testing.T) {
	tool := &HTMLExtractImagesTool{}
	ctx := context.Background()

	tests := []struct {
		name  string
		html  string
		count int
	}{
		{
			name:  "single image",
			html:  `<img src="image.png" alt="Test">`,
			count: 1,
		},
		{
			name:  "multiple images",
			html:  `<img src="a.png"> <img src="b.png">`,
			count: 2,
		},
		{
			name:  "no images",
			html:  "<p>No images</p>",
			count: 0,
		},
		{
			name:  "empty input",
			html:  "",
			count: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]any{"html": tt.html})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			var images []map[string]string
			if err := json.Unmarshal([]byte(result.Content), &images); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if len(images) != tt.count {
				t.Errorf("got %d images, want %d", len(images), tt.count)
			}
		})
	}
}

func TestHTMLExtractTextTool(t *testing.T) {
	tool := &HTMLExtractTextTool{}
	ctx := context.Background()

	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "block tags",
			html: "<p>Para 1</p><p>Para 2</p>",
			want: "Para 1\nPara 2",
		},
		{
			name: "nested tags",
			html: "<div><b>Bold</b> text</div>",
			want: "Bold text",
		},
		{
			name: "empty input",
			html: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]any{"html": tt.html})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.want {
				t.Errorf("got %q, want %q", result.Content, tt.want)
			}
		})
	}
}

func TestMarkdownExtractHeadersTool(t *testing.T) {
	tool := &MarkdownExtractHeadersTool{}
	ctx := context.Background()

	tests := []struct {
		name     string
		markdown string
		count    int
	}{
		{
			name:     "multiple headers",
			markdown: "# H1\n## H2\n### H3",
			count:    3,
		},
		{
			name:     "no headers",
			markdown: "Just text",
			count:    0,
		},
		{
			name:     "empty input",
			markdown: "",
			count:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]any{"markdown": tt.markdown})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			var headers []map[string]any
			if err := json.Unmarshal([]byte(result.Content), &headers); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if len(headers) != tt.count {
				t.Errorf("got %d headers, want %d", len(headers), tt.count)
			}
		})
	}
}

func TestMarkdownExtractCodeTool(t *testing.T) {
	tool := &MarkdownExtractCodeTool{}
	ctx := context.Background()

	tests := []struct {
		name     string
		markdown string
		count    int
	}{
		{
			name:     "single code block",
			markdown: "```go\nfunc main() {}\n```",
			count:    1,
		},
		{
			name:     "multiple blocks",
			markdown: "```js\ncode1\n```\n```py\ncode2\n```",
			count:    2,
		},
		{
			name:     "no code blocks",
			markdown: "Just text",
			count:    0,
		},
		{
			name:     "empty input",
			markdown: "",
			count:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]any{"markdown": tt.markdown})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			var blocks []map[string]string
			if err := json.Unmarshal([]byte(result.Content), &blocks); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if len(blocks) != tt.count {
				t.Errorf("got %d blocks, want %d", len(blocks), tt.count)
			}
		})
	}
}

func TestHTMLSanitizeTool(t *testing.T) {
	tool := &HTMLSanitizeTool{}
	ctx := context.Background()

	tests := []struct {
		name    string
		html    string
		allowed []any
		want    string
	}{
		{
			name:    "keep allowed",
			html:    "<p>Hello</p>",
			allowed: nil,
			want:    "<p>Hello</p>",
		},
		{
			name:    "remove disallowed",
			html:    "<script>alert()</script><p>Safe</p>",
			allowed: nil,
			want:    "<p>Safe</p>",
		},
		{
			name:    "custom allowed",
			html:    "<div>Test</div>",
			allowed: []any{"div"},
			want:    "<div>Test</div>",
		},
		{
			name:    "empty input",
			html:    "",
			allowed: nil,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{"html": tt.html}
			if tt.allowed != nil {
				args["allowed_tags"] = tt.allowed
			}
			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Content != tt.want {
				t.Errorf("got %q, want %q", result.Content, tt.want)
			}
		})
	}
}

func TestMarkdownHTMLToolsEmptyArgs(t *testing.T) {
	tools := []BuiltinTool{
		&MarkdownToTextTool{},
		&MarkdownToHTMLTool{},
		&HTMLStripTagsTool{},
		&HTMLExtractLinksTool{},
		&HTMLExtractImagesTool{},
		&HTMLExtractTextTool{},
		&MarkdownExtractHeadersTool{},
		&MarkdownExtractCodeTool{},
		&HTMLSanitizeTool{},
	}

	ctx := context.Background()
	for _, tool := range tools {
		t.Run(tool.Name()+"_empty_args", func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]any{})
			if err != nil {
				t.Fatalf("Execute with empty args failed: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			if strings.Contains(strings.ToLower(result.Content), "placeholder") {
				t.Errorf("result contains placeholder: %s", result.Content)
			}
		})
	}
}
