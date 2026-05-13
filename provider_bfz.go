package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	bfzStudentAPIURL             = "https://www.bfzks.com/"
	bfzPKey                      = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC7PqHF0hsGSBB1N3gBiQZRZ/EBnvO/ZVwvv01Y+NERdEt2+tMqDGseV7FJOXCfzMEaiOXwyvZ4tZYsJ8U7ZwcSC/uM5cmTfkada9Cx0iXwRshkf8AptQTEH/6PgzEa6cnxwWwUwzRuqafzM/VA8Mz1htBIHHPoFbV92IvNnpoWOQIDAQAB"
	bfzStudentViewstateGenerator = "C2EE9ABB"
)

var (
	bfzPublicKey              = mustRSAPublicKeyB64(bfzPKey)
	bfzStudentNamePattern     = regexp.MustCompile(`欢迎您，(.*?) 同学`)
	bfzHiddenNameValuePattern = regexp.MustCompile(
		`<input[^>]+name="([^"]*)"[^>]+value="([^"]*)"`,
	)
	bfzHiddenValueNamePattern = regexp.MustCompile(
		`<input[^>]+value="([^"]*)"[^>]+name="([^"]*)"`,
	)
)

func bfzEncryptPassword(password, pkey string) (string, error) {
	pubKey := bfzPublicKey
	if pkey != bfzPKey {
		var err error
		pubKey, err = parseRSAPublicKeyB64(pkey)
		if err != nil {
			return "", err
		}
	}
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, []byte(password))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func bfzExtractStudentName(html string) string {
	matches := bfzStudentNamePattern.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func extractHiddenValue(html string, name string) string {
	for _, matches := range bfzHiddenNameValuePattern.FindAllStringSubmatch(html, -1) {
		if len(matches) > 2 && matches[1] == name {
			return matches[2]
		}
	}
	for _, matches := range bfzHiddenValueNamePattern.FindAllStringSubmatch(html, -1) {
		if len(matches) > 2 && matches[2] == name {
			return matches[1]
		}
	}
	return ""
}

func bfzLoginInternalWithContext(ctx context.Context, username, password string) string {
	headersGet := map[string]string{
		"authority":                 "www.bfzks.com",
		"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"accept-language":           "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
		"cache-control":             "max-age=0",
		"referer":                   "https://www.bfzks.com/",
		"sec-ch-ua":                 `"Chromium";v="118", "Microsoft Edge";v="118", "Not=A?Brand";v="99"`,
		"sec-ch-ua-mobile":          "?1",
		"sec-ch-ua-platform":        `"Android"`,
		"sec-fetch-dest":            "document",
		"sec-fetch-mode":            "navigate",
		"sec-fetch-site":            "same-origin",
		"sec-fetch-user":            "?1",
		"upgrade-insecure-requests": "1",
		"user-agent":                "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Mobile Safari/537.36 Edg/118.0.2088.57",
	}
	headersPost := map[string]string{
		"authority":                 "www.bfzks.com",
		"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"accept-language":           "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
		"cache-control":             "max-age=0",
		"content-type":              "application/x-www-form-urlencoded",
		"origin":                    "https://www.bfzks.com",
		"referer":                   "https://www.bfzks.com/",
		"sec-ch-ua":                 `"Chromium";v="118", "Microsoft Edge";v="118", "Not=A?Brand";v="99"`,
		"sec-ch-ua-mobile":          "?1",
		"sec-ch-ua-platform":        `"Android"`,
		"sec-fetch-dest":            "document",
		"sec-fetch-mode":            "navigate",
		"sec-fetch-site":            "same-origin",
		"sec-fetch-user":            "?1",
		"upgrade-insecure-requests": "1",
		"user-agent":                "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Mobile Safari/537.36 Edg/118.0.2088.57",
	}

	html, _, err := doTextRequestWithContext(ctx, http.MethodGet, bfzStudentAPIURL, nil, headersGet, 20*time.Second)
	if err != nil {
		return ""
	}
	eventValidation := extractHiddenValue(html, "__EVENTVALIDATION")
	viewState := extractHiddenValue(html, "__VIEWSTATE")
	if eventValidation == "" || viewState == "" {
		return ""
	}

	encryptedPassword, err := bfzEncryptPassword(password, bfzPKey)
	if err != nil {
		return ""
	}

	form := url.Values{}
	form.Set("__EVENTTARGET", "")
	form.Set("__EVENTARGUMENT", "")
	form.Set("__VIEWSTATE", viewState)
	form.Set("__VIEWSTATEGENERATOR", bfzStudentViewstateGenerator)
	form.Set("__VIEWSTATEENCRYPTED", "")
	form.Set("__EVENTVALIDATION", eventValidation)
	form.Set("txbUserName", username)
	form.Set("txbPassword", encryptedPassword)
	form.Set("btnSubmit", "登 录")
	form.Set("hndSystem", "1")

	result, _, err := doTextRequestWithContext(ctx, http.MethodPost, bfzStudentAPIURL, strings.NewReader(form.Encode()), headersPost, 20*time.Second)
	if err != nil {
		return ""
	}
	return result
}

func bfzStudentLoginWithContext(ctx context.Context, username, password string) map[string]any {
	html := bfzLoginInternalWithContext(ctx, username, password)
	if html == "" {
		return map[string]any{"isSuccess": false}
	}
	name := bfzExtractStudentName(html)
	if name == "" {
		return map[string]any{"isSuccess": false}
	}
	return map[string]any{"isSuccess": true, "name": name}
}
