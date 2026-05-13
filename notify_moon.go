package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

func moonSendMsg(groupID, msg string) string {
	return moonSendMsgWithContext(context.Background(), groupID, msg)
}

func moonSendMsgWithContext(ctx context.Context, groupID, msg string) string {
	payload := map[string]any{
		"group_id": groupID,
		"message": []map[string]any{
			{
				"type": "text",
				"data": map[string]any{"text": msg},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	text, _, err := doTextRequestWithContext(ctx, http.MethodPost, appConfig.MoonEndpoint, bytes.NewReader(data), map[string]string{
		"Content-Type":  "application/json",
		"authorization": "Bearer " + appConfig.MoonBearerToken,
	}, 20*time.Second)
	if err != nil {
		return err.Error()
	}
	return text
}
