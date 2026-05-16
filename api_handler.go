package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
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

	data := asMap(examOverview["data"])
	papers := asSlice(data["papers"])
	subjects := make([]apiSubject, 0, len(papers))
	var totalScore, totalFull string
	for _, item := range papers {
		sub := asMap(item)
		subjects = append(subjects, apiSubject{
			ID:        asString(sub["paperId"]),
			Name:      asString(sub["subject"]),
			Score:     asString(sub["score"]),
			FullScore: asString(sub["manfen"]),
			Grade:     asString(sub["gradeRank"]),
		})
		// 累加科目分得到总分（HFS v3 接口的总分字段可能不准确）
	}

	totalScore = asString(data["score"])
	totalFull = asString(data["manfen"])
	schoolRank := asString(asMap(data["compare"])["curGradeRank"])
	gradeStuNum := asString(data["gradeStuNum"])

	detail := &apiExamDetail{
		QQID:      ctx.UserID,
		ExamID:    examID,
		ExamName:  asString(data["name"]),
		ExamTime:  formatExamDate(data["time"]),
		Score:     totalScore,
		FullScore: totalFull,
		Subjects:  subjects,
	}

	if schoolRank != "" {
		detail.Grade = "校排名 " + schoolRank
	}
	if gradeStuNum != "" {
		detail.TotalStudents = asInt(gradeStuNum)
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
	w.Write(indexHTML)
}

func StartAPIServer(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/bind", handleAPIBind)
	mux.HandleFunc("/api/query", handleAPIQuery)
	mux.HandleFunc("/manifest.json", handleManifest)
	mux.HandleFunc("/sw.js", handleSW)
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
	// minimal PNG: blue rounded-square icon with "S" letter
	buf := make([]byte, 0, size*size*3)
	buf = append(buf, pngHeader()...)
	buf = append(buf, pngIHDR(size)...)
	buf = append(buf, pngIDAT(size)...)
	buf = append(buf, pngIEND()...)
	return buf
}

func pngHeader() []byte {
	return []byte{137, 80, 78, 71, 13, 10, 26, 10}
}

func pngIHDR(size int) []byte {
	data := make([]byte, 13)
	data[0] = byte(size >> 24)
	data[1] = byte(size >> 16)
	data[2] = byte(size >> 8)
	data[3] = byte(size)
	data[4] = byte(size >> 24)
	data[5] = byte(size >> 16)
	data[6] = byte(size >> 8)
	data[7] = byte(size)
	data[8] = 8 // 8-bit color
	data[9] = 2 // RGB
	return pngChunk("IHDR", data)
}

func pngIDAT(size int) []byte {
	var raw []byte
	center := size / 2
	radius := int(float64(size) * 0.4)

	for y := 0; y < size; y++ {
		raw = append(raw, 0) // filter: none
		for x := 0; x < size; x++ {
			dx, dy := x-center, y-center
			dist := dx*dx + dy*dy
			innerR := int(float64(size) * 0.28)
			if dist <= innerR*innerR {
				// inner white circle
				raw = append(raw, 255, 255, 255)
			} else if dist <= radius*radius {
				// blue ring
				raw = append(raw, 59, 130, 246)
			} else {
				// transparent/white background
				raw = append(raw, 240, 242, 245)
			}
		}
	}

	compressed := zlibCompress(raw)
	return pngChunk("IDAT", compressed)
}

func pngIEND() []byte {
	return pngChunk("IEND", nil)
}

func pngChunk(typ string, data []byte) []byte {
	n := len(data)
	chunk := make([]byte, 8+n)
	chunk[0] = byte(n >> 24)
	chunk[1] = byte(n >> 16)
	chunk[2] = byte(n >> 8)
	chunk[3] = byte(n)
	copy(chunk[4:8], typ)
	copy(chunk[8:], data)
	crc := crc32IEEE(chunk[4 : 8+n])
	chunk = append(chunk, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))
	return chunk
}

func zlibCompress(data []byte) []byte {
	// minimal deflate + zlib wrapper for raw RGB data
	// zlib header: CMF=0x78 FLG=0x01 (no dict, level 0)
	out := []byte{0x78, 0x01}

	pos := 0
	for pos < len(data) {
		blockLen := len(data) - pos
		if blockLen > 65535 {
			blockLen = 65535
		}
		isLast := pos+blockLen >= len(data)
		if isLast {
			out = append(out, 1) // final block
		} else {
			out = append(out, 0) // non-final block
		}
		out = append(out, byte(blockLen), byte(blockLen>>8))
		out = append(out, byte(^blockLen), byte(^blockLen>>8))
		out = append(out, data[pos:pos+blockLen]...)
		pos += blockLen
	}

	// adler32 checksum of original data
	a32 := adler32(data)
	out = append(out, byte(a32>>24), byte(a32>>16), byte(a32>>8), byte(a32))
	return out
}

func crc32IEEE(data []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, b := range data {
		crc ^= uint32(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xEDB88320
			} else {
				crc >>= 1
			}
		}
	}
	return crc ^ 0xFFFFFFFF
}

func adler32(data []byte) uint32 {
	var a, b uint32 = 1, 0
	for _, v := range data {
		a = (a + uint32(v)) % 65521
		b = (b + a) % 65521
	}
	return b<<16 | a
}
