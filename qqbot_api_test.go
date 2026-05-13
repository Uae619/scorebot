package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewQQBotAPILazilyRefreshesToken(t *testing.T) {
	tokenRequests := 0
	messageRequests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			tokenRequests++
			json.NewEncoder(w).Encode(map[string]any{"access_token": "lazy-token", "expires_in": 3600})
		case "/dms/guild/messages":
			messageRequests++
			if got := r.Header.Get("Authorization"); got != "QQBot lazy-token" {
				t.Fatalf("Authorization = %q", got)
			}
			json.NewEncoder(w).Encode(map[string]any{"code": 0})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	api := newQQBotAPI("app-id", "secret")
	api.baseURL = server.URL
	api.tokenURL = server.URL + "/token"
	api.client = server.Client()

	if tokenRequests != 0 {
		t.Fatalf("constructor made %d token requests", tokenRequests)
	}
	result := api.dmsMessagesWithContext(context.Background(), "guild", "hello", "msg-1")
	if errText := asString(result["error"]); errText != "" {
		t.Fatalf("send returned error: %s", errText)
	}
	if tokenRequests != 1 {
		t.Fatalf("tokenRequests = %d want 1", tokenRequests)
	}
	if messageRequests != 1 {
		t.Fatalf("messageRequests = %d want 1", messageRequests)
	}
}
