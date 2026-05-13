package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
)

func withEmptyFinishedList(t *testing.T) {
	t.Helper()
	finishedListMu.Lock()
	previous := finishedList
	finishedList = map[string]struct{}{}
	finishedListMu.Unlock()
	t.Cleanup(func() {
		finishedListMu.Lock()
		finishedList = previous
		finishedListMu.Unlock()
	})
}

func encodedMNSMessage(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return map[string]any{"messageBody": base64.StdEncoding.EncodeToString(raw)}
}

func directMessageBody(userID, seq, messageID, content string) map[string]any {
	return map[string]any{
		"t": "DIRECT_MESSAGE_CREATE",
		"d": map[string]any{
			"id":         messageID,
			"seq":        seq,
			"guild_id":   "guild-1",
			"channel_id": "channel-1",
			"content":    content,
			"author": map[string]any{
				"id":       userID,
				"username": "tester",
			},
		},
	}
}

func TestHandleMNSBatchProcessesAllDirectMessages(t *testing.T) {
	withEmptyFinishedList(t)

	var dispatched []string
	claim := func(context.Context, *MessageContext) (bool, error) {
		return true, nil
	}
	dispatch := func(ctx *MessageContext) {
		dispatched = append(dispatched, ctx.Content)
	}

	batch := []map[string]any{
		encodedMNSMessage(t, directMessageBody("user-1", "1", "msg-1", "/帮助")),
		{"messageBody": "not-base64"},
		encodedMNSMessage(t, directMessageBody("user-2", "2", "msg-2", "/我的信息")),
	}
	raw, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("marshal batch: %v", err)
	}

	summary, err := handleMNSBatch(context.Background(), raw, claim, dispatch)
	if err != nil {
		t.Fatalf("handleMNSBatch error: %v", err)
	}
	if summary != "ok(total=3 processed=2 duplicate=0 skipped=1 failed=0)" {
		t.Fatalf("summary = %q", summary)
	}
	want := []string{"/帮助", "/我的信息"}
	if !reflect.DeepEqual(dispatched, want) {
		t.Fatalf("dispatched = %#v want %#v", dispatched, want)
	}
}

func TestHandleMNSBatchSkipsDuplicateClaim(t *testing.T) {
	withEmptyFinishedList(t)

	seen := map[string]struct{}{}
	dispatched := 0
	claim := func(_ context.Context, ctx *MessageContext) (bool, error) {
		key := messageDedupKey(ctx)
		if _, ok := seen[key]; ok {
			return false, nil
		}
		seen[key] = struct{}{}
		return true, nil
	}
	dispatch := func(*MessageContext) {
		dispatched++
	}
	message := encodedMNSMessage(t, directMessageBody("user-1", "1", "msg-1", "/帮助"))
	raw, err := json.Marshal([]map[string]any{message})
	if err != nil {
		t.Fatalf("marshal batch: %v", err)
	}

	if summary, err := handleMNSBatch(context.Background(), raw, claim, dispatch); err != nil || summary != "ok(total=1 processed=1 duplicate=0 skipped=0 failed=0)" {
		t.Fatalf("first handle summary=%q err=%v", summary, err)
	}
	withEmptyFinishedList(t)
	if summary, err := handleMNSBatch(context.Background(), raw, claim, dispatch); err != nil || summary != "ok(total=1 processed=0 duplicate=1 skipped=0 failed=0)" {
		t.Fatalf("second handle summary=%q err=%v", summary, err)
	}
	if dispatched != 1 {
		t.Fatalf("dispatched = %d want 1", dispatched)
	}
}
