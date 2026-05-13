package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type answerSheetFetch struct {
	subjectID string
	paperID   string
	pid       string
	subInfo   map[string]any
	urls      []any
	urlSource string
}

var subjectIDPattern = regexp.MustCompile(`\d+-\d{3,6}`)

type answerSheetImageResult struct {
	index      int
	url        string
	statusCode int
	empty      bool
	err        error
}

func (h *CommandHandler) dataIntegrityCheckSubject(content string) map[string]any {
	if !strings.Contains(content, "-") {
		return map[string]any{"Return": false, "msg": "科目ID格式无效，请输入正确的科目ID！"}
	}
	match := subjectIDPattern.FindString(content)
	if match == "" {
		return map[string]any{"Return": false, "msg": "命令不正确，未匹配到科目ID！"}
	}
	prefix, _, _ := strings.Cut(match, "-")
	value, err := strconv.Atoi(prefix)
	if err != nil {
		return map[string]any{"Return": false, "msg": "科目ID格式无效，必须为数字。"}
	}
	if value <= 5000000 || value >= 15000000 {
		return map[string]any{"Return": false, "msg": "科目ID有误，请输入正确的科目ID！"}
	}
	if _, ok := h.userdata["exam"]; !ok {
		return map[string]any{"Return": false, "msg": "未获取前置考试信息，请先获取对应考试详情后再操作。"}
	}
	return map[string]any{"Return": true, "msg": "", "subjectid": match}
}

func (h *CommandHandler) loadSubjectContext(subjectID string) (string, map[string]any, string) {
	examContext := opViewExamContext(asString(h.userdata["qqid"]))
	if examContext["Return"] != true {
		return "", nil, "* 错误: 未找到前置考试信息，请先使用 /考试详情 查询一次考试。"
	}
	examID := asString(examContext["exam"])
	if examID == "" {
		return "", nil, "* 错误: 未找到前置考试信息，请先使用 /考试详情 查询一次考试。"
	}
	paperInfo := asMap(asMap(examContext["subject_map"])[subjectID])
	if len(paperInfo) == 0 {
		return "", nil, "* 错误: 未查询到科目信息，请确认已获取对应考试详情。"
	}
	if asString(paperInfo["paperId"]) == "" || asString(paperInfo["pid"]) == "" {
		return "", nil, "* 错误: 科目上下文不完整，请重新使用 /考试详情 获取一次考试详情。"
	}
	return examID, paperInfo, ""
}

func (h *CommandHandler) loadSubjectInfoWithRelogin(ctx *MessageContext, examID string, paperInfo map[string]any, initialSubInfo map[string]any) (answerSheetFetch, string) {
	paperID := asString(paperInfo["paperId"])
	pid := asString(paperInfo["pid"])
	subInfo := initialSubInfo
	if len(subInfo) == 0 {
		subInfo = studentGetSubjectTinfoAnswerpicWithContext(ctx, asString(h.userdata["token"]), examID, paperID, pid)
	}
	if subInfo["getSuccess"] != true {
		if asInt(subInfo["code"]) == 3001 {
			if _, err := h.reloginStudentToken(ctx); err != "" {
				return answerSheetFetch{}, "* 错误: 自动重登录失败: " + err
			}
			subInfo = studentGetSubjectTinfoAnswerpicWithContext(ctx, asString(h.userdata["token"]), examID, paperID, pid)
			if subInfo["getSuccess"] != true {
				return answerSheetFetch{}, "* 错误: 重登录后未获取到数据，请尝试重新绑定账号"
			}
		} else if isHFSRiskLocked(subInfo) {
			return answerSheetFetch{}, "* 错误: 账号被好分数风控机制命中，暂时无法查询。"
		} else {
			return answerSheetFetch{}, "* 错误: " + defaultString(asString(subInfo["msg"]), "未知错误")
		}
	}
	return answerSheetFetch{
		subjectID: paperID,
		paperID:   paperID,
		pid:       pid,
		subInfo:   subInfo,
	}, ""
}

func (h *CommandHandler) getSubjectQuestionsDetails(ctx *MessageContext, onlyFalse bool) string {
	if asString(h.userdata["mode"]) == "bfz" {
		return messageBFZUseFailedUse
	}
	if asString(h.userdata["mode"]) == "qt" {
		command := "/答题详情"
		if onlyFalse {
			command = "/错题详情"
		}
		return h.qtGetAnswerSheet(ctx, command, true)
	}
	integrityCheck := h.dataIntegrityCheckSubject(ctx.Content)
	if integrityCheck["Return"] != true {
		if asString(integrityCheck["msg"]) == "账号存在风险，已被锁定" {
			return "* 错误: 账号被好分数风控机制命中，暂时无法查询。"
		}
		return "* 错误: " + defaultString(asString(integrityCheck["msg"]), "未知错误")
	}

	examID, paperInfo, errMsg := h.loadSubjectContext(asString(integrityCheck["subjectid"]))
	if errMsg != "" {
		return errMsg
	}

	fetch, errMsg := h.loadSubjectInfoWithRelogin(ctx, examID, paperInfo, nil)
	if errMsg != "" {
		return errMsg
	}
	if asInt(fetch.subInfo["code"]) != 0 {
		return "* 错误: 未查询到科目信息，请确认已获取对应考试详情。"
	}

	titleFragment := "题目"
	if onlyFalse {
		titleFragment = "错题"
	}
	lines := []string{fmt.Sprintf(" > %s得分数据 · %s", titleFragment, fetch.subjectID), "(类) > 题号 | 得分/满分 | 填涂/答案"}
	for _, item := range asSlice(asMap(fetch.subInfo["data"])["questions"]) {
		q := asMap(item)
		if onlyFalse && asString(q["score"]) == asString(q["manfen"]) {
			continue
		}
		if asInt(q["type"]) == 2 {
			mark := " ×"
			if asString(q["myAnswer"]) == asString(q["answer"]) {
				mark = " √"
			} else if strings.Contains(asString(q["answer"]), asString(q["myAnswer"])) {
				mark = " ×√"
			}
			lines = append(lines, fmt.Sprintf("(客) > %s | %s分/%s分 | %s/%s%s", asString(q["name"]), asString(q["score"]), asString(q["manfen"]), asString(q["myAnswer"]), asString(q["answer"]), mark))
		} else {
			lines = append(lines, fmt.Sprintf("(主) > %s | %s分/%s分", asString(q["name"]), asString(q["score"]), asString(q["manfen"])))
		}
	}
	return strings.Join(lines, "\n")
}

func (h *CommandHandler) subjectFalseQuestions(ctx *MessageContext) string {
	if ok, msg := h.requireAuth(ctx); !ok {
		return msg
	}
	return h.getSubjectQuestionsDetails(ctx, true)
}

func (h *CommandHandler) subjectAllQuestions(ctx *MessageContext) string {
	if ok, msg := h.requireAuth(ctx); !ok {
		return msg
	}
	return h.getSubjectQuestionsDetails(ctx, false)
}

func (h *CommandHandler) subjectInfo(ctx *MessageContext) string {
	return h.messageNone(ctx)
}

func (h *CommandHandler) sendAnswerSheetImages(ctx *MessageContext, fetch answerSheetFetch) string {
	if len(fetch.urls) == 0 {
		logCommandTrace(ctx, "answer_sheet.no_urls", map[string]any{
			"data_keys":  mapKeysJoined(asMap(fetch.subInfo["data"])),
			"data_type":  fmt.Sprintf("%T", fetch.subInfo["data"]),
			"exam_id":    asString(h.userdata["exam"]),
			"subject_id": fetch.subjectID,
			"url_source": fetch.urlSource,
		})
		return "* 错误: 已获取到答题卡信息，但返回结果中没有可发送的卡面图片。"
	}

	sentCount := 0
	downloadFailCount := 0
	uploadFailCount := 0
	emptyImageCount := 0

	client := httpClient(15 * time.Second)
	for index, rawURL := range fetch.urls {
		result, sendResult := h.sendAnswerSheetImage(ctx, client, index, asString(rawURL))
		if result.err != nil {
			downloadFailCount++
			logCommandTrace(ctx, "answer_sheet.image_download.error", map[string]any{
				"err":   result.err.Error(),
				"index": index + 1,
				"url":   result.url,
			})
			h.sender.SendText(messageContext(ctx), ctx, fmt.Sprintf("答题卡卡面(%d)下载失败: %v", index+1, result.err))
			continue
		}
		if result.empty {
			emptyImageCount++
			logCommandTrace(ctx, "answer_sheet.image_empty", map[string]any{
				"index":       index + 1,
				"status_code": result.statusCode,
				"url":         result.url,
			})
			h.sender.SendText(messageContext(ctx), ctx, fmt.Sprintf("答题卡卡面(%d)下载失败。", index+1))
			continue
		}

		sendError := asString(sendResult["error"])
		sendCode := asInt(sendResult["code"])
		if sendError != "" || sendCode != 0 {
			uploadFailCount++
			logCommandTrace(ctx, "answer_sheet.image_upload.result", map[string]any{
				"error":       sendError,
				"index":       index + 1,
				"response_id": asString(sendResult["id"]),
				"status_code": asInt(sendResult["status_code"]),
				"upload_code": sendCode,
			})
			continue
		}
		sentCount++
	}
	if downloadFailCount > 0 || emptyImageCount > 0 || uploadFailCount > 0 || sentCount == 0 {
		logCommandTrace(ctx, "answer_sheet.finish", map[string]any{
			"download_fail_count": downloadFailCount,
			"empty_image_count":   emptyImageCount,
			"sent_count":          sentCount,
			"upload_fail_count":   uploadFailCount,
			"url_count":           len(fetch.urls),
		})
	}
	if sentCount == 0 {
		return "* 错误: 已获取到答题卡图片地址，但发送失败。"
	}
	return ""
}

func (h *CommandHandler) sendAnswerSheetImage(ctx *MessageContext, client *http.Client, index int, urlValue string) (answerSheetImageResult, map[string]any) {
	result := answerSheetImageResult{
		index: index,
		url:   urlValue,
	}
	if strings.TrimSpace(urlValue) == "" {
		result.err = fmt.Errorf("empty image url")
		return result, nil
	}

	req, err := http.NewRequestWithContext(messageContext(ctx), http.MethodGet, urlValue, nil)
	if err != nil {
		result.err = err
		return result, nil
	}
	resp, err := client.Do(req)
	if err != nil {
		result.err = err
		return result, nil
	}
	result.statusCode = resp.StatusCode
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		result.err = fmt.Errorf("status code %d", resp.StatusCode)
		return result, nil
	}

	firstByte := make([]byte, 1)
	n, readErr := resp.Body.Read(firstByte)
	if readErr != nil && readErr != io.EOF {
		resp.Body.Close()
		result.err = readErr
		return result, nil
	}
	if n == 0 {
		resp.Body.Close()
		result.empty = true
		return result, nil
	}

	imageReader := io.MultiReader(bytes.NewReader(firstByte[:n]), resp.Body)
	sendResult := h.sender.SendImageReader(messageContext(ctx), ctx, imageReader, fmt.Sprintf("答题卡卡面 %d", index+1))
	resp.Body.Close()
	return result, sendResult
}

func (h *CommandHandler) getAnswerSheet(ctx *MessageContext) string {
	if ok, msg := h.requireAuth(ctx); !ok {
		return msg
	}
	if asString(h.userdata["mode"]) == "bfz" {
		return messageBFZUseFailedUse
	}
	if asString(h.userdata["mode"]) == "qt" {
		return h.qtGetAnswerSheet(ctx, "/答题卡", false)
	}

	integrityCheck := h.dataIntegrityCheckSubject(ctx.Content)
	if integrityCheck["Return"] != true {
		if asString(integrityCheck["msg"]) == "账号存在风险，已被锁定" {
			return "* 错误: 账号被好分数风控机制命中，暂时无法查询。"
		}
		return "* 错误: " + defaultString(asString(integrityCheck["msg"]), "未知错误")
	}
	subjectID := asString(integrityCheck["subjectid"])
	examID, paperInfo, errMsg := h.loadSubjectContext(subjectID)
	if errMsg != "" {
		return errMsg
	}

	subInfo := studentGetSubjectTinfoAnswerpicWithContext(ctx, asString(h.userdata["token"]), examID, asString(paperInfo["paperId"]), asString(paperInfo["pid"]))
	urls, urlSource := extractAnswerSheetURLs(subInfo)
	if subInfo["getSuccess"] != true || asInt(subInfo["code"]) != 0 || len(urls) == 0 {
		logCommandTrace(ctx, "answer_sheet.fetch_result", map[string]any{
			"code":        asInt(subInfo["code"]),
			"data_keys":   mapKeysJoined(asMap(subInfo["data"])),
			"data_type":   fmt.Sprintf("%T", subInfo["data"]),
			"get_success": subInfo["getSuccess"] == true,
			"msg":         asString(subInfo["msg"]),
			"url_count":   len(urls),
			"url_source":  urlSource,
		})
	}

	fetch, errMsg := h.loadSubjectInfoWithRelogin(ctx, examID, paperInfo, subInfo)
	if errMsg != "" {
		if strings.Contains(errMsg, "自动重登录失败") {
			logCommandTrace(ctx, "answer_sheet.relogin.failed", map[string]any{
				"msg": strings.TrimPrefix(errMsg, "* 错误: 自动重登录失败: "),
			})
		}
		return errMsg
	}
	fetch.urls, fetch.urlSource = extractAnswerSheetURLs(fetch.subInfo)
	if fetch.subInfo["getSuccess"] != true || asInt(fetch.subInfo["code"]) != 0 || len(fetch.urls) == 0 {
		logCommandTrace(ctx, "answer_sheet.relogin.fetch_result", map[string]any{
			"code":        asInt(fetch.subInfo["code"]),
			"data_keys":   mapKeysJoined(asMap(fetch.subInfo["data"])),
			"data_type":   fmt.Sprintf("%T", fetch.subInfo["data"]),
			"get_success": fetch.subInfo["getSuccess"] == true,
			"msg":         asString(fetch.subInfo["msg"]),
			"url_count":   len(fetch.urls),
			"url_source":  fetch.urlSource,
		})
	}
	if asInt(fetch.subInfo["code"]) != 0 {
		return "* 错误: 未查询到科目信息，请确认已获取对应考试详情。"
	}
	return h.sendAnswerSheetImages(ctx, fetch)
}
