package mcp

import (
	"context"
	"fmt"
	"math"
	"mime"
	"path"
	"sort"
	"strings"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"mime_type_from_extension": &MimeTypeFromExtensionTool{},
		"mime_extension_from_type": &MimeExtensionFromTypeTool{},
		"file_extension_get":       &FileExtensionGetTool{},
		"file_basename":            &FileBasenameTool{},
		"file_dirname":             &FileDirnameTool{},
		"file_path_join":           &FilePathJoinTool{},
		"file_path_clean":          &FilePathCleanTool{},
		"file_size_format":         &FileSizeFormatTool{},
	}, map[string]bool{
		"mime_type_from_extension": true,
		"mime_extension_from_type": true,
		"file_extension_get":       true,
		"file_basename":            true,
		"file_dirname":             true,
		"file_path_join":           true,
		"file_path_clean":          true,
		"file_size_format":         true,
	})
}

func fileMimeString(args map[string]any, key, fallback string) string {
	if args == nil {
		return fallback
	}
	if v, ok := args[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

// fileMimeFallbackTypes covers common extensions in case the host has no
// system mime table; mime.TypeByExtension consults it first via init order.
var fileMimeFallbackTypes = map[string]string{
	".txt":  "text/plain; charset=utf-8",
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".json": "application/json",
	".xml":  "text/xml; charset=utf-8",
	".csv":  "text/csv; charset=utf-8",
	".pdf":  "application/pdf",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".mp3":  "audio/mpeg",
	".mp4":  "video/mp4",
	".zip":  "application/zip",
	".gz":   "application/gzip",
	".md":   "text/markdown; charset=utf-8",
	".yaml": "application/yaml",
	".yml":  "application/yaml",
}

// MimeTypeFromExtensionTool resolves a MIME type from a filename extension.
type MimeTypeFromExtensionTool struct{}

func (t *MimeTypeFromExtensionTool) Name() string { return "mime_type_from_extension" }
func (t *MimeTypeFromExtensionTool) Description() string {
	return "Get the MIME type for a filename or extension"
}
func (t *MimeTypeFromExtensionTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filename": map[string]any{"type": "string", "description": "Filename or extension, e.g. report.pdf or .pdf", "default": "file.txt"},
		},
	}
}
func (t *MimeTypeFromExtensionTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	filename := fileMimeString(args, "filename", "file.txt")
	ext := strings.ToLower(path.Ext(filename))
	if ext == "" && strings.HasPrefix(filename, ".") {
		ext = strings.ToLower(filename)
	}
	if ext == "" {
		return &ToolResult{Content: fmt.Sprintf("no extension found in %q", filename), IsError: true}, nil
	}
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = fileMimeFallbackTypes[ext]
	}
	if mimeType == "" {
		return &ToolResult{Content: fmt.Sprintf("unknown MIME type for extension %q", ext), IsError: true}, nil
	}
	return &ToolResult{Content: mimeType}, nil
}

// MimeExtensionFromTypeTool lists known extensions for a MIME type.
type MimeExtensionFromTypeTool struct{}

func (t *MimeExtensionFromTypeTool) Name() string { return "mime_extension_from_type" }
func (t *MimeExtensionFromTypeTool) Description() string {
	return "Get known file extensions for a MIME type"
}
func (t *MimeExtensionFromTypeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mime_type": map[string]any{"type": "string", "description": "MIME type, e.g. application/json", "default": "text/plain"},
		},
	}
}
func (t *MimeExtensionFromTypeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	mimeType := fileMimeString(args, "mime_type", "text/plain")
	extensions, err := mime.ExtensionsByType(mimeType)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid MIME type %q", mimeType), IsError: true}, nil
	}
	if len(extensions) == 0 {
		for ext, fallbackType := range fileMimeFallbackTypes {
			if fallbackType == mimeType || strings.SplitN(fallbackType, ";", 2)[0] == mimeType {
				extensions = append(extensions, ext)
			}
		}
	}
	if len(extensions) == 0 {
		return &ToolResult{Content: fmt.Sprintf("no known extensions for MIME type %q", mimeType), IsError: true}, nil
	}
	sort.Strings(extensions)
	return &ToolResult{Content: strings.Join(extensions, ", ")}, nil
}

// FileExtensionGetTool extracts the extension from a filename.
type FileExtensionGetTool struct{}

func (t *FileExtensionGetTool) Name() string { return "file_extension_get" }
func (t *FileExtensionGetTool) Description() string {
	return "Extract the file extension from a filename"
}
func (t *FileExtensionGetTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filename": map[string]any{"type": "string", "description": "Filename or path", "default": "file.txt"},
		},
	}
}
func (t *FileExtensionGetTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	filename := fileMimeString(args, "filename", "file.txt")
	ext := path.Ext(filename)
	if ext == "" {
		return &ToolResult{Content: "(no extension)"}, nil
	}
	return &ToolResult{Content: ext}, nil
}

// FileBasenameTool returns the last element of a path.
type FileBasenameTool struct{}

func (t *FileBasenameTool) Name() string        { return "file_basename" }
func (t *FileBasenameTool) Description() string { return "Get the filename portion of a path" }
func (t *FileBasenameTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File path", "default": "."},
		},
	}
}
func (t *FileBasenameTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	return &ToolResult{Content: path.Base(fileMimeString(args, "path", "."))}, nil
}

// FileDirnameTool returns the directory portion of a path.
type FileDirnameTool struct{}

func (t *FileDirnameTool) Name() string        { return "file_dirname" }
func (t *FileDirnameTool) Description() string { return "Get the directory portion of a path" }
func (t *FileDirnameTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File path", "default": "."},
		},
	}
}
func (t *FileDirnameTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	return &ToolResult{Content: path.Dir(fileMimeString(args, "path", "."))}, nil
}

// FilePathJoinTool joins path components.
type FilePathJoinTool struct{}

func (t *FilePathJoinTool) Name() string { return "file_path_join" }
func (t *FilePathJoinTool) Description() string {
	return "Join path components into a single clean path"
}
func (t *FilePathJoinTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"parts": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Path components to join",
				"default":     []string{"."},
			},
		},
	}
}
func (t *FilePathJoinTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	parts := []string{"."}
	if args != nil {
		if raw, ok := args["parts"].([]any); ok && len(raw) > 0 {
			parts = make([]string, 0, len(raw))
			for _, item := range raw {
				str, isString := item.(string)
				if !isString {
					return &ToolResult{Content: "parts must be an array of strings", IsError: true}, nil
				}
				parts = append(parts, str)
			}
		}
	}
	return &ToolResult{Content: path.Join(parts...)}, nil
}

// FilePathCleanTool normalizes a path.
type FilePathCleanTool struct{}

func (t *FilePathCleanTool) Name() string        { return "file_path_clean" }
func (t *FilePathCleanTool) Description() string { return "Clean and normalize a file path" }
func (t *FilePathCleanTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File path", "default": "."},
		},
	}
}
func (t *FilePathCleanTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	return &ToolResult{Content: path.Clean(fileMimeString(args, "path", "."))}, nil
}

// FileSizeFormatTool renders a byte count as a human-readable size.
type FileSizeFormatTool struct{}

func (t *FileSizeFormatTool) Name() string { return "file_size_format" }
func (t *FileSizeFormatTool) Description() string {
	return "Format a byte count as a human-readable size"
}
func (t *FileSizeFormatTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"bytes":  map[string]any{"type": "integer", "description": "Size in bytes", "default": 0},
			"binary": map[string]any{"type": "boolean", "description": "Use 1024-based units (KiB) instead of 1000-based (KB)", "default": true},
		},
	}
}
func (t *FileSizeFormatTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	size := getFloat(args, "bytes", 0)
	if size < 0 {
		return &ToolResult{Content: "bytes must be non-negative", IsError: true}, nil
	}
	binary := true
	if args != nil {
		if v, ok := args["binary"].(bool); ok {
			binary = v
		}
	}
	base := 1024.0
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	if !binary {
		base = 1000.0
		units = []string{"B", "KB", "MB", "GB", "TB", "PB"}
	}
	if size < base {
		return &ToolResult{Content: fmt.Sprintf("%g %s", size, units[0])}, nil
	}
	exponent := int(math.Floor(math.Log(size) / math.Log(base)))
	if exponent >= len(units) {
		exponent = len(units) - 1
	}
	value := size / math.Pow(base, float64(exponent))
	rounded := math.Round(value*100) / 100
	return &ToolResult{Content: fmt.Sprintf("%g %s", rounded, units[exponent])}, nil
}
