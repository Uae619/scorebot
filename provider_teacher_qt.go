package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	qtTeacherBaseURL      = "https://fx.7net.cc"
	qtTeacherAppBaseURL   = "https://teacherapi.7net.cc"
	qtTeacherRuleCacheTTL = 5 * time.Minute
	qtTeacherOverallTTL   = 5 * time.Minute
	qtTeacherPublicKeyB64 = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCxqJjphDro7NUh8qg20I1n/K8NfpwN8dImRD7ml9OhShmWHL2AupiuOky0ws3Cm4k+811jTqsXzGs3UWChTGOYaLzYjK9k/Sq+WWaGjlkezXom5iVINGyTr4mKoIEmtO5bKucJv2UKhVa0mc1aJC4bO1EvWcwScYz5jHfSlxnzoQIDAQAB"
	qtTeacherAppVersion   = "3.3.4"
	qtTeacherAppUserAgent = "okhttp/3.10.0"
	qtTeacherAppAESKey    = "septnet0000000000000000000000000"
)

var (
	qtTeacherPublicKey      = mustRSAPublicKeyB64(qtTeacherPublicKeyB64)
	qtTeacherAppCipherBlock = mustAESCipherBlock(qtTeacherAppAESKey)
)

type qtTeacherLoginMode string

const (
	qtTeacherLoginModeWeb qtTeacherLoginMode = "web"
	qtTeacherLoginModeApp qtTeacherLoginMode = "app"
)

type qtTeacherRuleRef struct {
	RuleGuid string
	RuleType int
}

type qtTeacherRuleCandidate struct {
	Ref       qtTeacherRuleRef
	IsDefault bool
}

func (r qtTeacherRuleRef) Valid() bool {
	return strings.TrimSpace(r.RuleGuid) != ""
}

func qtTeacherLoginModeFromAny(value any) qtTeacherLoginMode {
	switch strings.ToLower(strings.TrimSpace(asString(value))) {
	case string(qtTeacherLoginModeApp):
		return qtTeacherLoginModeApp
	default:
		return qtTeacherLoginModeWeb
	}
}

type qtTeacherClient struct {
	token string
}

func newQTTeacherClient(token string) *qtTeacherClient {
	return &qtTeacherClient{token: token}
}

func qtTeacherEncryptPassword(password string) (string, error) {
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, qtTeacherPublicKey, []byte(password))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func qtTeacherPKCS7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	result := make([]byte, len(data)+padding)
	copy(result, data)
	for i := len(data); i < len(result); i++ {
		result[i] = byte(padding)
	}
	return result
}

func qtTeacherEncryptAppCredential(value string) (string, error) {
	block := qtTeacherAppCipherBlock
	padded := qtTeacherPKCS7Pad([]byte(value), block.BlockSize())
	encrypted := make([]byte, len(padded))
	for start := 0; start < len(padded); start += block.BlockSize() {
		block.Encrypt(encrypted[start:start+block.BlockSize()], padded[start:start+block.BlockSize()])
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func qtTeacherHeaders() map[string]string {
	return map[string]string{
		"accept":          "application/json, text/plain, */*",
		"accept-language": "zh-CN,zh;q=0.9,en;q=0.8",
		"cache-control":   "no-cache",
		"content-type":    "application/json; charset=UTF-8",
		"origin":          qtTeacherBaseURL,
		"pragma":          "no-cache",
		"referer":         qtTeacherBaseURL + "/",
		"user-agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 Edg/146.0.0.0",
	}
}

func qtTeacherAppHeaders() map[string]string {
	return map[string]string{
		"accept":          "*/*",
		"accept-encoding": "gzip, deflate, br",
		"connection":      "keep-alive",
		"content-type":    "application/json",
		"host":            "teacherapi.7net.cc",
		"user-agent":      qtTeacherAppUserAgent,
		"version":         qtTeacherAppVersion,
	}
}

func (c *qtTeacherClient) requestWithContext(ctx *MessageContext, method, rawURL string, payload any) map[string]any {
	data, raw, err := doJSONRequestWithContext(messageContext(ctx), method, rawURL, payload, qtTeacherHeaders(), 20*time.Second)
	if err != nil {
		return map[string]any{"getSuccess": false, "code": -1, "msg": err.Error()}
	}
	logAdminThirdPartyRequest(ctx, "7net_teacher", method, rawURL, payload, raw, nil)

	status := asInt(data["status"])
	message := asString(data["message"])
	if status != 200 {
		return map[string]any{
			"getSuccess": false,
			"code":       status,
			"msg":        message,
			"data":       data["data"],
		}
	}
	return map[string]any{
		"getSuccess": true,
		"code":       status,
		"msg":        message,
		"data":       data["data"],
	}
}

func (c *qtTeacherClient) loginWithModeWithContext(ctx *MessageContext, username, password string, mode qtTeacherLoginMode) map[string]any {
	switch mode {
	case qtTeacherLoginModeApp:
		return c.loginAppWithContext(ctx, username, password)
	default:
		return c.loginWebWithContext(ctx, username, password)
	}
}

func (c *qtTeacherClient) loginWebWithContext(ctx *MessageContext, username, password string) map[string]any {
	encrypted, err := qtTeacherEncryptPassword(password)
	if err != nil {
		return map[string]any{"getSuccess": false, "code": -1, "msg": err.Error()}
	}
	res := c.requestWithContext(ctx, http.MethodPost, qtTeacherBaseURL+"/fx/User/Login", map[string]any{
		"userCode": username,
		"password": encrypted,
		"token":    "",
	})
	if res["getSuccess"] == true {
		c.token = asString(asMap(res["data"])["token"])
	}
	return res
}

func (c *qtTeacherClient) loginAppWithContext(ctx *MessageContext, username, password string) map[string]any {
	encryptedUserCode, err := qtTeacherEncryptAppCredential(username)
	if err != nil {
		return map[string]any{"getSuccess": false, "code": -1, "msg": err.Error()}
	}
	encryptedPassword, err := qtTeacherEncryptAppCredential(password)
	if err != nil {
		return map[string]any{"getSuccess": false, "code": -1, "msg": err.Error()}
	}

	payload := map[string]any{
		"userCode": encryptedUserCode,
		"password": encryptedPassword,
	}
	data, raw, err := doJSONRequestWithContext(messageContext(ctx), http.MethodPost, qtTeacherAppBaseURL+"/api/User/Login", payload, qtTeacherAppHeaders(), 20*time.Second)
	if err != nil {
		return map[string]any{"getSuccess": false, "code": -1, "msg": err.Error()}
	}

	rawForLog := any(raw)
	if parsed := map[string]any{}; json.Unmarshal(raw, &parsed) == nil {
		rawForLog = parsed
	}
	logAdminThirdPartyRequest(ctx, "7net_teacher", http.MethodPost, qtTeacherAppBaseURL+"/api/User/Login", payload, rawForLog, nil)

	status := asInt(data["status"])
	message := asString(data["message"])
	if status != 200 {
		return map[string]any{
			"getSuccess": false,
			"code":       status,
			"msg":        message,
			"data":       data["data"],
		}
	}

	c.token = asString(asMap(data["data"])["token"])
	return map[string]any{
		"getSuccess": true,
		"code":       status,
		"msg":        message,
		"data":       data["data"],
	}
}

func (c *qtTeacherClient) tenantListWithContext(ctx *MessageContext, globalRole int) map[string]any {
	query := url.Values{}
	query.Set("pageIndex", "1")
	query.Set("pageSize", "10")
	query.Set("codeOrName", "")
	query.Set("globalRole", fmt.Sprintf("%d", globalRole))
	query.Set("token", c.token)
	query.Set("v", fmt.Sprintf("%d", time.Now().UnixMilli()))
	return c.requestWithContext(ctx, http.MethodGet, qtTeacherBaseURL+"/fx/User/TenantList?"+query.Encode(), nil)
}

func (c *qtTeacherClient) entryTenantWithContext(ctx *MessageContext, code string, globalRole int) map[string]any {
	return c.requestWithContext(ctx, http.MethodPost, qtTeacherBaseURL+"/fx/User/EntryTenant", map[string]any{
		"code":       code,
		"globalRole": globalRole,
		"token":      c.token,
	})
}

func (c *qtTeacherClient) examInfoRuleListWithContext(ctx *MessageContext, examGuid string) map[string]any {
	return c.requestWithContext(ctx, http.MethodPost, qtTeacherBaseURL+"/fx/ExamInfo/ExamInfoRuleList", map[string]any{
		"examGuid": examGuid,
		"token":    c.token,
	})
}

func (c *qtTeacherClient) singleSubjectAvgReportWithContext(ctx *MessageContext, examRuCode, examGuid, ruleGuid string) map[string]any {
	return c.requestWithContext(ctx, http.MethodPost, qtTeacherBaseURL+"/fx/CommonUse/SingleSubjectAvgReport", map[string]any{
		"examRuCode": examRuCode,
		"examGuid":   examGuid,
		"ruleGuid":   ruleGuid,
		"ClassType":  2,
		"OrgCodes":   []any{},
		"ElectiveKM": "",
		"AvgSettion": map[string]any{
			"Source":   0,
			"Type":     1,
			"Ranking":  "100",
			"Ranking1": "100",
			"Ranking2": "100",
		},
		"OrderBy": "",
		"Km":      "总分",
		"token":   c.token,
	})
}

func (c *qtTeacherClient) comprehensiveScoreListV2WithContext(ctx *MessageContext, examGuid, ruleGuid, studentCode string, selectCombination int) map[string]any {
	return c.requestWithContext(ctx, http.MethodPost, qtTeacherBaseURL+"/fx/ComprehensiveScore/ComprehensiveScoreListV2", map[string]any{
		"Km":                "总分",
		"Schooles":          []any{},
		"Classes":           []any{},
		"ClassType":         2,
		"StudentNameOrCode": studentCode,
		"SelectCombination": selectCombination,
		"PageIndex":         1,
		"PageSize":          1,
		"ExamGuid":          examGuid,
		"RuleGuid":          ruleGuid,
		"OrderBy":           "",
		"token":             c.token,
	})
}

func qtTeacherShouldRelogin(response map[string]any) bool {
	return asInt(response["code"]) == 401 && asString(response["msg"]) == "登录超时"
}

func qtTeacherIsForcedLogout(response map[string]any) bool {
	return asInt(response["code"]) == 401 && strings.Contains(asString(response["msg"]), "已被迫下线")
}

func qtTeacherIsCredentialInvalid(response map[string]any) bool {
	msg := asString(response["msg"])
	return asInt(response["code"]) == 403 && (msg == "账号不存在" || msg == "密码错误")
}

func qtTeacherRuleDeleted(response map[string]any) bool {
	return asInt(response["code"]) == 44001 && strings.Contains(asString(response["msg"]), "规则已删除")
}

func qtTeacherStudentNoPermission(response map[string]any) bool {
	return asInt(response["code"]) == 500 && strings.Contains(asString(response["msg"]), "程序异常，请联系管理员")
}

func qtTeacherTargetSchoolName(teacherSchool string) string {
	return normalizeSchoolName(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(teacherSchool), "QT-")))
}

func qtTeacherSelectTenantCode(teacherSchool string, globalRole int, tenants []any) string {
	targetSchool := qtTeacherTargetSchoolName(teacherSchool)
	singleTenantCode := ""
	validTenantCount := 0
	for _, item := range tenants {
		tenant := asMap(item)
		code := asString(tenant["Code"])
		if code == "" {
			continue
		}
		validTenantCount++
		if validTenantCount == 1 {
			singleTenantCode = code
		}
		if schoolNamesEquivalent(asString(tenant["Name"]), targetSchool) {
			return code
		}
	}
	if globalRole == 2 && validTenantCount == 1 {
		return singleTenantCode
	}
	return ""
}

func qtTeacherTenantNames(tenants []any) []string {
	names := make([]string, 0, len(tenants))
	for _, item := range tenants {
		name := strings.TrimSpace(asString(asMap(item)["Name"]))
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}

func qtTeacherListRules(data any) []qtTeacherRuleCandidate {
	list := asSlice(asMap(data)["list"])
	rules := make([]qtTeacherRuleCandidate, 0, len(list))
	for _, item := range list {
		entry := asMap(item)
		ruleGuid := asString(entry["RuleGuid"])
		if ruleGuid == "" {
			continue
		}
		rules = append(rules, qtTeacherRuleCandidate{
			Ref: qtTeacherRuleRef{
				RuleGuid: ruleGuid,
				RuleType: asInt(entry["RuleType"]),
			},
			IsDefault: entry["IsDefault"] == true,
		})
	}
	return rules
}

func qtTeacherSelectRule(data any) qtTeacherRuleRef {
	rules := qtTeacherListRules(data)
	if len(rules) == 0 {
		return qtTeacherRuleRef{}
	}
	for _, rule := range rules {
		if rule.IsDefault {
			return rule.Ref
		}
	}
	return rules[0].Ref
}

func qtTeacherSelectSupplementalRule(data any) qtTeacherRuleRef {
	rules := qtTeacherListRules(data)
	if len(rules) != 2 {
		return qtTeacherRuleRef{}
	}

	defaultRule := qtTeacherRuleRef{}
	groupRule := qtTeacherRuleRef{}
	for _, rule := range rules {
		switch rule.Ref.RuleType {
		case 0:
			if !rule.IsDefault || defaultRule.Valid() {
				return qtTeacherRuleRef{}
			}
			defaultRule = rule.Ref
		case 2:
			if groupRule.Valid() {
				return qtTeacherRuleRef{}
			}
			groupRule = rule.Ref
		default:
			return qtTeacherRuleRef{}
		}
	}
	if !defaultRule.Valid() || !groupRule.Valid() || defaultRule.RuleGuid == groupRule.RuleGuid {
		return qtTeacherRuleRef{}
	}
	return groupRule
}

func qtTeacherCloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func qtTeacherFirstStudentRow(response map[string]any) map[string]any {
	table := asMap(asMap(response["data"])["Table"])
	items := asSlice(table["Items"])
	if len(items) == 0 {
		return map[string]any{}
	}
	return asMap(items[0])
}

func qtTeacherHasStudentResult(response map[string]any) bool {
	return len(qtTeacherFirstStudentRow(response)) > 0
}

func qtTeacherNamedTotalItem(student map[string]any) map[string]any {
	total := qtTeacherCloneMap(asMap(student["TotalKmItem"]))
	if len(total) == 0 {
		return map[string]any{}
	}
	if asString(total["Km"]) == "" {
		total["Km"] = "总分"
	}
	return total
}

func qtTeacherIsPlaceholderTotalItem(item map[string]any) bool {
	scoreTexts := []string{
		strings.TrimSpace(asString(item["OriginalScoreText"])),
		strings.TrimSpace(asString(item["PaperScoreText"])),
		strings.TrimSpace(asString(item["FuFenScoreText"])),
	}
	scoreMeaningful := false
	for _, text := range scoreTexts {
		if text != "" && text != "-" && text != "0" && text != "0.0" && text != "0.00" {
			scoreMeaningful = true
			break
		}
	}
	return !scoreMeaningful && qtTeacherFormatRankLine(item) == ""
}

func qtTeacherStudentItems(student map[string]any) map[string]map[string]any {
	items := map[string]map[string]any{}

	total := qtTeacherNamedTotalItem(student)
	if len(total) > 0 && !qtTeacherIsPlaceholderTotalItem(total) {
		if _, ok := qtTeacherDisplayScore(total); ok {
			items["总分"] = total
		}
	}

	for _, raw := range asSlice(student["JiFenKmItems"]) {
		item := qtTeacherCloneMap(asMap(raw))
		subject := asString(item["Km"])
		if subject == "" {
			continue
		}
		if _, ok := qtTeacherDisplayScore(item); !ok {
			continue
		}
		items[subject] = item
	}

	return items
}

func qtTeacherDisplayScore(item map[string]any) (string, bool) {
	original := strings.TrimSpace(asString(item["OriginalScoreText"]))
	if qtTeacherIsSentinelText(original) {
		return "", false
	}
	fufen := strings.TrimSpace(asString(item["FuFenScoreText"]))
	if !qtTeacherIsSentinelText(fufen) {
		if fufen == original {
			return fufen, true
		}
		return fmt.Sprintf("%s(原%s)", fufen, original), true
	}
	return original, true
}

func qtTeacherFormatRankLine(item map[string]any) string {
	parts := make([]string, 0, 4)
	unionRank := strings.TrimSpace(asString(item["UnionRank"]))
	schoolRank := strings.TrimSpace(asString(item["SchoolRank"]))
	combinationRank := strings.TrimSpace(asString(item["CombinationRank"]))
	classRank := strings.TrimSpace(asString(item["ClassRank"]))
	isJointExam := !qtTeacherIsSentinelText(unionRank)

	if isJointExam {
		parts = append(parts, "联"+unionRank)
		if !qtTeacherIsSentinelText(combinationRank) {
			parts = append(parts, "组"+combinationRank)
		}
		if !qtTeacherIsSentinelText(schoolRank) {
			parts = append(parts, "校"+schoolRank)
		}
	} else {
		if !qtTeacherIsSentinelText(schoolRank) {
			parts = append(parts, "校"+schoolRank)
		}
		if !qtTeacherIsSentinelText(combinationRank) {
			parts = append(parts, "组"+combinationRank)
		}
	}
	if !qtTeacherIsSentinelText(classRank) {
		parts = append(parts, "班"+classRank)
	}
	if len(parts) == 0 {
		return ""
	}
	return " [" + strings.Join(parts, "|") + "]"
}

func qtTeacherOverallRows(response map[string]any, className string) (map[string]any, map[string]any, map[string]any) {
	table := asMap(asMap(response["data"])["Table"])
	items := asSlice(table["Items"])
	unionRow := map[string]any{}
	schoolRow := map[string]any{}
	classRow := map[string]any{}
	for _, raw := range items {
		item := asMap(raw)
		orgName := strings.TrimSpace(asString(item["OrgName"]))
		switch {
		case orgName == "联考全体":
			unionRow = item
		case orgName == "全校":
			schoolRow = item
		case className != "" && orgName == className:
			classRow = item
		}
	}
	return unionRow, schoolRow, classRow
}

func qtTeacherOverallSubjectMap(row map[string]any) map[string]map[string]any {
	result := map[string]map[string]any{}
	for _, raw := range asSlice(row["SingleAvgInfos"]) {
		item := qtTeacherCloneMap(asMap(raw))
		subject := asString(item["Km"])
		if subject == "" {
			continue
		}
		result[subject] = item
	}
	return result
}

func qtTeacherMetricEmpty(value any) bool {
	text := strings.TrimSpace(asString(value))
	if qtTeacherIsSentinelText(text) {
		return true
	}
	number, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return false
	}
	return number == 0
}

func qtTeacherOverallMetricText(item map[string]any, field string) string {
	if len(item) == 0 {
		return "-"
	}
	value := strings.TrimSpace(asString(item[field]))
	if qtTeacherIsSentinelText(value) {
		return "-"
	}
	return value
}

func qtTeacherOverallCountText(item map[string]any) string {
	if len(item) == 0 {
		return "-"
	}
	if value := strings.TrimSpace(asString(item["TotalStr"])); !qtTeacherIsSentinelText(value) {
		return value
	}
	if value := strings.TrimSpace(asString(item["Total"])); !qtTeacherIsSentinelText(value) {
		return value
	}
	return "-"
}

func qtTeacherIsSentinelText(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == "-" || value == "-9999"
}

func qtTeacherJoinLabeledStats(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "-" {
			continue
		}
		filtered = append(filtered, part)
	}
	if len(filtered) == 0 {
		return "-"
	}
	return strings.Join(filtered, " ")
}

func qtTeacherLabeledStat(label, value string) string {
	value = strings.TrimSpace(value)
	if qtTeacherIsSentinelText(value) {
		return ""
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return value
	}
	return label + " " + value
}

func qtTeacherHideExamLine(items ...map[string]any) bool {
	for _, item := range items {
		if len(item) == 0 {
			continue
		}
		if !qtTeacherMetricEmpty(item["Avg"]) || !qtTeacherMetricEmpty(item["Max"]) {
			return false
		}
	}
	return true
}

func qtTeacherSelectCombination(student map[string]any) int {
	selectSubject := strings.TrimSpace(asString(student["SelectSubject"]))
	for _, char := range selectSubject {
		switch char {
		case '物':
			return 10
		case '历', '史':
			return 20
		default:
			return 0
		}
	}
	return 0
}

func qtTeacherRenderAnalysisInternal(overallResponse, studentResponse map[string]any, includeExam bool) string {
	student := qtTeacherFirstStudentRow(studentResponse)
	if len(student) == 0 {
		return ""
	}

	studentItems := qtTeacherStudentItems(student)
	if len(studentItems) == 0 {
		return ""
	}

	unionRow, schoolRow, classRow := qtTeacherOverallRows(overallResponse, asString(student["ClassName"]))
	unionItems := qtTeacherOverallSubjectMap(unionRow)
	schoolItems := qtTeacherOverallSubjectMap(schoolRow)
	classItems := qtTeacherOverallSubjectMap(classRow)

	personalLines := map[string]string{}
	examLines := map[string]string{}
	for subject, item := range studentItems {
		score, ok := qtTeacherDisplayScore(item)
		if !ok {
			continue
		}

		rankLine := qtTeacherFormatRankLine(item)
		personalLines[subject] = fmt.Sprintf("- %s %s%s", subject, score, rankLine)

		if !includeExam {
			continue
		}
		unionItem := unionItems[subject]
		schoolItem := schoolItems[subject]
		classItem := classItems[subject]
		if qtTeacherHideExamLine(unionItem, schoolItem, classItem) {
			continue
		}
		if len(unionItem) > 0 {
			examLines[subject] = fmt.Sprintf("- %s %s%s\n考试人数 | %s\n平均分数 | %s\n最高分数 | %s",
				subject,
				score,
				rankLine,
				qtTeacherJoinLabeledStats(
					qtTeacherLabeledStat("联", qtTeacherOverallCountText(unionItem)),
					qtTeacherLabeledStat("校", qtTeacherOverallCountText(schoolItem)),
					qtTeacherLabeledStat("班", qtTeacherOverallCountText(classItem)),
				),
				qtTeacherJoinLabeledStats(
					qtTeacherLabeledStat("联", qtTeacherOverallMetricText(unionItem, "Avg")),
					qtTeacherLabeledStat("校", qtTeacherOverallMetricText(schoolItem, "Avg")),
					qtTeacherLabeledStat("班", qtTeacherOverallMetricText(classItem, "Avg")),
				),
				qtTeacherJoinLabeledStats(
					qtTeacherLabeledStat("联", qtTeacherOverallMetricText(unionItem, "Max")),
					qtTeacherLabeledStat("校", qtTeacherOverallMetricText(schoolItem, "Max")),
					qtTeacherLabeledStat("班", qtTeacherOverallMetricText(classItem, "Max")),
				),
			)
			continue
		}
		examLines[subject] = fmt.Sprintf("- %s %s%s\n考试人数 | %s\n平均分数 | %s\n最高分数 | %s",
			subject,
			score,
			rankLine,
			qtTeacherJoinLabeledStats(
				qtTeacherLabeledStat("校", qtTeacherOverallCountText(schoolItem)),
				qtTeacherLabeledStat("班", qtTeacherOverallCountText(classItem)),
			),
			qtTeacherJoinLabeledStats(
				qtTeacherLabeledStat("校", qtTeacherOverallMetricText(schoolItem, "Avg")),
				qtTeacherLabeledStat("班", qtTeacherOverallMetricText(classItem, "Avg")),
			),
			qtTeacherJoinLabeledStats(
				qtTeacherLabeledStat("校", qtTeacherOverallMetricText(schoolItem, "Max")),
				qtTeacherLabeledStat("班", qtTeacherOverallMetricText(classItem, "Max")),
			),
		)
	}

	sections := make([]string, 0, 2)
	if includeExam && len(examLines) > 0 {
		sections = append(sections, "===== 考试数据 =====\n"+textformatSublistFromTeacher(examLines))
	}
	if len(personalLines) > 0 {
		sections = append(sections, "===== 个人数据 =====\n"+textformatSublistFromTeacher(personalLines))
	}

	return strings.Join(sections, "\n")
}

func qtTeacherRenderPersonalData(studentResponse map[string]any) string {
	student := qtTeacherFirstStudentRow(studentResponse)
	if len(student) == 0 {
		return ""
	}

	personalLines := map[string]string{}
	for subject, item := range qtTeacherStudentItems(student) {
		score, ok := qtTeacherDisplayScore(item)
		if !ok {
			continue
		}
		rankLine := qtTeacherFormatRankLine(item)
		personalLines[subject] = fmt.Sprintf("- %s %s%s", subject, score, rankLine)
	}
	if len(personalLines) == 0 {
		return ""
	}
	return textformatSublistFromTeacher(personalLines)
}

func qtTeacherRenderGroupRankingWithTitle(groupResponse map[string]any, title string) string {
	student := qtTeacherFirstStudentRow(groupResponse)
	if len(student) == 0 {
		return ""
	}

	personalLines := map[string]string{}
	hasGroupRank := false
	for subject, item := range qtTeacherStudentItems(student) {
		score, ok := qtTeacherDisplayScore(item)
		if !ok {
			continue
		}
		if value := strings.TrimSpace(asString(item["CombinationRank"])); value != "" && value != "-" {
			hasGroupRank = true
		}
		rankLine := qtTeacherFormatRankLine(item)
		personalLines[subject] = fmt.Sprintf("- %s %s%s", subject, score, rankLine)
	}
	if len(personalLines) == 0 {
		return ""
	}

	text := textformatSublistFromTeacher(personalLines)
	if strings.TrimSpace(title) == "" {
		title = "分组排名"
	}
	if hasGroupRank {
		text += "\n* '组'指代选科组合（如：物化地）排名。"
	}
	return "===== " + title + " =====\n" + text
}

func qtTeacherRenderAnalysis(overallResponse, studentResponse map[string]any) string {
	return qtTeacherRenderAnalysisInternal(overallResponse, studentResponse, true)
}

func qtTeacherRenderPersonalAnalysis(studentResponse map[string]any) string {
	text := qtTeacherRenderPersonalData(studentResponse)
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return "===== 个人数据 =====\n" + text
}
