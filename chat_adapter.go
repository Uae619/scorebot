package main

import (
	"context"
	"io"
)

type ChatMessage struct {
	Platform       string
	UserID         string
	UserName       string
	ConversationID string
	MessageID      string
	Sequence       string
	Content        string
	Raw            map[string]any
}

type ChatSender interface {
	SendText(ctx context.Context, msg *MessageContext, content string) map[string]any
	SendImage(ctx context.Context, msg *MessageContext, imageContent []byte, content string) map[string]any
	SendImageReader(ctx context.Context, msg *MessageContext, imageContent io.Reader, content string) map[string]any
}

type QQChatSender struct {
	api *QQBotAPI
}

func NewQQChatSender(api *QQBotAPI) *QQChatSender {
	return &QQChatSender{api: api}
}

func (s *QQChatSender) SendText(ctx context.Context, msg *MessageContext, content string) map[string]any {
	if s == nil || s.api == nil {
		return map[string]any{"error": "chat sender is not configured"}
	}
	return s.api.dmsMessagesWithContext(ctx, msg.GuildID, content, msg.ID)
}

func (s *QQChatSender) SendImage(ctx context.Context, msg *MessageContext, imageContent []byte, content string) map[string]any {
	if s == nil || s.api == nil {
		return map[string]any{"error": "chat sender is not configured"}
	}
	return s.api.dmsMessagesPicWithContext(ctx, msg.GuildID, imageContent, msg.ID, content)
}

func (s *QQChatSender) SendImageReader(ctx context.Context, msg *MessageContext, imageContent io.Reader, content string) map[string]any {
	if s == nil || s.api == nil {
		return map[string]any{"error": "chat sender is not configured"}
	}
	return s.api.dmsMessagesPicReaderWithContext(ctx, msg.GuildID, imageContent, msg.ID, content)
}
