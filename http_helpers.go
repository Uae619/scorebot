package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var sharedHTTPTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   10,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   5 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func httpClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: sharedHTTPTransport,
	}
}

func copyHeaders(dst *http.Request, headers map[string]string) {
	for key, value := range headers {
		if strings.EqualFold(key, "Accept-Encoding") {
			continue
		}
		dst.Header.Set(key, value)
	}
}

func requestContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	reader := io.Reader(resp.Body)
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}
	return io.ReadAll(reader)
}

func doJSONRequestWithContext(ctx context.Context, method, rawURL string, payload any, headers map[string]string, timeout time.Duration) (map[string]any, []byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(requestContext(ctx), method, rawURL, body)
	if err != nil {
		return nil, nil, err
	}
	copyHeaders(req, headers)

	resp, err := httpClient(timeout).Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	raw, err := readResponseBody(resp)
	if err != nil {
		return nil, nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, raw, err
	}
	return result, raw, nil
}

func doTextRequestWithContext(ctx context.Context, method, rawURL string, body io.Reader, headers map[string]string, timeout time.Duration) (string, *http.Response, error) {
	req, err := http.NewRequestWithContext(requestContext(ctx), method, rawURL, body)
	if err != nil {
		return "", nil, err
	}
	copyHeaders(req, headers)

	resp, err := httpClient(timeout).Do(req)
	if err != nil {
		return "", nil, err
	}
	raw, err := readResponseBody(resp)
	resp.Body.Close()
	if err != nil {
		return "", resp, err
	}
	return string(raw), resp, nil
}
