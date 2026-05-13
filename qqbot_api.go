package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"
)

const qqBotTokenURL = "https://bots.qq.com/app/getAppAccessToken"

type QQBotAPI struct {
	baseURL        string
	tokenURL       string
	appID          string
	clientSecret   string
	client         *http.Client
	accessToken    string
	tokenExpiresAt time.Time
	headers        map[string]string
	mu             sync.Mutex
}

func newQQBotAPI(appID, clientSecret string) *QQBotAPI {
	api := &QQBotAPI{
		baseURL:      "https://api.sgroup.qq.com",
		tokenURL:     qqBotTokenURL,
		appID:        appID,
		clientSecret: clientSecret,
		client:       httpClient(30 * time.Second),
		headers: map[string]string{
			"Accept":          "*/*",
			"Accept-Encoding": "gzip, deflate, br",
			"User-Agent":      "PostmanRuntime-ApipostRuntime/1.1.0",
			"Connection":      "keep-alive",
		},
	}
	return api
}

func (q *QQBotAPI) updateAuthHeaders() {
	if q.headers == nil {
		q.headers = map[string]string{}
	}
	q.headers["Authorization"] = "QQBot " + q.accessToken
}

func (q *QQBotAPI) ensureTokenWithContext(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.accessToken == "" || time.Now().After(q.tokenExpiresAt) {
		return q.refreshTokenLocked(ctx)
	}
	return nil
}

func (q *QQBotAPI) refreshTokenLocked(ctx context.Context) error {
	payload := map[string]any{"appId": q.appID, "clientSecret": q.clientSecret}
	tokenURL := q.tokenURL
	if tokenURL == "" {
		tokenURL = qqBotTokenURL
	}
	data, _, err := doJSONRequestWithContext(ctx, http.MethodPost, tokenURL, payload, map[string]string{"Content-Type": "application/json"}, 20*time.Second)
	if err != nil {
		return err
	}
	q.accessToken = asString(data["access_token"])
	expiresIn := asInt(data["expires_in"])
	if q.accessToken == "" || expiresIn <= 0 {
		return fmt.Errorf("invalid access token response")
	}
	q.tokenExpiresAt = time.Now().Add(time.Duration(expiresIn-60) * time.Second)
	q.updateAuthHeaders()
	return nil
}

func (q *QQBotAPI) requestWithContext(ctx context.Context, method, endpoint string, payload any) (map[string]any, []byte, error) {
	if err := q.ensureTokenWithContext(ctx); err != nil {
		logger.Printf("failed to refresh access token: %v", err)
		return nil, nil, err
	}
	headers := map[string]string{}
	for key, value := range q.headers {
		headers[key] = value
	}
	if payload != nil {
		headers["Content-Type"] = "application/json"
	}
	return doJSONRequestWithContext(ctx, method, q.baseURL+endpoint, payload, headers, 20*time.Second)
}

func (q *QQBotAPI) dmsMessagesWithContext(ctx context.Context, channelID, content, msgID string) map[string]any {
	data, raw, err := q.requestWithContext(ctx, http.MethodPost, "/dms/"+channelID+"/messages", map[string]any{
		"content": content,
		"msg_id":  msgID,
	})
	if err != nil {
		logger.Printf("[qqbot.send.text] channel_id=%q msg_id=%q content=%q err=%v", channelID, msgID, truncateForLog(content, 180), err)
		return map[string]any{"error": err.Error()}
	}
	if asInt(data["code"]) != 0 || asString(data["error"]) != "" {
		logger.Printf("[qqbot.send.text] channel_id=%q msg_id=%q content=%q %s", channelID, msgID, truncateForLog(content, 180), apiResultSummary(data, raw))
	}
	return data
}

func (q *QQBotAPI) dmsMessagesPicWithContext(ctx context.Context, channelID string, imageContent []byte, msgID string, content string) map[string]any {
	return q.dmsMessagesPicReaderWithSizeAndContext(ctx, channelID, bytes.NewReader(imageContent), msgID, content, int64(len(imageContent)))
}

func (q *QQBotAPI) dmsMessagesPicReaderWithContext(ctx context.Context, channelID string, imageContent io.Reader, msgID string, content string) map[string]any {
	return q.dmsMessagesPicReaderWithSizeAndContext(ctx, channelID, imageContent, msgID, content, -1)
}

func (q *QQBotAPI) dmsMessagesPicReaderWithSizeAndContext(ctx context.Context, channelID string, imageContent io.Reader, msgID string, content string, imageSize int64) map[string]any {
	if err := q.ensureTokenWithContext(ctx); err != nil {
		logger.Printf("failed to refresh access token: %v", err)
		return map[string]any{"error": err.Error()}
	}

	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)

	req, err := http.NewRequestWithContext(requestContext(ctx), http.MethodPost, q.baseURL+"/dms/"+channelID+"/messages", pipeReader)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return map[string]any{"error": err.Error()}
	}
	for key, value := range q.headers {
		if strings.EqualFold(key, "Content-Type") || strings.EqualFold(key, "Accept-Encoding") {
			continue
		}
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	writeErrCh := make(chan error, 1)
	go func() {
		err := func() error {
			fileWriter, err := writer.CreateFormFile("file_image", "image.png")
			if err != nil {
				return err
			}
			if _, err := io.Copy(fileWriter, imageContent); err != nil {
				return err
			}
			if err := writer.WriteField("content", content); err != nil {
				return err
			}
			if err := writer.WriteField("msg_id", msgID); err != nil {
				return err
			}
			return writer.Close()
		}()
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
		} else {
			_ = pipeWriter.Close()
		}
		writeErrCh <- err
	}()

	resp, err := q.client.Do(req)
	if err != nil {
		_ = pipeReader.CloseWithError(err)
		if writeErr := <-writeErrCh; writeErr != nil {
			logger.Printf("[qqbot.send.pic] channel_id=%q msg_id=%q content=%q image_bytes=%d write_err=%v", channelID, msgID, truncateForLog(content, 120), imageSize, writeErr)
		}
		logger.Printf("[qqbot.send.pic] channel_id=%q msg_id=%q content=%q image_bytes=%d err=%v", channelID, msgID, truncateForLog(content, 120), imageSize, err)
		return map[string]any{"error": err.Error()}
	}
	raw, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	_ = pipeReader.Close()
	if writeErr := <-writeErrCh; writeErr != nil {
		logger.Printf("[qqbot.send.pic] channel_id=%q msg_id=%q content=%q image_bytes=%d err=%v", channelID, msgID, truncateForLog(content, 120), imageSize, writeErr)
		return map[string]any{"error": writeErr.Error(), "status_code": resp.StatusCode}
	}
	if readErr != nil {
		logger.Printf("[qqbot.send.pic] channel_id=%q msg_id=%q content=%q image_bytes=%d status=%d err=%v", channelID, msgID, truncateForLog(content, 120), imageSize, resp.StatusCode, readErr)
		return map[string]any{"error": readErr.Error(), "status_code": resp.StatusCode}
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		logger.Printf("[qqbot.send.pic] channel_id=%q msg_id=%q content=%q image_bytes=%d status=%d err=%v raw=%q", channelID, msgID, truncateForLog(content, 120), imageSize, resp.StatusCode, err, truncateForLog(string(raw), 180))
		return map[string]any{"error": err.Error(), "status_code": resp.StatusCode, "raw": string(raw)}
	}
	if resp.StatusCode >= 400 || asInt(result["code"]) != 0 || asString(result["error"]) != "" {
		logger.Printf("[qqbot.send.pic] channel_id=%q msg_id=%q content=%q image_bytes=%d status=%d %s", channelID, msgID, truncateForLog(content, 120), imageSize, resp.StatusCode, apiResultSummary(result, raw))
	}
	return result
}
