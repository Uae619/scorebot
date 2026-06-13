package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

//go:embed index.html
var indexHTML []byte

//go:embed manifest.json
var manifestJSON []byte

//go:embed sw.js
var swJS []byte

//go:embed egg.gif
var eggGIF []byte

var icon192PNG = generateIconPNG(192)
var icon512PNG = generateIconPNG(512)

type noopSender struct{}

func (s *noopSender) SendText(ctx context.Context, msg *MessageContext, content string) map[string]any {
	return map[string]any{"sent": false, "reason": "noop"}
}
func (s *noopSender) SendImage(ctx context.Context, msg *MessageContext, imageContent []byte, content string) map[string]any {
	return map[string]any{"sent": false, "reason": "noop"}
}
func (s *noopSender) SendImageReader(ctx context.Context, msg *MessageContext, imageContent io.Reader, content string) map[string]any {
	return map[string]any{"sent": false, "reason": "noop"}
}

// ---------- JSON types ----------

type apiResponse struct {
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type apiExamItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Time      string `json:"time"`
	Score     string `json:"score,omitempty"`
	FullScore string `json:"fullScore,omitempty"`
}

type apiQueryResult struct {
	QQID   string        `json:"qqid"`
	Mode   string        `json:"mode"`
	Name   string        `json:"name"`
	School string        `json:"school"`
	Grade  string        `json:"grade"`
	Class  string        `json:"class"`
	Exams  []apiExamItem `json:"exams"`
}

type apiExamDetail struct {
	QQID          string       `json:"qqid"`
	ExamID        string       `json:"examId"`
	ExamName      string       `json:"examName"`
	ExamTime      string       `json:"examTime"`
	Score         string       `json:"score"`
	FullScore     string       `json:"fullScore"`
	Grade         string       `json:"grade"`
	RankLow       int          `json:"rankLow"`
	RankHigh      int          `json:"rankHigh"`
	TotalStudents int          `json:"totalStudents"`
	SchoolRank    string       `json:"schoolRank,omitempty"`
	ClassRank     string       `json:"classRank,omitempty"`
	GradeStuNum   string       `json:"gradeStuNum,omitempty"`
	ClassStuNum   string       `json:"classStuNum,omitempty"`
	Subjects      []apiSubject `json:"subjects"`
}

type apiSubject struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Score     string `json:"score"`
	FullScore string `json:"fullScore"`
	Grade     string `json:"grade,omitempty"`
	RankLow   int    `json:"rankLow,omitempty"`
	RankHigh  int    `json:"rankHigh,omitempty"`
	Rank      string `json:"rank,omitempty"`
	ClassRank string `json:"classRank,omitempty"`
}

type apiBindRequest struct {
	QQID     string `json:"qqid"`
	Platform string `json:"platform"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func apiMessageContext(r *http.Request, qqid string) *MessageContext {
	return &MessageContext{
		Ctx:    r.Context(),
		UserID: qqid,
	}
}

func modeLabel(mode string) string {
	switch mode {
	case "student":
		return "好分数(学生版)"
	case "parent":
		return "好分数(家长版)"
	case "qt":
		return "七天网络"
	case "bfz":
		return "百分智"
	default:
		return mode
	}
}

// ---------- POST /api/bind ----------

func handleAPIBind(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "仅支持 POST 请求"})
		return
	}

	var req apiBindRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "请求格式错误: " + err.Error()})
		return
	}

	qqid := strings.TrimSpace(req.QQID)
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	platform := strings.TrimSpace(req.Platform)

	if qqid == "" || username == "" || password == "" || platform == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "缺少必填参数 qqid / platform / username / password"})
		return
	}

	ctx := apiMessageContext(r, qqid)
	var resultMsg string
	var bindErr string

	switch platform {
	case "hfs-student":
		resultMsg, bindErr = bindHFS(ctx, username, password, 1)
	case "hfs-parent":
		resultMsg, bindErr = bindHFS(ctx, username, password, 2)
	case "qt":
		resultMsg, bindErr = bindQT(ctx, username, password)
	case "bfz":
		resultMsg, bindErr = bindBFZ(ctx, username, password)
	default:
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "不支持的平台: " + platform + "，可选: hfs-student / hfs-parent / qt"})
		return
	}

	if bindErr != "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: bindErr})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: map[string]string{"message": resultMsg}})
}

func bindHFS(ctx *MessageContext, username, password string, accountType int) (string, string) {
	response := studentLoginWithContext(ctx, username, password, accountType)
	if len(response) == 1 {
		return "", "登录失败: " + response[0]
	}

	token := response[1]
	snapshot := studentSnapshotWithContext(ctx, token)
	if asString(snapshot["msg"]) != "信息获取成功" {
		return "", "获取用户信息失败: " + asString(snapshot["msg"])
	}

	hiddenConfig := studentGetHiddenConfigWithContext(ctx, token)
	if hiddenConfig["getSuccess"] != true {
		if isHFSRiskLocked(hiddenConfig) {
			return "", "账号被好分数风控机制命中，暂时无法绑定"
		}
		return "", "获取学校配置失败"
	}

	school := asString(snapshot["school"])
	if school == "wxyunxiaozb" || school == "" {
		return "", "账号尚未绑定学生，请在网页端/APP绑定学生后使用"
	}

	mode := "parent"
	if accountType == 1 {
		mode = "student"
	}

	replaceExistingBinding(ctx)
	opNew(ctx.UserID)
	opWrite(ctx.UserID, map[string]any{
		"mode":   mode,
		"xuehao": snapshot["xuehao"],
		"zh":     username,
		"pw":     password,
		"id":     snapshot["studentid"],
		"school": snapshot["school"],
		"grade":  snapshot["grade"],
		"banji":  snapshot["class"],
		"name":   snapshot["name"],
		"token":  token,
	})

	result := fmt.Sprintf("绑定成功！[%s] %s(%s) / 类型: %s",
		asString(snapshot["school"]),
		asString(snapshot["name"]),
		asString(snapshot["xuehao"]),
		modeLabel(mode))
	return result, ""
}

func bindQT(ctx *MessageContext, username, password string) (string, string) {
	response := qtStudentLoginWithContext(ctx, username, password)
	if response["isSuccess"] != true {
		return "", "七天网络登录失败: " + asString(response["msg"])
	}

	replaceExistingBinding(ctx)
	opNew(ctx.UserID)
	opWrite(ctx.UserID, map[string]any{
		"mode":   "qt",
		"xuehao": nil,
		"zh":     username,
		"pw":     password,
		"name":   response["name"],
		"school": response["school"],
		"grade":  response["grade"],
		"banji":  nil,
		"token":  response["token"],
		"id":     nil,
	})

	return fmt.Sprintf("绑定成功！（七天网络）%s / 学校: %s", asString(response["name"]), asString(response["school"])), ""
}

func bindBFZ(ctx *MessageContext, username, password string) (string, string) {
	response := bfzStudentLoginWithContext(messageContext(ctx), username, password)
	if response["isSuccess"] != true {
		return "", "百分智平台登录失败，请检查账号和密码"
	}

	replaceExistingBinding(ctx)
	opNew(ctx.UserID)
	opWrite(ctx.UserID, map[string]any{
		"mode":   "bfz",
		"xuehao": username,
		"zh":     username,
		"pw":     password,
		"name":   response["name"],
		"school": nil,
		"grade":  nil,
		"banji":  nil,
		"token":  nil,
		"id":     nil,
	})

	return fmt.Sprintf("绑定成功！（百分智）%s", asString(response["name"])), ""
}

func replaceExistingBinding(ctx *MessageContext) {
	userdata := opView(ctx.UserID)
	if ok, _ := userdata["Return"].(bool); ok {
		opDeleteQTStudentExamCache(ctx.UserID)
		opDelete(ctx.UserID)
	}
}

// ---------- GET /api/answersheet ----------

func proxyImage(w http.ResponseWriter, imgURL string) {
	client := httpClient(30 * time.Second)
	resp, err := client.Get(imgURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, apiResponse{Success: false, Error: "下载图片失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" || strings.HasPrefix(contentType, "text/html") {
		contentType = "image/jpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, resp.Body)
}

func handleAnswerSheet(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	qqid := strings.TrimSpace(r.URL.Query().Get("qqid"))
	subjectID := strings.TrimSpace(r.URL.Query().Get("subject_id"))

	if qqid == "" || subjectID == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "缺少 qqid 或 subject_id 参数"})
		return
	}

	userdata := opView(qqid)
	if ok, _ := userdata["Return"].(bool); !ok {
		writeJSON(w, http.StatusNotFound, apiResponse{Success: false, Error: "该 QQ 号未绑定账号"})
		return
	}

	ctx := apiMessageContext(r, qqid)
	handler := newCommandHandler(&noopSender{})
	handler.userdata = userdata
	mode := asString(userdata["mode"])

	var imgURL string
	var errMsg string

	switch mode {
	case "student", "parent":
		imgURL, errMsg = getHFSAnswerSheet(ctx, handler, subjectID)
	case "qt":
		imgURL, errMsg = getQTAnswerSheet(ctx, handler, subjectID)
	default:
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "该平台不支持答题卡查询"})
		return
	}

	if errMsg != "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: errMsg})
		return
	}

	// proxy the image: download from provider, serve to browser
	proxyImage(w, imgURL)
}

func getHFSAnswerSheet(ctx *MessageContext, handler *CommandHandler, subjectID string) (string, string) {
	examContext := opViewExamContext(ctx.UserID)
	if examContext["Return"] != true {
		return "", "未找到前置考试信息，请先查询考试详情"
	}
	subjectMap := asMap(examContext["subject_map"])
	paperInfo := asMap(subjectMap[subjectID])
	if len(paperInfo) == 0 {
		return "", "未找到对应科目信息"
	}

	paperID := asString(paperInfo["paperId"])
	pid := asString(paperInfo["pid"])
	if paperID == "" || pid == "" {
		return "", "科目信息不完整"
	}

	examID := asString(handler.userdata["exam"])
	token := asString(handler.userdata["token"])
	if token == "" {
		return "", "未找到登录凭证"
	}

	// check if relogin needed
	resp := studentGetSubjectTinfoAnswerpicWithContext(ctx, token, examID, paperID, pid)
	if resp["getSuccess"] != true {
		if asInt(resp["code"]) == 3001 {
			if _, err := handler.reloginStudentToken(ctx); err != "" {
				return "", "重新登录失败"
			}
			token = asString(handler.userdata["token"])
			resp = studentGetSubjectTinfoAnswerpicWithContext(ctx, token, examID, paperID, pid)
			if resp["getSuccess"] != true {
				return "", "获取答题卡失败: " + asString(resp["msg"])
			}
		} else {
			return "", "获取答题卡失败: " + asString(resp["msg"])
		}
	}

	// use the project's own URL extractor
	urls, _ := extractAnswerSheetURLs(resp)
	if len(urls) > 0 {
		return asString(urls[0]), ""
	}
	return "", "答题卡 URL 为空"
}

func getQTAnswerSheet(ctx *MessageContext, handler *CommandHandler, subjectID string) (string, string) {
	examContext := opViewExamContext(ctx.UserID)
	if examContext["Return"] != true {
		return "", "未找到前置考试信息，请先查询考试详情"
	}
	subjectMap := asMap(examContext["subject_map"])
	if asString(subjectMap["__provider"]) != "qt" {
		return "", "当前不是七天网络考试"
	}

	items := asMap(subjectMap["__items"])
	aliases := asMap(subjectMap["__aliases"])
	if alias := asString(aliases[subjectID]); alias != "" {
		subjectID = alias
	}
	subject := asMap(items[subjectID])
	if len(subject) == 0 {
		return "", "未找到对应科目信息"
	}

	exam := asMap(subjectMap["__exam"])
	if len(exam) == 0 || asString(exam["examGuid"]) == "" {
		return "", "考试上下文不完整"
	}

	token, userInfo, errMsg := handler.qtLoadUserInfoWithRelogin(ctx)
	if errMsg != "" {
		return "", errMsg
	}

	resp := qtGetQuestionAnswerCardURLWithContext(ctx, token, userInfo, exam, subject, true)
	if resp["getSuccess"] != true {
		return "", "获取答题卡失败: " + asString(resp["msg"])
	}

	urls, _ := extractAnswerSheetURLs(resp)
	if len(urls) > 0 {
		return asString(urls[0]), ""
	}
	return "", "答题卡 URL 为空"
}

// ---------- GET /api/query ----------

func handleAPIQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "仅支持 GET 请求"})
		return
	}

	qqid := strings.TrimSpace(r.URL.Query().Get("qqid"))
	if qqid == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "缺少 qqid 参数"})
		return
	}

	userdata := opView(qqid)
	if ok, _ := userdata["Return"].(bool); !ok {
		writeJSON(w, http.StatusNotFound, apiResponse{Success: false, Error: "该 QQ 号未绑定账号，请先绑定"})
		return
	}

	ctx := apiMessageContext(r, qqid)
	handler := newCommandHandler(&noopSender{})
	handler.userdata = userdata

	mode := asString(userdata["mode"])

	// if exam_id is specified, return exam detail
	examID := strings.TrimSpace(r.URL.Query().Get("exam_id"))
	if examID != "" {
		detail, errMsg := queryExamDetail(ctx, handler, mode, examID)
		if errMsg != "" {
			writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: errMsg})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: detail})
		return
	}

	result := &apiQueryResult{
		QQID:   qqid,
		Mode:   modeLabel(mode),
		Name:   asString(userdata["name"]),
		School: asString(userdata["school"]),
		Grade:  asString(userdata["grade"]),
		Class:  asString(userdata["banji"]),
	}

	switch mode {
	case "student", "parent":
		result.Exams = queryHFSExams(ctx, handler)
	case "qt":
		result.Exams = queryQTExams(ctx, handler)
	case "bfz":
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "百分智平台暂不支持网页查询"})
		return
	default:
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "未知的绑定类型: " + mode})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: result})
}

func queryExamDetail(ctx *MessageContext, handler *CommandHandler, mode, examID string) (*apiExamDetail, string) {
	switch mode {
	case "student", "parent":
		return queryHFSExamDetail(ctx, handler, examID)
	case "qt":
		return queryQTExamDetail(ctx, handler, examID)
	default:
		return nil, "该平台不支持查询考试详情"
	}
}

func queryHFSExamDetail(ctx *MessageContext, handler *CommandHandler, examID string) (*apiExamDetail, string) {
	opWrite(ctx.UserID, map[string]any{"exam": examID})
	handler.userdata["exam"] = examID

	examOverview, errMsg := handler.loadExamOverviewWithRelogin(ctx, examID)
	if errMsg != "" {
		return nil, errMsg
	}

	// store exam context for answer sheet lookup
	subjectMap := extractExamPaperContext(examOverview)
	if len(subjectMap) > 0 {
		opWriteExamContext(ctx.UserID, examID, subjectMap)
	}

	data := asMap(examOverview["data"])
	papers := asSlice(data["papers"])
	subjects := make([]apiSubject, 0, len(papers))
	for _, item := range papers {
		sub := asMap(item)
		subjects = append(subjects, apiSubject{
			ID:        asString(sub["paperId"]),
			Name:      asString(sub["subject"]),
			Score:     asString(sub["score"]),
			FullScore: asString(sub["manfen"]),
			Rank:      asString(sub["gradeRank"]),
			ClassRank: asString(sub["classRank"]),
		})
	}

	schoolRank := asString(asMap(data["compare"])["curGradeRank"])
	detail := &apiExamDetail{
		QQID:        ctx.UserID,
		ExamID:      examID,
		ExamName:    asString(data["name"]),
		ExamTime:    formatExamDate(data["time"]),
		Score:       asString(data["score"]),
		FullScore:   asString(data["manfen"]),
		SchoolRank:  schoolRank,
		GradeStuNum: asString(data["gradeStuNum"]),
		ClassStuNum: asString(data["classStuNum"]),
		Subjects:    subjects,
	}

	// HFS returns exact rank
	if schoolRank != "" {
		detail.Grade = "校排名"
		detail.RankLow = asInt(schoolRank)
		detail.RankHigh = asInt(schoolRank)
	}
	if detail.GradeStuNum != "" {
		detail.TotalStudents = asInt(detail.GradeStuNum)
	}

	return detail, ""
}

func queryQTExamDetail(ctx *MessageContext, handler *CommandHandler, examID string) (*apiExamDetail, string) {
	var exams []map[string]any
	var exam map[string]any
	if cached := opViewQTStudentExamCache(ctx.UserID); len(cached) > 0 {
		if e, err := qtResolveExamSelector(examID, cached); err == "" {
			exams = cached
			exam = e
		}
	}

	if exam == nil {
		var errMsg string
		exams, _, _, errMsg = handler.qtLoadExamsWithAutoClaim(ctx)
		if errMsg != "" {
			return nil, errMsg
		}
		exam, errMsg = qtResolveExamSelector(examID, exams)
		if errMsg != "" {
			return nil, errMsg
		}
	}

	// 科目列表
	subjectsRes, _, errMsg := handler.qtExecuteWithProfile(ctx, "question_subjects",
		func(token string, userInfo map[string]any) map[string]any {
			return qtGetQuestionSubjectsWithContext(ctx, token, userInfo, exam)
		})
	if errMsg != "" {
		return nil, errMsg
	}
	subjectsData := asMap(subjectsRes["data"])

	// store exam context for answer sheet lookup
	qtContext := qtBuildSubjectContext(exam, subjectsData)
	opWrite(ctx.UserID, map[string]any{"exam": asString(exam["examGuid"])})
	opWriteExamContext(ctx.UserID, asString(exam["examGuid"]), qtContext)
	handler.userdata["exam"] = asString(exam["examGuid"])

	// 总分排名
	gradeRes, _, errMsg := handler.qtExecuteWithProfile(ctx, "question_subject_grade:总分",
		func(token string, userInfo map[string]any) map[string]any {
			subjectCount := qtSubjectCount(subjectsData)
			return qtGetQuestionSubjectGradeWithContext(ctx, token, userInfo, exam, "总分", subjectCount, 1)
		})
	if errMsg != "" {
		return nil, errMsg
	}

	totalReport := asMap(asMap(gradeRes["data"])["report"])
	totalGrade := strings.ToUpper(asString(totalReport["grade"]))
	totalStudents := asInt(totalReport["total"])
	rankLow, rankHigh, _ := qtEstimateRankRange(totalGrade, totalStudents)

	// 逐科排名
	visibleSubjects := qtVisibleSubjects(subjectsData)
	subjectCount := qtSubjectCount(subjectsData)
	subjects := make([]apiSubject, 0, len(visibleSubjects))
	for i, subject := range visibleSubjects {
		km := asString(subject["km"])
		sub := apiSubject{
			ID:        fmt.Sprintf("%03d", i+1),
			Name:      km,
			Score:     asString(subject["myScore"]),
			FullScore: asString(subject["fullScore"]),
		}

		// 只对非"总分"科目查排名
		if km != "总分" {
			subGradeRes, _, subErr := handler.qtExecuteWithProfile(ctx, "question_subject_grade:"+km,
				func(token string, userInfo map[string]any) map[string]any {
					return qtGetQuestionSubjectGradeWithContext(ctx, token, userInfo, exam, km, subjectCount, 1)
				})
			if subErr == "" {
				subReport := asMap(asMap(subGradeRes["data"])["report"])
				sub.Grade = strings.ToUpper(asString(subReport["grade"]))
				if sub.Grade != "" {
					subTotal := asInt(subReport["total"])
					sub.RankLow, sub.RankHigh, _ = qtEstimateRankRange(sub.Grade, subTotal)
				}
			}
		}

		subjects = append(subjects, sub)
	}

	shortIDs := buildQTExamShortIDs(exams)
	return &apiExamDetail{
		QQID:          ctx.UserID,
		ExamID:        shortIDs[asString(exam["examGuid"])],
		ExamName:      defaultString(asString(exam["examName"]), "未知"),
		ExamTime:      stringOrNA(exam["time"]),
		Score:         defaultString(asString(totalReport["myScore"]), asString(exam["score"])),
		FullScore:     defaultString(asString(totalReport["fullScore"]), "—"),
		Grade:         totalGrade,
		RankLow:       rankLow,
		RankHigh:      rankHigh,
		TotalStudents: totalStudents,
		Subjects:      subjects,
	}, ""
}

// ---------- exam list helpers ----------

func queryHFSExams(ctx *MessageContext, handler *CommandHandler) []apiExamItem {
	resJSON, errMsg := handler.loadExamListWithRelogin(ctx)
	if errMsg != "" {
		return nil
	}
	examList, _ := extractExamList(resJSON)
	items := make([]apiExamItem, 0, len(examList))
	for _, raw := range examList {
		record := asMap(raw)
		examID := asInt(record["examId"])
		if examID == 0 {
			examID = asInt(record["examid"])
		}
		timeVal := record["time"]
		if timeVal == nil {
			timeVal = record["examTime"]
		}
		items = append(items, apiExamItem{
			ID:        fmt.Sprintf("%d", examID),
			Name:      defaultString(asString(record["name"]), asString(record["examName"])),
			Time:      formatExamDate(timeVal),
			Score:     asString(record["score"]),
			FullScore: asString(record["manfen"]),
		})
	}
	return items
}

func queryQTExams(ctx *MessageContext, handler *CommandHandler) []apiExamItem {
	if cached := opViewQTStudentExamCache(ctx.UserID); len(cached) > 0 {
		return formatQTExamItems(cached)
	}
	exams, _, _, errMsg := handler.qtLoadExamsWithAutoClaim(ctx)
	if errMsg != "" {
		return nil
	}
	return formatQTExamItems(exams)
}

func formatQTExamItems(exams []map[string]any) []apiExamItem {
	shortIDs := buildQTExamShortIDs(exams)
	items := make([]apiExamItem, 0, len(exams))
	for _, exam := range exams {
		items = append(items, apiExamItem{
			ID:   shortIDs[asString(exam["examGuid"])],
			Name: defaultString(asString(exam["examName"]), "未知"),
			Time: stringOrNA(exam["time"]),
		})
	}
	return items
}

// ---------- static ----------

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(indexHTML)
}

func StartAPIServer(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/bind", handleAPIBind)
	mux.HandleFunc("/api/query", handleAPIQuery)
	mux.HandleFunc("/api/answersheet", handleAnswerSheet)
	mux.HandleFunc("/manifest.json", handleManifest)
	mux.HandleFunc("/sw.js", handleSW)
	mux.HandleFunc("/egg.gif", handleEgg)
	mux.HandleFunc("/icon-192.png", handleIcon192)
	mux.HandleFunc("/icon-512.png", handleIcon512)
	mux.HandleFunc("/", handleIndex)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	logger.Printf("API 服务启动在 http://%s", addr)
	return server.ListenAndServe()
}

func handleManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(manifestJSON)
}
func handleEgg(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(eggGIF)
}
func handleSW(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write(swJS)
}
func handleIcon192(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(icon192PNG)
}
func handleIcon512(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(icon512PNG)
}

func generateIconPNG(size int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	red := color.RGBA{0xC6, 0x28, 0x28, 0xFF}
	gold := color.RGBA{0xFF, 0xD7, 0x00, 0xFF}

	// red circle background
	drawCircle(img, size/2, size/2, size/2, red)

	cx, cy := size/2, size/2
	s := float64(size)

	// filled V-shape chevron (two triangles forming a ^)
	tipX, tipY := cx, cy+int(-0.30*s)
	baseY := cy + int(0.22*s)
	thick := int(s * 0.06)

	// draw filled triangles for left and right wings
	for dy := 0; dy <= baseY-tipY; dy++ {
		progress := float64(dy) / float64(baseY-tipY)
		centerDist := progress * float64(cy+int(0.31*s)-tipX)
		for dx := -thick / 2; dx <= thick/2; dx++ {
			xL := tipX - int(centerDist) + dx
			xR := tipX + int(centerDist) + dx
			y := tipY + dy
			if xL >= 0 && xL < size && y >= 0 && y < size {
				img.Set(xL, y, gold)
			}
			if xR >= 0 && xR < size && y >= 0 && y < size {
				img.Set(xR, y, gold)
			}
		}
	}

	// rounded caps at the bottom of each wing
	bottomLX := cx - int(0.31*s)
	bottomRX := cx + int(0.31*s)
	capR := thick / 2
	drawCircle(img, bottomLX, baseY, capR, gold)
	drawCircle(img, bottomRX, baseY, capR, gold)
	// rounded cap at the tip
	drawCircle(img, cx, tipY, capR, gold)

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func drawCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	rr := r * r
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= rr {
				px, py := cx+x, cy+y
				if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
					img.Set(px, py, c)
				}
			}
		}
	}
	_ = draw.Draw
}

func drawRect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			px, py := img.Bounds().Dx()/2+x+dx, img.Bounds().Dy()/2+y+dy
			if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
				img.Set(px, py, c)
			}
		}
	}
}

func drawDiamond(img *image.RGBA, cxOffset, cyOffset, hh, hw int, c color.RGBA) {
	cx := img.Bounds().Dx()/2 + cxOffset
	cy := img.Bounds().Dy()/2 + cyOffset
	for dy := -hh; dy <= hh; dy++ {
		maxDX := hw * (hh - absI(dy)) / hh
		for dx := -maxDX; dx <= maxDX; dx++ {
			px, py := cx+dx, cy+dy
			if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
				img.Set(px, py, c)
			}
		}
	}
}

func absI(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ensure math is used (for future trig if needed)
var _ = math.Pi
