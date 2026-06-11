package mcp

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"convert_temperature":  &ConvertTemperatureTool{},
		"convert_length":       &ConvertUnitTool{name: "convert_length", desc: "Convert between length units", factors: lengthFactors, defaultFrom: "m", defaultTo: "km"},
		"convert_weight":       &ConvertUnitTool{name: "convert_weight", desc: "Convert between weight units", factors: weightFactors, defaultFrom: "kg", defaultTo: "g"},
		"convert_volume":       &ConvertUnitTool{name: "convert_volume", desc: "Convert between volume units", factors: volumeFactors, defaultFrom: "l", defaultTo: "ml"},
		"convert_area":         &ConvertUnitTool{name: "convert_area", desc: "Convert between area units", factors: areaFactors, defaultFrom: "m2", defaultTo: "km2"},
		"convert_speed":        &ConvertUnitTool{name: "convert_speed", desc: "Convert between speed units", factors: speedFactors, defaultFrom: "mps", defaultTo: "kph"},
		"convert_data_size":    &ConvertUnitTool{name: "convert_data_size", desc: "Convert between data size units", factors: dataSizeFactors, defaultFrom: "b", defaultTo: "kib"},
		"convert_pressure":     &ConvertUnitTool{name: "convert_pressure", desc: "Convert between pressure units", factors: pressureFactors, defaultFrom: "pa", defaultTo: "kpa"},
		"convert_energy":       &ConvertUnitTool{name: "convert_energy", desc: "Convert between energy units", factors: energyFactors, defaultFrom: "j", defaultTo: "kj"},
		"convert_power":        &ConvertUnitTool{name: "convert_power", desc: "Convert between power units", factors: powerFactors, defaultFrom: "w", defaultTo: "kw"},
		"convert_number_base":  &ConvertNumberBaseTool{},
		"convert_roman_to_int": &ConvertRomanToIntTool{},
		"convert_int_to_roman": &ConvertIntToRomanTool{},
		"convert_color":        &ConvertColorTool{},
	}, map[string]bool{
		"convert_temperature":  true,
		"convert_length":       true,
		"convert_weight":       true,
		"convert_volume":       true,
		"convert_area":         true,
		"convert_speed":        true,
		"convert_data_size":    true,
		"convert_pressure":     true,
		"convert_energy":       true,
		"convert_power":        true,
		"convert_number_base":  true,
		"convert_roman_to_int": true,
		"convert_int_to_roman": true,
		"convert_color":        true,
	})
}

func conversionString(args map[string]any, key, fallback string) string {
	if args == nil {
		return fallback
	}
	if v, ok := args[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

var (
	lengthFactors = map[string]float64{
		"m": 1, "km": 1000, "cm": 0.01, "mm": 0.001,
		"in": 0.0254, "ft": 0.3048, "yd": 0.9144, "mi": 1609.344,
	}
	weightFactors = map[string]float64{
		"kg": 1, "g": 0.001, "mg": 1e-6,
		"lb": 0.45359237, "oz": 0.028349523125, "ton": 1000,
	}
	volumeFactors = map[string]float64{
		"l": 1, "ml": 0.001, "gal": 3.785411784, "qt": 0.946352946,
		"pt": 0.473176473, "cup": 0.2365882365, "oz": 0.0295735295625,
	}
	areaFactors = map[string]float64{
		"m2": 1, "km2": 1e6, "cm2": 1e-4, "ft2": 0.09290304,
		"acre": 4046.8564224, "hectare": 10000,
	}
	speedFactors = map[string]float64{
		"mps": 1, "kph": 1.0 / 3.6, "mph": 0.44704, "knot": 463.0 / 900.0,
	}
	dataSizeFactors = map[string]float64{
		"b": 1, "kb": 1e3, "mb": 1e6, "gb": 1e9, "tb": 1e12,
		"kib": 1024, "mib": 1024 * 1024, "gib": 1024 * 1024 * 1024, "tib": 1024 * 1024 * 1024 * 1024,
	}
	pressureFactors = map[string]float64{
		"pa": 1, "kpa": 1000, "bar": 1e5, "psi": 6894.757293168,
		"atm": 101325, "mmhg": 133.322387415,
	}
	energyFactors = map[string]float64{
		"j": 1, "kj": 1000, "cal": 4.184, "kcal": 4184,
		"wh": 3600, "kwh": 3.6e6,
	}
	powerFactors = map[string]float64{
		"w": 1, "kw": 1000, "mw": 1e6, "hp": 745.6998715822702,
	}
)

func supportedUnits(factors map[string]float64) string {
	units := make([]string, 0, len(factors))
	for unit := range factors {
		units = append(units, unit)
	}
	sort.Strings(units)
	return strings.Join(units, ", ")
}

// ConvertUnitTool is a generic factor-based unit converter shared by the
// length/weight/volume/area/speed/data-size/pressure/energy/power tools.
type ConvertUnitTool struct {
	name        string
	desc        string
	factors     map[string]float64
	defaultFrom string
	defaultTo   string
}

func (t *ConvertUnitTool) Name() string        { return t.name }
func (t *ConvertUnitTool) Description() string { return t.desc }
func (t *ConvertUnitTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{"type": "number", "description": "Value to convert", "default": 0},
			"from":  map[string]any{"type": "string", "description": "Source unit: " + supportedUnits(t.factors), "default": t.defaultFrom},
			"to":    map[string]any{"type": "string", "description": "Target unit: " + supportedUnits(t.factors), "default": t.defaultTo},
		},
	}
}
func (t *ConvertUnitTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	value := getFloat(args, "value", 0)
	from := strings.ToLower(conversionString(args, "from", t.defaultFrom))
	to := strings.ToLower(conversionString(args, "to", t.defaultTo))
	fromFactor, ok := t.factors[from]
	if !ok {
		return &ToolResult{Content: fmt.Sprintf("unsupported source unit %q; supported units: %s", from, supportedUnits(t.factors)), IsError: true}, nil
	}
	toFactor, ok := t.factors[to]
	if !ok {
		return &ToolResult{Content: fmt.Sprintf("unsupported target unit %q; supported units: %s", to, supportedUnits(t.factors)), IsError: true}, nil
	}
	converted := value * fromFactor / toFactor
	return &ToolResult{Content: fmt.Sprintf("%g %s", converted, to)}, nil
}

// ConvertTemperatureTool converts between Celsius, Fahrenheit and Kelvin.
type ConvertTemperatureTool struct{}

func (t *ConvertTemperatureTool) Name() string        { return "convert_temperature" }
func (t *ConvertTemperatureTool) Description() string { return "Convert between temperature units" }
func (t *ConvertTemperatureTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{"type": "number", "description": "Temperature value", "default": 0},
			"from":  map[string]any{"type": "string", "description": "Source unit: C, F or K", "default": "C"},
			"to":    map[string]any{"type": "string", "description": "Target unit: C, F or K", "default": "F"},
		},
	}
}
func (t *ConvertTemperatureTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	value := getFloat(args, "value", 0)
	from := strings.ToUpper(conversionString(args, "from", "C"))
	to := strings.ToUpper(conversionString(args, "to", "F"))

	var celsius float64
	switch from {
	case "C":
		celsius = value
	case "F":
		celsius = (value - 32) * 5 / 9
	case "K":
		celsius = value - 273.15
	default:
		return &ToolResult{Content: fmt.Sprintf("unsupported source unit %q; supported units: C, F, K", from), IsError: true}, nil
	}

	var converted float64
	switch to {
	case "C":
		converted = celsius
	case "F":
		converted = celsius*9/5 + 32
	case "K":
		converted = celsius + 273.15
	default:
		return &ToolResult{Content: fmt.Sprintf("unsupported target unit %q; supported units: C, F, K", to), IsError: true}, nil
	}

	rounded := math.Round(converted*1e9) / 1e9
	return &ToolResult{Content: fmt.Sprintf("%g %s", rounded, to)}, nil
}

// ConvertNumberBaseTool converts integer strings between bases 2-36.
type ConvertNumberBaseTool struct{}

func (t *ConvertNumberBaseTool) Name() string        { return "convert_number_base" }
func (t *ConvertNumberBaseTool) Description() string { return "Convert a number between bases 2-36" }
func (t *ConvertNumberBaseTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number":    map[string]any{"type": "string", "description": "Number to convert", "default": "0"},
			"from_base": map[string]any{"type": "integer", "description": "Source base (2-36)", "default": 10},
			"to_base":   map[string]any{"type": "integer", "description": "Target base (2-36)", "default": 2},
		},
	}
}
func (t *ConvertNumberBaseTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	number := conversionString(args, "number", "0")
	fromBase := getInt(args, "from_base", 10)
	toBase := getInt(args, "to_base", 2)
	if fromBase < 2 || fromBase > 36 || toBase < 2 || toBase > 36 {
		return &ToolResult{Content: "from_base and to_base must be between 2 and 36", IsError: true}, nil
	}
	parsed, err := strconv.ParseInt(strings.ToLower(number), fromBase, 64)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("cannot parse %q in base %d", number, fromBase), IsError: true}, nil
	}
	return &ToolResult{Content: strconv.FormatInt(parsed, toBase)}, nil
}

var romanSymbols = []struct {
	value  int
	symbol string
}{
	{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"},
	{100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"},
	{10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
}

var romanValues = map[byte]int{
	'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000,
}

// ConvertRomanToIntTool parses a Roman numeral into an integer.
type ConvertRomanToIntTool struct{}

func (t *ConvertRomanToIntTool) Name() string        { return "convert_roman_to_int" }
func (t *ConvertRomanToIntTool) Description() string { return "Convert a Roman numeral to an integer" }
func (t *ConvertRomanToIntTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"roman": map[string]any{"type": "string", "description": "Roman numeral (I, V, X, L, C, D, M)", "default": "I"},
		},
	}
}
func (t *ConvertRomanToIntTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	roman := strings.ToUpper(conversionString(args, "roman", "I"))
	total := 0
	for i := 0; i < len(roman); i++ {
		value, ok := romanValues[roman[i]]
		if !ok {
			return &ToolResult{Content: fmt.Sprintf("invalid Roman numeral character %q", string(roman[i])), IsError: true}, nil
		}
		if i+1 < len(roman) && romanValues[roman[i+1]] > value {
			total -= value
			continue
		}
		total += value
	}
	if total < 1 || total > 3999 {
		return &ToolResult{Content: fmt.Sprintf("Roman numeral %q is out of supported range 1-3999", roman), IsError: true}, nil
	}
	if canonical := intToRoman(total); canonical != roman {
		return &ToolResult{Content: fmt.Sprintf("invalid Roman numeral %q (canonical form for %d is %s)", roman, total, canonical), IsError: true}, nil
	}
	return &ToolResult{Content: strconv.Itoa(total)}, nil
}

func intToRoman(number int) string {
	var builder strings.Builder
	remaining := number
	for _, entry := range romanSymbols {
		for remaining >= entry.value {
			builder.WriteString(entry.symbol)
			remaining -= entry.value
		}
	}
	return builder.String()
}

// ConvertIntToRomanTool renders an integer 1-3999 as a Roman numeral.
type ConvertIntToRomanTool struct{}

func (t *ConvertIntToRomanTool) Name() string { return "convert_int_to_roman" }
func (t *ConvertIntToRomanTool) Description() string {
	return "Convert an integer (1-3999) to a Roman numeral"
}
func (t *ConvertIntToRomanTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number": map[string]any{"type": "integer", "description": "Integer between 1 and 3999", "default": 1},
		},
	}
}
func (t *ConvertIntToRomanTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	number := getInt(args, "number", 1)
	if number < 1 || number > 3999 {
		return &ToolResult{Content: "number must be between 1 and 3999", IsError: true}, nil
	}
	return &ToolResult{Content: intToRoman(number)}, nil
}

// ConvertColorTool converts colors between hex, rgb and hsl notations.
type ConvertColorTool struct{}

func (t *ConvertColorTool) Name() string { return "convert_color" }
func (t *ConvertColorTool) Description() string {
	return "Convert a color between hex, rgb and hsl formats"
}
func (t *ConvertColorTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"color": map[string]any{"type": "string", "description": "Color value, e.g. #ff8800, rgb(255, 136, 0) or hsl(32, 100%, 50%)", "default": "#000000"},
			"from":  map[string]any{"type": "string", "description": "Source format: hex, rgb or hsl", "default": "hex"},
			"to":    map[string]any{"type": "string", "description": "Target format: hex, rgb or hsl", "default": "rgb"},
		},
	}
}
func (t *ConvertColorTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	color := conversionString(args, "color", "#000000")
	from := strings.ToLower(conversionString(args, "from", "hex"))
	to := strings.ToLower(conversionString(args, "to", "rgb"))

	var r, g, b int
	var err error
	switch from {
	case "hex":
		r, g, b, err = parseHexColor(color)
	case "rgb":
		r, g, b, err = parseRGBColor(color)
	case "hsl":
		r, g, b, err = parseHSLColor(color)
	default:
		return &ToolResult{Content: fmt.Sprintf("unsupported source format %q; supported formats: hex, rgb, hsl", from), IsError: true}, nil
	}
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}

	switch to {
	case "hex":
		return &ToolResult{Content: fmt.Sprintf("#%02x%02x%02x", r, g, b)}, nil
	case "rgb":
		return &ToolResult{Content: fmt.Sprintf("rgb(%d, %d, %d)", r, g, b)}, nil
	case "hsl":
		h, s, l := rgbToHSL(r, g, b)
		return &ToolResult{Content: fmt.Sprintf("hsl(%d, %d%%, %d%%)", h, s, l)}, nil
	default:
		return &ToolResult{Content: fmt.Sprintf("unsupported target format %q; supported formats: hex, rgb, hsl", to), IsError: true}, nil
	}
}

func parseHexColor(color string) (int, int, int, error) {
	hex := strings.TrimPrefix(strings.TrimSpace(color), "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color %q", color)
	}
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid hex color %q", color)
	}
	return int(value >> 16 & 0xff), int(value >> 8 & 0xff), int(value & 0xff), nil
}

func parseColorTriple(color, prefix string) ([]string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(color))
	if !strings.HasPrefix(trimmed, prefix+"(") || !strings.HasSuffix(trimmed, ")") {
		return nil, fmt.Errorf("invalid %s color %q", prefix, color)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(trimmed, prefix+"("), ")")
	parts := strings.Split(inner, ",")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid %s color %q", prefix, color)
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts, nil
}

func parseRGBColor(color string) (int, int, int, error) {
	parts, err := parseColorTriple(color, "rgb")
	if err != nil {
		return 0, 0, 0, err
	}
	channels := make([]int, 3)
	for i, part := range parts {
		channel, convErr := strconv.Atoi(part)
		if convErr != nil || channel < 0 || channel > 255 {
			return 0, 0, 0, fmt.Errorf("invalid rgb channel %q", part)
		}
		channels[i] = channel
	}
	return channels[0], channels[1], channels[2], nil
}

func parseHSLColor(color string) (int, int, int, error) {
	parts, err := parseColorTriple(color, "hsl")
	if err != nil {
		return 0, 0, 0, err
	}
	h, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid hsl hue %q", parts[0])
	}
	s, err := strconv.ParseFloat(strings.TrimSuffix(parts[1], "%"), 64)
	if err != nil || s < 0 || s > 100 {
		return 0, 0, 0, fmt.Errorf("invalid hsl saturation %q", parts[1])
	}
	l, err := strconv.ParseFloat(strings.TrimSuffix(parts[2], "%"), 64)
	if err != nil || l < 0 || l > 100 {
		return 0, 0, 0, fmt.Errorf("invalid hsl lightness %q", parts[2])
	}
	r, g, b := hslToRGB(math.Mod(math.Mod(h, 360)+360, 360), s/100, l/100)
	return r, g, b, nil
}

func hslToRGB(h, s, l float64) (int, int, int) {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	round := func(v float64) int { return int(math.Round((v + m) * 255)) }
	return round(r), round(g), round(b)
}

func rgbToHSL(r, g, b int) (int, int, int) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	maxC := math.Max(rf, math.Max(gf, bf))
	minC := math.Min(rf, math.Min(gf, bf))
	l := (maxC + minC) / 2
	if maxC == minC {
		return 0, 0, int(math.Round(l * 100))
	}
	d := maxC - minC
	var s float64
	if l > 0.5 {
		s = d / (2 - maxC - minC)
	} else {
		s = d / (maxC + minC)
	}
	var h float64
	switch maxC {
	case rf:
		h = math.Mod((gf-bf)/d, 6)
	case gf:
		h = (bf-rf)/d + 2
	default:
		h = (rf-gf)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return int(math.Round(h)), int(math.Round(s * 100)), int(math.Round(l * 100))
}
