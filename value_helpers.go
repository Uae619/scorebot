package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func asString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case json.Number:
		return x.String()
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case bool:
		if x {
			return "TRUE"
		}
		return "FALSE"
	default:
		return fmt.Sprint(v)
	}
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(x))
		return i
	default:
		return 0
	}
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func formatValue(v any) string {
	s := strings.TrimSpace(asString(v))
	if s == "<nil>" {
		return ""
	}
	return s
}

func joinExamID(s string) string {
	if len(s) <= 7 {
		return s
	}
	parts := make([]string, 0, (len(s)+6)/7)
	for len(s) > 7 {
		parts = append(parts, s[:7])
		s = s[7:]
	}
	if s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "|")
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
