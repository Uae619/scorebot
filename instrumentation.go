package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func truncateForLog(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}

type logFormatOptions struct {
	noTruncate bool
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 6 {
		return fmt.Sprintf("len=%d", len(value))
	}
	return fmt.Sprintf("len=%d tail=%s", len(value), value[len(value)-4:])
}

func sanitizeLogValueWithOptions(key string, value any, options logFormatOptions) string {
	lowerKey := strings.ToLower(key)
	switch v := value.(type) {
	case string:
		switch {
		case strings.Contains(lowerKey, "token"),
			strings.Contains(lowerKey, "password"),
			strings.Contains(lowerKey, "secret"),
			strings.Contains(lowerKey, "cookie"),
			strings.HasSuffix(lowerKey, "pw"):
			return maskSecret(v)
		case strings.Contains(lowerKey, "content"),
			strings.Contains(lowerKey, "raw"),
			strings.Contains(lowerKey, "msg"),
			strings.Contains(lowerKey, "url"),
			strings.Contains(lowerKey, "response"):
			if options.noTruncate {
				return fmt.Sprintf("%q", strings.TrimSpace(v))
			}
			return fmt.Sprintf("%q", truncateForLog(v, 180))
		default:
			if options.noTruncate {
				return fmt.Sprintf("%q", strings.TrimSpace(v))
			}
			return fmt.Sprintf("%q", truncateForLog(v, 120))
		}
	case bool:
		if v {
			return "true"
		}
		return "false"
	case nil:
		return `""`
	default:
		return fmt.Sprint(v)
	}
}

func formatLogFieldsWithOptions(fields map[string]any, options logFormatOptions) string {
	if len(fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, sanitizeLogValueWithOptions(key, fields[key], options)))
	}
	return strings.Join(parts, " ")
}

func formatLogFields(fields map[string]any) string {
	return formatLogFieldsWithOptions(fields, logFormatOptions{})
}

func logCommandTrace(ctx *MessageContext, stage string, fields map[string]any) {
	base := map[string]any{
		"stage": stage,
	}
	if ctx != nil {
		base["event"] = ctx.Event
		base["guild_id"] = ctx.GuildID
		base["message_id"] = ctx.ID
		base["seq"] = ctx.Seq
		base["user_id"] = ctx.UserID
	}
	for key, value := range fields {
		base[key] = value
	}
	logger.Printf("[trace] %s", formatLogFields(base))
}

func isAdminTraceContext(ctx *MessageContext) bool {
	return false
}

func marshalLogJSON(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func logAdminThirdPartyRequest(ctx *MessageContext, provider, method, route string, requestBody, responseBody any, extra map[string]any) {
	if !isAdminTraceContext(ctx) {
		return
	}
	fields := map[string]any{
		"provider":      provider,
		"method":        method,
		"route":         route,
		"request_body":  marshalLogJSON(requestBody),
		"response_body": marshalLogJSON(responseBody),
	}
	for key, value := range extra {
		fields[key] = value
	}
	base := map[string]any{
		"stage": "third_party.request",
	}
	if ctx != nil {
		base["event"] = ctx.Event
		base["guild_id"] = ctx.GuildID
		base["message_id"] = ctx.ID
		base["seq"] = ctx.Seq
		base["user_id"] = ctx.UserID
	}
	for key, value := range fields {
		base[key] = value
	}
	logger.Printf("[trace] %s", formatLogFieldsWithOptions(base, logFormatOptions{noTruncate: true}))
}

func apiResultSummary(result map[string]any, raw []byte) string {
	fields := map[string]any{
		"code":  asInt(result["code"]),
		"error": asString(result["error"]),
		"id":    asString(result["id"]),
		"msg":   asString(result["message"]),
		"raw":   string(raw),
	}
	return formatLogFields(fields)
}

func mapKeysJoined(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}
