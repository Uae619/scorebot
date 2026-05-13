package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type examOverviewView struct {
	summary      string
	overviewData map[string]any
}

var examIDPattern = regexp.MustCompile(`\d+`)

func shouldReloginExamList(response map[string]any) bool {
	return response["retryRelogin"] == true ||
		asInt(response["code"]) == 3001 ||
		asInt(response["code"]) == 1000
}

func shouldReloginExamOverview(response map[string]any) bool {
	msg := asString(response["msg"])
	return asInt(response["code"]) == 3001 ||
		asInt(response["code"]) == 1000 ||
		(asInt(response["code"]) == 10002 && msg == "服务出错，稍后再试")
}

func isHFSRiskLocked(response map[string]any) bool {
	msg := asString(response["msg"])
	return asInt(response["code"]) == 4031 || msg == "系统异常" || msg == "账号存在风险，已被锁定"
}

func isExamOverviewNoData(response map[string]any) bool {
	msg := asString(response["msg"])
	return msg == "暂无stu exam" ||
		(asInt(response["code"]) == 10003 && msg == "获取一场考试的成绩概览错误")
}

func containsErrorText(text string) bool {
	for _, item := range errorTextList {
		if item == text {
			return true
		}
	}
	return false
}

func parseExamID(content string) (string, int, string) {
	examID := examIDPattern.FindString(content)
	if examID == "" {
		return "", 0, "* 错误: 指令格式有误！\n格式：/考试详情 [考试ID]\n示例：/考试详情 1234567"
	}
	examIDInt, err := strconv.Atoi(examID)
	if err != nil {
		return "", 0, "* 错误: 考试ID必须是数字！"
	}
	if examIDInt <= 10000 {
		return "", 0, "* 错误: 考试ID有误，请输入正确的考试ID！"
	}
	if examIDInt >= 5000000 || examIDInt <= 1500000 {
		return "", 0, "* 错误: 考试ID有误，请输入正确的考试ID！"
	}
	return examID, examIDInt, ""
}

func isSixDigitExamSelector(content string) bool {
	examID := examIDPattern.FindString(content)
	return len(examID) == 6
}

func hfsAccountType(mode string) int {
	if mode == "parent" {
		return 2
	}
	return 1
}

func (h *CommandHandler) reloginStudentToken(ctx *MessageContext) (string, string) {
	h.sender.SendText(messageContext(ctx), ctx, "* 凭证过期，正在尝试重新登录...")
	response := studentLoginWithContext(ctx, asString(h.userdata["zh"]), asString(h.userdata["pw"]), hfsAccountType(asString(h.userdata["mode"])))
	if len(response) == 1 {
		return "", response[0]
	}
	opWrite(ctx.UserID, map[string]any{"token": response[1]})
	h.userdata["token"] = response[1]
	return response[1], ""
}

func (h *CommandHandler) loadExamListWithRelogin(ctx *MessageContext) (map[string]any, string) {
	resJSON := studentGetExamlistWithContext(ctx, asString(h.userdata["token"]))
	examList, examListSource := extractExamList(resJSON)
	if resJSON["getSuccess"] != true || len(examList) == 0 || examListSource != "data.list" {
		logCommandTrace(ctx, "search_exams.fetch_examlist.result", map[string]any{
			"code":             asInt(resJSON["code"]),
			"data_keys":        mapKeysJoined(asMap(resJSON["data"])),
			"data_type":        fmt.Sprintf("%T", resJSON["data"]),
			"exam_count":       len(examList),
			"exam_list_source": examListSource,
			"get_success":      resJSON["getSuccess"] == true,
			"msg":              asString(resJSON["msg"]),
		})
	}
	if resJSON["getSuccess"] != true {
		switch {
		case shouldReloginExamList(resJSON):
			if _, err := h.reloginStudentToken(ctx); err != "" {
				logCommandTrace(ctx, "search_exams.relogin.failed", map[string]any{
					"msg": err,
				})
				return nil, "* 错误: 自动重登录失败: " + err
			}
			resJSON = studentGetExamlistWithContext(ctx, asString(h.userdata["token"]))
			examList, examListSource = extractExamList(resJSON)
			if resJSON["getSuccess"] != true || len(examList) == 0 || examListSource != "data.list" {
				logCommandTrace(ctx, "search_exams.relogin.fetch_examlist.result", map[string]any{
					"code":             asInt(resJSON["code"]),
					"data_keys":        mapKeysJoined(asMap(resJSON["data"])),
					"data_type":        fmt.Sprintf("%T", resJSON["data"]),
					"exam_count":       len(examList),
					"exam_list_source": examListSource,
					"get_success":      resJSON["getSuccess"] == true,
					"msg":              asString(resJSON["msg"]),
				})
			}
			if resJSON["getSuccess"] != true {
				return nil, "* 错误: 重登录后未获取到数据，请尝试重新绑定账号"
			}
		case isHFSRiskLocked(resJSON):
			return nil, "* 错误: 暂时无法获取考试列表，请尝试重新绑定账号。"
		default:
			return nil, "* 错误: " + defaultString(asString(resJSON["msg"]), "未知错误")
		}
	}
	if len(examList) == 0 {
		logCommandTrace(ctx, "search_exams.empty_list", map[string]any{
			"code":             asInt(resJSON["code"]),
			"data_keys":        mapKeysJoined(asMap(resJSON["data"])),
			"data_type":        fmt.Sprintf("%T", resJSON["data"]),
			"exam_list_source": examListSource,
			"msg":              asString(resJSON["msg"]),
			"mode":             asString(h.userdata["mode"]),
		})
	}
	return resJSON, ""
}

func (h *CommandHandler) loadExamOverviewWithRelogin(ctx *MessageContext, examID string) (map[string]any, string) {
	examOverview := map[string]any{"getSuccess": false, "code": 3001}
	if token := asString(h.userdata["token"]); token != "" {
		examOverview = studentGetExamInfoWithContext(ctx, token, examID)
	}
	if examOverview["getSuccess"] == true {
		return examOverview, ""
	}

	switch {
	case shouldReloginExamOverview(examOverview):
		if _, err := h.reloginStudentToken(ctx); err != "" {
			return nil, "* 错误: " + err
		}
		examOverview = studentGetExamInfoWithContext(ctx, asString(h.userdata["token"]), examID)
		if examOverview["getSuccess"] != true {
			// moonSendMsg(appConfig.MoonGroupID, fmt.Sprintf("【机器人服务推送】\n用户 %s 获取考试 %s 信息失败\n方法：student_get_exam_info\n返回信息：%v", ctx.UserID, examID, examOverview))
			return nil, "* 错误: 重登录后未获取到数据，请尝试重新绑定账号"
		}
	case isExamOverviewNoData(examOverview):
		return nil, "* 错误: 暂无数据，请确认考试ID是否正确、本场考试是否属于你的学校、是否参与本场考试及本场考试是否已有科目发布"
	case isHFSRiskLocked(examOverview):
		return nil, "* 错误: 账号被好分数风控机制命中，暂时无法查询。"
	default:
		return nil, "* 错误: " + defaultString(asString(examOverview["msg"]), "未知错误")
	}
	return examOverview, ""
}

func extractExamPaperContext(examOverview map[string]any) map[string]any {
	result := map[string]any{}
	for _, item := range asSlice(asMap(examOverview["data"])["papers"]) {
		paper := asMap(item)
		paperID := asString(paper["paperId"])
		pid := asString(paper["pid"])
		if paperID == "" || pid == "" {
			continue
		}
		result[paperID] = map[string]any{
			"paperId": paperID,
			"pid":     pid,
			"subject": asString(paper["subject"]),
			"score":   asString(paper["score"]),
			"manfen":  asString(paper["manfen"]),
		}
	}
	return result
}

func encodeExamPaperContext(subjectMap map[string]any) string {
	if len(subjectMap) == 0 {
		return ""
	}
	raw, err := json.Marshal(subjectMap)
	if err != nil {
		return ""
	}
	return string(raw)
}

func formatExamDate(value any) string {
	ms := int64(asInt(value))
	if ms <= 0 {
		return stringOrNA(value)
	}
	return time.UnixMilli(ms).In(time.Local).Format("2006/01/02")
}

func renderExamOverview(examOverview map[string]any) examOverviewView {
	data := asMap(examOverview["data"])
	lines := make([]string, 0, 6)
	lines = append(lines, asString(data["name"]))
	lines = append(lines, fmt.Sprintf(" > 考试ID %s | 时间 %s", stringOrNA(data["examId"]), formatExamDate(data["time"])))
	scoreLine := fmt.Sprintf(" > 总分 %s", stringOrNA(data["score"]))
	if asString(data["score"]) != asString(data["scoreBeforeGrading"]) {
		scoreLine += fmt.Sprintf("(原始%s)", stringOrNA(data["scoreBeforeGrading"]))
	}
	scoreLine += "/" + stringOrNA(data["manfen"])
	lines = append(lines, scoreLine)
	countLabel := "人数"
	if asInt(data["mode"]) == 3 {
		countLabel = "人数(分组)"
	}
	countLine := fmt.Sprintf(" > %s 校 %s 班 %s", countLabel, stringOrNA(data["gradeStuNum"]), stringOrNA(data["classStuNum"]))
	if rank := asString(asMap(data["compare"])["curGradeRank"]); rank != "" {
		countLine += " | 学校排名 " + rank
	}
	lines = append(lines, countLine)

	subjectLines := map[string]string{}
	for _, item := range asSlice(data["papers"]) {
		sub := asMap(item)
		line := fmt.Sprintf("%s | %s | %s", stringOrNA(sub["paperId"]), stringOrNA(sub["subject"]), stringOrNA(sub["score"]))
		if asInt(sub["gradingType"]) == 1 {
			line += fmt.Sprintf("(原%s)", stringOrNA(sub["scoreBeforeGrading"]))
		}
		line += "/" + stringOrNA(sub["manfen"])
		subjectLines[asString(sub["subject"])] = line
	}
	lines = append(lines, " > 科目ID-短号 | 科目 | 分数/满分")
	if len(subjectLines) > 0 {
		lines = append(lines, textformatSublistFromTeacher(subjectLines))
	}
	lines = append(lines, "回复：/答题详情 [科目ID-短号] 以查询科目得分情况。")
	return examOverviewView{
		summary:      strings.Join(lines, "\n"),
		overviewData: data,
	}
}

func (h *CommandHandler) loadTeacherAnalysis(ctx *MessageContext, examID string, teadata map[string]any, overviewData map[string]any) string {
	if asString(teadata["tofenxi"]) == "TRUE" {
		textFenxi := strings.ReplaceAll(fenxiSearchWithContext(messageContext(ctx), asString(teadata["cookie"]), examID, h.userdata["xuehao"], 1), "Response:\n", "")
		if containsErrorText(textFenxi) {
			h.sender.SendText(messageContext(ctx), ctx, "* 系统凭证过期，正在自动更新，请稍候..")
			result := h.teacherLogin(ctx, asString(h.userdata["school"]), asString(teadata["account"]), asString(teadata["password"]))
			if result["loginSuccess"] != true {
				return "* 错误: " + defaultString(asString(result["message"]), "教师登录失败")
			}
			teadata = opViewTeacher(asString(h.userdata["school"]))
			textFenxi = strings.ReplaceAll(fenxiSearchWithContext(messageContext(ctx), asString(teadata["cookie"]), examID, h.userdata["name"], 1), "Response:\n", "")
		}
		return textFenxi
	}
	if asString(teadata["tofenxi"]) == "FAILED" {
		return "本学校暂无相应查询能力，请使用其它查询方式。"
	}

	fenxiJSON := hfsjsAnalysisRankinfoWithContext(messageContext(ctx), asString(teadata["cookie"]), examID, h.userdata["id"])
	if fenxiJSON["isSuccess"] != true {
		h.sender.SendText(messageContext(ctx), ctx, "* 系统凭证过期，正在自动更新，请稍候..")
		result := h.teacherLogin(ctx, asString(h.userdata["school"]), asString(teadata["account"]), asString(teadata["password"]))
		if result["loginSuccess"] != true {
			return "* 错误: " + defaultString(asString(result["message"]), "教师登录失败")
		}
		teadata = opViewTeacher(asString(h.userdata["school"]))
		fenxiJSON = hfsjsAnalysisRankinfoWithContext(messageContext(ctx), asString(teadata["cookie"]), examID, h.userdata["id"])
		if fenxiJSON["isSuccess"] != true {
			return "* 错误: 重登录后未获取到数据，请联系管理员"
		}
	}

	fenxiPapersJSON := hfsjsAnalysisPapersWithContext(messageContext(ctx), asString(teadata["cookie"]), examID, h.userdata["id"])
	dataFenxi := asMap(asMap(fenxiJSON["data"])["data"])
	if len(dataFenxi) == 0 {
		return "* 错误: 暂无数据"
	}

	number := asMap(dataFenxi["number"])
	rank := asMap(dataFenxi["rank"])
	avg := asMap(dataFenxi["avg"])
	highest := asMap(dataFenxi["highest"])
	isLiankao := asInt(number["grade"]) != asInt(number["liankao"])

	fenxiLines := []string{"===== 考试数据 ====="}
	personalLines := []string{"===== 个人数据 ====="}
	totalLine := fmt.Sprintf("- 总分 %s [", stringOrNA(overviewData["score"]))
	if isLiankao {
		totalLine += fmt.Sprintf("联%s|", stringOrNA(rank["liankao"]))
	}
	totalLine += fmt.Sprintf("校%s|班%s]", stringOrNA(rank["grade"]), stringOrNA(rank["class"]))
	fenxiLines = append(fenxiLines, totalLine)
	personalLines = append(personalLines, totalLine)
	numberLine := "参考人数 |"
	if isLiankao {
		numberLine += fmt.Sprintf(" 联 %s", stringOrNA(number["liankao"]))
	}
	numberLine += fmt.Sprintf(" 校 %s 班 %s", stringOrNA(number["grade"]), stringOrNA(number["class"]))
	fenxiLines = append(fenxiLines, numberLine)
	avgLine := "平均分数 |"
	if isLiankao {
		avgLine += fmt.Sprintf(" 联 %s", stringOrNA(avg["liankao"]))
	}
	avgLine += fmt.Sprintf(" 校 %s 班 %s", stringOrNA(avg["grade"]), stringOrNA(avg["class"]))
	fenxiLines = append(fenxiLines, avgLine)
	highestLine := "最高分数 |"
	if isLiankao {
		highestLine += fmt.Sprintf(" 联 %s", stringOrNA(highest["liankao"]))
	}
	highestLine += fmt.Sprintf(" 校 %s 班 %s", stringOrNA(highest["grade"]), stringOrNA(highest["class"]))
	fenxiLines = append(fenxiLines, highestLine)

	for _, item := range asSlice(asMap(asMap(fenxiPapersJSON["data"])["data"])["papers"]) {
		sub := asMap(item)
		line := fmt.Sprintf("- %s %s [校%s|班%s]", asString(sub["subject"]), asString(sub["score"]), asString(sub["gradeRank"]), asString(sub["classRank"]))
		fenxiLines = append(fenxiLines,
			line,
			fmt.Sprintf("参考人数 | 校 %s 班 %s", asString(sub["gradeStuNum"]), asString(sub["classStuNum"])),
			fmt.Sprintf("平均分数 | 校 %s 班 %s", asString(sub["gradeAvg"]), asString(sub["classAvg"])),
		)
		personalLines = append(personalLines, line)
	}
	return strings.Join(fenxiLines, "\n") + "\n" + strings.Join(personalLines, "\n")
}

func (h *CommandHandler) examInfo(ctx *MessageContext) string {
	if ok, msg := h.requireAuth(ctx); !ok {
		return msg
	}
	if asString(h.userdata["mode"]) == "bfz" {
		return messageBFZUseFailedUse
	}
	if asString(h.userdata["mode"]) == "qt" {
		return h.qtExamInfo(ctx)
	}
	if isSixDigitExamSelector(ctx.Content) {
		return messageHFSUseQTExamID
	}

	examID, _, errMsg := parseExamID(ctx.Content)
	if errMsg != "" {
		return errMsg
	}

	opWrite(ctx.UserID, map[string]any{"exam": examID})
	h.userdata["exam"] = examID
	teadata := opViewTeacher(asString(h.userdata["school"]))

	examOverview, errMsg := h.loadExamOverviewWithRelogin(ctx, examID)
	if errMsg != "" {
		return errMsg
	}
	subjectMap := extractExamPaperContext(examOverview)
	if len(subjectMap) == 0 {
		return "* 错误: 考试详情缺少科目上下文信息，暂时无法继续查询答题卡。"
	}
	opWriteExamContext(ctx.UserID, examID, subjectMap)
	h.userdata["paper_map"] = encodeExamPaperContext(subjectMap)

	view := renderExamOverview(examOverview)
	h.sender.SendText(messageContext(ctx), ctx, view.summary)
	return h.loadTeacherAnalysis(ctx, examID, teadata, view.overviewData)
}

func (h *CommandHandler) searchExams(ctx *MessageContext) string {
	if ok, msg := h.requireAuth(ctx); !ok {
		return msg
	}
	if asString(h.userdata["mode"]) == "bfz" {
		return messageBFZUseFailedUse
	}
	if asString(h.userdata["mode"]) == "qt" {
		return h.searchQTExams(ctx)
	}

	resJSON, errMsg := h.loadExamListWithRelogin(ctx)
	if errMsg != "" {
		return errMsg
	}
	return textformatExamlist(resJSON)
}
