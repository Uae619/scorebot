package main

import (
	"context"
	"strings"
	"unicode"
)

type MessageContext struct {
	Ctx       context.Context
	Body      map[string]any
	Platform  string
	Event     string
	ID        string
	Timestamp string
	UserID    string
	UserName  string
	GuildID   string
	ChannelID string
	Seq       string
	Content   string
}

func newMessageContext(ctx context.Context, body map[string]any) MessageContext {
	if ctx == nil {
		ctx = context.Background()
	}
	d := asMap(body["d"])
	author := asMap(d["author"])
	return MessageContext{
		Ctx:       ctx,
		Body:      body,
		Platform:  "qq",
		Event:     asString(body["t"]),
		ID:        asString(d["id"]),
		Timestamp: asString(d["timestamp"]),
		UserID:    asString(author["id"]),
		UserName:  asString(author["username"]),
		GuildID:   asString(d["guild_id"]),
		ChannelID: asString(d["channel_id"]),
		Seq:       asString(d["seq"]),
		Content:   normalizeContent(asString(d["content"])),
	}
}

func (ctx *MessageContext) ChatMessage() ChatMessage {
	if ctx == nil {
		return ChatMessage{}
	}
	return ChatMessage{
		Platform:       defaultString(ctx.Platform, "qq"),
		UserID:         ctx.UserID,
		UserName:       ctx.UserName,
		ConversationID: ctx.GuildID,
		MessageID:      ctx.ID,
		Sequence:       ctx.Seq,
		Content:        ctx.Content,
		Raw:            ctx.Body,
	}
}

func messageContext(ctx *MessageContext) context.Context {
	if ctx == nil || ctx.Ctx == nil {
		return context.Background()
	}
	return ctx.Ctx
}

func normalizeContent(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "／") {
		content = "/" + strings.TrimPrefix(content, "／")
	}
	if strings.HasPrefix(content, "/") {
		content = "/" + strings.TrimLeftFunc(content[1:], unicode.IsSpace)
	}
	replacements := map[string]string{
		"|":     "",
		"～":     "",
		"。":     ".",
		"&amp;": "&",
		"&lt;":  "<",
		"&gt;":  ">",
		"？":     "?",
		"！":     "!",
	}
	for old, newValue := range replacements {
		content = strings.ReplaceAll(content, old, newValue)
	}
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}
	return content
}
