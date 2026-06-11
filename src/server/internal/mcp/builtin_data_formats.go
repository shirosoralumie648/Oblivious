package mcp

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"json_to_yaml":       &JSONToYAMLTool{},
		"yaml_to_json":       &YAMLToJSONTool{},
		"csv_to_json":        &CSVToJSONTool{},
		"json_to_csv":        &JSONToCSVTool{},
		"tsv_to_json":        &TSVToJSONTool{},
		"json_to_tsv":        &JSONToTSVTool{},
		"xml_to_json":        &XMLToJSONTool{},
		"json_to_xml":        &JSONToXMLTool{},
		"json_query":         &JSONQueryTool{},
		"json_merge":         &JSONMergeTool{},
		"json_array_flatten": &JSONArrayFlattenTool{},
		"json_keys":          &JSONKeysTool{},
	}, map[string]bool{
		"json_to_yaml":       true,
		"yaml_to_json":       true,
		"csv_to_json":        true,
		"json_to_csv":        true,
		"tsv_to_json":        true,
		"json_to_tsv":        true,
		"xml_to_json":        true,
		"json_to_xml":        true,
		"json_query":         true,
		"json_merge":         true,
		"json_array_flatten": true,
		"json_keys":          true,
	})
}

type JSONToYAMLTool struct{}

func (t *JSONToYAMLTool) Name() string { return "json_to_yaml" }
func (t *JSONToYAMLTool) Description() string {
	return "Convert JSON to YAML (simple subset)"
}
func (t *JSONToYAMLTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"json": map[string]any{"type": "string", "description": "JSON string to convert", "default": "{}"},
		},
	}
}
func (t *JSONToYAMLTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, _ := args["json"].(string)
	if input == "" {
		input = "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return &ToolResult{Content: "invalid JSON: " + err.Error(), IsError: true}, nil
	}
	return &ToolResult{Content: toYAML(v, 0)}, nil
}

func toYAML(v any, indent int) string {
	prefix := strings.Repeat("  ", indent)
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			return "{}"
		}
		var b strings.Builder
		for k, v := range val {
			b.WriteString(prefix + k + ": ")
			nested := toYAML(v, indent+1)
			if strings.Contains(nested, "\n") {
				b.WriteString("\n" + nested)
			} else {
				b.WriteString(nested + "\n")
			}
		}
		return strings.TrimRight(b.String(), "\n")
	case []any:
		if len(val) == 0 {
			return "[]"
		}
		var b strings.Builder
		for _, item := range val {
			b.WriteString(prefix + "- ")
			nested := toYAML(item, indent+1)
			if strings.Contains(nested, "\n") {
				b.WriteString("\n" + strings.TrimPrefix(nested, strings.Repeat("  ", indent+1)))
			} else {
				b.WriteString(nested)
			}
			b.WriteString("\n")
		}
		return strings.TrimRight(b.String(), "\n")
	case string:
		return val
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case nil:
		return "null"
	default:
		return fmt.Sprint(val)
	}
}

type YAMLToJSONTool struct{}

func (t *YAMLToJSONTool) Name() string { return "yaml_to_json" }
func (t *YAMLToJSONTool) Description() string {
	return "Convert YAML to JSON (simple subset)"
}
func (t *YAMLToJSONTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"yaml": map[string]any{"type": "string", "description": "YAML string to convert", "default": "{}"},
		},
	}
}
func (t *YAMLToJSONTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, _ := args["yaml"].(string)
	if input == "" {
		input = "{}"
	}
	v, err := parseYAML(input)
	if err != nil {
		return &ToolResult{Content: "invalid YAML: " + err.Error(), IsError: true}, nil
	}
	out, _ := json.Marshal(v)
	return &ToolResult{Content: string(out)}, nil
}

func parseYAML(input string) (any, error) {
	lines := strings.Split(strings.TrimSpace(input), "\n")
	if len(lines) == 0 || input == "" {
		return map[string]any{}, nil
	}
	return parseYAMLLines(lines, 0, 0)
}

func parseYAMLLines(lines []string, start, baseIndent int) (any, error) {
	if start >= len(lines) {
		return nil, nil
	}
	firstLine := strings.TrimRight(lines[start], " \t")
	indent := len(firstLine) - len(strings.TrimLeft(firstLine, " "))
	if strings.HasPrefix(strings.TrimSpace(firstLine), "- ") {
		arr := []any{}
		i := start
		for i < len(lines) {
			line := strings.TrimRight(lines[i], " \t")
			lineIndent := len(line) - len(strings.TrimLeft(line, " "))
			if lineIndent < indent || (lineIndent == indent && !strings.HasPrefix(strings.TrimSpace(line), "- ")) {
				break
			}
			if lineIndent == indent && strings.HasPrefix(strings.TrimSpace(line), "- ") {
				val := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
				if val == "" || strings.Contains(val, ":") {
					nested, err := parseYAMLLines(lines, i+1, indent+2)
					if err != nil {
						return nil, err
					}
					arr = append(arr, nested)
					for i+1 < len(lines) {
						nextIndent := len(lines[i+1]) - len(strings.TrimLeft(lines[i+1], " "))
						if nextIndent <= indent {
							break
						}
						i++
					}
				} else {
					arr = append(arr, parseYAMLValue(val))
				}
			}
			i++
		}
		return arr, nil
	}
	obj := map[string]any{}
	i := start
	for i < len(lines) {
		line := strings.TrimRight(lines[i], " \t")
		lineIndent := len(line) - len(strings.TrimLeft(line, " "))
		if lineIndent < baseIndent {
			break
		}
		if lineIndent == baseIndent && strings.Contains(line, ":") {
			parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
			key := strings.TrimSpace(parts[0])
			valStr := ""
			if len(parts) > 1 {
				valStr = strings.TrimSpace(parts[1])
			}
			if valStr == "" {
				nested, err := parseYAMLLines(lines, i+1, lineIndent+2)
				if err != nil {
					return nil, err
				}
				obj[key] = nested
				for i+1 < len(lines) {
					nextIndent := len(lines[i+1]) - len(strings.TrimLeft(lines[i+1], " "))
					if nextIndent <= lineIndent {
						break
					}
					i++
				}
			} else {
				obj[key] = parseYAMLValue(valStr)
			}
		}
		i++
	}
	return obj, nil
}

func parseYAMLValue(s string) any {
	s = strings.TrimSpace(s)
	if s == "null" {
		return nil
	}
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		return s[1 : len(s)-1]
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f
	}
	return s
}

type CSVToJSONTool struct{}

func (t *CSVToJSONTool) Name() string { return "csv_to_json" }
func (t *CSVToJSONTool) Description() string {
	return "Convert CSV to JSON array"
}
func (t *CSVToJSONTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"csv":        map[string]any{"type": "string", "description": "CSV string", "default": ""},
			"has_header": map[string]any{"type": "boolean", "description": "First row is header", "default": true},
		},
	}
}
func (t *CSVToJSONTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, _ := args["csv"].(string)
	hasHeader, _ := args["has_header"].(bool)
	if _, ok := args["has_header"]; !ok {
		hasHeader = true
	}
	if input == "" {
		return &ToolResult{Content: "[]"}, nil
	}
	r := csv.NewReader(strings.NewReader(input))
	rows, err := r.ReadAll()
	if err != nil {
		return &ToolResult{Content: "invalid CSV: " + err.Error(), IsError: true}, nil
	}
	if len(rows) == 0 {
		return &ToolResult{Content: "[]"}, nil
	}
	var result []any
	if hasHeader && len(rows) > 1 {
		header := rows[0]
		for _, row := range rows[1:] {
			obj := map[string]any{}
			for i, val := range row {
				if i < len(header) {
					obj[header[i]] = val
				}
			}
			result = append(result, obj)
		}
	} else {
		for _, row := range rows {
			arr := make([]any, len(row))
			for i, v := range row {
				arr[i] = v
			}
			result = append(result, arr)
		}
	}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}

type JSONToCSVTool struct{}

func (t *JSONToCSVTool) Name() string { return "json_to_csv" }
func (t *JSONToCSVTool) Description() string {
	return "Convert JSON array to CSV"
}
func (t *JSONToCSVTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"json": map[string]any{"type": "string", "description": "JSON array", "default": "[]"},
		},
	}
}
func (t *JSONToCSVTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, _ := args["json"].(string)
	if input == "" {
		input = "[]"
	}
	var arr []any
	if err := json.Unmarshal([]byte(input), &arr); err != nil {
		return &ToolResult{Content: "invalid JSON array: " + err.Error(), IsError: true}, nil
	}
	if len(arr) == 0 {
		return &ToolResult{Content: ""}, nil
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	if obj, ok := arr[0].(map[string]any); ok {
		keys := []string{}
		for k := range obj {
			keys = append(keys, k)
		}
		w.Write(keys)
		for _, item := range arr {
			if row, ok := item.(map[string]any); ok {
				vals := make([]string, len(keys))
				for i, k := range keys {
					vals[i] = fmt.Sprint(row[k])
				}
				w.Write(vals)
			}
		}
	} else {
		for _, item := range arr {
			if row, ok := item.([]any); ok {
				vals := make([]string, len(row))
				for i, v := range row {
					vals[i] = fmt.Sprint(v)
				}
				w.Write(vals)
			}
		}
	}
	w.Flush()
	return &ToolResult{Content: b.String()}, nil
}

type TSVToJSONTool struct{}

func (t *TSVToJSONTool) Name() string { return "tsv_to_json" }
func (t *TSVToJSONTool) Description() string {
	return "Convert TSV to JSON array"
}
func (t *TSVToJSONTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tsv":        map[string]any{"type": "string", "description": "TSV string", "default": ""},
			"has_header": map[string]any{"type": "boolean", "description": "First row is header", "default": true},
		},
	}
}
func (t *TSVToJSONTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, _ := args["tsv"].(string)
	hasHeader, _ := args["has_header"].(bool)
	if _, ok := args["has_header"]; !ok {
		hasHeader = true
	}
	if input == "" {
		return &ToolResult{Content: "[]"}, nil
	}
	lines := strings.Split(input, "\n")
	rows := [][]string{}
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			rows = append(rows, strings.Split(line, "\t"))
		}
	}
	if len(rows) == 0 {
		return &ToolResult{Content: "[]"}, nil
	}
	var result []any
	if hasHeader && len(rows) > 1 {
		header := rows[0]
		for _, row := range rows[1:] {
			obj := map[string]any{}
			for i, val := range row {
				if i < len(header) {
					obj[header[i]] = val
				}
			}
			result = append(result, obj)
		}
	} else {
		for _, row := range rows {
			arr := make([]any, len(row))
			for i, v := range row {
				arr[i] = v
			}
			result = append(result, arr)
		}
	}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}

type JSONToTSVTool struct{}

func (t *JSONToTSVTool) Name() string { return "json_to_tsv" }
func (t *JSONToTSVTool) Description() string {
	return "Convert JSON array to TSV"
}
func (t *JSONToTSVTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"json": map[string]any{"type": "string", "description": "JSON array", "default": "[]"},
		},
	}
}
func (t *JSONToTSVTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, _ := args["json"].(string)
	if input == "" {
		input = "[]"
	}
	var arr []any
	if err := json.Unmarshal([]byte(input), &arr); err != nil {
		return &ToolResult{Content: "invalid JSON array: " + err.Error(), IsError: true}, nil
	}
	if len(arr) == 0 {
		return &ToolResult{Content: ""}, nil
	}
	var b strings.Builder
	if obj, ok := arr[0].(map[string]any); ok {
		keys := []string{}
		for k := range obj {
			keys = append(keys, k)
		}
		b.WriteString(strings.Join(keys, "\t") + "\n")
		for _, item := range arr {
			if row, ok := item.(map[string]any); ok {
				vals := make([]string, len(keys))
				for i, k := range keys {
					vals[i] = fmt.Sprint(row[k])
				}
				b.WriteString(strings.Join(vals, "\t") + "\n")
			}
		}
	} else {
		for _, item := range arr {
			if row, ok := item.([]any); ok {
				vals := make([]string, len(row))
				for i, v := range row {
					vals[i] = fmt.Sprint(v)
				}
				b.WriteString(strings.Join(vals, "\t") + "\n")
			}
		}
	}
	return &ToolResult{Content: strings.TrimRight(b.String(), "\n")}, nil
}

type XMLToJSONTool struct{}

func (t *XMLToJSONTool) Name() string { return "xml_to_json" }
func (t *XMLToJSONTool) Description() string {
	return "Convert XML to JSON (simple)"
}
func (t *XMLToJSONTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"xml": map[string]any{"type": "string", "description": "XML string", "default": "<root/>"},
		},
	}
}
func (t *XMLToJSONTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, _ := args["xml"].(string)
	if input == "" {
		input = "<root/>"
	}
	var v any
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		return &ToolResult{Content: "invalid XML: " + err.Error(), IsError: true}, nil
	}
	out, _ := json.Marshal(v)
	return &ToolResult{Content: string(out)}, nil
}

type JSONToXMLTool struct{}

func (t *JSONToXMLTool) Name() string { return "json_to_xml" }
func (t *JSONToXMLTool) Description() string {
	return "Convert JSON to XML (simple)"
}
func (t *JSONToXMLTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"json":     map[string]any{"type": "string", "description": "JSON string", "default": "{}"},
			"root_tag": map[string]any{"type": "string", "description": "Root XML tag", "default": "root"},
		},
	}
}
func (t *JSONToXMLTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, _ := args["json"].(string)
	rootTag, _ := args["root_tag"].(string)
	if input == "" {
		input = "{}"
	}
	if rootTag == "" {
		rootTag = "root"
	}
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return &ToolResult{Content: "invalid JSON: " + err.Error(), IsError: true}, nil
	}
	out, _ := xml.Marshal(v)
	result := fmt.Sprintf("<%s>%s</%s>", rootTag, string(out), rootTag)
	return &ToolResult{Content: result}, nil
}

type JSONQueryTool struct{}

func (t *JSONQueryTool) Name() string { return "json_query" }
func (t *JSONQueryTool) Description() string {
	return "Query JSON with JSONPath-like syntax"
}
func (t *JSONQueryTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"json": map[string]any{"type": "string", "description": "JSON string", "default": "{}"},
			"path": map[string]any{"type": "string", "description": "Dot notation path", "default": ""},
		},
	}
}
func (t *JSONQueryTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, _ := args["json"].(string)
	path, _ := args["path"].(string)
	if input == "" {
		input = "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return &ToolResult{Content: "invalid JSON: " + err.Error(), IsError: true}, nil
	}
	if path == "" {
		out, _ := json.Marshal(v)
		return &ToolResult{Content: string(out)}, nil
	}
	parts := strings.Split(path, ".")
	current := v
	for _, part := range parts {
		if m, ok := current.(map[string]any); ok {
			current = m[part]
		} else {
			current = nil
			break
		}
	}
	out, _ := json.Marshal(current)
	return &ToolResult{Content: string(out)}, nil
}

type JSONMergeTool struct{}

func (t *JSONMergeTool) Name() string { return "json_merge" }
func (t *JSONMergeTool) Description() string {
	return "Deep merge two JSON objects"
}
func (t *JSONMergeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"json1": map[string]any{"type": "string", "description": "First JSON object", "default": "{}"},
			"json2": map[string]any{"type": "string", "description": "Second JSON object", "default": "{}"},
		},
	}
}
func (t *JSONMergeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input1, _ := args["json1"].(string)
	input2, _ := args["json2"].(string)
	if input1 == "" {
		input1 = "{}"
	}
	if input2 == "" {
		input2 = "{}"
	}
	var v1, v2 any
	if err := json.Unmarshal([]byte(input1), &v1); err != nil {
		return &ToolResult{Content: "invalid JSON in json1: " + err.Error(), IsError: true}, nil
	}
	if err := json.Unmarshal([]byte(input2), &v2); err != nil {
		return &ToolResult{Content: "invalid JSON in json2: " + err.Error(), IsError: true}, nil
	}
	merged := deepMerge(v1, v2)
	out, _ := json.Marshal(merged)
	return &ToolResult{Content: string(out)}, nil
}

func deepMerge(v1, v2 any) any {
	m1, ok1 := v1.(map[string]any)
	m2, ok2 := v2.(map[string]any)
	if ok1 && ok2 {
		result := map[string]any{}
		for k, v := range m1 {
			result[k] = v
		}
		for k, v := range m2 {
			if existing, exists := result[k]; exists {
				result[k] = deepMerge(existing, v)
			} else {
				result[k] = v
			}
		}
		return result
	}
	return v2
}

type JSONArrayFlattenTool struct{}

func (t *JSONArrayFlattenTool) Name() string { return "json_array_flatten" }
func (t *JSONArrayFlattenTool) Description() string {
	return "Flatten nested JSON array"
}
func (t *JSONArrayFlattenTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"json": map[string]any{"type": "string", "description": "JSON array", "default": "[]"},
		},
	}
}
func (t *JSONArrayFlattenTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, _ := args["json"].(string)
	if input == "" {
		input = "[]"
	}
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return &ToolResult{Content: "invalid JSON: " + err.Error(), IsError: true}, nil
	}
	flat := flatten(v)
	out, _ := json.Marshal(flat)
	return &ToolResult{Content: string(out)}, nil
}

func flatten(v any) []any {
	if arr, ok := v.([]any); ok {
		result := []any{}
		for _, item := range arr {
			if inner, ok := item.([]any); ok {
				result = append(result, flatten(inner)...)
			} else {
				result = append(result, item)
			}
		}
		return result
	}
	return []any{v}
}

type JSONKeysTool struct{}

func (t *JSONKeysTool) Name() string { return "json_keys" }
func (t *JSONKeysTool) Description() string {
	return "Extract all keys from JSON object"
}
func (t *JSONKeysTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"json":      map[string]any{"type": "string", "description": "JSON object", "default": "{}"},
			"recursive": map[string]any{"type": "boolean", "description": "Recursively extract keys", "default": false},
		},
	}
}
func (t *JSONKeysTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, _ := args["json"].(string)
	recursive, _ := args["recursive"].(bool)
	if input == "" {
		input = "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return &ToolResult{Content: "invalid JSON: " + err.Error(), IsError: true}, nil
	}
	keys := extractKeys(v, "", recursive)
	out, _ := json.Marshal(keys)
	return &ToolResult{Content: string(out)}, nil
}

func extractKeys(v any, prefix string, recursive bool) []string {
	if m, ok := v.(map[string]any); ok {
		keys := []string{}
		for k, val := range m {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			keys = append(keys, path)
			if recursive {
				keys = append(keys, extractKeys(val, path, recursive)...)
			}
		}
		return keys
	}
	return []string{}
}
