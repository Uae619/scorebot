package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDoJSONRequestWithContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := doJSONRequestWithContext(ctx, http.MethodGet, server.URL, nil, nil, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v want context.Canceled", err)
	}
}
