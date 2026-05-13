package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type CLIChatSender struct {
	out io.Writer
}

func NewCLIChatSender(out io.Writer) *CLIChatSender {
	if out == nil {
		out = os.Stdout
	}
	return &CLIChatSender{out: out}
}

func (s *CLIChatSender) SendText(ctx context.Context, msg *MessageContext, content string) map[string]any {
	fmt.Fprintln(s.out, content)
	return map[string]any{"code": 0}
}

func (s *CLIChatSender) SendImage(ctx context.Context, msg *MessageContext, imageContent []byte, content string) map[string]any {
	path, err := writeCLIImage(imageContent)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	if strings.TrimSpace(content) != "" {
		fmt.Fprintln(s.out, content)
	}
	fmt.Fprintf(s.out, "[image] %s\n", path)
	return map[string]any{"code": 0, "path": path}
}

func (s *CLIChatSender) SendImageReader(ctx context.Context, msg *MessageContext, imageContent io.Reader, content string) map[string]any {
	if imageContent == nil {
		return map[string]any{"error": "image reader is nil"}
	}
	data, err := io.ReadAll(imageContent)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return s.SendImage(ctx, msg, data, content)
}

func writeCLIImage(imageContent []byte) (string, error) {
	file, err := os.CreateTemp("", "scorebot-*.png")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if _, err := file.Write(imageContent); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return path, nil
}

func runCLIChat(ctx context.Context) error {
	userID := defaultString(os.Getenv("CLI_USER_ID"), "cli-user")
	userName := defaultString(os.Getenv("CLI_USER_NAME"), "CLI User")
	conversationID := defaultString(os.Getenv("CLI_CONVERSATION_ID"), "cli")
	handler := newCommandHandler(NewCLIChatSender(os.Stdout))

	fmt.Println("CLI chat started. Type /exit to quit.")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	seq := 0
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			return nil
		}
		seq++
		msgCtx := &MessageContext{
			Ctx:       ctx,
			Body:      map[string]any{},
			Platform:  "cli",
			Event:     "CLI_MESSAGE_CREATE",
			ID:        fmt.Sprintf("cli-%d", seq),
			Timestamp: time.Now().Format(time.RFC3339),
			UserID:    userID,
			UserName:  userName,
			GuildID:   conversationID,
			ChannelID: conversationID,
			Seq:       fmt.Sprintf("%d", seq),
			Content:   normalizeContent(line),
		}
		handler.handle(msgCtx)
	}
	return scanner.Err()
}
