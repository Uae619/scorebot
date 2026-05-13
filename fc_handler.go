package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

type messageClaimFunc func(context.Context, *MessageContext) (bool, error)
type messageDispatchFunc func(*MessageContext)

type mnsBatchStats struct {
	total     int
	processed int
	duplicate int
	skipped   int
	failed    int
}

func (s mnsBatchStats) summary() string {
	return fmt.Sprintf("ok(total=%d processed=%d duplicate=%d skipped=%d failed=%d)", s.total, s.processed, s.duplicate, s.skipped, s.failed)
}

var (
	claimMessageForProcessing messageClaimFunc    = opTryClaimMessage
	dispatchDirectMessage     messageDispatchFunc = func(ctx *MessageContext) {
		newCommandHandler(chatSender).handle(ctx)
	}
)

func HandleRequest(ctx context.Context, event []byte) (string, error) {
	return handleMNSBatch(ctx, event, claimMessageForProcessing, dispatchDirectMessage)
}

func handleMNSBatch(ctx context.Context, event []byte, claim messageClaimFunc, dispatch messageDispatchFunc) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var mnsBatch []map[string]any
	if err := json.Unmarshal(event, &mnsBatch); err != nil {
		logger.Printf("Body json load error: %v. Raw event: %s", err, string(event))
		return "bad event json", err
	}
	if len(mnsBatch) == 0 {
		logger.Print("收到空MNS消息，跳过")
		return "ok(empty)", nil
	}

	stats := mnsBatchStats{total: len(mnsBatch)}
	var batchErr error
	for index, item := range mnsBatch {
		status, err := handleMNSRecord(ctx, index, item, claim, dispatch)
		switch status {
		case "processed":
			stats.processed++
		case "duplicate":
			stats.duplicate++
		case "failed":
			stats.failed++
		default:
			stats.skipped++
		}
		if err != nil {
			batchErr = errors.Join(batchErr, err)
		}
	}
	if batchErr != nil {
		return stats.summary(), batchErr
	}
	return stats.summary(), nil
}

func handleMNSRecord(ctx context.Context, index int, mnsMessage map[string]any, claim messageClaimFunc, dispatch messageDispatchFunc) (string, error) {
	messageBodyBase64 := asString(mnsMessage["messageBody"])
	if messageBodyBase64 == "" {
		logger.Printf("第%d条MNS消息没有messageBody: %+v", index, mnsMessage)
		return "skipped", nil
	}

	messageBodyBytes, err := base64.StdEncoding.DecodeString(messageBodyBase64)
	if err != nil {
		logger.Printf("第%d条MNS消息base64 decode error: %v", index, err)
		return "skipped", nil
	}
	var bodyJSON map[string]any
	if err := json.Unmarshal(messageBodyBytes, &bodyJSON); err != nil {
		logger.Printf("第%d条MNS消息body decode error: %v", index, err)
		return "skipped", nil
	}

	msgCtx := newMessageContext(ctx, bodyJSON)
	if msgCtx.Event != "DIRECT_MESSAGE_CREATE" {
		return "skipped", nil
	}

	uniqueID := messageDedupKey(&msgCtx)
	if uniqueID == "" {
		logger.Printf("第%d条MNS消息缺少可用于去重的消息标识: %+v", index, bodyJSON)
		return "skipped", nil
	}
	if isLocalMessageFinished(uniqueID) {
		logger.Printf("Return了一条重复推送的消息: %s", uniqueID)
		return "duplicate", nil
	}

	if claim == nil {
		claim = opTryClaimMessage
	}
	claimed, err := claim(ctx, &msgCtx)
	if err != nil {
		logger.Printf("消息去重占位失败: %s err=%v", uniqueID, err)
		return "failed", err
	}
	if !claimed {
		markLocalMessageFinished(uniqueID)
		logger.Printf("Return了一条跨实例重复推送的消息: %s", uniqueID)
		return "duplicate", nil
	}
	markLocalMessageFinished(uniqueID)

	logger.Printf("收到频道私信消息: %s(%s|%s): %s", msgCtx.UserName, msgCtx.UserID, msgCtx.Seq, msgCtx.Content)
	if dispatch == nil {
		dispatch = dispatchDirectMessage
	}
	dispatch(&msgCtx)
	return "processed", nil
}

func messageDedupKey(ctx *MessageContext) string {
	if ctx == nil {
		return ""
	}
	if ctx.UserID != "" && ctx.Seq != "" {
		return ctx.UserID + "_" + ctx.Seq
	}
	if ctx.ID != "" {
		return "message_" + ctx.ID
	}
	return ""
}

func isLocalMessageFinished(uniqueID string) bool {
	finishedListMu.Lock()
	defer finishedListMu.Unlock()
	_, exists := finishedList[uniqueID]
	return exists
}

func markLocalMessageFinished(uniqueID string) {
	if uniqueID == "" {
		return
	}
	finishedListMu.Lock()
	finishedList[uniqueID] = struct{}{}
	if len(finishedList) > 10000 {
		for key := range finishedList {
			delete(finishedList, key)
			break
		}
	}
	finishedListMu.Unlock()
}
