package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSendAnswerSheetImagesStreamsSuccessfulImagesInOrder(t *testing.T) {
	var uploads []string
	var captions []string
	var textMessages []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/img/1":
			w.Write([]byte("one"))
		case "/img/2":
			w.Write([]byte("two"))
		case "/img/bad":
			http.Error(w, "bad image", http.StatusInternalServerError)
		case "/img/empty":
			w.WriteHeader(http.StatusOK)
		case "/dms/guild/messages":
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				if err := r.ParseMultipartForm(1024); err != nil {
					t.Fatalf("parse multipart: %v", err)
				}
				file, _, err := r.FormFile("file_image")
				if err != nil {
					t.Fatalf("missing file_image: %v", err)
				}
				data, err := io.ReadAll(file)
				file.Close()
				if err != nil {
					t.Fatalf("read file_image: %v", err)
				}
				uploads = append(uploads, string(data))
				captions = append(captions, r.FormValue("content"))
			} else {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read text body: %v", err)
				}
				textMessages = append(textMessages, string(data))
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"code": 0})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	handler := &CommandHandler{
		sender: NewQQChatSender(&QQBotAPI{
			baseURL:        server.URL,
			client:         server.Client(),
			accessToken:    "token",
			tokenExpiresAt: time.Now().Add(time.Hour),
			headers:        map[string]string{},
		}),
		userdata: map[string]any{"exam": "exam-1"},
	}
	ctx := &MessageContext{
		GuildID: "guild",
		ID:      "msg-1",
		UserID:  "user-1",
	}

	errMsg := handler.sendAnswerSheetImages(ctx, answerSheetFetch{
		subjectID: "001",
		subInfo:   map[string]any{"data": map[string]any{}},
		urls: []any{
			server.URL + "/img/1",
			server.URL + "/img/bad",
			"",
			server.URL + "/img/2",
			server.URL + "/img/empty",
		},
	})
	if errMsg != "" {
		t.Fatalf("unexpected error message: %s", errMsg)
	}
	if strings.Join(uploads, ",") != "one,two" {
		t.Fatalf("uploads = %q", uploads)
	}
	if strings.Join(captions, ",") != "答题卡卡面 1,答题卡卡面 4" {
		t.Fatalf("captions = %q", captions)
	}
	if len(textMessages) != 3 {
		t.Fatalf("text message count = %d want 3", len(textMessages))
	}
}
