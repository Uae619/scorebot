package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

var headersMain = map[string]string{
	"Accept":                   "application/json, text/plain, */*",
	"Accept-Language":          "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
	"Connection":               "keep-alive",
	"Origin":                   "https://app.haofenshu.com",
	"Referer":                  "https://app.haofenshu.com/",
	"Sec-Fetch-Dest":           "empty",
	"Sec-Fetch-Mode":           "cors",
	"Sec-Fetch-Site":           "cross-site",
	"Sec-Fetch-Storage-Access": "active",
	"User-Agent":               "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 Edg/146.0.0.0",
	"deviceType":               "3",
	"sec-ch-ua-mobile":         "?0",
	"sec-ch-ua-platform":       `"Windows"`,
	"Accept-Encoding":          "gzip, deflate, br, zstd",
}

func newHFSHeaders(token string) map[string]string {
	headers := map[string]string{}
	for key, value := range headersMain {
		headers[key] = value
	}
	if token != "" {
		headers["hfs-token"] = token
		headers["Cookie"] = "hfs-session-id=" + token
	}
	return headers
}

func newHFSMobileArchiveHeaders(token string) map[string]string {
	return map[string]string{
		"Host":                     "hfs-be.yunxiao.com",
		"Connection":               "keep-alive",
		"sec-ch-ua-platform":       `"Android"`,
		"User-Agent":               "Mozilla/5.0 (Linux; Android 15; RMX3850 Build/UKQ1.231108.001; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/134.0.6998.135 Mobile Safari/537.36HFS_XSversion=4.31.71",
		"hfs-token":                token,
		"Content-Type":             "application/json",
		"sec-ch-ua-mobile":         "?1",
		"Accept":                   "*/*",
		"Origin":                   "https://mobile.haofenshu.com",
		"X-Requested-With":         "com.yunxiao.haofenshu",
		"Sec-Fetch-Site":           "cross-site",
		"Sec-Fetch-Mode":           "cors",
		"Sec-Fetch-Dest":           "empty",
		"Sec-Fetch-Storage-Access": "active",
		"Referer":                  "https://mobile.haofenshu.com/",
		"Accept-Encoding":          "gzip, deflate, br, zstd",
		"Accept-Language":          "zh-CN,zh;q=0.9,ja-JP;q=0.8,ja;q=0.7,en-US;q=0.6,en;q=0.5",
		"Cookie":                   "hfs-session-id=" + token,
	}
}

func studentLoginWithContext(ctx *MessageContext, loginName, password string, accountType int) []string {
	payload := map[string]any{
		"loginName":  loginName,
		"password":   base64.StdEncoding.EncodeToString([]byte(password)),
		"roleType":   accountType,
		"loginType":  1,
		"rememberMe": 2,
	}
	headers := map[string]string{
		"accept":                   "application/json, text/plain, */*",
		"accept-language":          "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
		"cache-control":            "no-cache",
		"content-type":             "application/json;charset=UTF-8",
		"devicetype":               "3",
		"origin":                   "https://app.haofenshu.com",
		"pragma":                   "no-cache",
		"priority":                 "u=1, i",
		"referer":                  "https://app.haofenshu.com/",
		"sec-ch-ua-mobile":         "?0",
		"sec-fetch-dest":           "empty",
		"sec-fetch-mode":           "cors",
		"sec-fetch-site":           "cross-site",
		"sec-fetch-storage-access": "active",
		"user-agent":               "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36 Edg/138.0.0.0",
		"Accept-Encoding":          "gzip, deflate, br",
		"Connection":               "keep-alive",
		"Host":                     "hfs-be.yunxiao.com",
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return []string{err.Error()}
	}
	req, err := http.NewRequestWithContext(messageContext(ctx), http.MethodPost, "https://hfs-be.yunxiao.com/v2/users/sessions", bytes.NewReader(reqBody))
	if err != nil {
		return []string{err.Error()}
	}
	copyHeaders(req, headers)

	resp, err := httpClient(20 * time.Second).Do(req)
	if err != nil {
		return []string{err.Error()}
	}
	defer resp.Body.Close()

	raw, err := readResponseBody(resp)
	if err != nil {
		return []string{err.Error()}
	}
	logAdminThirdPartyRequest(ctx, "hfs", http.MethodPost, "https://hfs-be.yunxiao.com/v2/users/sessions", payload, raw, nil)
	var responseJSON map[string]any
	if err := json.Unmarshal(raw, &responseJSON); err != nil {
		return []string{"获取到空JSON"}
	}

	switch asInt(responseJSON["code"]) {
	case 4049:
		return []string{"出现错误 code:4049，本错误可能是由于机器人服务器IP被好分数封禁，请稍后再行尝试"}
	case 4046:
		msg := asString(responseJSON["msg"])
		if msg == "" {
			return []string{"获取到空JSON"}
		}
		if msg == "密码错误" {
			return []string{"学生&家长端登录密码错误，请重新绑定账号以刷新数据"}
		}
		return []string{msg}
	}

	msg := asString(responseJSON["msg"])
	if msg == "" {
		msg = "获取到空JSON"
	}

	token := asString(asMap(responseJSON["data"])["token"])
	if token == "" {
		return []string{"登录成功但未返回token"}
	}
	return []string{msg, token}
}

func studentSnapshotWithContext(ctx *MessageContext, hfsToken string) map[string]any {
	headers := newHFSHeaders(hfsToken)
	data, _, err := doJSONRequestWithContext(messageContext(ctx), http.MethodGet, "https://hfs-be.yunxiao.com/v2/user-center/user-snapshot", nil, headers, 20*time.Second)
	if err != nil {
		return map[string]any{"msg": err.Error()}
	}
	logAdminThirdPartyRequest(ctx, "hfs", http.MethodGet, "https://hfs-be.yunxiao.com/v2/user-center/user-snapshot", nil, data, nil)

	linkedStudent := asMap(asMap(data["data"])["linkedStudent"])
	if len(linkedStudent) == 0 {
		return map[string]any{
			"msg":       asString(data["msg"]),
			"xuehao":    "",
			"studentid": "",
			"name":      "",
			"school":    "",
			"class":     "",
			"grade":     "",
		}
	}

	xuehao := ""
	if xuehaoList := asSlice(linkedStudent["xuehao"]); len(xuehaoList) > 0 {
		xuehao = asString(xuehaoList[0])
	}

	return map[string]any{
		"msg":       asString(data["msg"]),
		"xuehao":    xuehao,
		"studentid": linkedStudent["studentId"],
		"name":      asString(linkedStudent["studentName"]),
		"school":    asString(linkedStudent["schoolName"]),
		"class":     asString(linkedStudent["className"]),
		"grade":     asString(linkedStudent["grade"]),
	}
}

func studentGetHiddenConfigWithContext(ctx *MessageContext, hfsToken string) map[string]any {
	data, raw, err := doJSONRequestWithContext(messageContext(ctx), http.MethodGet, "https://hfs-be.yunxiao.com/v2/config/school/hidden-config", nil, newHFSHeaders(hfsToken), 20*time.Second)
	if err != nil {
		return map[string]any{"getSuccess": false, "msg": "向好分数服务器请求数据失败", "code": -1}
	}
	logAdminThirdPartyRequest(ctx, "hfs", http.MethodGet, "https://hfs-be.yunxiao.com/v2/config/school/hidden-config", nil, data, nil)
	if asInt(data["code"]) != 0 {
		log.Printf("> 获取学校配置失败 %s", string(raw))
		return map[string]any{"getSuccess": false, "msg": asString(data["msg"]), "code": data["code"]}
	}
	data["getSuccess"] = true
	return data
}

func studentGetExamlistWithContext(ctx *MessageContext, hfsToken string) map[string]any {
	data, raw, err := doJSONRequestWithContext(messageContext(ctx), http.MethodGet, "https://hfs-be.yunxiao.com/v3/exam/list?start=0&limit=20", nil, newHFSHeaders(hfsToken), 20*time.Second)
	if err != nil {
		return map[string]any{"getSuccess": false, "msg": "向好分数服务器请求数据失败", "code": -1}
	}
	logAdminThirdPartyRequest(ctx, "hfs", http.MethodGet, "https://hfs-be.yunxiao.com/v3/exam/list?start=0&limit=20", nil, data, nil)
	if asInt(data["code"]) == 0 {
		data["getSuccess"] = true
		return data
	}

	if asInt(data["code"]) == 1 && asString(data["msg"]) == "暂无数据examsIndex" {
		data["code"] = 0
		data["msg"] = ""
		data["data"] = []any{}
		data["getSuccess"] = true
		return data
	}

	if asInt(data["code"]) == 1 && asString(data["msg"]) == "账号存在风险，已被锁定" {
		log.Printf("> 获取考试列表(v3)命中风控，回退旧链路 %s", string(raw))
		fallbackURL := "https://hfs-be.yunxiao.com/v4/exam/archives?grade="
		fallbackHeaders := newHFSMobileArchiveHeaders(hfsToken)
		data, raw, err = doJSONRequestWithContext(messageContext(ctx), http.MethodGet, fallbackURL, nil, fallbackHeaders, 20*time.Second)
		if err != nil {
			return map[string]any{"getSuccess": false, "msg": "向好分数服务器请求数据失败", "code": -1}
		}
		if asInt(data["code"]) != 0 {
			if asInt(data["code"]) == 1 && asString(data["msg"]) == "账号存在风险，已被锁定" {
				log.Printf("> 获取考试列表(v4)仍命中风控，标记为重登录重试")
				return map[string]any{
					"getSuccess":   false,
					"msg":          asString(data["msg"]),
					"code":         data["code"],
					"retryRelogin": true,
				}
			}
			log.Printf("> 获取考试列表(v4)失败 %s", string(raw))
			return map[string]any{"getSuccess": false, "msg": asString(data["msg"]), "code": data["code"]}
		}
		data["getSuccess"] = true
		return data
	}

	log.Printf("> 获取考试列表失败 %s", string(raw))
	return map[string]any{"getSuccess": false, "msg": asString(data["msg"]), "code": data["code"]}
}

func studentGetExamInfoWithContext(ctx *MessageContext, hfsToken string, examID any) map[string]any {
	data, raw, err := doJSONRequestWithContext(messageContext(ctx), http.MethodGet, fmt.Sprintf("https://hfs-be.yunxiao.com/v3/exam/%v/overview", examID), nil, newHFSHeaders(hfsToken), 20*time.Second)
	if err != nil {
		return map[string]any{"getSuccess": false, "msg": "向好分数服务器请求数据失败", "code": -1}
	}
	logAdminThirdPartyRequest(ctx, "hfs", http.MethodGet, fmt.Sprintf("https://hfs-be.yunxiao.com/v3/exam/%v/overview", examID), nil, data, nil)
	if asInt(data["code"]) != 0 {
		log.Printf("> 获取考试信息失败 %s", string(raw))
		return map[string]any{"getSuccess": false, "msg": asString(data["msg"]), "code": data["code"]}
	}
	data["getSuccess"] = true
	return data
}

func studentGetSubjectTinfoAnswerpicWithContext(ctx *MessageContext, hfsToken string, examID any, paperID any, pid any) map[string]any {
	data, _, err := doJSONRequestWithContext(messageContext(ctx), http.MethodGet, fmt.Sprintf("https://hfs-be.yunxiao.com/v3/exam/%v/papers/%v/answer-picture?pid=%v", examID, paperID, pid), nil, newHFSHeaders(hfsToken), 20*time.Second)
	if err != nil {
		return map[string]any{"getSuccess": false, "msg": err.Error()}
	}
	logAdminThirdPartyRequest(ctx, "hfs", http.MethodGet, fmt.Sprintf("https://hfs-be.yunxiao.com/v3/exam/%v/papers/%v/answer-picture?pid=%v", examID, paperID, pid), nil, data, nil)
	msg := asString(data["msg"])
	if asInt(data["code"]) == 3001 || msg == "登录无效，请重新登录" || msg == "服务出错，稍后再试" {
		return map[string]any{"getSuccess": false, "code": 3001, "msg": msg}
	}
	data["getSuccess"] = true
	return data
}
