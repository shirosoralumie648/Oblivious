package mcp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestUUIDV4Tool(t *testing.T) {
	tool := &UUIDV4Tool{}
	ctx := context.Background()

	t.Run("empty args", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		uuid := result.Content
		parts := strings.Split(uuid, "-")
		if len(parts) != 5 || len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
			t.Errorf("invalid UUID format: %s", uuid)
		}
		if parts[2][0] != '4' {
			t.Errorf("expected version 4, got %c", parts[2][0])
		}
	})
}

func TestUUIDParseTool(t *testing.T) {
	tool := &UUIDParseTool{}
	ctx := context.Background()

	t.Run("valid UUID v4", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"uuid": "550e8400-e29b-41d4-a716-446655440000"})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(result.Content), &parsed); err != nil {
			t.Fatalf("failed to parse result JSON: %v", err)
		}
		if !parsed["valid"].(bool) {
			t.Errorf("expected valid=true")
		}
		if int(parsed["version"].(float64)) != 4 {
			t.Errorf("expected version=4, got %v", parsed["version"])
		}
	})

	t.Run("invalid UUID", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"uuid": "not-a-uuid"})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(result.Content), &parsed); err != nil {
			t.Fatalf("failed to parse result JSON: %v", err)
		}
		if parsed["valid"].(bool) {
			t.Errorf("expected valid=false")
		}
	})

	t.Run("missing uuid", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error result")
		}
	})
}

func TestRandomStringTool(t *testing.T) {
	tool := &RandomStringTool{}
	ctx := context.Background()

	t.Run("empty args default", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		if len(result.Content) != 16 {
			t.Errorf("expected length 16, got %d", len(result.Content))
		}
	})

	t.Run("custom length and charset", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"length": float64(10), "charset": "numeric"})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		if len(result.Content) != 10 {
			t.Errorf("expected length 10, got %d", len(result.Content))
		}
		for _, c := range result.Content {
			if c < '0' || c > '9' {
				t.Errorf("expected only digits, got %c", c)
			}
		}
	})

	t.Run("invalid length", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"length": float64(0)})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error result")
		}
	})
}

func TestRandomIntTool(t *testing.T) {
	tool := &RandomIntTool{}
	ctx := context.Background()

	t.Run("empty args default", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		var n int
		if _, err := fmt.Sscanf(result.Content, "%d", &n); err != nil {
			t.Errorf("invalid integer: %s", result.Content)
		}
		if n < 0 || n > 100 {
			t.Errorf("expected value in [0, 100], got %d", n)
		}
	})

	t.Run("custom range", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"min": float64(10), "max": float64(20)})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		var n int
		if _, err := fmt.Sscanf(result.Content, "%d", &n); err != nil {
			t.Errorf("invalid integer: %s", result.Content)
		}
		if n < 10 || n > 20 {
			t.Errorf("expected value in [10, 20], got %d", n)
		}
	})

	t.Run("invalid range", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"min": float64(20), "max": float64(10)})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error result")
		}
	})
}

func TestRandomBytesTool(t *testing.T) {
	tool := &RandomBytesTool{}
	ctx := context.Background()

	t.Run("empty args default", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		b, err := hex.DecodeString(result.Content)
		if err != nil {
			t.Errorf("invalid hex: %v", err)
		}
		if len(b) != 16 {
			t.Errorf("expected 16 bytes, got %d", len(b))
		}
	})

	t.Run("custom length", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"length": float64(8)})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		b, err := hex.DecodeString(result.Content)
		if err != nil {
			t.Errorf("invalid hex: %v", err)
		}
		if len(b) != 8 {
			t.Errorf("expected 8 bytes, got %d", len(b))
		}
	})

	t.Run("invalid length", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"length": float64(2000)})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error result")
		}
	})
}

func TestRandomFloatTool(t *testing.T) {
	tool := &RandomFloatTool{}
	ctx := context.Background()

	t.Run("empty args default", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		var f float64
		if _, err := fmt.Sscanf(result.Content, "%f", &f); err != nil {
			t.Errorf("invalid float: %s", result.Content)
		}
		if f < 0.0 || f >= 1.0 {
			t.Errorf("expected value in [0.0, 1.0), got %f", f)
		}
	})

	t.Run("custom range", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"min": 10.0, "max": 20.0})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		var f float64
		if _, err := fmt.Sscanf(result.Content, "%f", &f); err != nil {
			t.Errorf("invalid float: %s", result.Content)
		}
		if f < 10.0 || f >= 20.0 {
			t.Errorf("expected value in [10.0, 20.0), got %f", f)
		}
	})

	t.Run("invalid range", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"min": 20.0, "max": 10.0})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error result")
		}
	})
}

func TestRandomChoiceTool(t *testing.T) {
	tool := &RandomChoiceTool{}
	ctx := context.Background()

	t.Run("valid choices", func(t *testing.T) {
		choices := []any{"a", "b", "c"}
		result, err := tool.Execute(ctx, map[string]any{"choices": choices})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		found := false
		for _, c := range choices {
			if result.Content == c {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("result %s not in choices", result.Content)
		}
	})

	t.Run("missing choices", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error result")
		}
	})

	t.Run("empty choices", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"choices": []any{}})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error result")
		}
	})
}

func TestRandomShuffleTool(t *testing.T) {
	tool := &RandomShuffleTool{}
	ctx := context.Background()

	t.Run("empty args", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.IsError {
			t.Errorf("expected error result")
		}
	})

	t.Run("valid items", func(t *testing.T) {
		items := []any{"a", "b", "c", "d"}
		result, err := tool.Execute(ctx, map[string]any{"items": items})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		var shuffled []string
		if err := json.Unmarshal([]byte(result.Content), &shuffled); err != nil {
			t.Fatalf("failed to parse result JSON: %v", err)
		}
		if len(shuffled) != len(items) {
			t.Errorf("expected length %d, got %d", len(items), len(shuffled))
		}
		for _, item := range items {
			found := false
			for _, s := range shuffled {
				if s == item {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("item %s not found in shuffled result", item)
			}
		}
	})

	t.Run("empty items", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]any{"items": []any{}})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", result.Content)
		}
		var shuffled []string
		if err := json.Unmarshal([]byte(result.Content), &shuffled); err != nil {
			t.Fatalf("failed to parse result JSON: %v", err)
		}
		if len(shuffled) != 0 {
			t.Errorf("expected empty array, got %v", shuffled)
		}
	})
}
