package main

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

var qtGradeRankPercent = map[string][2]float64{
	"A1": {0, 1},
	"A2": {1, 3},
	"A3": {3, 6},
	"A4": {6, 10},
	"A5": {10, 15},
	"B1": {15, 21},
	"B2": {21, 28},
	"B3": {28, 36},
	"B4": {36, 43},
	"B5": {43, 50},
	"C1": {50, 56},
	"C2": {56, 64},
	"C3": {64, 71},
	"C4": {71, 78},
	"C5": {78, 84},
	"D1": {84, 89},
	"D2": {89, 93},
	"D3": {93, 96},
	"D4": {96, 98},
	"D5": {98, 99},
	"E":  {99, 100},
}

var qtSubjectShortIDPattern = regexp.MustCompile(`^\d{1,3}$`)

func qtMapSlice(items []any) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if mapped := asMap(item); len(mapped) > 0 {
			result = append(result, mapped)
		}
	}
	return result
}

func buildUniqueNumericPrefixes(digits map[string]string, minLen int) map[string]string {
	lengths := make(map[string]int, len(digits))
	for key := range digits {
		lengths[key] = minLen
	}

	for {
		buckets := make(map[string][]string, len(digits))
		for key, digitString := range digits {
			length := lengths[key]
			if length > len(digitString) {
				length = len(digitString)
			}
			buckets[digitString[:length]] = append(buckets[digitString[:length]], key)
		}

		hasConflict := false
		for _, keys := range buckets {
			if len(keys) <= 1 {
				continue
			}
			hasConflict = true
			for _, key := range keys {
				lengths[key]++
			}
		}
		if !hasConflict {
			break
		}
	}

	result := make(map[string]string, len(digits))
	for key, digitString := range digits {
		length := lengths[key]
		if length > len(digitString) {
			length = len(digitString)
		}
		result[key] = digitString[:length]
	}
	return result
}

func buildQTExamShortIDs(exams []map[string]any) map[string]string {
	digits := make(map[string]string, len(exams))
	for _, exam := range exams {
		examGuid := asString(exam["examGuid"])
		if examGuid == "" {
			continue
		}
		digits[examGuid] = qtHashDigits(examGuid)
	}
	return buildUniqueNumericPrefixes(digits, 6)
}

func qtClaimedExamList(data any) []map[string]any {
	return qtMapSlice(asSlice(asMap(data)["list"]))
}

func qtFlattenUnclaimedExams(data any) []map[string]any {
	groups := qtMapSlice(asSlice(data))
	items := make([]map[string]any, 0)
	for _, group := range groups {
		items = append(items, qtMapSlice(asSlice(group["list"]))...)
	}
	return items
}

func qtEstimateRankRange(grade string, total int) (int, int, bool) {
	ratio, ok := qtGradeRankPercent[strings.ToUpper(strings.TrimSpace(grade))]
	if !ok || total <= 0 {
		return 0, 0, false
	}
	start := 1
	if ratio[0] > 0 {
		start = int(math.Ceil(float64(total)*ratio[0]/100)) + 1
	}
	end := int(math.Ceil(float64(total) * ratio[1] / 100))
	if end < start {
		end = start
	}
	if end > total {
		end = total
	}
	if start > total {
		start = total
	}
	return start, end, true
}

func qtVisibleSubjects(subjectsData map[string]any) []map[string]any {
	subjects := qtMapSlice(asSlice(subjectsData["subjects"]))
	result := make([]map[string]any, 0, len(subjects))
	for _, subject := range subjects {
		if asString(subject["km"]) == "总分" {
			continue
		}
		if asString(asMap(subject["question"])["asiresponse"]) == "" {
			continue
		}
		result = append(result, subject)
	}
	return result
}

func qtSubjectCount(subjectsData map[string]any) int {
	if count := asInt(subjectsData["subjectCount"]); count > 0 {
		return count
	}
	return len(qtVisibleSubjects(subjectsData))
}

func qtTotalSubject(subjectsData map[string]any) map[string]any {
	for _, subject := range qtMapSlice(asSlice(subjectsData["subjects"])) {
		if asString(subject["km"]) == "总分" {
			return subject
		}
	}
	return map[string]any{}
}

func qtBuildSubjectContext(exam map[string]any, subjectsData map[string]any) map[string]any {
	items := map[string]any{}
	aliases := map[string]any{}
	visible := qtVisibleSubjects(subjectsData)
	for index, subject := range visible {
		shortID := fmt.Sprintf("%03d", index+1)
		entry := map[string]any{
			"shortID":     shortID,
			"subject":     asString(subject["km"]),
			"asiResponse": asString(asMap(subject["question"])["asiresponse"]),
			"score":       subject["myScore"],
			"fullScore":   subject["fullScore"],
		}
		items[shortID] = entry
		aliases[asString(subject["km"])] = shortID
	}
	return map[string]any{
		"__provider":     "qt",
		"__exam":         exam,
		"__items":        items,
		"__aliases":      aliases,
		"__subjectCount": qtSubjectCount(subjectsData),
	}
}

func qtRenderExamOverview(exam map[string]any, shortID string, subjectsData, totalGradeRes map[string]any) string {
	report := asMap(asMap(totalGradeRes["data"])["report"])
	totalSubject := qtTotalSubject(subjectsData)

	score := defaultString(asString(report["myScore"]), defaultString(asString(totalSubject["myScore"]), asString(exam["score"])))
	fullScore := defaultString(asString(report["fullScore"]), asString(totalSubject["fullScore"]))
	totalStudents := asInt(report["total"])
	grade := strings.ToUpper(asString(report["grade"]))

	text := asString(exam["examName"])
	text += fmt.Sprintf("\n > 考试ID %s | 时间 %s", shortID, stringOrNA(exam["time"]))
	if !(score == "0" && fullScore == "0") {
		text += fmt.Sprintf("\n > 总分 %s/%s", stringOrNA(score), stringOrNA(fullScore))
	}

	if totalStudents > 0 && grade != "" {
		text += fmt.Sprintf("\n > 年级人数 %d | 等第 %s", totalStudents, grade)
		if start, end, ok := qtEstimateRankRange(grade, totalStudents); ok {
			if start == end {
				text += fmt.Sprintf("\n > 预估排名（可能有错误） %d", start)
			} else {
				text += fmt.Sprintf("\n > 预估排名（可能有错误） %d-%d", start, end)
			}
		}
	} else if totalStudents > 0 {
		text += fmt.Sprintf("\n > 年级人数 %d", totalStudents)
	} else if grade != "" {
		text += fmt.Sprintf("\n > 等第 %s", grade)
	}

	text += "\n > 科目短号 | 科目 | 分数/满分\n"
	for index, subject := range qtVisibleSubjects(subjectsData) {
		text += fmt.Sprintf(" > %03d | %s | %s/%s\n", index+1, stringOrNA(subject["km"]), stringOrNA(subject["myScore"]), stringOrNA(subject["fullScore"]))
	}
	text = strings.TrimRight(text, "\n")
	text += "\n回复：/答题详情 [科目短号] 以获取详细信息。"
	return text
}

func qtResolveExamSelector(selector string, exams []map[string]any) (map[string]any, string) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, "* 错误: 指令格式有误！\n格式：/考试详情 [考试ID]\n示例：/考试详情 123456"
	}

	if strings.Contains(selector, "-") {
		for _, exam := range exams {
			if asString(exam["examGuid"]) == selector {
				return exam, ""
			}
		}
		return nil, "* 错误: 未找到对应考试，请先使用 /查询 确认考试是否存在。"
	}

	shortIDs := buildQTExamShortIDs(exams)
	for _, exam := range exams {
		if shortIDs[asString(exam["examGuid"])] == selector {
			return exam, ""
		}
	}
	return nil, "* 错误: 未找到对应考试，请先使用 /查询 确认考试ID是否正确。"
}

func qtSubjectSelector(content, command string) (string, string) {
	selector := strings.TrimSpace(strings.TrimPrefix(content, command))
	if selector == "" {
		return "", fmt.Sprintf("* 错误: 指令格式有误！\n格式：%s [科目短号]\n示例：%s 001", command, command)
	}
	if qtSubjectShortIDPattern.MatchString(selector) {
		return fmt.Sprintf("%03d", asInt(selector)), ""
	}
	return selector, ""
}

func qtLoadStoredSubjectContext(ctx *MessageContext, selector string) (map[string]any, map[string]any, map[string]any, string) {
	examContext := opViewExamContext(ctx.UserID)
	if examContext["Return"] != true {
		return nil, nil, nil, "* 错误: 未找到前置考试信息，请先使用 /考试详情 查询一次考试。"
	}
	subjectMap := asMap(examContext["subject_map"])
	if asString(subjectMap["__provider"]) != "qt" {
		return nil, nil, nil, "* 错误: 当前前置考试信息不属于七天网络，请重新使用 /考试详情 查询一次七天考试。"
	}

	items := asMap(subjectMap["__items"])
	aliases := asMap(subjectMap["__aliases"])
	if alias := asString(aliases[selector]); alias != "" {
		selector = alias
	}
	subject := asMap(items[selector])
	if len(subject) == 0 {
		return nil, nil, nil, "* 错误: 未查询到科目信息，请确认已获取对应考试详情。"
	}
	exam := asMap(subjectMap["__exam"])
	if len(exam) == 0 || asString(exam["examGuid"]) == "" {
		return nil, nil, nil, "* 错误: 前置考试上下文不完整，请重新使用 /考试详情 获取一次考试详情。"
	}
	return exam, subject, subjectMap, ""
}

func qtStoredSubjectCount(subjectMap map[string]any) int {
	if count := asInt(subjectMap["__subjectCount"]); count > 0 {
		return count
	}
	return len(asMap(subjectMap["__items"]))
}

func qtRenderSubjectGradeDetail(subject map[string]any, gradeRes map[string]any) string {
	report := asMap(asMap(gradeRes["data"])["report"])
	subjectName := defaultString(asString(subject["subject"]), "该科目")
	score := defaultString(asString(report["myScore"]), asString(subject["score"]))
	fullScore := defaultString(asString(report["fullScore"]), asString(subject["fullScore"]))
	totalStudents := asInt(report["total"])
	grade := strings.ToUpper(asString(report["grade"]))

	text := fmt.Sprintf(" > %s ", subjectName)
	text += fmt.Sprintf("分数 %s/%s", stringOrNA(score), stringOrNA(fullScore))
	if totalStudents > 0 && grade != "" {
		text += fmt.Sprintf("\n > 年级人数 %d | 等第 %s", totalStudents, grade)
		if start, end, ok := qtEstimateRankRange(grade, totalStudents); ok {
			if start == end {
				text += fmt.Sprintf("\n > 预估年级排名 %d", start)
			} else {
				text += fmt.Sprintf("\n > 预估年级排名 %d-%d", start, end)
			}
		}
	} else if totalStudents > 0 {
		text += fmt.Sprintf("\n > 年级人数 %d", totalStudents)
	} else if grade != "" {
		text += fmt.Sprintf("\n > 等第 %s", grade)
	}
	return text
}

func qtTeacherLookupEnabled(teadata map[string]any) bool {
	return teadata["Return"] == true && asString(teadata["tofenxi"]) != "FAILED"
}

func (h *CommandHandler) qtTeacherMarkFailed(teadata map[string]any, scene, reason string) {
	schoolKey := asString(teadata["school"])
	if schoolKey == "" {
		return
	}
	opWriteTeacher(schoolKey, map[string]any{"tofenxi": "FAILED"})
	moonSendMsg(appConfig.MoonGroupID, fmt.Sprintf(
		"【机器人服务推送】\n七天教师端已标记FAILED\n学校：%s\n教师账号：%s\n触发场景：%s\n失败原因：%s",
		schoolKey,
		asString(teadata["account"]),
		scene,
		defaultString(reason, "未知错误"),
	))
}

type qtTeacherLoginAttemptResult struct {
	Updated           map[string]any
	Success           bool
	ForcedLogout      bool
	CredentialInvalid bool
	Reason            string
}

func (h *CommandHandler) qtTeacherLogin(ctx *MessageContext, teadata map[string]any) (map[string]any, bool) {
	return h.qtTeacherLoginWithMode(ctx, teadata, qtTeacherLoginModeFromAny(teadata["login_mode"]))
}

func (h *CommandHandler) qtTeacherLoginWithMode(ctx *MessageContext, teadata map[string]any, preferredMode qtTeacherLoginMode) (map[string]any, bool) {
	result := h.qtTeacherLoginSingleMode(ctx, teadata, preferredMode)
	switch {
	case result.Success:
		return result.Updated, true
	case result.CredentialInvalid:
		h.qtTeacherMarkFailed(teadata, "登录失败", result.Reason)
		return nil, false
	case result.ForcedLogout && preferredMode == qtTeacherLoginModeWeb:
		fallbackResult := h.qtTeacherLoginSingleMode(ctx, teadata, qtTeacherLoginModeApp)
		switch {
		case fallbackResult.Success:
			return fallbackResult.Updated, true
		case fallbackResult.CredentialInvalid:
			h.qtTeacherMarkFailed(teadata, "登录失败", fallbackResult.Reason)
		case fallbackResult.ForcedLogout:
			h.qtTeacherMarkFailed(teadata, "被顶号", fallbackResult.Reason)
		}
		return nil, false
	case result.ForcedLogout:
		h.qtTeacherMarkFailed(teadata, "被顶号", result.Reason)
		return nil, false
	default:
		return nil, false
	}
}

func (h *CommandHandler) qtTeacherLoginSingleMode(ctx *MessageContext, teadata map[string]any, mode qtTeacherLoginMode) qtTeacherLoginAttemptResult {
	schoolKey := asString(teadata["school"])
	username := asString(teadata["account"])
	password := asString(teadata["password"])
	if schoolKey == "" || username == "" || password == "" {
		return qtTeacherLoginAttemptResult{}
	}

	lastReason := ""
	for attempt := 0; attempt < 2; attempt++ {
		client := newQTTeacherClient("")
		loginRes := client.loginWithModeWithContext(ctx, username, password, mode)
		switch {
		case qtTeacherIsCredentialInvalid(loginRes):
			return qtTeacherLoginAttemptResult{
				CredentialInvalid: true,
				Reason:            asString(loginRes["msg"]),
			}
		case qtTeacherIsForcedLogout(loginRes):
			return qtTeacherLoginAttemptResult{
				ForcedLogout: true,
				Reason:       asString(loginRes["msg"]),
			}
		case qtTeacherShouldRelogin(loginRes):
			lastReason = asString(loginRes["msg"])
			continue
		case loginRes["getSuccess"] != true:
			return qtTeacherLoginAttemptResult{Reason: asString(loginRes["msg"])}
		}

		type tenantListResult struct {
			role int
			res  map[string]any
		}
		tenantListCh := make(chan tenantListResult, 2)
		for _, role := range []int{1, 2} {
			go func(role int) {
				tenantListCh <- tenantListResult{
					role: role,
					res:  client.tenantListWithContext(ctx, role),
				}
			}(role)
		}

		var roleOneRes map[string]any
		var roleTwoRes map[string]any
		for i := 0; i < 2; i++ {
			result := <-tenantListCh
			if result.role == 1 {
				roleOneRes = result.res
			} else {
				roleTwoRes = result.res
			}
		}

		switch {
		case qtTeacherIsForcedLogout(roleOneRes):
			return qtTeacherLoginAttemptResult{
				ForcedLogout: true,
				Reason:       asString(roleOneRes["msg"]),
			}
		case qtTeacherShouldRelogin(roleOneRes):
			lastReason = asString(roleOneRes["msg"])
			continue
		case roleOneRes["getSuccess"] != true:
			return qtTeacherLoginAttemptResult{Reason: asString(roleOneRes["msg"])}
		}

		roleOneTenants := asSlice(asMap(roleOneRes["data"])["list"])
		switch {
		case qtTeacherIsForcedLogout(roleTwoRes):
			return qtTeacherLoginAttemptResult{
				ForcedLogout: true,
				Reason:       asString(roleTwoRes["msg"]),
			}
		case qtTeacherShouldRelogin(roleTwoRes):
			lastReason = asString(roleTwoRes["msg"])
			continue
		case roleTwoRes["getSuccess"] != true:
			return qtTeacherLoginAttemptResult{Reason: asString(roleTwoRes["msg"])}
		}
		roleTwoTenants := asSlice(asMap(roleTwoRes["data"])["list"])

		globalRole := 0
		tenantCode := qtTeacherSelectTenantCode(schoolKey, 1, roleOneTenants)
		if tenantCode != "" {
			globalRole = 1
		} else {
			tenantCode = qtTeacherSelectTenantCode(schoolKey, 2, roleTwoTenants)
			if tenantCode != "" {
				globalRole = 2
			}
		}
		if globalRole == 0 {
			h.qtTeacherMarkFailed(
				teadata,
				"学校角色匹配失败",
				fmt.Sprintf(
					"未找到学校名完全一致的租户，目标学校：%s，roleType=1 可选学校：%s，roleType=2 可选学校：%s",
					qtTeacherTargetSchoolName(schoolKey),
					strings.Join(qtTeacherTenantNames(roleOneTenants), "、"),
					strings.Join(qtTeacherTenantNames(roleTwoTenants), "、"),
				),
			)
			return qtTeacherLoginAttemptResult{}
		}
		if tenantCode == "" {
			return qtTeacherLoginAttemptResult{}
		}

		entryRes := client.entryTenantWithContext(ctx, tenantCode, globalRole)
		switch {
		case qtTeacherIsForcedLogout(entryRes):
			return qtTeacherLoginAttemptResult{
				ForcedLogout: true,
				Reason:       asString(entryRes["msg"]),
			}
		case qtTeacherShouldRelogin(entryRes):
			lastReason = asString(entryRes["msg"])
			continue
		case entryRes["getSuccess"] != true:
			return qtTeacherLoginAttemptResult{Reason: asString(entryRes["msg"])}
		}

		opWriteTeacher(schoolKey, map[string]any{
			"cookie":     client.token,
			"cookie_fx":  tenantCode,
			"login_mode": string(mode),
		})
		return qtTeacherLoginAttemptResult{
			Updated: opViewTeacher(schoolKey),
			Success: true,
		}
	}
	return qtTeacherLoginAttemptResult{Reason: lastReason}
}

func (h *CommandHandler) qtTeacherRecoverFromAuthError(ctx *MessageContext, teadata, response map[string]any, reloginUsed, fallbackUsed *bool) (map[string]any, bool) {
	currentMode := qtTeacherLoginModeFromAny(teadata["login_mode"])
	switch {
	case qtTeacherShouldRelogin(response):
		if *reloginUsed {
			return nil, false
		}
		updated, ok := h.qtTeacherLoginWithMode(ctx, teadata, currentMode)
		if !ok {
			return nil, false
		}
		*reloginUsed = true
		return updated, true
	case qtTeacherIsForcedLogout(response):
		if currentMode == qtTeacherLoginModeWeb && !*fallbackUsed {
			updated, ok := h.qtTeacherLoginWithMode(ctx, teadata, qtTeacherLoginModeApp)
			if !ok {
				return nil, false
			}
			*fallbackUsed = true
			return updated, true
		}
		h.qtTeacherMarkFailed(teadata, "被顶号", asString(response["msg"]))
		return nil, false
	default:
		return nil, false
	}
}

func (h *CommandHandler) qtTeacherResolveRuleRef(ctx *MessageContext, teadata map[string]any, examGuid string, forceRefresh bool) (qtTeacherRuleRef, any, map[string]any) {
	schoolKey := asString(teadata["school"])
	if !forceRefresh {
		if cachedRule := opViewQTTeacherRuleCache(schoolKey, examGuid); cachedRule.Valid() {
			return cachedRule, nil, nil
		}
	}

	client := newQTTeacherClient(asString(teadata["cookie"]))
	ruleRes := client.examInfoRuleListWithContext(ctx, examGuid)
	if ruleRes["getSuccess"] != true {
		return qtTeacherRuleRef{}, nil, ruleRes
	}

	rule := qtTeacherSelectRule(ruleRes["data"])
	if !rule.Valid() {
		return qtTeacherRuleRef{}, ruleRes["data"], map[string]any{"getSuccess": true, "skip": true}
	}
	opWriteQTTeacherRuleCache(schoolKey, examGuid, rule.RuleGuid, rule.RuleType, qtTeacherRuleCacheTTL)
	return rule, ruleRes["data"], nil
}

func qtTeacherDecodeOverallCachePayload(payload string) map[string]any {
	if strings.TrimSpace(payload) == "" {
		return nil
	}
	data := map[string]any{}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return nil
	}
	return map[string]any{
		"getSuccess": true,
		"data":       data,
	}
}

func (h *CommandHandler) qtTeacherResolveOverallReport(ctx *MessageContext, teadata map[string]any, examGuid string, rule qtTeacherRuleRef, forceRefresh bool) (map[string]any, map[string]any) {
	schoolKey := asString(teadata["school"])
	examRuCode := asString(teadata["cookie_fx"])
	if schoolKey == "" || examRuCode == "" || examGuid == "" || !rule.Valid() {
		return nil, map[string]any{"getSuccess": false}
	}

	if !forceRefresh {
		if payload := opViewQTTeacherOverallCache(schoolKey, examRuCode, examGuid, rule.RuleGuid); payload != "" {
			if cached := qtTeacherDecodeOverallCachePayload(payload); len(cached) > 0 {
				return cached, nil
			}
		}
	}

	client := newQTTeacherClient(asString(teadata["cookie"]))
	reportRes := client.singleSubjectAvgReportWithContext(ctx, examRuCode, examGuid, rule.RuleGuid)
	if reportRes["getSuccess"] != true {
		return nil, reportRes
	}

	if raw, err := json.Marshal(reportRes["data"]); err == nil {
		opWriteQTTeacherOverallCache(schoolKey, examRuCode, examGuid, rule.RuleGuid, string(raw), qtTeacherOverallTTL)
	}
	return reportRes, nil
}

func (h *CommandHandler) qtTeacherResolveSupplementalRuleRef(ctx *MessageContext, teadata map[string]any, examGuid string, primary qtTeacherRuleRef, rulesData any) qtTeacherRuleRef {
	if !primary.Valid() || primary.RuleType != 0 {
		return qtTeacherRuleRef{}
	}
	if rulesData != nil {
		rule := qtTeacherSelectSupplementalRule(rulesData)
		if !rule.Valid() || rule.RuleGuid == primary.RuleGuid {
			return qtTeacherRuleRef{}
		}
		return rule
	}

	client := newQTTeacherClient(asString(teadata["cookie"]))
	ruleRes := client.examInfoRuleListWithContext(ctx, examGuid)
	if ruleRes["getSuccess"] != true {
		return qtTeacherRuleRef{}
	}
	rule := qtTeacherSelectSupplementalRule(ruleRes["data"])
	if !rule.Valid() || rule.RuleGuid == primary.RuleGuid {
		return qtTeacherRuleRef{}
	}
	return rule
}

type qtTeacherAnalysisMessages struct {
	Main  string
	Group string
}

func qtTeacherWrapAnalysisTitle(title, text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if strings.TrimSpace(title) == "" {
		return text
	}
	return "===== " + title + " =====\n" + text
}

func qtTeacherSupplementalGroupTitle(selectCombination int) string {
	switch selectCombination {
	case 10:
		return "赋分报告（物理组）"
	case 20:
		return "赋分报告（历史组）"
	default:
		return "赋分报告（分组）"
	}
}

func (h *CommandHandler) qtTeacherRenderRuleAnalysis(ctx *MessageContext, teadata map[string]any, examGuid, studentCode string, rule qtTeacherRuleRef, mainIncludeExam bool, groupTitle string) qtTeacherAnalysisMessages {
	if !rule.Valid() {
		return qtTeacherAnalysisMessages{}
	}

	overallRes, overallErr := h.qtTeacherResolveOverallReport(ctx, teadata, examGuid, rule, false)
	if overallErr != nil {
		return qtTeacherAnalysisMessages{}
	}

	client := newQTTeacherClient(asString(teadata["cookie"]))
	scoreRes := client.comprehensiveScoreListV2WithContext(ctx, examGuid, rule.RuleGuid, studentCode, -1)
	switch {
	case qtTeacherIsForcedLogout(scoreRes):
		return qtTeacherAnalysisMessages{}
	case qtTeacherShouldRelogin(scoreRes):
		return qtTeacherAnalysisMessages{}
	case qtTeacherStudentNoPermission(scoreRes):
		return qtTeacherAnalysisMessages{}
	case scoreRes["getSuccess"] != true:
		return qtTeacherAnalysisMessages{}
	case !qtTeacherHasStudentResult(scoreRes):
		return qtTeacherAnalysisMessages{}
	}

	groupRes := map[string]any{}
	resolvedGroupTitle := groupTitle
	if rule.RuleType == 2 {
		combination := qtTeacherSelectCombination(qtTeacherFirstStudentRow(scoreRes))
		if combination != 0 {
			if strings.TrimSpace(resolvedGroupTitle) == "" {
				resolvedGroupTitle = qtTeacherSupplementalGroupTitle(combination)
			}
			groupRes = client.comprehensiveScoreListV2WithContext(ctx, examGuid, rule.RuleGuid, studentCode, combination)
			switch {
			case qtTeacherIsForcedLogout(groupRes):
				return qtTeacherAnalysisMessages{}
			case qtTeacherShouldRelogin(groupRes):
				return qtTeacherAnalysisMessages{}
			case qtTeacherStudentNoPermission(groupRes):
				groupRes = nil
			case groupRes["getSuccess"] != true:
				groupRes = nil
			case !qtTeacherHasStudentResult(groupRes):
				groupRes = nil
			}
		}
	}

	return qtTeacherAnalysisMessages{
		Main: func() string {
			if mainIncludeExam {
				return qtTeacherRenderAnalysis(overallRes, scoreRes)
			}
			return qtTeacherRenderPersonalData(scoreRes)
		}(),
		Group: qtTeacherRenderGroupRankingWithTitle(groupRes, resolvedGroupTitle),
	}
}

func (h *CommandHandler) qtLoadTeacherAnalysis(ctx *MessageContext, exam map[string]any) []string {
	examGuid := asString(exam["examGuid"])
	studentCode := asString(exam["studentCode"])
	if examGuid == "" || studentCode == "" {
		return nil
	}

	teadata := opViewTeacherQT(asString(h.userdata["school"]))
	if !qtTeacherLookupEnabled(teadata) {
		return nil
	}
	if asString(teadata["cookie"]) == "" || asString(teadata["cookie_fx"]) == "" {
		updated, ok := h.qtTeacherLogin(ctx, teadata)
		if !ok {
			return nil
		}
		teadata = updated
	}

	forceRuleRefresh := false
	reloginUsed := false
	fallbackToAppUsed := false
	for attempt := 0; attempt < 4; attempt++ {
		rule, rulesData, ruleRes := h.qtTeacherResolveRuleRef(ctx, teadata, examGuid, forceRuleRefresh)
		if ruleRes != nil {
			switch {
			case qtTeacherIsForcedLogout(ruleRes), qtTeacherShouldRelogin(ruleRes):
				updated, ok := h.qtTeacherRecoverFromAuthError(ctx, teadata, ruleRes, &reloginUsed, &fallbackToAppUsed)
				if !ok {
					return nil
				}
				teadata = updated
				continue
			case ruleRes["skip"] == true:
				return nil
			default:
				return nil
			}
		}

		overallRes, overallErr := h.qtTeacherResolveOverallReport(ctx, teadata, examGuid, rule, forceRuleRefresh)
		if overallErr != nil {
			switch {
			case qtTeacherIsForcedLogout(overallErr), qtTeacherShouldRelogin(overallErr):
				updated, ok := h.qtTeacherRecoverFromAuthError(ctx, teadata, overallErr, &reloginUsed, &fallbackToAppUsed)
				if !ok {
					return nil
				}
				teadata = updated
				continue
			case qtTeacherRuleDeleted(overallErr):
				if forceRuleRefresh {
					return nil
				}
				opDeleteQTTeacherRuleCache(asString(teadata["school"]), examGuid)
				forceRuleRefresh = true
				continue
			default:
				return nil
			}
		}

		client := newQTTeacherClient(asString(teadata["cookie"]))
		scoreRes := client.comprehensiveScoreListV2WithContext(ctx, examGuid, rule.RuleGuid, studentCode, -1)
		switch {
		case qtTeacherIsForcedLogout(scoreRes), qtTeacherShouldRelogin(scoreRes):
			updated, ok := h.qtTeacherRecoverFromAuthError(ctx, teadata, scoreRes, &reloginUsed, &fallbackToAppUsed)
			if !ok {
				return nil
			}
			teadata = updated
			continue
		case qtTeacherStudentNoPermission(scoreRes):
			return nil
		case scoreRes["getSuccess"] != true:
			return nil
		case !qtTeacherHasStudentResult(scoreRes):
			continue
		}

		groupRes := map[string]any{}
		if rule.RuleType == 2 {
			combination := qtTeacherSelectCombination(qtTeacherFirstStudentRow(scoreRes))
			if combination != 0 {
				groupRes = client.comprehensiveScoreListV2WithContext(ctx, examGuid, rule.RuleGuid, studentCode, combination)
				switch {
				case qtTeacherIsForcedLogout(groupRes), qtTeacherShouldRelogin(groupRes):
					updated, ok := h.qtTeacherRecoverFromAuthError(ctx, teadata, groupRes, &reloginUsed, &fallbackToAppUsed)
					if !ok {
						return nil
					}
					teadata = updated
					continue
				case qtTeacherStudentNoPermission(groupRes):
					groupRes = nil
				case groupRes["getSuccess"] != true:
					groupRes = nil
				case !qtTeacherHasStudentResult(groupRes):
					groupRes = nil
				}
			}
		}

		messages := make([]string, 0, 4)
		mainText := qtTeacherRenderAnalysis(overallRes, scoreRes)
		if strings.TrimSpace(mainText) != "" {
			messages = append(messages, mainText)
		}
		groupText := qtTeacherRenderGroupRankingWithTitle(groupRes, "物理/历史类排名")
		if strings.TrimSpace(groupText) != "" {
			messages = append(messages, groupText)
		}
		if len(messages) == 0 {
			return nil
		}

		extraRule := h.qtTeacherResolveSupplementalRuleRef(ctx, teadata, examGuid, rule, rulesData)
		extraMessages := h.qtTeacherRenderRuleAnalysis(ctx, teadata, examGuid, studentCode, extraRule, false, "")
		if strings.TrimSpace(extraMessages.Main) != "" {
			messages = append(messages, qtTeacherWrapAnalysisTitle("新高考赋分报告", extraMessages.Main))
		}
		if strings.TrimSpace(extraMessages.Group) != "" {
			messages = append(messages, extraMessages.Group)
		}
		return messages
	}
	return nil
}

func (h *CommandHandler) reloginQT(ctx *MessageContext) (map[string]any, string) {
	h.sender.SendText(messageContext(ctx), ctx, "* 凭证过期，正在尝试重新登录...")
	response := qtStudentLoginWithContext(ctx, asString(h.userdata["zh"]), asString(h.userdata["pw"]))
	if response["isSuccess"] != true {
		return nil, qtStudentReloginErrorMessage(asString(response["msg"]))
	}
	opWrite(ctx.UserID, map[string]any{
		"token":  response["token"],
		"name":   response["name"],
		"school": response["school"],
		"grade":  response["grade"],
	})
	h.userdata["token"] = response["token"]
	h.userdata["name"] = response["name"]
	h.userdata["school"] = response["school"]
	h.userdata["grade"] = response["grade"]
	userInfo := asMap(response["userInfo"])
	h.qtProfileToken = asString(response["token"])
	h.qtProfileData = userInfo
	return userInfo, ""
}

func qtStudentReloginErrorMessage(msg string) string {
	if strings.Contains(msg, "密码错误") {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			return messageQTPasswordChanged
		}
		return msg + "\n" + messageQTPasswordChanged
	}
	return msg
}

func qtStudentVisibleErrorMessage(msg string) string {
	errMsg := "* 错误: " + defaultString(msg, "未知错误")
	if errMsg == "* 错误: 访问受限！" {
		errMsg += "\n本消息由七天服务器返回，具体原因暂时未知。"
	}
	return errMsg
}

func (h *CommandHandler) qtLoadUserInfoWithRelogin(ctx *MessageContext) (string, map[string]any, string) {
	token := asString(h.userdata["token"])
	if token == "" {
		userInfo, errMsg := h.reloginQT(ctx)
		if errMsg != "" {
			return "", nil, "* 错误: 自动重登录失败: " + errMsg
		}
		return asString(h.userdata["token"]), userInfo, ""
	}
	if token == h.qtProfileToken && len(h.qtProfileData) > 0 {
		return token, h.qtProfileData, ""
	}

	res := qtGetUserInfoWithContext(ctx, token)
	if res["getSuccess"] == true {
		userInfo := asMap(res["data"])
		h.qtProfileToken = token
		h.qtProfileData = userInfo
		return token, userInfo, ""
	}
	if qtShouldRelogin(res) {
		userInfo, errMsg := h.reloginQT(ctx)
		if errMsg != "" {
			return "", nil, "* 错误: 自动重登录失败: " + errMsg
		}
		return asString(h.userdata["token"]), userInfo, ""
	}
	return "", nil, qtStudentVisibleErrorMessage(asString(res["msg"]))
}

func (h *CommandHandler) qtExecuteWithProfile(ctx *MessageContext, operation string, fn func(string, map[string]any) map[string]any) (map[string]any, map[string]any, string) {
	h.qtLastOperation = operation
	token, userInfo, errMsg := h.qtLoadUserInfoWithRelogin(ctx)
	if errMsg != "" {
		return nil, nil, errMsg
	}

	res := fn(token, userInfo)
	if res["getSuccess"] == true {
		h.qtLastOperation = ""
		return res, userInfo, ""
	}
	if qtShouldRelogin(res) {
		userInfo, errMsg = h.reloginQT(ctx)
		if errMsg != "" {
			return nil, nil, "* 错误: 自动重登录失败: " + errMsg
		}
		res = fn(asString(h.userdata["token"]), userInfo)
		if res["getSuccess"] == true {
			h.qtLastOperation = ""
			return res, userInfo, ""
		}
	}
	return nil, nil, qtStudentVisibleErrorMessage(asString(res["msg"]))
}

func (h *CommandHandler) qtFetchAllClaimedExams(ctx *MessageContext) ([]map[string]any, map[string]any, string) {
	all := make([]map[string]any, 0)
	var userInfo map[string]any
	for startIndex := 0; ; startIndex += 5 {
		res, info, errMsg := h.qtExecuteWithProfile(ctx, fmt.Sprintf("get_claim_exams:start=%d rows=5", startIndex), func(token string, userInfo map[string]any) map[string]any {
			return qtGetClaimExamsWithContext(ctx, token, userInfo, startIndex, 5)
		})
		if errMsg != "" {
			return nil, nil, errMsg
		}
		userInfo = info
		page := qtClaimedExamList(res["data"])
		all = append(all, page...)
		if len(page) < 5 {
			break
		}
	}
	return all, userInfo, ""
}

func (h *CommandHandler) qtLoadExamsWithAutoClaim(ctx *MessageContext) ([]map[string]any, []string, map[string]any, string) {
	exams, userInfo, errMsg := h.qtFetchAllClaimedExams(ctx)
	if errMsg != "" {
		return nil, nil, nil, errMsg
	}

	unclaimedRes, latestUserInfo, errMsg := h.qtExecuteWithProfile(ctx, "get_unclaim_exams", func(token string, userInfo map[string]any) map[string]any {
		return qtGetUnClaimExamsWithContext(ctx, token, userInfo)
	})
	if errMsg != "" {
		return nil, nil, nil, errMsg
	}
	if len(latestUserInfo) > 0 {
		userInfo = latestUserInfo
	}

	notes := make([]string, 0)
	autoClaimed := 0
	skippedMulti := 0
	for _, item := range qtFlattenUnclaimedExams(unclaimedRes["data"]) {
		candidates := asSlice(item["studentCodeList"])
		if len(candidates) != 1 {
			skippedMulti++
			continue
		}
		_, _, claimErr := h.qtExecuteWithProfile(ctx, fmt.Sprintf("claim_exam:exam_guid=%s", asString(item["examGuid"])), func(token string, userInfo map[string]any) map[string]any {
			return qtClaimExamWithContext(ctx, token, asString(item["examGuid"]), asString(candidates[0]))
		})
		if claimErr != "" {
			notes = append(notes, fmt.Sprintf("* 提示: 自动认领考试[%s]失败：%s", asString(item["examName"]), strings.TrimPrefix(claimErr, "* 错误: ")))
			continue
		}
		autoClaimed++
	}

	if autoClaimed > 0 {
		exams, userInfo, errMsg = h.qtFetchAllClaimedExams(ctx)
		if errMsg != "" {
			return nil, nil, nil, errMsg
		}
		notes = append(notes, fmt.Sprintf("* 提示: 已自动认领 %d 场未认领考试。", autoClaimed))
	}
	if skippedMulti > 0 {
		notes = append(notes, fmt.Sprintf("* 提示: 有 %d 场未认领考试存在多个 studentCode 候选，已跳过自动认领。", skippedMulti))
	}
	opWriteQTStudentExamCache(ctx.UserID, exams, qtStudentExamCacheTTL)
	return exams, notes, userInfo, ""
}

func renderQTExamList(exams []map[string]any, notes []string) string {
	if len(exams) == 0 {
		return "* 错误: 暂未查询到任何七天考试数据。"
	}
	shortIDs := buildQTExamShortIDs(exams)
	lines := make([]string, 0, len(exams)+len(notes)+2)
	lines = append(lines, "* 七天网络相关功能正在测试中，若有问题还请谅解。")
	lines = append(lines, "【七天网络考试列表】")
	lines = append(lines, "考试ID ✱ 考试名称(时间)")
	for _, exam := range exams {
		lines = append(lines, fmt.Sprintf("%s ✱ %s(%s)",
			shortIDs[asString(exam["examGuid"])],
			defaultString(asString(exam["examName"]), "未知"),
			stringOrNA(exam["time"]),
		))
	}
	if len(notes) > 0 {
		lines = append(lines, notes...)
	}
	lines = append(lines, "➤回复：/考试详情 [考试ID] 以查询详细信息。")
	return strings.Join(lines, "\n")
}

func isQTAccessRestrictedError(message string) bool {
	return strings.Contains(message, "访问受限")
}

func (h *CommandHandler) logQTAccessRestrictedQuery(ctx *MessageContext, errMsg string) {
	fields := map[string]any{
		"error":        errMsg,
		"operation":    h.qtLastOperation,
		"mode":         h.userdata["mode"],
		"account":      h.userdata["zh"],
		"bound_school": h.userdata["school"],
		"bound_name":   h.userdata["name"],
		"bound_grade":  h.userdata["grade"],
		"bound_class":  h.userdata["banji"],
		"bound_xuehao": h.userdata["xuehao"],
	}
	if userInfo := h.qtProfileData; len(userInfo) > 0 {
		fields["qt_user_code"] = userInfo["userCode"]
		fields["qt_school_name"] = userInfo["schoolName"]
		fields["qt_school_guid"] = userInfo["schoolGuid"]
		fields["qt_student_name"] = userInfo["studentName"]
		fields["qt_student_guid"] = userInfo["studentGuid"]
		fields["qt_grade"] = userInfo["grade"]
		fields["qt_current_grade"] = userInfo["currentGrade"]
	}
	logCommandTrace(ctx, "qt_query_access_restricted", fields)
}

func (h *CommandHandler) searchQTExams(ctx *MessageContext) string {
	if cachedExams := opViewQTStudentExamCache(ctx.UserID); len(cachedExams) > 0 {
		return renderQTExamList(cachedExams, nil)
	}

	exams, notes, _, errMsg := h.qtLoadExamsWithAutoClaim(ctx)
	if errMsg != "" {
		if isQTAccessRestrictedError(errMsg) {
			h.logQTAccessRestrictedQuery(ctx, errMsg)
		}
		return errMsg
	}
	return renderQTExamList(exams, notes)
}

func (h *CommandHandler) qtExamInfo(ctx *MessageContext) string {
	selector := strings.TrimSpace(strings.TrimPrefix(ctx.Content, "/考试详情"))
	var exams []map[string]any
	var exam map[string]any
	if cachedExams := opViewQTStudentExamCache(ctx.UserID); len(cachedExams) > 0 {
		if cachedExam, cachedErr := qtResolveExamSelector(selector, cachedExams); cachedErr == "" {
			exams = cachedExams
			exam = cachedExam
		}
	}
	if exam == nil {
		var errMsg string
		exams, _, _, errMsg = h.qtLoadExamsWithAutoClaim(ctx)
		if errMsg != "" {
			if isQTAccessRestrictedError(errMsg) {
				h.logQTAccessRestrictedQuery(ctx, errMsg)
			}
			return errMsg
		}
		exam, errMsg = qtResolveExamSelector(selector, exams)
		if errMsg != "" {
			return errMsg
		}
	}

	subjectsRes, userInfo, errMsg := h.qtExecuteWithProfile(ctx, fmt.Sprintf("question_subjects:exam_guid=%s", asString(exam["examGuid"])), func(token string, userInfo map[string]any) map[string]any {
		return qtGetQuestionSubjectsWithContext(ctx, token, userInfo, exam)
	})
	if errMsg != "" {
		if isQTAccessRestrictedError(errMsg) {
			h.logQTAccessRestrictedQuery(ctx, errMsg)
		}
		return errMsg
	}
	subjectsData := asMap(subjectsRes["data"])
	subjectCount := qtSubjectCount(subjectsData)
	if subjectCount == 0 {
		return "* 错误: 当前考试缺少可查询科目数据。"
	}

	gradeRes, _, errMsg := h.qtExecuteWithProfile(ctx, fmt.Sprintf("question_subject_grade:exam_guid=%s subject=总分", asString(exam["examGuid"])), func(token string, userInfo map[string]any) map[string]any {
		return qtGetQuestionSubjectGradeWithContext(ctx, token, userInfo, exam, "总分", subjectCount, 1)
	})
	if errMsg != "" {
		if isQTAccessRestrictedError(errMsg) {
			h.logQTAccessRestrictedQuery(ctx, errMsg)
		}
		return errMsg
	}

	shortIDs := buildQTExamShortIDs(exams)
	view := qtRenderExamOverview(exam, shortIDs[asString(exam["examGuid"])], subjectsData, gradeRes)
	context := qtBuildSubjectContext(exam, subjectsData)
	if len(asMap(context["__items"])) == 0 {
		return "* 错误: 当前考试缺少可查询答题卡科目。"
	}

	opWrite(ctx.UserID, map[string]any{"exam": asString(exam["examGuid"])})
	h.userdata["exam"] = asString(exam["examGuid"])
	opWriteExamContext(ctx.UserID, asString(exam["examGuid"]), context)

	if len(userInfo) > 0 {
		h.userdata["name"] = qtProfileStudentName(userInfo)
		h.userdata["school"] = qtMapSchoolName(asString(userInfo["schoolName"]))
		opWrite(ctx.UserID, map[string]any{
			"name":   qtProfileStudentName(userInfo),
			"school": qtMapSchoolName(asString(userInfo["schoolName"])),
		})
	}

	h.sender.SendText(messageContext(ctx), ctx, view)
	for _, message := range h.qtLoadTeacherAnalysis(ctx, exam) {
		if strings.TrimSpace(message) == "" {
			continue
		}
		h.sender.SendText(messageContext(ctx), ctx, message)
	}
	return ""
}

func (h *CommandHandler) qtGetSnapshot(ctx *MessageContext) string {
	response := qtStudentLoginWithContext(ctx, asString(h.userdata["zh"]), asString(h.userdata["pw"]))
	if response["isSuccess"] != true {
		return "* 错误: " + asString(response["msg"])
	}

	opWrite(ctx.UserID, map[string]any{
		"name":   response["name"],
		"school": response["school"],
		"grade":  response["grade"],
		"token":  response["token"],
	})
	h.userdata["name"] = response["name"]
	h.userdata["school"] = response["school"]
	h.userdata["grade"] = response["grade"]
	h.userdata["token"] = response["token"]
	return fmt.Sprintf("* 获取快照成功！\n[%s]%s(七天网络)", asString(response["school"]), asString(response["name"]))
}

func (h *CommandHandler) qtGetAnswerSheet(ctx *MessageContext, command string, isWatermark bool) string {
	selector, errMsg := qtSubjectSelector(ctx.Content, command)
	if errMsg != "" {
		return errMsg
	}
	exam, subject, subjectMap, errMsg := qtLoadStoredSubjectContext(ctx, selector)
	if errMsg != "" {
		return errMsg
	}

	if command == "/答题详情" {
		subjectCount := qtStoredSubjectCount(subjectMap)
		gradeRes, _, errMsg := h.qtExecuteWithProfile(ctx, fmt.Sprintf("question_subject_grade:exam_guid=%s subject=%s", asString(exam["examGuid"]), asString(subject["subject"])), func(token string, userInfo map[string]any) map[string]any {
			return qtGetQuestionSubjectGradeWithContext(ctx, token, userInfo, exam, asString(subject["subject"]), subjectCount, 1)
		})
		if errMsg != "" {
			return errMsg
		}
		h.sender.SendText(messageContext(ctx), ctx, qtRenderSubjectGradeDetail(subject, gradeRes))
	}

	answerRes, _, errMsg := h.qtExecuteWithProfile(ctx, fmt.Sprintf("question_answer_card_url:exam_guid=%s subject=%s", asString(exam["examGuid"]), asString(subject["subject"])), func(token string, userInfo map[string]any) map[string]any {
		return qtGetQuestionAnswerCardURLWithContext(ctx, token, userInfo, exam, subject, isWatermark)
	})
	if errMsg != "" {
		return errMsg
	}
	urls, urlSource := extractAnswerSheetURLs(answerRes)
	fetch := answerSheetFetch{
		subjectID: asString(subject["shortID"]),
		subInfo:   answerRes,
		urls:      urls,
		urlSource: urlSource,
	}
	return h.sendAnswerSheetImages(ctx, fetch)
}

func qtStudentLoginWithContext(ctx *MessageContext, username, password string) map[string]any {
	return qtLoginAndSnapshotWithContext(ctx, username, password)
}
