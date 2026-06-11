package mcp

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestMathAbsReturnsAbsoluteValue(t *testing.T) {
	tool, ok := GetBuiltinTool("math_abs")
	if !ok {
		t.Fatal("math_abs builtin not found")
	}

	cases := []struct {
		input float64
		want  string
	}{
		{-5.5, "5.5"},
		{5.5, "5.5"},
		{0, "0"},
		{-100, "100"},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"number": tt.input})
		if err != nil {
			t.Fatalf("math_abs(%v) returned error: %v", tt.input, err)
		}
		if result.Content != tt.want {
			t.Fatalf("math_abs(%v) = %q, want %q", tt.input, result.Content, tt.want)
		}
	}
}

func TestMathAbsEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_abs")
	if !ok {
		t.Fatal("math_abs builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_abs with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_abs empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathRoundRoundsToNearestInteger(t *testing.T) {
	tool, ok := GetBuiltinTool("math_round")
	if !ok {
		t.Fatal("math_round builtin not found")
	}

	cases := []struct {
		number   float64
		decimals int
		want     string
	}{
		{3.7, 0, "4"},
		{3.2, 0, "3"},
		{3.14159, 2, "3.14"},
		{3.14159, 3, "3.142"},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"number": tt.number, "decimals": tt.decimals})
		if err != nil {
			t.Fatalf("math_round(%v, %d) returned error: %v", tt.number, tt.decimals, err)
		}
		if result.Content != tt.want {
			t.Fatalf("math_round(%v, %d) = %q, want %q", tt.number, tt.decimals, result.Content, tt.want)
		}
	}
}

func TestMathRoundEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_round")
	if !ok {
		t.Fatal("math_round builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_round with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_round empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathCeilRoundsUp(t *testing.T) {
	tool, ok := GetBuiltinTool("math_ceil")
	if !ok {
		t.Fatal("math_ceil builtin not found")
	}

	cases := []struct {
		input float64
		want  string
	}{
		{3.1, "4"},
		{3.9, "4"},
		{-3.1, "-3"},
		{5, "5"},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"number": tt.input})
		if err != nil {
			t.Fatalf("math_ceil(%v) returned error: %v", tt.input, err)
		}
		if result.Content != tt.want {
			t.Fatalf("math_ceil(%v) = %q, want %q", tt.input, result.Content, tt.want)
		}
	}
}

func TestMathCeilEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_ceil")
	if !ok {
		t.Fatal("math_ceil builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_ceil with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_ceil empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathFloorRoundsDown(t *testing.T) {
	tool, ok := GetBuiltinTool("math_floor")
	if !ok {
		t.Fatal("math_floor builtin not found")
	}

	cases := []struct {
		input float64
		want  string
	}{
		{3.9, "3"},
		{3.1, "3"},
		{-3.1, "-4"},
		{5, "5"},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"number": tt.input})
		if err != nil {
			t.Fatalf("math_floor(%v) returned error: %v", tt.input, err)
		}
		if result.Content != tt.want {
			t.Fatalf("math_floor(%v) = %q, want %q", tt.input, result.Content, tt.want)
		}
	}
}

func TestMathFloorEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_floor")
	if !ok {
		t.Fatal("math_floor builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_floor with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_floor empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathSqrtComputesSquareRoot(t *testing.T) {
	tool, ok := GetBuiltinTool("math_sqrt")
	if !ok {
		t.Fatal("math_sqrt builtin not found")
	}

	cases := []struct {
		input float64
		want  string
	}{
		{0, "0"},
		{4, "2"},
		{9, "3"},
		{2, "1.4142135623730951"},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"number": tt.input})
		if err != nil {
			t.Fatalf("math_sqrt(%v) returned error: %v", tt.input, err)
		}
		if result.Content != tt.want {
			t.Fatalf("math_sqrt(%v) = %q, want %q", tt.input, result.Content, tt.want)
		}
	}
}

func TestMathSqrtRejectsNegative(t *testing.T) {
	tool, ok := GetBuiltinTool("math_sqrt")
	if !ok {
		t.Fatal("math_sqrt builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"number": -1})
	if err != nil {
		t.Fatalf("math_sqrt(-1) returned error instead of IsError result: %v", err)
	}
	if !result.IsError {
		t.Fatalf("math_sqrt(-1) IsError = false, want true")
	}
	if !strings.Contains(result.Content, "negative") {
		t.Fatalf("math_sqrt(-1) content = %q, want negative error", result.Content)
	}
}

func TestMathSqrtEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_sqrt")
	if !ok {
		t.Fatal("math_sqrt builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_sqrt with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_sqrt empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathPowComputesPower(t *testing.T) {
	tool, ok := GetBuiltinTool("math_pow")
	if !ok {
		t.Fatal("math_pow builtin not found")
	}

	cases := []struct {
		base     float64
		exponent float64
		want     string
	}{
		{2, 3, "8"},
		{5, 2, "25"},
		{10, 0, "1"},
		{2, -1, "0.5"},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"base": tt.base, "exponent": tt.exponent})
		if err != nil {
			t.Fatalf("math_pow(%v, %v) returned error: %v", tt.base, tt.exponent, err)
		}
		if result.Content != tt.want {
			t.Fatalf("math_pow(%v, %v) = %q, want %q", tt.base, tt.exponent, result.Content, tt.want)
		}
	}
}

func TestMathPowEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_pow")
	if !ok {
		t.Fatal("math_pow builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_pow with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_pow empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathLogComputesNaturalLog(t *testing.T) {
	tool, ok := GetBuiltinTool("math_log")
	if !ok {
		t.Fatal("math_log builtin not found")
	}

	cases := []struct {
		input float64
		want  float64
	}{
		{1, 0},
		{math.E, 1},
		{10, 2.302585092994046},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"number": tt.input})
		if err != nil {
			t.Fatalf("math_log(%v) returned error: %v", tt.input, err)
		}
		if math.Abs(parseFloat(result.Content)-tt.want) > 0.0001 {
			t.Fatalf("math_log(%v) = %q, want approximately %v", tt.input, result.Content, tt.want)
		}
	}
}

func TestMathLogRejectsNonPositive(t *testing.T) {
	tool, ok := GetBuiltinTool("math_log")
	if !ok {
		t.Fatal("math_log builtin not found")
	}

	for _, input := range []float64{0, -1} {
		result, err := tool.Execute(context.Background(), map[string]any{"number": input})
		if err != nil {
			t.Fatalf("math_log(%v) returned error instead of IsError result: %v", input, err)
		}
		if !result.IsError {
			t.Fatalf("math_log(%v) IsError = false, want true", input)
		}
	}
}

func TestMathLogEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_log")
	if !ok {
		t.Fatal("math_log builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_log with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_log empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathLog10ComputesBase10Log(t *testing.T) {
	tool, ok := GetBuiltinTool("math_log10")
	if !ok {
		t.Fatal("math_log10 builtin not found")
	}

	cases := []struct {
		input float64
		want  string
	}{
		{1, "0"},
		{10, "1"},
		{100, "2"},
		{1000, "3"},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"number": tt.input})
		if err != nil {
			t.Fatalf("math_log10(%v) returned error: %v", tt.input, err)
		}
		if result.Content != tt.want {
			t.Fatalf("math_log10(%v) = %q, want %q", tt.input, result.Content, tt.want)
		}
	}
}

func TestMathLog10RejectsNonPositive(t *testing.T) {
	tool, ok := GetBuiltinTool("math_log10")
	if !ok {
		t.Fatal("math_log10 builtin not found")
	}

	for _, input := range []float64{0, -1} {
		result, err := tool.Execute(context.Background(), map[string]any{"number": input})
		if err != nil {
			t.Fatalf("math_log10(%v) returned error instead of IsError result: %v", input, err)
		}
		if !result.IsError {
			t.Fatalf("math_log10(%v) IsError = false, want true", input)
		}
	}
}

func TestMathLog10EmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_log10")
	if !ok {
		t.Fatal("math_log10 builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_log10 with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_log10 empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathMinFindsMinimum(t *testing.T) {
	tool, ok := GetBuiltinTool("math_min")
	if !ok {
		t.Fatal("math_min builtin not found")
	}

	cases := []struct {
		numbers []float64
		want    string
	}{
		{[]float64{1, 2, 3}, "1"},
		{[]float64{5, -2, 8}, "-2"},
		{[]float64{10}, "10"},
	}

	for _, tt := range cases {
		numbersAny := make([]any, len(tt.numbers))
		for i, n := range tt.numbers {
			numbersAny[i] = n
		}
		result, err := tool.Execute(context.Background(), map[string]any{"numbers": numbersAny})
		if err != nil {
			t.Fatalf("math_min(%v) returned error: %v", tt.numbers, err)
		}
		if result.Content != tt.want {
			t.Fatalf("math_min(%v) = %q, want %q", tt.numbers, result.Content, tt.want)
		}
	}
}

func TestMathMinEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_min")
	if !ok {
		t.Fatal("math_min builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_min with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_min empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathMaxFindsMaximum(t *testing.T) {
	tool, ok := GetBuiltinTool("math_max")
	if !ok {
		t.Fatal("math_max builtin not found")
	}

	cases := []struct {
		numbers []float64
		want    string
	}{
		{[]float64{1, 2, 3}, "3"},
		{[]float64{5, -2, 8}, "8"},
		{[]float64{10}, "10"},
	}

	for _, tt := range cases {
		numbersAny := make([]any, len(tt.numbers))
		for i, n := range tt.numbers {
			numbersAny[i] = n
		}
		result, err := tool.Execute(context.Background(), map[string]any{"numbers": numbersAny})
		if err != nil {
			t.Fatalf("math_max(%v) returned error: %v", tt.numbers, err)
		}
		if result.Content != tt.want {
			t.Fatalf("math_max(%v) = %q, want %q", tt.numbers, result.Content, tt.want)
		}
	}
}

func TestMathMaxEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_max")
	if !ok {
		t.Fatal("math_max builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_max with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_max empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathMeanComputesAverage(t *testing.T) {
	tool, ok := GetBuiltinTool("math_mean")
	if !ok {
		t.Fatal("math_mean builtin not found")
	}

	cases := []struct {
		numbers []float64
		want    string
	}{
		{[]float64{1, 2, 3}, "2"},
		{[]float64{5, 10, 15}, "10"},
		{[]float64{10}, "10"},
	}

	for _, tt := range cases {
		numbersAny := make([]any, len(tt.numbers))
		for i, n := range tt.numbers {
			numbersAny[i] = n
		}
		result, err := tool.Execute(context.Background(), map[string]any{"numbers": numbersAny})
		if err != nil {
			t.Fatalf("math_mean(%v) returned error: %v", tt.numbers, err)
		}
		if result.Content != tt.want {
			t.Fatalf("math_mean(%v) = %q, want %q", tt.numbers, result.Content, tt.want)
		}
	}
}

func TestMathMeanEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_mean")
	if !ok {
		t.Fatal("math_mean builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_mean with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_mean empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathMedianFindsMiddleValue(t *testing.T) {
	tool, ok := GetBuiltinTool("math_median")
	if !ok {
		t.Fatal("math_median builtin not found")
	}

	cases := []struct {
		numbers []float64
		want    string
	}{
		{[]float64{1, 2, 3}, "2"},
		{[]float64{1, 2, 3, 4}, "2.5"},
		{[]float64{5, 1, 3}, "3"},
		{[]float64{10}, "10"},
	}

	for _, tt := range cases {
		numbersAny := make([]any, len(tt.numbers))
		for i, n := range tt.numbers {
			numbersAny[i] = n
		}
		result, err := tool.Execute(context.Background(), map[string]any{"numbers": numbersAny})
		if err != nil {
			t.Fatalf("math_median(%v) returned error: %v", tt.numbers, err)
		}
		if result.Content != tt.want {
			t.Fatalf("math_median(%v) = %q, want %q", tt.numbers, result.Content, tt.want)
		}
	}
}

func TestMathMedianEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_median")
	if !ok {
		t.Fatal("math_median builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_median with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_median empty args = %q, want %q", result.Content, "0")
	}
}

func TestMathStddevComputesStandardDeviation(t *testing.T) {
	tool, ok := GetBuiltinTool("math_stddev")
	if !ok {
		t.Fatal("math_stddev builtin not found")
	}

	cases := []struct {
		numbers []float64
		want    float64
	}{
		{[]float64{2, 4, 4, 4, 5, 5, 7, 9}, 2.0},
		{[]float64{10}, 0},
	}

	for _, tt := range cases {
		numbersAny := make([]any, len(tt.numbers))
		for i, n := range tt.numbers {
			numbersAny[i] = n
		}
		result, err := tool.Execute(context.Background(), map[string]any{"numbers": numbersAny})
		if err != nil {
			t.Fatalf("math_stddev(%v) returned error: %v", tt.numbers, err)
		}
		got := parseFloat(result.Content)
		if math.Abs(got-tt.want) > 0.0001 {
			t.Fatalf("math_stddev(%v) = %q (parsed %v), want approximately %v", tt.numbers, result.Content, got, tt.want)
		}
	}
}

func TestMathStddevEmptyArgs(t *testing.T) {
	tool, ok := GetBuiltinTool("math_stddev")
	if !ok {
		t.Fatal("math_stddev builtin not found")
	}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("math_stddev with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("math_stddev empty args = %q, want %q", result.Content, "0")
	}
}

func parseFloat(s string) float64 {
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0
	}
	return f
}
