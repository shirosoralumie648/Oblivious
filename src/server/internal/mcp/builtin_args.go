package mcp

import "strconv"

// Shared argument-coercion helpers for builtin tool category files
// (builtin_<category>.go). Tools accept loosely-typed JSON arguments, so these
// helpers normalize common encodings (float64, json.Number-style strings,
// ints) and fall back to a caller-provided default when the argument is
// missing or not coercible. Defaults keep default-commercial-enabled tools
// functional with empty args per TestDefaultEnabledBuiltinsDoNotReturnPlaceholderOutput.

func getFloat(args map[string]any, key string, fallback float64) float64 {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func getInt(args map[string]any, key string, fallback int) int {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func getFloatSlice(args map[string]any, key string, fallback []float64) []float64 {
	if args == nil {
		return fallback
	}
	raw, ok := args[key].([]any)
	if !ok {
		return fallback
	}
	numbers := make([]float64, 0, len(raw))
	for _, item := range raw {
		switch v := item.(type) {
		case float64:
			numbers = append(numbers, v)
		case float32:
			numbers = append(numbers, float64(v))
		case int:
			numbers = append(numbers, float64(v))
		case int64:
			numbers = append(numbers, float64(v))
		case string:
			parsed, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fallback
			}
			numbers = append(numbers, parsed)
		default:
			return fallback
		}
	}
	if len(numbers) == 0 {
		return fallback
	}
	return numbers
}
