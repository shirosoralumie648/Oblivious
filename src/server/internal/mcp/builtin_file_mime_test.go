package mcp

import (
	"context"
	"strings"
	"testing"
)

func executeFileMimeTool(t *testing.T, name string, args map[string]any) *ToolResult {
	t.Helper()
	tool, ok := GetBuiltinTool(name)
	if !ok {
		t.Fatalf("builtin %s not registered", name)
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("%s returned error: %v", name, err)
	}
	return result
}

var fileMimeToolNames = []string{
	"mime_type_from_extension", "mime_extension_from_type", "file_extension_get",
	"file_basename", "file_dirname", "file_path_join", "file_path_clean", "file_size_format",
}

func TestFileMimeToolsRegisteredAndDefaultEnabled(t *testing.T) {
	for _, name := range fileMimeToolNames {
		if _, ok := GetBuiltinTool(name); !ok {
			t.Fatalf("expected %s to be registered", name)
		}
		if !IsDefaultCommercialBuiltin(name) {
			t.Fatalf("expected %s to be default commercial enabled", name)
		}
	}
}

func TestMimeTypeFromExtension(t *testing.T) {
	result := executeFileMimeTool(t, "mime_type_from_extension", map[string]any{"filename": "photo.png"})
	if result.Content != "image/png" {
		t.Fatalf("mime_type_from_extension(photo.png) = %q, want image/png", result.Content)
	}
	result = executeFileMimeTool(t, "mime_type_from_extension", map[string]any{"filename": "doc.pdf"})
	if result.Content != "application/pdf" {
		t.Fatalf("mime_type_from_extension(doc.pdf) = %q, want application/pdf", result.Content)
	}
	result = executeFileMimeTool(t, "mime_type_from_extension", map[string]any{"filename": "noextension"})
	if !result.IsError {
		t.Fatalf("expected missing extension error, got %q", result.Content)
	}
}

func TestMimeExtensionFromType(t *testing.T) {
	result := executeFileMimeTool(t, "mime_extension_from_type", map[string]any{"mime_type": "image/png"})
	if !strings.Contains(result.Content, ".png") {
		t.Fatalf("mime_extension_from_type(image/png) = %q, want it to contain .png", result.Content)
	}
	result = executeFileMimeTool(t, "mime_extension_from_type", map[string]any{"mime_type": "not a mime"})
	if !result.IsError {
		t.Fatalf("expected invalid MIME error, got %q", result.Content)
	}
}

func TestFilePathTools(t *testing.T) {
	result := executeFileMimeTool(t, "file_extension_get", map[string]any{"filename": "archive.tar.gz"})
	if result.Content != ".gz" {
		t.Fatalf("file_extension_get = %q, want .gz", result.Content)
	}
	result = executeFileMimeTool(t, "file_extension_get", map[string]any{"filename": "README"})
	if result.Content != "(no extension)" {
		t.Fatalf("file_extension_get(README) = %q, want (no extension)", result.Content)
	}
	result = executeFileMimeTool(t, "file_basename", map[string]any{"path": "/var/log/app/server.log"})
	if result.Content != "server.log" {
		t.Fatalf("file_basename = %q, want server.log", result.Content)
	}
	result = executeFileMimeTool(t, "file_dirname", map[string]any{"path": "/var/log/app/server.log"})
	if result.Content != "/var/log/app" {
		t.Fatalf("file_dirname = %q, want /var/log/app", result.Content)
	}
	result = executeFileMimeTool(t, "file_path_join", map[string]any{"parts": []any{"/var", "log", "..", "tmp", "file.txt"}})
	if result.Content != "/var/tmp/file.txt" {
		t.Fatalf("file_path_join = %q, want /var/tmp/file.txt", result.Content)
	}
	result = executeFileMimeTool(t, "file_path_join", map[string]any{"parts": []any{"/var", 42}})
	if !result.IsError {
		t.Fatalf("expected non-string part rejection, got %q", result.Content)
	}
	result = executeFileMimeTool(t, "file_path_clean", map[string]any{"path": "/var//log/../tmp/./file.txt"})
	if result.Content != "/var/tmp/file.txt" {
		t.Fatalf("file_path_clean = %q, want /var/tmp/file.txt", result.Content)
	}
}

func TestFileSizeFormat(t *testing.T) {
	cases := []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"bytes": 0}, "0 B"},
		{map[string]any{"bytes": 1536, "binary": true}, "1.5 KiB"},
		{map[string]any{"bytes": 1500000, "binary": false}, "1.5 MB"},
		{map[string]any{"bytes": 1073741824, "binary": true}, "1 GiB"},
	}
	for _, tc := range cases {
		result := executeFileMimeTool(t, "file_size_format", tc.args)
		if result.IsError {
			t.Fatalf("file_size_format(%v) returned tool error: %s", tc.args, result.Content)
		}
		if result.Content != tc.want {
			t.Fatalf("file_size_format(%v) = %q, want %q", tc.args, result.Content, tc.want)
		}
	}
	result := executeFileMimeTool(t, "file_size_format", map[string]any{"bytes": -5})
	if !result.IsError {
		t.Fatalf("expected negative bytes rejection, got %q", result.Content)
	}
}

func TestFileMimeToolsSucceedWithEmptyArgs(t *testing.T) {
	for _, name := range fileMimeToolNames {
		result := executeFileMimeTool(t, name, map[string]any{})
		if result.IsError {
			t.Fatalf("%s with empty args returned tool error: %s", name, result.Content)
		}
		if strings.Contains(strings.ToLower(result.Content), "placeholder") {
			t.Fatalf("%s with empty args returned placeholder output: %q", name, result.Content)
		}
	}
}
