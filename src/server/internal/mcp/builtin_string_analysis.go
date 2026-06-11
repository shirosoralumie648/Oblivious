package mcp

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"unicode"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"string_similarity":    &StringSimilarityTool{},
		"string_contains":      &StringContainsTool{},
		"string_starts_with":   &StringStartsWithTool{},
		"string_ends_with":     &StringEndsWithTool{},
		"string_index_of":      &StringIndexOfTool{},
		"string_last_index_of": &StringLastIndexOfTool{},
		"string_count":         &StringCountTool{},
		"slug_generate":        &SlugGenerateTool{},
		"lorem_ipsum":          &LoremIpsumTool{},
		"string_deduplicate":   &StringDeduplicateTool{},
		"string_sort_lines":    &StringSortLinesTool{},
		"string_unique_chars":  &StringUniqueCharsTool{},
	}, map[string]bool{
		"string_similarity":    true,
		"string_contains":      true,
		"string_starts_with":   true,
		"string_ends_with":     true,
		"string_index_of":      true,
		"string_last_index_of": true,
		"string_count":         true,
		"slug_generate":        true,
		"lorem_ipsum":          true,
		"string_deduplicate":   true,
		"string_sort_lines":    true,
		"string_unique_chars":  true,
	})
}

type StringSimilarityTool struct{}

func (t *StringSimilarityTool) Name() string        { return "string_similarity" }
func (t *StringSimilarityTool) Description() string { return "Calculate Levenshtein distance" }
func (t *StringSimilarityTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"str1": map[string]any{"type": "string", "description": "First string", "default": ""},
			"str2": map[string]any{"type": "string", "description": "Second string", "default": ""},
		},
	}
}
func (t *StringSimilarityTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	str1, _ := args["str1"].(string)
	str2, _ := args["str2"].(string)
	dist := levenshtein(str1, str2)
	return &ToolResult{Content: fmt.Sprintf("%d", dist)}, nil
}

func levenshtein(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	if len(r1) == 0 {
		return len(r2)
	}
	if len(r2) == 0 {
		return len(r1)
	}
	prev := make([]int, len(r2)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(r1); i++ {
		curr := make([]int, len(r2)+1)
		curr[0] = i
		for j := 1; j <= len(r2); j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev = curr
	}
	return prev[len(r2)]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type StringContainsTool struct{}

func (t *StringContainsTool) Name() string        { return "string_contains" }
func (t *StringContainsTool) Description() string { return "Check if string contains substring" }
func (t *StringContainsTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":           map[string]any{"type": "string", "description": "Text to search in", "default": ""},
			"substring":      map[string]any{"type": "string", "description": "Substring to find", "default": ""},
			"case_sensitive": map[string]any{"type": "boolean", "description": "Case sensitive", "default": true},
		},
	}
}
func (t *StringContainsTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	substring, _ := args["substring"].(string)
	caseSensitive := true
	if cs, ok := args["case_sensitive"].(bool); ok {
		caseSensitive = cs
	}
	result := false
	if caseSensitive {
		result = strings.Contains(text, substring)
	} else {
		result = strings.Contains(strings.ToLower(text), strings.ToLower(substring))
	}
	if result {
		return &ToolResult{Content: "true"}, nil
	}
	return &ToolResult{Content: "false"}, nil
}

type StringStartsWithTool struct{}

func (t *StringStartsWithTool) Name() string        { return "string_starts_with" }
func (t *StringStartsWithTool) Description() string { return "Check if string starts with prefix" }
func (t *StringStartsWithTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":           map[string]any{"type": "string", "description": "Text to check", "default": ""},
			"prefix":         map[string]any{"type": "string", "description": "Prefix to find", "default": ""},
			"case_sensitive": map[string]any{"type": "boolean", "description": "Case sensitive", "default": true},
		},
	}
}
func (t *StringStartsWithTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	prefix, _ := args["prefix"].(string)
	caseSensitive := true
	if cs, ok := args["case_sensitive"].(bool); ok {
		caseSensitive = cs
	}
	result := false
	if caseSensitive {
		result = strings.HasPrefix(text, prefix)
	} else {
		result = strings.HasPrefix(strings.ToLower(text), strings.ToLower(prefix))
	}
	if result {
		return &ToolResult{Content: "true"}, nil
	}
	return &ToolResult{Content: "false"}, nil
}

type StringEndsWithTool struct{}

func (t *StringEndsWithTool) Name() string        { return "string_ends_with" }
func (t *StringEndsWithTool) Description() string { return "Check if string ends with suffix" }
func (t *StringEndsWithTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":           map[string]any{"type": "string", "description": "Text to check", "default": ""},
			"suffix":         map[string]any{"type": "string", "description": "Suffix to find", "default": ""},
			"case_sensitive": map[string]any{"type": "boolean", "description": "Case sensitive", "default": true},
		},
	}
}
func (t *StringEndsWithTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	suffix, _ := args["suffix"].(string)
	caseSensitive := true
	if cs, ok := args["case_sensitive"].(bool); ok {
		caseSensitive = cs
	}
	result := false
	if caseSensitive {
		result = strings.HasSuffix(text, suffix)
	} else {
		result = strings.HasSuffix(strings.ToLower(text), strings.ToLower(suffix))
	}
	if result {
		return &ToolResult{Content: "true"}, nil
	}
	return &ToolResult{Content: "false"}, nil
}

type StringIndexOfTool struct{}

func (t *StringIndexOfTool) Name() string        { return "string_index_of" }
func (t *StringIndexOfTool) Description() string { return "Find first occurrence index" }
func (t *StringIndexOfTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":      map[string]any{"type": "string", "description": "Text to search in", "default": ""},
			"substring": map[string]any{"type": "string", "description": "Substring to find", "default": ""},
		},
	}
}
func (t *StringIndexOfTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	substring, _ := args["substring"].(string)
	if substring == "" {
		return &ToolResult{Content: "-1"}, nil
	}
	idx := strings.Index(text, substring)
	return &ToolResult{Content: fmt.Sprintf("%d", idx)}, nil
}

type StringLastIndexOfTool struct{}

func (t *StringLastIndexOfTool) Name() string        { return "string_last_index_of" }
func (t *StringLastIndexOfTool) Description() string { return "Find last occurrence index" }
func (t *StringLastIndexOfTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":      map[string]any{"type": "string", "description": "Text to search in", "default": ""},
			"substring": map[string]any{"type": "string", "description": "Substring to find", "default": ""},
		},
	}
}
func (t *StringLastIndexOfTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	substring, _ := args["substring"].(string)
	if substring == "" {
		return &ToolResult{Content: "-1"}, nil
	}
	idx := strings.LastIndex(text, substring)
	return &ToolResult{Content: fmt.Sprintf("%d", idx)}, nil
}

type StringCountTool struct{}

func (t *StringCountTool) Name() string        { return "string_count" }
func (t *StringCountTool) Description() string { return "Count substring occurrences" }
func (t *StringCountTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":      map[string]any{"type": "string", "description": "Text to search in", "default": ""},
			"substring": map[string]any{"type": "string", "description": "Substring to count", "default": ""},
		},
	}
}
func (t *StringCountTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	substring, _ := args["substring"].(string)
	count := strings.Count(text, substring)
	return &ToolResult{Content: fmt.Sprintf("%d", count)}, nil
}

type SlugGenerateTool struct{}

func (t *SlugGenerateTool) Name() string        { return "slug_generate" }
func (t *SlugGenerateTool) Description() string { return "Generate URL-safe slug from text" }
func (t *SlugGenerateTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":      map[string]any{"type": "string", "description": "Text to slugify", "default": "example text"},
			"separator": map[string]any{"type": "string", "description": "Separator", "default": "-"},
			"lowercase": map[string]any{"type": "boolean", "description": "Convert to lowercase", "default": true},
		},
	}
}
func (t *SlugGenerateTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	if text == "" {
		text = "example text"
	}
	separator, _ := args["separator"].(string)
	if separator == "" {
		separator = "-"
	}
	lowercase := true
	if lc, ok := args["lowercase"].(bool); ok {
		lowercase = lc
	}

	var result strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
		} else if unicode.IsSpace(r) || r == '-' || r == '_' {
			if result.Len() > 0 && result.String()[result.Len()-1] != separator[0] {
				result.WriteString(separator)
			}
		}
	}
	slug := result.String()
	slug = strings.Trim(slug, separator)
	if lowercase {
		slug = strings.ToLower(slug)
	}
	return &ToolResult{Content: slug}, nil
}

type LoremIpsumTool struct{}

var loremWords = []string{"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing", "elit", "sed", "do", "eiusmod", "tempor", "incididunt", "ut", "labore", "et", "dolore", "magna", "aliqua"}

func (t *LoremIpsumTool) Name() string        { return "lorem_ipsum" }
func (t *LoremIpsumTool) Description() string { return "Generate Lorem Ipsum placeholder text" }
func (t *LoremIpsumTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"paragraphs":     map[string]any{"type": "integer", "description": "Number of paragraphs", "default": 1},
			"words_per_para": map[string]any{"type": "integer", "description": "Words per paragraph", "default": 50},
		},
	}
}
func (t *LoremIpsumTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	paragraphs := 1
	if p, ok := args["paragraphs"].(float64); ok {
		paragraphs = int(p)
	}
	wordsPerPara := 50
	if w, ok := args["words_per_para"].(float64); ok {
		wordsPerPara = int(w)
	}
	if paragraphs < 1 {
		paragraphs = 1
	}
	if wordsPerPara < 1 {
		wordsPerPara = 1
	}

	var result strings.Builder
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < paragraphs; i++ {
		if i > 0 {
			result.WriteString("\n\n")
		}
		for j := 0; j < wordsPerPara; j++ {
			if j > 0 {
				result.WriteString(" ")
			}
			word := loremWords[rng.Intn(len(loremWords))]
			if j == 0 {
				word = strings.ToUpper(string(word[0])) + word[1:]
			}
			result.WriteString(word)
		}
		result.WriteString(".")
	}
	return &ToolResult{Content: result.String()}, nil
}

type StringDeduplicateTool struct{}

func (t *StringDeduplicateTool) Name() string        { return "string_deduplicate" }
func (t *StringDeduplicateTool) Description() string { return "Remove duplicate lines from text" }
func (t *StringDeduplicateTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":           map[string]any{"type": "string", "description": "Text with lines", "default": ""},
			"preserve_order": map[string]any{"type": "boolean", "description": "Preserve order", "default": true},
		},
	}
}
func (t *StringDeduplicateTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	preserveOrder := true
	if po, ok := args["preserve_order"].(bool); ok {
		preserveOrder = po
	}

	lines := strings.Split(text, "\n")
	seen := make(map[string]bool)
	var result []string
	for _, line := range lines {
		if !seen[line] {
			seen[line] = true
			result = append(result, line)
		}
	}
	if !preserveOrder {
		sort.Strings(result)
	}
	return &ToolResult{Content: strings.Join(result, "\n")}, nil
}

type StringSortLinesTool struct{}

func (t *StringSortLinesTool) Name() string        { return "string_sort_lines" }
func (t *StringSortLinesTool) Description() string { return "Sort lines in text" }
func (t *StringSortLinesTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":           map[string]any{"type": "string", "description": "Text with lines", "default": ""},
			"order":          map[string]any{"type": "string", "description": "Sort order", "enum": []string{"asc", "desc"}, "default": "asc"},
			"case_sensitive": map[string]any{"type": "boolean", "description": "Case sensitive", "default": true},
		},
	}
}
func (t *StringSortLinesTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	order, _ := args["order"].(string)
	if order == "" {
		order = "asc"
	}
	caseSensitive := true
	if cs, ok := args["case_sensitive"].(bool); ok {
		caseSensitive = cs
	}

	lines := strings.Split(text, "\n")
	if caseSensitive {
		sort.Strings(lines)
	} else {
		sort.Slice(lines, func(i, j int) bool {
			return strings.ToLower(lines[i]) < strings.ToLower(lines[j])
		})
	}
	if order == "desc" {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}
	return &ToolResult{Content: strings.Join(lines, "\n")}, nil
}

type StringUniqueCharsTool struct{}

func (t *StringUniqueCharsTool) Name() string        { return "string_unique_chars" }
func (t *StringUniqueCharsTool) Description() string { return "Get unique characters from text" }
func (t *StringUniqueCharsTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":           map[string]any{"type": "string", "description": "Input text", "default": ""},
			"preserve_order": map[string]any{"type": "boolean", "description": "Preserve order", "default": false},
		},
	}
}
func (t *StringUniqueCharsTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	preserveOrder := false
	if po, ok := args["preserve_order"].(bool); ok {
		preserveOrder = po
	}

	seen := make(map[rune]bool)
	var result []rune
	for _, r := range text {
		if !seen[r] {
			seen[r] = true
			result = append(result, r)
		}
	}
	if !preserveOrder {
		sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	}
	return &ToolResult{Content: string(result)}, nil
}
