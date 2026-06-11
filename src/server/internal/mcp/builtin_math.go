package mcp

import (
	"context"
	"fmt"
	"math"
	"sort"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"math_abs":    &MathAbsTool{},
		"math_round":  &MathRoundTool{},
		"math_ceil":   &MathCeilTool{},
		"math_floor":  &MathFloorTool{},
		"math_sqrt":   &MathSqrtTool{},
		"math_pow":    &MathPowTool{},
		"math_log":    &MathLogTool{},
		"math_log10":  &MathLog10Tool{},
		"math_min":    &MathMinTool{},
		"math_max":    &MathMaxTool{},
		"math_mean":   &MathMeanTool{},
		"math_median": &MathMedianTool{},
		"math_stddev": &MathStddevTool{},
	}, map[string]bool{
		"math_abs":    true,
		"math_round":  true,
		"math_ceil":   true,
		"math_floor":  true,
		"math_sqrt":   true,
		"math_pow":    true,
		"math_log":    true,
		"math_log10":  true,
		"math_min":    true,
		"math_max":    true,
		"math_mean":   true,
		"math_median": true,
		"math_stddev": true,
	})
}

type MathAbsTool struct{}

func (t *MathAbsTool) Name() string        { return "math_abs" }
func (t *MathAbsTool) Description() string { return "Absolute value" }
func (t *MathAbsTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number": map[string]any{"type": "number", "description": "Input number", "default": 0},
		},
	}
}
func (t *MathAbsTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	num := getFloat(args, "number", 0)
	return &ToolResult{Content: fmt.Sprintf("%g", math.Abs(num))}, nil
}

type MathRoundTool struct{}

func (t *MathRoundTool) Name() string        { return "math_round" }
func (t *MathRoundTool) Description() string { return "Round to nearest integer" }
func (t *MathRoundTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number":   map[string]any{"type": "number", "description": "Input number", "default": 0},
			"decimals": map[string]any{"type": "integer", "description": "Decimal places", "default": 0},
		},
	}
}
func (t *MathRoundTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	num := getFloat(args, "number", 0)
	decimals := getInt(args, "decimals", 0)
	if decimals < 0 {
		decimals = 0
	}
	scale := math.Pow(10, float64(decimals))
	rounded := math.Round(num*scale) / scale
	return &ToolResult{Content: fmt.Sprintf("%g", rounded)}, nil
}

type MathCeilTool struct{}

func (t *MathCeilTool) Name() string        { return "math_ceil" }
func (t *MathCeilTool) Description() string { return "Round up to integer" }
func (t *MathCeilTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number": map[string]any{"type": "number", "description": "Input number", "default": 0},
		},
	}
}
func (t *MathCeilTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	num := getFloat(args, "number", 0)
	return &ToolResult{Content: fmt.Sprintf("%g", math.Ceil(num))}, nil
}

type MathFloorTool struct{}

func (t *MathFloorTool) Name() string        { return "math_floor" }
func (t *MathFloorTool) Description() string { return "Round down to integer" }
func (t *MathFloorTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number": map[string]any{"type": "number", "description": "Input number", "default": 0},
		},
	}
}
func (t *MathFloorTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	num := getFloat(args, "number", 0)
	return &ToolResult{Content: fmt.Sprintf("%g", math.Floor(num))}, nil
}

type MathSqrtTool struct{}

func (t *MathSqrtTool) Name() string        { return "math_sqrt" }
func (t *MathSqrtTool) Description() string { return "Square root" }
func (t *MathSqrtTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number": map[string]any{"type": "number", "description": "Input number", "default": 0, "minimum": 0},
		},
	}
}
func (t *MathSqrtTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	num := getFloat(args, "number", 0)
	if num < 0 {
		return &ToolResult{Content: "cannot compute square root of negative number", IsError: true}, nil
	}
	return &ToolResult{Content: fmt.Sprintf("%g", math.Sqrt(num))}, nil
}

type MathPowTool struct{}

func (t *MathPowTool) Name() string        { return "math_pow" }
func (t *MathPowTool) Description() string { return "Power (exponentiation)" }
func (t *MathPowTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"base":     map[string]any{"type": "number", "description": "Base", "default": 0},
			"exponent": map[string]any{"type": "number", "description": "Exponent", "default": 1},
		},
	}
}
func (t *MathPowTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	base := getFloat(args, "base", 0)
	exponent := getFloat(args, "exponent", 1)
	return &ToolResult{Content: fmt.Sprintf("%g", math.Pow(base, exponent))}, nil
}

type MathLogTool struct{}

func (t *MathLogTool) Name() string        { return "math_log" }
func (t *MathLogTool) Description() string { return "Natural logarithm" }
func (t *MathLogTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number": map[string]any{"type": "number", "description": "Input number", "default": 1, "minimum": 0, "exclusiveMinimum": true},
		},
	}
}
func (t *MathLogTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	num := getFloat(args, "number", 1)
	if num <= 0 {
		return &ToolResult{Content: "cannot compute logarithm of non-positive number", IsError: true}, nil
	}
	return &ToolResult{Content: fmt.Sprintf("%g", math.Log(num))}, nil
}

type MathLog10Tool struct{}

func (t *MathLog10Tool) Name() string        { return "math_log10" }
func (t *MathLog10Tool) Description() string { return "Base-10 logarithm" }
func (t *MathLog10Tool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number": map[string]any{"type": "number", "description": "Input number", "default": 1, "minimum": 0, "exclusiveMinimum": true},
		},
	}
}
func (t *MathLog10Tool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	num := getFloat(args, "number", 1)
	if num <= 0 {
		return &ToolResult{Content: "cannot compute logarithm of non-positive number", IsError: true}, nil
	}
	return &ToolResult{Content: fmt.Sprintf("%g", math.Log10(num))}, nil
}

type MathMinTool struct{}

func (t *MathMinTool) Name() string        { return "math_min" }
func (t *MathMinTool) Description() string { return "Minimum of numbers" }
func (t *MathMinTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"numbers": map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "description": "Array of numbers", "default": []float64{0}},
		},
	}
}
func (t *MathMinTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	numbers := getFloatSlice(args, "numbers", []float64{0})
	if len(numbers) == 0 {
		return &ToolResult{Content: "no numbers provided", IsError: true}, nil
	}
	min := numbers[0]
	for _, n := range numbers[1:] {
		if n < min {
			min = n
		}
	}
	return &ToolResult{Content: fmt.Sprintf("%g", min)}, nil
}

type MathMaxTool struct{}

func (t *MathMaxTool) Name() string        { return "math_max" }
func (t *MathMaxTool) Description() string { return "Maximum of numbers" }
func (t *MathMaxTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"numbers": map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "description": "Array of numbers", "default": []float64{0}},
		},
	}
}
func (t *MathMaxTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	numbers := getFloatSlice(args, "numbers", []float64{0})
	if len(numbers) == 0 {
		return &ToolResult{Content: "no numbers provided", IsError: true}, nil
	}
	max := numbers[0]
	for _, n := range numbers[1:] {
		if n > max {
			max = n
		}
	}
	return &ToolResult{Content: fmt.Sprintf("%g", max)}, nil
}

type MathMeanTool struct{}

func (t *MathMeanTool) Name() string        { return "math_mean" }
func (t *MathMeanTool) Description() string { return "Arithmetic mean (average)" }
func (t *MathMeanTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"numbers": map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "description": "Array of numbers", "default": []float64{0}},
		},
	}
}
func (t *MathMeanTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	numbers := getFloatSlice(args, "numbers", []float64{0})
	if len(numbers) == 0 {
		return &ToolResult{Content: "no numbers provided", IsError: true}, nil
	}
	sum := 0.0
	for _, n := range numbers {
		sum += n
	}
	return &ToolResult{Content: fmt.Sprintf("%g", sum/float64(len(numbers)))}, nil
}

type MathMedianTool struct{}

func (t *MathMedianTool) Name() string        { return "math_median" }
func (t *MathMedianTool) Description() string { return "Median value" }
func (t *MathMedianTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"numbers": map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "description": "Array of numbers", "default": []float64{0}},
		},
	}
}
func (t *MathMedianTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	numbers := getFloatSlice(args, "numbers", []float64{0})
	if len(numbers) == 0 {
		return &ToolResult{Content: "no numbers provided", IsError: true}, nil
	}
	sorted := make([]float64, len(numbers))
	copy(sorted, numbers)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	var median float64
	if len(sorted)%2 == 0 {
		median = (sorted[mid-1] + sorted[mid]) / 2
	} else {
		median = sorted[mid]
	}
	return &ToolResult{Content: fmt.Sprintf("%g", median)}, nil
}

type MathStddevTool struct{}

func (t *MathStddevTool) Name() string        { return "math_stddev" }
func (t *MathStddevTool) Description() string { return "Standard deviation" }
func (t *MathStddevTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"numbers": map[string]any{"type": "array", "items": map[string]any{"type": "number"}, "description": "Array of numbers", "default": []float64{0}},
		},
	}
}
func (t *MathStddevTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	numbers := getFloatSlice(args, "numbers", []float64{0})
	if len(numbers) == 0 {
		return &ToolResult{Content: "no numbers provided", IsError: true}, nil
	}
	sum := 0.0
	for _, n := range numbers {
		sum += n
	}
	mean := sum / float64(len(numbers))
	variance := 0.0
	for _, n := range numbers {
		diff := n - mean
		variance += diff * diff
	}
	variance /= float64(len(numbers))
	return &ToolResult{Content: fmt.Sprintf("%g", math.Sqrt(variance))}, nil
}
