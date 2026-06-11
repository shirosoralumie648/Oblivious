package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
)

func init() {
	registerBuiltins(
		map[string]BuiltinTool{
			"markdown_to_text":         &MarkdownToTextTool{},
			"markdown_to_html":         &MarkdownToHTMLTool{},
			"html_strip_tags":          &HTMLStripTagsTool{},
			"html_extract_links":       &HTMLExtractLinksTool{},
			"html_extract_images":      &HTMLExtractImagesTool{},
			"html_extract_text":        &HTMLExtractTextTool{},
			"markdown_extract_headers": &MarkdownExtractHeadersTool{},
			"markdown_extract_code":    &MarkdownExtractCodeTool{},
			"html_sanitize":            &HTMLSanitizeTool{},
		},
		map[string]bool{
			"markdown_to_text":         true,
			"markdown_to_html":         true,
			"html_strip_tags":          true,
			"html_extract_links":       true,
			"html_extract_images":      true,
			"html_extract_text":        true,
			"markdown_extract_headers": true,
			"markdown_extract_code":    true,
			"html_sanitize":            true,
		},
	)
}

type MarkdownToTextTool struct{}

func (t *MarkdownToTextTool) Name() string { return "markdown_to_text" }
func (t *MarkdownToTextTool) Description() string {
	return "Strip Markdown formatting to plain text"
}
func (t *MarkdownToTextTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"markdown": map[string]any{"type": "string", "description": "Markdown text"},
		},
		"required": []string{"markdown"},
	}
}
func (t *MarkdownToTextTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	markdown, _ := args["markdown"].(string)
	text := markdown
	text = regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllString(text, "")
	text = regexp.MustCompile("`[^`]+`").ReplaceAllString(text, "$0")
	text = regexp.MustCompile(`!\[([^\]]*)\]\([^\)]+\)`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`^#{1,6}\s+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`__([^_]+)__`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`_([^_]+)_`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile("(?m)^[*+-]\\s+").ReplaceAllString(text, "")
	text = regexp.MustCompile("(?m)^\\d+\\.\\s+").ReplaceAllString(text, "")
	text = regexp.MustCompile("(?m)^>\\s*").ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	return &ToolResult{Content: text}, nil
}

type MarkdownToHTMLTool struct{}

func (t *MarkdownToHTMLTool) Name() string { return "markdown_to_html" }
func (t *MarkdownToHTMLTool) Description() string {
	return "Convert Markdown to HTML (basic subset)"
}
func (t *MarkdownToHTMLTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"markdown": map[string]any{"type": "string", "description": "Markdown text"},
		},
		"required": []string{"markdown"},
	}
}
func (t *MarkdownToHTMLTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	markdown, _ := args["markdown"].(string)
	lines := strings.Split(markdown, "\n")
	var out strings.Builder
	inCode := false
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if !inCode {
				inCode = true
				out.WriteString("<pre><code>")
			} else {
				inCode = false
				out.WriteString("</code></pre>\n")
			}
			continue
		}
		if inCode {
			out.WriteString(html.EscapeString(line) + "\n")
			continue
		}
		if m := regexp.MustCompile(`^(#{1,6})\s+(.+)$`).FindStringSubmatch(line); m != nil {
			level := len(m[1])
			text := inlineMarkdownToHTML(m[2])
			out.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, text, level))
		} else if strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "+ ") {
			out.WriteString("<li>" + inlineMarkdownToHTML(line[2:]) + "</li>\n")
		} else if m := regexp.MustCompile(`^(\d+)\.\s+(.+)$`).FindStringSubmatch(line); m != nil {
			out.WriteString("<li>" + inlineMarkdownToHTML(m[2]) + "</li>\n")
		} else if strings.TrimSpace(line) == "" {
			out.WriteString("<br>\n")
		} else {
			out.WriteString("<p>" + inlineMarkdownToHTML(line) + "</p>\n")
		}
	}
	return &ToolResult{Content: out.String()}, nil
}

func inlineMarkdownToHTML(text string) string {
	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "<strong>$1</strong>")
	text = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(text, "<em>$1</em>")
	text = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(text, "<strong>$1</strong>")
	text = regexp.MustCompile(`_(.+?)_`).ReplaceAllString(text, "<em>$1</em>")
	text = regexp.MustCompile("`([^`]+)`").ReplaceAllString(text, "<code>$1</code>")
	text = regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`).ReplaceAllString(text, `<a href="$2">$1</a>`)
	return text
}

type HTMLStripTagsTool struct{}

func (t *HTMLStripTagsTool) Name() string        { return "html_strip_tags" }
func (t *HTMLStripTagsTool) Description() string { return "Remove all HTML tags" }
func (t *HTMLStripTagsTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"html": map[string]any{"type": "string", "description": "HTML text"},
		},
		"required": []string{"html"},
	}
}
func (t *HTMLStripTagsTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	htmlText, _ := args["html"].(string)
	text := regexp.MustCompile("<[^>]*>").ReplaceAllString(htmlText, "")
	text = html.UnescapeString(text)
	return &ToolResult{Content: strings.TrimSpace(text)}, nil
}

type HTMLExtractLinksTool struct{}

func (t *HTMLExtractLinksTool) Name() string        { return "html_extract_links" }
func (t *HTMLExtractLinksTool) Description() string { return "Extract all links from HTML" }
func (t *HTMLExtractLinksTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"html": map[string]any{"type": "string", "description": "HTML text"},
		},
		"required": []string{"html"},
	}
}
func (t *HTMLExtractLinksTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	htmlText, _ := args["html"].(string)
	re := regexp.MustCompile(`<a[^>]+href=["']([^"']+)["'][^>]*>([^<]*)</a>`)
	matches := re.FindAllStringSubmatch(htmlText, -1)
	var links []map[string]string
	for _, m := range matches {
		links = append(links, map[string]string{"href": m[1], "text": m[2]})
	}
	data, _ := json.Marshal(links)
	return &ToolResult{Content: string(data)}, nil
}

type HTMLExtractImagesTool struct{}

func (t *HTMLExtractImagesTool) Name() string        { return "html_extract_images" }
func (t *HTMLExtractImagesTool) Description() string { return "Extract all images from HTML" }
func (t *HTMLExtractImagesTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"html": map[string]any{"type": "string", "description": "HTML text"},
		},
		"required": []string{"html"},
	}
}
func (t *HTMLExtractImagesTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	htmlText, _ := args["html"].(string)
	re := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["'][^>]*(?:alt=["']([^"']*)["'])?[^>]*>`)
	matches := re.FindAllStringSubmatch(htmlText, -1)
	var images []map[string]string
	for _, m := range matches {
		alt := ""
		if len(m) > 2 {
			alt = m[2]
		}
		images = append(images, map[string]string{"src": m[1], "alt": alt})
	}
	data, _ := json.Marshal(images)
	return &ToolResult{Content: string(data)}, nil
}

type HTMLExtractTextTool struct{}

func (t *HTMLExtractTextTool) Name() string { return "html_extract_text" }
func (t *HTMLExtractTextTool) Description() string {
	return "Extract visible text from HTML (preserve structure)"
}
func (t *HTMLExtractTextTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"html": map[string]any{"type": "string", "description": "HTML text"},
		},
		"required": []string{"html"},
	}
}
func (t *HTMLExtractTextTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	htmlText, _ := args["html"].(string)
	blockTags := regexp.MustCompile(`</(p|div|h[1-6]|li|br|tr|td|th)>`)
	htmlText = blockTags.ReplaceAllString(htmlText, "$0\n")
	text := regexp.MustCompile("<[^>]*>").ReplaceAllString(htmlText, "")
	text = html.UnescapeString(text)
	lines := strings.Split(text, "\n")
	var out []string
	for _, line := range lines {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return &ToolResult{Content: strings.Join(out, "\n")}, nil
}

type MarkdownExtractHeadersTool struct{}

func (t *MarkdownExtractHeadersTool) Name() string { return "markdown_extract_headers" }
func (t *MarkdownExtractHeadersTool) Description() string {
	return "Extract headers from Markdown"
}
func (t *MarkdownExtractHeadersTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"markdown": map[string]any{"type": "string", "description": "Markdown text"},
		},
		"required": []string{"markdown"},
	}
}
func (t *MarkdownExtractHeadersTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	markdown, _ := args["markdown"].(string)
	re := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	matches := re.FindAllStringSubmatch(markdown, -1)
	var headers []map[string]any
	for _, m := range matches {
		headers = append(headers, map[string]any{"level": len(m[1]), "text": m[2]})
	}
	data, _ := json.Marshal(headers)
	return &ToolResult{Content: string(data)}, nil
}

type MarkdownExtractCodeTool struct{}

func (t *MarkdownExtractCodeTool) Name() string { return "markdown_extract_code" }
func (t *MarkdownExtractCodeTool) Description() string {
	return "Extract code blocks from Markdown"
}
func (t *MarkdownExtractCodeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"markdown": map[string]any{"type": "string", "description": "Markdown text"},
		},
		"required": []string{"markdown"},
	}
}
func (t *MarkdownExtractCodeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	markdown, _ := args["markdown"].(string)
	re := regexp.MustCompile("(?s)```([a-z]*)\n(.*?)```")
	matches := re.FindAllStringSubmatch(markdown, -1)
	var blocks []map[string]string
	for _, m := range matches {
		blocks = append(blocks, map[string]string{"language": m[1], "code": m[2]})
	}
	data, _ := json.Marshal(blocks)
	return &ToolResult{Content: string(data)}, nil
}

type HTMLSanitizeTool struct{}

func (t *HTMLSanitizeTool) Name() string { return "html_sanitize" }
func (t *HTMLSanitizeTool) Description() string {
	return "Sanitize HTML (keep safe tags only)"
}
func (t *HTMLSanitizeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"html": map[string]any{"type": "string", "description": "HTML text"},
			"allowed_tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Allowed HTML tags",
			},
		},
		"required": []string{"html"},
	}
}
func (t *HTMLSanitizeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	htmlText, _ := args["html"].(string)
	allowedTags := []string{"p", "br", "b", "i", "em", "strong", "a"}
	if tags, ok := args["allowed_tags"].([]any); ok {
		allowedTags = nil
		for _, tag := range tags {
			if s, ok := tag.(string); ok {
				allowedTags = append(allowedTags, s)
			}
		}
	}
	allowed := make(map[string]bool)
	for _, tag := range allowedTags {
		allowed[tag] = true
	}
	// Drop dangerous container tags together with their content so script
	// bodies never leak into the sanitized text.
	for _, dangerous := range []string{"script", "style", "iframe", "object", "embed"} {
		if allowed[dangerous] {
			continue
		}
		blockRe := regexp.MustCompile(`(?is)<` + dangerous + `\b[^>]*>.*?</` + dangerous + `\s*>`)
		htmlText = blockRe.ReplaceAllString(htmlText, "")
	}
	re := regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)[^>]*>`)
	result := re.ReplaceAllStringFunc(htmlText, func(match string) string {
		m := re.FindStringSubmatch(match)
		if len(m) < 3 {
			return ""
		}
		tag := strings.ToLower(m[2])
		if !allowed[tag] {
			return ""
		}
		if tag == "a" && m[1] == "" {
			hrefRe := regexp.MustCompile(`href=["']([^"']+)["']`)
			if href := hrefRe.FindStringSubmatch(match); href != nil {
				return fmt.Sprintf(`<a href="%s">`, html.EscapeString(href[1]))
			}
			return "<a>"
		}
		if m[1] == "/" {
			return fmt.Sprintf("</%s>", tag)
		}
		return fmt.Sprintf("<%s>", tag)
	})
	return &ToolResult{Content: result}, nil
}
