package mcp

import (
	"context"
	"strings"
	"testing"
)

func TestJSONToYAML(t *testing.T) {
	tool := &JSONToYAMLTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"json": `{"name":"test","count":42,"enabled":true}`,
	})
	if err != nil {
		t.Fatalf("json_to_yaml error: %v", err)
	}
	if !strings.Contains(result.Content, "name: test") || !strings.Contains(result.Content, "count: 42") {
		t.Fatalf("json_to_yaml output invalid: %q", result.Content)
	}
}

func TestJSONToYAMLEmpty(t *testing.T) {
	tool := &JSONToYAMLTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("json_to_yaml empty error: %v", err)
	}
	if result.Content != "{}" {
		t.Fatalf("json_to_yaml empty = %q, want {}", result.Content)
	}
}

func TestJSONToYAMLInvalid(t *testing.T) {
	tool := &JSONToYAMLTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"json": `{invalid`})
	if err != nil || !result.IsError {
		t.Fatal("json_to_yaml should reject invalid JSON")
	}
}

func TestYAMLToJSON(t *testing.T) {
	tool := &YAMLToJSONTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"yaml": "name: test\ncount: 42\nenabled: true",
	})
	if err != nil {
		t.Fatalf("yaml_to_json error: %v", err)
	}
	if !strings.Contains(result.Content, `"name":"test"`) || !strings.Contains(result.Content, `"count":42`) {
		t.Fatalf("yaml_to_json output invalid: %q", result.Content)
	}
}

func TestYAMLToJSONEmpty(t *testing.T) {
	tool := &YAMLToJSONTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("yaml_to_json empty error: %v", err)
	}
	if result.Content != "{}" {
		t.Fatalf("yaml_to_json empty = %q, want {}", result.Content)
	}
}

func TestCSVToJSON(t *testing.T) {
	tool := &CSVToJSONTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"csv":        "name,age\nAlice,30\nBob,25",
		"has_header": true,
	})
	if err != nil {
		t.Fatalf("csv_to_json error: %v", err)
	}
	if !strings.Contains(result.Content, `"name":"Alice"`) || !strings.Contains(result.Content, `"age":"30"`) {
		t.Fatalf("csv_to_json output invalid: %q", result.Content)
	}
}

func TestCSVToJSONEmpty(t *testing.T) {
	tool := &CSVToJSONTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("csv_to_json empty error: %v", err)
	}
	if result.Content != "[]" {
		t.Fatalf("csv_to_json empty = %q, want []", result.Content)
	}
}

func TestCSVToJSONInvalid(t *testing.T) {
	tool := &CSVToJSONTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"csv": "name,age\n\"unclosed"})
	if err != nil || !result.IsError {
		t.Fatal("csv_to_json should reject invalid CSV")
	}
}

func TestJSONToCSV(t *testing.T) {
	tool := &JSONToCSVTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"json": `[{"name":"Alice","age":30},{"name":"Bob","age":25}]`,
	})
	if err != nil {
		t.Fatalf("json_to_csv error: %v", err)
	}
	if !strings.Contains(result.Content, "Alice") || !strings.Contains(result.Content, "30") {
		t.Fatalf("json_to_csv output invalid: %q", result.Content)
	}
}

func TestJSONToCSVEmpty(t *testing.T) {
	tool := &JSONToCSVTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("json_to_csv empty error: %v", err)
	}
	if result.Content != "" {
		t.Fatalf("json_to_csv empty = %q, want empty", result.Content)
	}
}

func TestJSONToCSVInvalid(t *testing.T) {
	tool := &JSONToCSVTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"json": `[invalid]`})
	if err != nil || !result.IsError {
		t.Fatal("json_to_csv should reject invalid JSON")
	}
}

func TestTSVToJSON(t *testing.T) {
	tool := &TSVToJSONTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"tsv":        "name\tage\nAlice\t30\nBob\t25",
		"has_header": true,
	})
	if err != nil {
		t.Fatalf("tsv_to_json error: %v", err)
	}
	if !strings.Contains(result.Content, `"name":"Alice"`) || !strings.Contains(result.Content, `"age":"30"`) {
		t.Fatalf("tsv_to_json output invalid: %q", result.Content)
	}
}

func TestTSVToJSONEmpty(t *testing.T) {
	tool := &TSVToJSONTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("tsv_to_json empty error: %v", err)
	}
	if result.Content != "[]" {
		t.Fatalf("tsv_to_json empty = %q, want []", result.Content)
	}
}

func TestJSONToTSV(t *testing.T) {
	tool := &JSONToTSVTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"json": `[{"name":"Alice","age":30},{"name":"Bob","age":25}]`,
	})
	if err != nil {
		t.Fatalf("json_to_tsv error: %v", err)
	}
	if !strings.Contains(result.Content, "Alice") || !strings.Contains(result.Content, "30") {
		t.Fatalf("json_to_tsv output invalid: %q", result.Content)
	}
}

func TestJSONToTSVEmpty(t *testing.T) {
	tool := &JSONToTSVTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("json_to_tsv empty error: %v", err)
	}
	if result.Content != "" {
		t.Fatalf("json_to_tsv empty = %q, want empty", result.Content)
	}
}

func TestJSONToTSVInvalid(t *testing.T) {
	tool := &JSONToTSVTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"json": `{not:array}`})
	if err != nil || !result.IsError {
		t.Fatal("json_to_tsv should reject invalid JSON")
	}
}

func TestXMLToJSON(t *testing.T) {
	tool := &XMLToJSONTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"xml": `<root><name>test</name></root>`,
	})
	if err != nil {
		t.Fatalf("xml_to_json error: %v", err)
	}
	if result.Content == "" {
		t.Fatal("xml_to_json returned empty")
	}
}

func TestXMLToJSONEmpty(t *testing.T) {
	tool := &XMLToJSONTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("xml_to_json empty error: %v", err)
	}
	if result.Content == "" {
		t.Fatal("xml_to_json empty should return valid JSON")
	}
}

func TestXMLToJSONInvalid(t *testing.T) {
	tool := &XMLToJSONTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"xml": `<unclosed`})
	if err != nil || !result.IsError {
		t.Fatal("xml_to_json should reject invalid XML")
	}
}

func TestJSONToXML(t *testing.T) {
	tool := &JSONToXMLTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"json":     `{"name":"test"}`,
		"root_tag": "data",
	})
	if err != nil {
		t.Fatalf("json_to_xml error: %v", err)
	}
	if !strings.Contains(result.Content, "<data>") || !strings.Contains(result.Content, "</data>") {
		t.Fatalf("json_to_xml output invalid: %q", result.Content)
	}
}

func TestJSONToXMLEmpty(t *testing.T) {
	tool := &JSONToXMLTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("json_to_xml empty error: %v", err)
	}
	if !strings.Contains(result.Content, "<root>") || !strings.Contains(result.Content, "</root>") {
		t.Fatalf("json_to_xml empty = %q, want root tags", result.Content)
	}
}

func TestJSONToXMLInvalid(t *testing.T) {
	tool := &JSONToXMLTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"json": `{invalid}`})
	if err != nil || !result.IsError {
		t.Fatal("json_to_xml should reject invalid JSON")
	}
}

func TestJSONQuery(t *testing.T) {
	tool := &JSONQueryTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"json": `{"user":{"name":"Alice","age":30}}`,
		"path": "user.name",
	})
	if err != nil {
		t.Fatalf("json_query error: %v", err)
	}
	if result.Content != `"Alice"` {
		t.Fatalf("json_query = %q, want \"Alice\"", result.Content)
	}
}

func TestJSONQueryEmpty(t *testing.T) {
	tool := &JSONQueryTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("json_query empty error: %v", err)
	}
	if result.Content != "{}" {
		t.Fatalf("json_query empty = %q, want {}", result.Content)
	}
}

func TestJSONQueryInvalid(t *testing.T) {
	tool := &JSONQueryTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"json": `{bad`, "path": "x"})
	if err != nil || !result.IsError {
		t.Fatal("json_query should reject invalid JSON")
	}
}

func TestJSONMerge(t *testing.T) {
	tool := &JSONMergeTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"json1": `{"a":1,"b":2}`,
		"json2": `{"b":3,"c":4}`,
	})
	if err != nil {
		t.Fatalf("json_merge error: %v", err)
	}
	if !strings.Contains(result.Content, `"a":1`) || !strings.Contains(result.Content, `"b":3`) || !strings.Contains(result.Content, `"c":4`) {
		t.Fatalf("json_merge output invalid: %q", result.Content)
	}
}

func TestJSONMergeEmpty(t *testing.T) {
	tool := &JSONMergeTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("json_merge empty error: %v", err)
	}
	if result.Content != "{}" {
		t.Fatalf("json_merge empty = %q, want {}", result.Content)
	}
}

func TestJSONMergeInvalid(t *testing.T) {
	tool := &JSONMergeTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"json1": `{bad}`, "json2": `{}`})
	if err != nil || !result.IsError {
		t.Fatal("json_merge should reject invalid JSON")
	}
}

func TestJSONArrayFlatten(t *testing.T) {
	tool := &JSONArrayFlattenTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"json": `[1,[2,3],[4,[5,6]]]`,
	})
	if err != nil {
		t.Fatalf("json_array_flatten error: %v", err)
	}
	if result.Content != "[1,2,3,4,5,6]" {
		t.Fatalf("json_array_flatten = %q, want [1,2,3,4,5,6]", result.Content)
	}
}

func TestJSONArrayFlattenEmpty(t *testing.T) {
	tool := &JSONArrayFlattenTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("json_array_flatten empty error: %v", err)
	}
	if result.Content != "[]" {
		t.Fatalf("json_array_flatten empty = %q, want []", result.Content)
	}
}

func TestJSONArrayFlattenInvalid(t *testing.T) {
	tool := &JSONArrayFlattenTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"json": `[bad`})
	if err != nil || !result.IsError {
		t.Fatal("json_array_flatten should reject invalid JSON")
	}
}

func TestJSONKeys(t *testing.T) {
	tool := &JSONKeysTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"json":      `{"a":1,"b":{"c":2}}`,
		"recursive": false,
	})
	if err != nil {
		t.Fatalf("json_keys error: %v", err)
	}
	if !strings.Contains(result.Content, `"a"`) || !strings.Contains(result.Content, `"b"`) {
		t.Fatalf("json_keys output invalid: %q", result.Content)
	}
}

func TestJSONKeysRecursive(t *testing.T) {
	tool := &JSONKeysTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"json":      `{"a":1,"b":{"c":2}}`,
		"recursive": true,
	})
	if err != nil {
		t.Fatalf("json_keys recursive error: %v", err)
	}
	if !strings.Contains(result.Content, `"b.c"`) {
		t.Fatalf("json_keys recursive should include nested keys: %q", result.Content)
	}
}

func TestJSONKeysEmpty(t *testing.T) {
	tool := &JSONKeysTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("json_keys empty error: %v", err)
	}
	if result.Content != "[]" {
		t.Fatalf("json_keys empty = %q, want []", result.Content)
	}
}

func TestJSONKeysInvalid(t *testing.T) {
	tool := &JSONKeysTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"json": `{bad}`})
	if err != nil || !result.IsError {
		t.Fatal("json_keys should reject invalid JSON")
	}
}

func TestDataFormatToolsRegistration(t *testing.T) {
	tools := []string{
		"json_to_yaml", "yaml_to_json", "csv_to_json", "json_to_csv",
		"tsv_to_json", "json_to_tsv", "xml_to_json", "json_to_xml",
		"json_query", "json_merge", "json_array_flatten", "json_keys",
	}
	for _, name := range tools {
		if _, ok := GetBuiltinTool(name); !ok {
			t.Fatalf("data format tool %s not registered", name)
		}
		if !IsDefaultCommercialBuiltin(name) {
			t.Fatalf("data format tool %s should be default commercial enabled", name)
		}
	}
}
