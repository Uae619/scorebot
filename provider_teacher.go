package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func teacherLoginFenxiWithContext(ctx context.Context, loginName, password string) map[string]any {
	headers := map[string]string{
		"accept":          "application/json, text/plain, */*",
		"accept-language": "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
		"cache-control":   "no-cache",
		"content-type":    "application/json;charset=UTF-8",
		"origin":          "https://www.yunxiao.com",
		"pragma":          "no-cache",
		"priority":        "u=1, i",
		"referer":         "https://www.yunxiao.com/",
		"user-agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36 Edg/139.0.0.0",
	}

	// Step 1: query user list by phone
	userListResp, _, err := doJSONRequestWithContext(ctx, http.MethodPost,
		"https://yx-auth-wan.yunxiao.com/user/query/yj/userList",
		map[string]any{"phone": loginName}, headers, 20*time.Second)
	if err != nil {
		return map[string]any{"loginSuccess": false, "msg": err.Error()}
	}
	if asInt(userListResp["code"]) != 0 {
		return map[string]any{"loginSuccess": false, "msg": fmt.Sprintf("该用户不存在或密码错误(%v)", userListResp["message"])}
	}
	userListData := asSlice(userListResp["data"])
	if len(userListData) == 0 {
		return map[string]any{"loginSuccess": false, "msg": "该用户不存在或密码错误"}
	}
	firstUser := asMap(userListData[0])
	account := asString(firstUser["loginName"])
	if account == "" {
		account = asString(firstUser["phone"])
	}
	if account == "" {
		return map[string]any{"loginSuccess": false, "msg": "该用户不存在或密码错误"}
	}

	// Step 2: login with account + base64 password
	loginPayload := map[string]any{
		"account":  account,
		"password": base64.StdEncoding.EncodeToString([]byte(password)),
	}
	loginResp, _, err := doJSONRequestWithContext(ctx, http.MethodPost,
		"https://yx-auth-wan.yunxiao.com/login/userInfo/verify/forPlatform",
		loginPayload, headers, 20*time.Second)
	if err != nil {
		return map[string]any{"loginSuccess": false, "msg": err.Error()}
	}
	if asInt(loginResp["code"]) != 0 {
		return map[string]any{"loginSuccess": false, "msg": fmt.Sprintf("该用户不存在或密码错误(%v)", loginResp["msg"])}
	}
	unifyToken := asString(asMap(loginResp["data"])["unifyToken"])
	if unifyToken == "" {
		return map[string]any{"loginSuccess": false, "msg": "unifyToken missing in response"}
	}
	return map[string]any{"loginSuccess": true, "unify_sid": unifyToken}
}

func fenxiSearchWithContext(ctx context.Context, unifySID string, examID any, searchKey any, limit int) string {
	examIDStr := asString(examID)
	searchKeyStr := asString(searchKey)
	if unifySID == "" || examIDStr == "" || searchKeyStr == "" {
		return ""
	}

	cookieHeader := "unify_sid=" + unifySID
	jsonHeaders := map[string]string{
		"cookie":       cookieHeader,
		"content-type": "application/json",
	}

	// Step 1: get exam info
	examIDInt, _ := strconv.Atoi(examIDStr)
	org := "0"
	if examIDInt <= 800000 {
		org = "2"
	}
	examInfoText, _, err := doTextRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://fenxi.haofenshu.com/report/obt/v1/exam/info?examid=%s&org=%s", examIDStr, org),
		nil, map[string]string{"cookie": cookieHeader}, 20*time.Second)
	if err != nil {
		return err.Error()
	}
	if strings.Contains(examInfoText, "Internal Server Error") {
		return "内部错误"
	}

	var examInfoResp map[string]any
	if err := json.Unmarshal([]byte(examInfoText), &examInfoResp); err != nil {
		return "访问好分数服务器时出现错误"
	}
	if asInt(examInfoResp["code"]) != 0 {
		return asString(examInfoResp["msg"])
	}

	examInfo := asMap(asMap(examInfoResp["data"])["examInfo"])
	orgType := asString(examInfo["org"])
	schoolID := examInfo["schoolid"]
	groups := asSlice(examInfo["groups"])
	classIDs := make([]any, len(groups))
	for i, g := range groups {
		classIDs[i] = g
	}

	// Step 2: search for student
	searchPayload := map[string]any{
		"paperid": -1,
		"page":    map[string]any{"offset": 1, "limit": 10},
		"sort":    map[string]any{"subject": -1, "key": "score", "order": "desc"},
		"search":  searchKeyStr,
		"isTeach": false, "isEs": false, "authLimit": false,
		"isOverMean": false, "isStandardScore": false,
		"scopeField": "classids",
		"schoolid":   schoolID,
		"classids":   classIDs,
	}
	searchPayloadBytes, _ := json.Marshal(searchPayload)

	searchOrg := org
	if searchOrg == "0" {
		searchOrg = "1"
	}
	searchText, _, err := doTextRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://fenxi.haofenshu.com/report/analysis/v1/exam/irrank/page?examid=%s&org=%s", examIDStr, searchOrg),
		bytes.NewReader(searchPayloadBytes), jsonHeaders, 20*time.Second)
	if err != nil {
		return err.Error()
	}
	if strings.Contains(searchText, "Internal Server Error") {
		return "鉴权凭证已过期，请重新登陆"
	}

	var searchResp map[string]any
	if err := json.Unmarshal([]byte(searchText), &searchResp); err != nil {
		return "访问好分数服务器时出现错误"
	}
	if asString(searchResp["msg"]) != "success" {
		return asString(searchResp["msg"])
	}

	items := asSlice(asMap(searchResp["data"])["items"])
	if len(items) == 0 {
		return "未查询到得分数据"
	}

	// Find student by xuehao matching searchKey
	var student map[string]any
	for _, item := range items {
		it := asMap(item)
		if asString(it["xuehao"]) == searchKeyStr {
			student = it
			break
		}
	}
	if student == nil {
		student = asMap(items[0])
	}

	// Build basic report
	var sb strings.Builder
	sb.WriteString(asString(student["name"]))
	if ss := asString(student["subjectSelection"]); ss != "" {
		sb.WriteString("(" + ss + ")")
	}
	if s := asString(student["school"]); s != "" {
		sb.WriteString(" " + s)
	}
	sb.WriteString(" " + asString(student["xuehao"]) + " " + asString(student["class"]))
	sb.WriteString("\n=====基本报告=====")

	type subjectInfo struct {
		name       string
		score      string
		orgScore   string
		allRank    string
		countyRank string
		schoolRank string
		classRank  string
		paperID    string

		classTotal   string
		classMean    string
		classMax     string
		schoolTotal  string
		schoolMean   string
		schoolMax    string
		liankaoTotal string
		liankaoMean  string
		liankaoMax   string
	}
	subjects := map[string]*subjectInfo{}
	userClass := asString(student["class"])

	for _, subj := range asSlice(student["subjects"]) {
		s := asMap(subj)
		score := asString(s["score"])
		if score == "-2" || score == "**" {
			if score == "**" {
				return "数据被屏蔽"
			}
			continue
		}
		pid := asString(s["paperid"])
		info := &subjectInfo{
			name:       asString(s["subjectName"]),
			score:      score,
			schoolRank: asString(s["schoolRank"]),
			classRank:  asString(s["classRank"]),
			paperID:    pid,
		}
		if v := asString(s["orgScore"]); v != "" {
			info.orgScore = v
		}
		if v := asString(s["allRank"]); v != "" {
			info.allRank = v
		}
		if v := asString(s["countyRank"]); v != "" {
			info.countyRank = v
		}
		subjects[pid] = info
	}

	// Step 3: get exam factors (stats)
	factorsPayload := map[string]any{"fields": []string{"total", "max", "mean", "median"}}
	factorsBytes, _ := json.Marshal(factorsPayload)
	factorsText, _, err := doTextRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://fenxi.haofenshu.com/report/obt/v2/exam/factors?org=%s&examid=%s", orgType, examIDStr),
		bytes.NewReader(factorsBytes), jsonHeaders, 20*time.Second)
	if err != nil {
		return sb.String()
	}

	var factorsResp map[string]any
	if err := json.Unmarshal([]byte(factorsText), &factorsResp); err != nil {
		return sb.String()
	}
	if asInt(factorsResp["code"]) != 0 {
		return sb.String()
	}

	isLiankao := false
	for subKey, subVal := range asMap(asMap(factorsResp["data"])["factors"]) {
		factorMap := asMap(subVal)
		if subKey == "totalScore" {
			subKey = "-1"
		}
		info, ok := subjects[subKey]
		if !ok {
			continue
		}
		if classStats := asMap(factorMap[userClass]); classStats != nil {
			info.classTotal = asString(classStats["total"])
			info.classMean = asString(classStats["mean"])
			info.classMax = asString(classStats["max"])
		}
		if allStats := asMap(factorMap["all"]); allStats != nil {
			info.schoolTotal = asString(allStats["total"])
			info.schoolMean = asString(allStats["mean"])
			info.schoolMax = asString(allStats["max"])
		}
		if lkStats := asMap(factorMap["lkall"]); lkStats != nil {
			info.liankaoTotal = asString(lkStats["total"])
			info.liankaoMean = asString(lkStats["mean"])
			info.liankaoMax = asString(lkStats["max"])
			isLiankao = true
		}
	}

	// Format basic report lines
	for _, info := range subjects {
		sb.WriteString("\n★" + info.name + " " + info.score)
		if info.orgScore != "" && info.orgScore != info.score {
			sb.WriteString("(原" + info.orgScore + ")")
		}
		sb.WriteString(" [")
		if isLiankao && info.allRank != "" {
			sb.WriteString("联" + info.allRank + "|")
		}
		if info.countyRank != "" {
			sb.WriteString("区" + info.countyRank + "|")
		}
		sb.WriteString("校" + info.schoolRank + "|班" + info.classRank + "]")
	}

	// Format analysis report
	sb.WriteString("\n=====分析报告=====")
	for _, info := range subjects {
		sb.WriteString("\n" + info.name + " ★ " + info.score)
		if info.orgScore != "" && info.orgScore != info.score {
			sb.WriteString("(原" + info.orgScore + ")")
		}
		sb.WriteString(" [")
		if isLiankao && info.allRank != "" {
			sb.WriteString("联" + info.allRank + "|")
		}
		if info.countyRank != "" {
			sb.WriteString("区" + info.countyRank + "|")
		}
		sb.WriteString("校" + info.schoolRank + "|班" + info.classRank)

		// percentage
		if info.allRank != "" && info.liankaoTotal != "" {
			r, _ := strconv.ParseFloat(info.allRank, 64)
			t, _ := strconv.ParseFloat(info.liankaoTotal, 64)
			if t > 0 {
				sb.WriteString(fmt.Sprintf("|%.2f%%", r/t*100))
			}
		} else if info.schoolRank != "" && info.schoolTotal != "" {
			r, _ := strconv.ParseFloat(info.schoolRank, 64)
			t, _ := strconv.ParseFloat(info.schoolTotal, 64)
			if t > 0 {
				sb.WriteString(fmt.Sprintf("|%.2f%%", r/t*100))
			}
		}
		sb.WriteString("]")

		sb.WriteString("\n参考人数 | ")
		if info.liankaoTotal != "" {
			sb.WriteString("联 " + info.liankaoTotal + " ")
		}
		sb.WriteString("校 " + info.schoolTotal + " 班 " + info.classTotal)

		sb.WriteString("\n平均分数 | ")
		if info.liankaoMean != "" {
			sb.WriteString("联 " + info.liankaoMean + " ")
		}
		sb.WriteString("校 " + info.schoolMean + " 班 " + info.classMean)

		sb.WriteString("\n最高分数 | ")
		if info.liankaoMax != "" {
			sb.WriteString("联 " + info.liankaoMax + " ")
		}
		sb.WriteString("校 " + info.schoolMax + " 班 " + info.classMax)
	}

	return sb.String()
}

func teacherAnalysisGETWithContext(ctx context.Context, endpoint, unifySID string) map[string]any {
	headers := map[string]string{
		"accept":          "application/json, text/plain, */*",
		"accept-language": "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
		"cache-control":   "no-cache",
		"origin":          "https://hfsjs.haofenshu.com",
		"pragma":          "no-cache",
		"priority":        "u=1, i",
		"referer":         "https://hfsjs.haofenshu.com/",
		"user-agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36 Edg/139.0.0.0",
		"Cookie":          "unify_sid=" + unifySID,
	}
	text, _, err := doTextRequestWithContext(ctx, http.MethodGet, endpoint, nil, headers, 20*time.Second)
	if err != nil {
		return map[string]any{"isSuccess": false, "msg": "向服务器请求数据失败"}
	}
	if strings.Contains(text, "Internal Server Error") {
		return map[string]any{"isSuccess": false, "msg": "向服务器请求数据失败: 内部错误"}
	}

	var responseJSON map[string]any
	if err := json.Unmarshal([]byte(text), &responseJSON); err != nil {
		return map[string]any{"isSuccess": false, "msg": "向服务器请求数据失败"}
	}
	if asInt(responseJSON["code"]) == 1 {
		return map[string]any{"isSuccess": true, "msg": "暂无数据"}
	}
	if asInt(responseJSON["code"]) != 0 {
		return map[string]any{"isSuccess": false, "msg": asString(responseJSON["msg"])}
	}
	return map[string]any{"isSuccess": true, "data": responseJSON}
}

func hfsjsAnalysisRankinfoWithContext(ctx context.Context, unifySID string, examID any, studentID any) map[string]any {
	return teacherAnalysisGETWithContext(ctx, fmt.Sprintf("https://www.haofenshu.com/proxy-teacher/v3/students/%s/analysis/exam/%v/rank-info", asString(studentID), examID), unifySID)
}

func hfsjsAnalysisPapersWithContext(ctx context.Context, unifySID string, examID any, studentID any) map[string]any {
	return teacherAnalysisGETWithContext(ctx, fmt.Sprintf("https://www.haofenshu.com/proxy-teacher/v3/students/%s/analysis/exam/%v/papers-analysis?groupType=0", asString(studentID), examID), unifySID)
}
