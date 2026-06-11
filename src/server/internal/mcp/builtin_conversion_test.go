package mcp

import (
	"context"
	"strings"
	"testing"
)

func executeConversionTool(t *testing.T, name string, args map[string]any) *ToolResult {
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

func TestConversionToolsRegisteredAndDefaultEnabled(t *testing.T) {
	names := []string{
		"convert_temperature", "convert_length", "convert_weight", "convert_volume",
		"convert_area", "convert_speed", "convert_data_size", "convert_pressure",
		"convert_energy", "convert_power", "convert_number_base",
		"convert_roman_to_int", "convert_int_to_roman", "convert_color",
	}
	for _, name := range names {
		if _, ok := GetBuiltinTool(name); !ok {
			t.Fatalf("expected %s to be registered", name)
		}
		if !IsDefaultCommercialBuiltin(name) {
			t.Fatalf("expected %s to be default commercial enabled", name)
		}
	}
}

func TestConversionUnitToolsHappyPath(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
		want string
	}{
		{"convert_temperature", map[string]any{"value": 100.0, "from": "C", "to": "F"}, "212 F"},
		{"convert_temperature", map[string]any{"value": 0.0, "from": "C", "to": "K"}, "273.15 K"},
		{"convert_length", map[string]any{"value": 5000.0, "from": "m", "to": "km"}, "5 km"},
		{"convert_length", map[string]any{"value": 1.0, "from": "in", "to": "cm"}, "2.54 cm"},
		{"convert_weight", map[string]any{"value": 2.0, "from": "kg", "to": "g"}, "2000 g"},
		{"convert_volume", map[string]any{"value": 3.0, "from": "l", "to": "ml"}, "3000 ml"},
		{"convert_area", map[string]any{"value": 1.0, "from": "hectare", "to": "m2"}, "10000 m2"},
		{"convert_speed", map[string]any{"value": 36.0, "from": "kph", "to": "mps"}, "10 mps"},
		{"convert_data_size", map[string]any{"value": 2048.0, "from": "b", "to": "kib"}, "2 kib"},
		{"convert_pressure", map[string]any{"value": 1.0, "from": "atm", "to": "pa"}, "101325 pa"},
		{"convert_energy", map[string]any{"value": 1.0, "from": "kcal", "to": "cal"}, "1000 cal"},
		{"convert_power", map[string]any{"value": 3.0, "from": "kw", "to": "w"}, "3000 w"},
	}
	for _, tc := range cases {
		result := executeConversionTool(t, tc.tool, tc.args)
		if result.IsError {
			t.Fatalf("%s(%v) returned tool error: %s", tc.tool, tc.args, result.Content)
		}
		if result.Content != tc.want {
			t.Fatalf("%s(%v) = %q, want %q", tc.tool, tc.args, result.Content, tc.want)
		}
	}
}

func TestConversionUnitToolsRejectUnknownUnits(t *testing.T) {
	result := executeConversionTool(t, "convert_length", map[string]any{"value": 1.0, "from": "parsec", "to": "m"})
	if !result.IsError {
		t.Fatalf("expected unsupported source unit error, got %q", result.Content)
	}
	result = executeConversionTool(t, "convert_temperature", map[string]any{"value": 1.0, "from": "C", "to": "R"})
	if !result.IsError {
		t.Fatalf("expected unsupported target unit error, got %q", result.Content)
	}
}

func TestConvertNumberBase(t *testing.T) {
	result := executeConversionTool(t, "convert_number_base", map[string]any{"number": "255", "from_base": 10, "to_base": 16})
	if result.Content != "ff" {
		t.Fatalf("convert_number_base(255, 10->16) = %q, want %q", result.Content, "ff")
	}
	result = executeConversionTool(t, "convert_number_base", map[string]any{"number": "ff", "from_base": 16, "to_base": 2})
	if result.Content != "11111111" {
		t.Fatalf("convert_number_base(ff, 16->2) = %q, want %q", result.Content, "11111111")
	}
	result = executeConversionTool(t, "convert_number_base", map[string]any{"number": "zz", "from_base": 10, "to_base": 2})
	if !result.IsError {
		t.Fatalf("expected parse error for invalid digits, got %q", result.Content)
	}
	result = executeConversionTool(t, "convert_number_base", map[string]any{"number": "1", "from_base": 1, "to_base": 10})
	if !result.IsError {
		t.Fatalf("expected base range error, got %q", result.Content)
	}
}

func TestConvertRomanRoundTrip(t *testing.T) {
	result := executeConversionTool(t, "convert_roman_to_int", map[string]any{"roman": "MCMXCIV"})
	if result.Content != "1994" {
		t.Fatalf("convert_roman_to_int(MCMXCIV) = %q, want 1994", result.Content)
	}
	result = executeConversionTool(t, "convert_int_to_roman", map[string]any{"number": 1994})
	if result.Content != "MCMXCIV" {
		t.Fatalf("convert_int_to_roman(1994) = %q, want MCMXCIV", result.Content)
	}
	result = executeConversionTool(t, "convert_roman_to_int", map[string]any{"roman": "IIII"})
	if !result.IsError {
		t.Fatalf("expected non-canonical numeral rejection, got %q", result.Content)
	}
	result = executeConversionTool(t, "convert_roman_to_int", map[string]any{"roman": "ABC"})
	if !result.IsError {
		t.Fatalf("expected invalid character rejection, got %q", result.Content)
	}
	result = executeConversionTool(t, "convert_int_to_roman", map[string]any{"number": 4000})
	if !result.IsError {
		t.Fatalf("expected out-of-range rejection, got %q", result.Content)
	}
}

func TestConvertColor(t *testing.T) {
	cases := []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"color": "#ff8800", "from": "hex", "to": "rgb"}, "rgb(255, 136, 0)"},
		{map[string]any{"color": "rgb(255, 136, 0)", "from": "rgb", "to": "hex"}, "#ff8800"},
		{map[string]any{"color": "#fff", "from": "hex", "to": "rgb"}, "rgb(255, 255, 255)"},
		{map[string]any{"color": "hsl(0, 100%, 50%)", "from": "hsl", "to": "hex"}, "#ff0000"},
		{map[string]any{"color": "rgb(255, 0, 0)", "from": "rgb", "to": "hsl"}, "hsl(0, 100%, 50%)"},
	}
	for _, tc := range cases {
		result := executeConversionTool(t, "convert_color", tc.args)
		if result.IsError {
			t.Fatalf("convert_color(%v) returned tool error: %s", tc.args, result.Content)
		}
		if result.Content != tc.want {
			t.Fatalf("convert_color(%v) = %q, want %q", tc.args, result.Content, tc.want)
		}
	}

	result := executeConversionTool(t, "convert_color", map[string]any{"color": "#xyz", "from": "hex", "to": "rgb"})
	if !result.IsError {
		t.Fatalf("expected invalid hex rejection, got %q", result.Content)
	}
	result = executeConversionTool(t, "convert_color", map[string]any{"color": "rgb(999, 0, 0)", "from": "rgb", "to": "hex"})
	if !result.IsError {
		t.Fatalf("expected invalid channel rejection, got %q", result.Content)
	}
}

func TestConversionToolsSucceedWithEmptyArgs(t *testing.T) {
	names := []string{
		"convert_temperature", "convert_length", "convert_weight", "convert_volume",
		"convert_area", "convert_speed", "convert_data_size", "convert_pressure",
		"convert_energy", "convert_power", "convert_number_base",
		"convert_roman_to_int", "convert_int_to_roman", "convert_color",
	}
	for _, name := range names {
		result := executeConversionTool(t, name, map[string]any{})
		if result.IsError {
			t.Fatalf("%s with empty args returned tool error: %s", name, result.Content)
		}
		if strings.Contains(strings.ToLower(result.Content), "placeholder") {
			t.Fatalf("%s with empty args returned placeholder output: %q", name, result.Content)
		}
	}
}
