package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	qtPasswordSuffix      = "{MTgyMjU2MDU0MjF7c3pvbmV9}"
	qtHashDigitsLen       = 50
	qtStudentExamCacheTTL = 5 * time.Minute
	qtAESKey              = "c0f1a30cba2147949ee71cf71cba3c20"
)

var qtSevenNetCipherBlock = mustAESCipherBlock(qtAESKey)

var schoolMapping = map[string]string{
	"北京师范大学宁德实验学校": "北师大宁德实验学校",
	"08福安市第八中学":    "福安八中",
	"福安市第八中学":      "福安八中",
	"31福安市德艺学校":    "福安德艺中学",
	"31福安市德艺中学":    "福安德艺中学",
	"福安市德艺学校":      "福安德艺中学",
	"福安市第二中学":      "福安二中",
	"福安市高级中学":      "福安高级中学",
	"19福安市老区中学":    "福安老区中学",
	"06福安市第六中学":    "福安六中",
	"福安市第六中学":      "福安六中",
	"03福安市第三中学":    "福安三中",
	"福安市第三中学":      "福安三中",
	"福安市第一中学":      "福安一中",
	"福建省福安第一中学":    "福安一中",
	"福建省福安一中":      "福安一中",
	"35福安市扆山中学":    "福安扆山中学",
	"福安市扆山中学":      "福安扆山中学",
	"福鼎市第二中学":      "福鼎二中",
	"福鼎六中山前校区":     "福鼎六中",
	"福鼎市第六中学":      "福鼎六中",
	"福鼎茂华高级中学":     "福鼎茂华学校",
	"福鼎市茂华学校":      "福鼎茂华学校",
	"福鼎市第七中学（福鼎市店下职业高级中学）": "福鼎七中",
	"福鼎市第七中学高中部":           "福鼎七中",
	"福鼎市第三中学":              "福鼎三中",
	"福鼎市第三中学（沙埕校区）":        "福鼎三中",
	"福鼎市第三中学2024":          "福鼎三中",
	"福鼎市第四中学":              "福鼎四中",
	"福鼎市第五中学":              "福鼎五中",
	"福鼎市第一中学":              "福鼎一中",
	"福建省福鼎第一中学":            "福鼎一中",
	"福建省福鼎市第一中学":           "福鼎一中",
	"古田县第六中学":              "古田六中",
	"古田县第三中学":              "古田三中",
	"古田县溪山高级中学":            "古田溪山高级中学",
	"宁德市古田溪山高级中学":          "古田溪山高级中学",
	"宁德市古田溪山高级中学有限公司":      "古田溪山高级中学",
	"古田县第一中学":              "古田一中",
	"古田县玉田中学":              "古田玉田中学",
	"古田县职业中专学校":            "古田职业中专学校",
	"宁德市博雅培文学校":            "宁德博雅培文学校",
	"宁德市博雅外国语高级中学有限公司":     "宁德博雅外国语高级中学",
	"福建省宁德市鼎石高级中学":         "宁德鼎石高级中学",
	"福建省宁德市鼎石高级中学有限公司":     "宁德鼎石高级中学",
	"宁德市鼎石高级中学":            "宁德鼎石高级中学",
	"宁德市第二中学":              "宁德二中",
	"宁德市高级中学":              "宁德高级中学",
	"宁德实验学校":               "宁德国杰高级中学",
	"宁德市国杰高级中学":            "宁德国杰高级中学",
	"宁德市国杰高级中学有限公司":        "宁德国杰高级中学",
	"宁德市衡水育才中学":            "宁德衡水育才中学",
	"宁德市第九中学":              "宁德九中",
	"宁德市第六中学":              "宁德六中",
	"00宁德市民族中学":            "宁德民中",
	"宁德市民族中学":              "宁德民中",
	"宁德市第四中学":              "宁德四中",
	"宁德市第五中学":              "宁德五中",
	"福建省宁德第一中学":            "宁德一中",
	"福建省宁德市第一中学":           "宁德一中",
	"宁德第一中学":               "宁德一中",
	"宁德市第一中学":              "宁德一中",
	"福建省屏南第二中学":            "屏南二中",
	"屏南县第二中学":              "屏南二中",
	"屏南县第三中学":              "屏南三中",
	"福建省屏南职业中专学校":          "屏南县职业中专学校",
	"屏南职业中专学校":             "屏南县职业中专学校",
	"屏南县第一中学":              "屏南一中",
	"寿宁县第二中学":              "寿宁二中",
	"寿宁县第四中学":              "寿宁四中",
	"寿宁县第一中学":              "寿宁一中",
	"福建宏翔高级中学":             "霞浦宏翔高级中学",
	"福建宏翔高级中学有限公司":         "霞浦宏翔高级中学",
	"宁德市宏翔高级中学":            "霞浦宏翔高级中学",
	"宁德市宏翔投资有限公司":          "霞浦宏翔高级中学",
	"霞浦县宏翔高级中学":            "霞浦宏翔高级中学",
	"霞浦县第六中学":              "霞浦六中",
	"霞浦县民族中学":              "霞浦民中",
	"霞浦县第七中学":              "霞浦七中",
	"霞浦县第三中学":              "霞浦三中",
	"福建省霞浦职业中专学校":          "霞浦县职业中专学校",
	"福建省霞浦第一中学":            "霞浦一中",
	"霞浦县第一中学":              "霞浦一中",
	"福建省柘荣职业技术学校":          "柘荣县职业技术学校",
	"柘荣县第一中学":              "柘荣一中",
	"周宁县第二中学":              "周宁二中",
	"周宁县第十中学":              "周宁十中",
	"周宁县第一中学":              "周宁一中",
}

var qtBindableSchools = map[string]struct{}{
	"北师大宁德实验学校":   {},
	"福安八中":        {},
	"福安德艺中学":      {},
	"福安二中":        {},
	"福安高级中学":      {},
	"福安老区中学":      {},
	"福安六中":        {},
	"福安三中":        {},
	"福安一中":        {},
	"福安扆山中学":      {},
	"福鼎二中":        {},
	"福鼎六中":        {},
	"福鼎茂华学校":      {},
	"福鼎七中":        {},
	"福鼎三中":        {},
	"福鼎四中":        {},
	"福鼎五中":        {},
	"福鼎一中":        {},
	"古田六中":        {},
	"古田三中":        {},
	"古田溪山高级中学":    {},
	"古田一中":        {},
	"古田玉田中学":      {},
	"古田职业中专学校":    {},
	"宁德博雅培文学校":    {},
	"宁德博雅外国语高级中学": {},
	"宁德鼎石高级中学":    {},
	"宁德二中":        {},
	"宁德高级中学":      {},
	"宁德国杰高级中学":    {},
	"宁德衡水育才中学":    {},
	"宁德九中":        {},
	"宁德六中":        {},
	"宁德民中":        {},
	"宁德四中":        {},
	"宁德五中":        {},
	"宁德一中":        {},
	"屏南二中":        {},
	"屏南三中":        {},
	"屏南县职业中专学校":   {},
	"屏南一中":        {},
	"寿宁二中":        {},
	"寿宁四中":        {},
	"寿宁一中":        {},
	"霞浦宏翔高级中学":    {},
	"霞浦六中":        {},
	"霞浦民中":        {},
	"霞浦七中":        {},
	"霞浦三中":        {},
	"霞浦县职业中专学校":   {},
	"霞浦一中":        {},
	"柘荣县职业技术学校":   {},
	"柘荣一中":        {},
	"周宁二中":        {},
	"周宁十中":        {},
	"周宁一中":        {},
}

type SevenNetClient struct {
	myBaseURL    string
	scoreBaseURL string
	key          string
	version      string
	token        string
	client       *http.Client
}

func newSevenNetClient(token string) *SevenNetClient {
	return &SevenNetClient{
		myBaseURL:    "https://szone-my.7net.cc",
		scoreBaseURL: "https://szone-score.7net.cc",
		key:          qtAESKey,
		version:      "4.5.7",
		token:        token,
		client:       httpClient(20 * time.Second),
	}
}

func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	padding := int(data[len(data)-1])
	if padding <= 0 || padding > len(data) {
		return data
	}
	return data[:len(data)-padding]
}

func decryptECBJSON(key, encrypted string) (any, error) {
	if encrypted == "" {
		return map[string]any{}, nil
	}
	block := qtSevenNetCipherBlock
	if key != qtAESKey {
		var err error
		block, err = newAESCipherBlock(key)
		if err != nil {
			return nil, err
		}
	}
	rawEncrypted, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}
	if len(rawEncrypted)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("invalid encrypted payload size")
	}

	decrypted := make([]byte, len(rawEncrypted))
	for start := 0; start < len(rawEncrypted); start += block.BlockSize() {
		block.Decrypt(decrypted[start:start+block.BlockSize()], rawEncrypted[start:start+block.BlockSize()])
	}
	decrypted = pkcs7Unpad(decrypted)

	var result any
	if err := json.Unmarshal(decrypted, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func qtEncodePassword(password string) string {
	return base64.StdEncoding.EncodeToString([]byte(password + qtPasswordSuffix))
}

func qtNormalizeGrade(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "g1", "a10", "高一", "高1":
		return "高一", true
	case "g2", "a11", "高二", "高2":
		return "高二", true
	case "g3", "a12", "高三", "高3":
		return "高三", true
	default:
		return "", false
	}
}

func qtMapSchoolName(school string) string {
	return normalizeSchoolName(school)
}

func qtUserAgent() string {
	return "Mozilla/5.0 (Linux; Android 15; V2408A Build/AP3A.240905.015.A2; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/136.0.7103.125 Mobile Safari/537.36"
}

func (c *SevenNetClient) requestWithContext(ctx *MessageContext, method, rawURL string, form url.Values) map[string]any {
	if form == nil {
		form = url.Values{}
	}
	requestURL := rawURL
	var body io.Reader
	if method == http.MethodGet && len(form) > 0 {
		requestURL += "?" + form.Encode()
	} else if method == http.MethodPost {
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(messageContext(ctx), method, requestURL, body)
	if err != nil {
		return map[string]any{"getSuccess": false, "code": -1, "msg": err.Error()}
	}
	copyHeaders(req, map[string]string{
		"Accept-Charset":  "UTF-8",
		"Accept-Encoding": "gzip",
		"Connection":      "Keep-Alive",
		"User-Agent":      qtUserAgent(),
		"Version":         c.version,
	})
	if c.token != "" {
		req.Header.Set("Token", c.token)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return map[string]any{"getSuccess": false, "code": -1, "msg": err.Error()}
	}
	defer resp.Body.Close()

	raw, err := readResponseBody(resp)
	if err != nil {
		return map[string]any{"getSuccess": false, "code": -1, "msg": err.Error()}
	}

	var resJSON map[string]any
	if err := json.Unmarshal(raw, &resJSON); err != nil {
		return map[string]any{"getSuccess": false, "code": -1, "msg": err.Error()}
	}

	status := asInt(resJSON["status"])
	message := asString(resJSON["message"])
	if status != 200 {
		return map[string]any{
			"getSuccess": false,
			"code":       status,
			"msg":        message,
			"data":       resJSON["data"],
		}
	}

	dataAny := resJSON["data"]
	responseBodyForLog := any(resJSON)
	if dataMap := asMap(dataAny); dataMap["isEncrypt"] == true {
		decrypted, err := decryptECBJSON(c.key, asString(dataMap["content"]))
		if err != nil {
			return map[string]any{"getSuccess": false, "code": -1, "msg": "解密七天网络返回结果失败"}
		}
		dataAny = decrypted
		responseBodyForLog = map[string]any{
			"status":            resJSON["status"],
			"message":           resJSON["message"],
			"data":              decrypted,
			"encrypted_payload": asString(dataMap["content"]),
		}
	}
	requestBodyForLog := ""
	if method == http.MethodPost {
		requestBodyForLog = form.Encode()
	}
	logAdminThirdPartyRequest(ctx, "7net", method, requestURL, requestBodyForLog, responseBodyForLog, nil)

	return map[string]any{
		"getSuccess": true,
		"code":       status,
		"msg":        message,
		"data":       dataAny,
	}
}

func (c *SevenNetClient) loginWithContext(ctx *MessageContext, userCode, password string) map[string]any {
	form := url.Values{}
	form.Set("userCode", userCode)
	form.Set("password", qtEncodePassword(password))
	res := c.requestWithContext(ctx, http.MethodPost, c.myBaseURL+"/login", form)
	if res["getSuccess"] == true {
		c.token = asString(asMap(res["data"])["token"])
	}
	return res
}

func (c *SevenNetClient) getUserInfoWithContext(ctx *MessageContext) map[string]any {
	return c.requestWithContext(ctx, http.MethodGet, c.myBaseURL+"/userInfo/GetUserInfo", nil)
}

func (c *SevenNetClient) getClaimExamsWithContext(ctx *MessageContext, studentName, schoolGuid, grade string, startIndex, rows int) map[string]any {
	query := url.Values{}
	query.Set("startIndex", strconv.Itoa(startIndex))
	query.Set("rows", strconv.Itoa(rows))
	query.Set("studentName", studentName)
	query.Set("schoolGuid", schoolGuid)
	query.Set("grade", grade)
	return c.requestWithContext(ctx, http.MethodGet, c.scoreBaseURL+"/exam/getClaimExams", query)
}

func (c *SevenNetClient) getUnClaimExamsWithContext(ctx *MessageContext, studentName, schoolGuid, grade string) map[string]any {
	query := url.Values{}
	query.Set("studentName", studentName)
	query.Set("schoolGuid", schoolGuid)
	query.Set("grade", grade)
	return c.requestWithContext(ctx, http.MethodGet, c.scoreBaseURL+"/exam/getUnClaimExams", query)
}

func (c *SevenNetClient) claimExamWithContext(ctx *MessageContext, examGuid, studentCode string) map[string]any {
	form := url.Values{}
	form.Set("examGuid", examGuid)
	form.Set("studentCode", studentCode)
	return c.requestWithContext(ctx, http.MethodPost, c.scoreBaseURL+"/exam/claimExam", form)
}

func (c *SevenNetClient) questionSubjectsWithContext(ctx *MessageContext, form url.Values) map[string]any {
	return c.requestWithContext(ctx, http.MethodPost, c.scoreBaseURL+"/Question/Subjects", form)
}

func (c *SevenNetClient) questionSubjectGradeWithContext(ctx *MessageContext, form url.Values) map[string]any {
	return c.requestWithContext(ctx, http.MethodPost, c.scoreBaseURL+"/Question/SubjectGrade", form)
}

func (c *SevenNetClient) questionAnswerCardURLWithContext(ctx *MessageContext, form url.Values) map[string]any {
	return c.requestWithContext(ctx, http.MethodPost, c.scoreBaseURL+"/Question/AnswerCardUrl", form)
}

func qtLoginAndSnapshotWithContext(ctx *MessageContext, username, password string, allowUnsupportedGrade bool) map[string]any {
	client := newSevenNetClient("")
	loginRes := client.loginWithContext(ctx, username, password)
	if loginRes["getSuccess"] != true {
		return map[string]any{"isSuccess": false, "msg": defaultString(asString(loginRes["msg"]), "无报错信息")}
	}
	infoRes := client.getUserInfoWithContext(ctx)
	if infoRes["getSuccess"] != true {
		return map[string]any{"isSuccess": false, "msg": fmt.Sprintf("登录成功，但获取用户信息失败: %s", asString(infoRes["msg"]))}
	}

	userData := asMap(infoRes["data"])
	grade, ok := qtNormalizeGrade(asString(userData["currentGrade"]))
	if !ok && !allowUnsupportedGrade {
		return map[string]any{"isSuccess": false, "msg": fmt.Sprintf("登录成功，但年级不被允许绑定: %s", asString(userData["currentGrade"]))}
	}
	if !ok {
		grade = asString(userData["currentGrade"])
	}

	school, ok := normalizeQTBindableSchool(asString(userData["schoolName"]))
	if !ok {
		return map[string]any{"isSuccess": false, "msg": fmt.Sprintf("登录成功，但暂不支持学校【%s】进行绑定。若您确认学校应当支持，请联系管理员。", school)}
	}

	return map[string]any{
		"isSuccess": true,
		"token":     client.token,
		"name":      asString(userData["studentName"]),
		"school":    school,
		"grade":     grade,
		"userInfo":  userData,
	}
}

func qtGetUserInfoWithContext(ctx *MessageContext, token string) map[string]any {
	return newSevenNetClient(token).getUserInfoWithContext(ctx)
}

func qtShouldRelogin(response map[string]any) bool {
	if response == nil {
		return false
	}
	msg := asString(response["msg"])
	code := asInt(response["code"])
	return code == 401 ||
		strings.Contains(msg, "已被迫下线") ||
		strings.Contains(msg, "请重新登录")
}

func qtProfileGrade(userInfo map[string]any) string {
	if grade := asString(userInfo["currentGrade"]); grade != "" {
		return strings.ToUpper(grade)
	}
	return strings.ToUpper(asString(userInfo["grade"]))
}

func qtProfileStudentName(userInfo map[string]any) string {
	if name := asString(userInfo["studentName"]); name != "" {
		return name
	}
	return asString(userInfo["nickName"])
}

func qtBoolIntString(value any) string {
	switch v := value.(type) {
	case bool:
		if v {
			return "1"
		}
		return "0"
	default:
		if asInt(value) != 0 {
			return "1"
		}
		return "0"
	}
}

func qtExamCommonForm(userInfo, exam map[string]any) url.Values {
	form := url.Values{}
	form.Set("examSchoolGuid", asString(exam["schoolGuid"]))
	form.Set("examGuid", asString(exam["examGuid"]))
	form.Set("isMock", strconv.FormatBool(asString(exam["isMock"]) == "true" || exam["isMock"] == true))
	form.Set("studentCode", asString(exam["studentCode"]))
	form.Set("schoolRuCode", asString(userInfo["ruCode"]))
	form.Set("grade", qtProfileGrade(userInfo))
	form.Set("ruCode", asString(exam["ruCode"]))
	form.Set("examType", asString(exam["examType"]))
	form.Set("schoolGuid", asString(userInfo["schoolGuid"]))
	return form
}

func qtGetClaimExamsWithContext(ctx *MessageContext, token string, userInfo map[string]any, startIndex, rows int) map[string]any {
	client := newSevenNetClient(token)
	return client.getClaimExamsWithContext(ctx, qtProfileStudentName(userInfo), asString(userInfo["schoolGuid"]), qtProfileGrade(userInfo), startIndex, rows)
}

func qtGetUnClaimExamsWithContext(ctx *MessageContext, token string, userInfo map[string]any) map[string]any {
	client := newSevenNetClient(token)
	return client.getUnClaimExamsWithContext(ctx, qtProfileStudentName(userInfo), asString(userInfo["schoolGuid"]), qtProfileGrade(userInfo))
}

func qtClaimExamWithContext(ctx *MessageContext, token, examGuid, studentCode string) map[string]any {
	client := newSevenNetClient(token)
	return client.claimExamWithContext(ctx, examGuid, studentCode)
}

func qtGetQuestionSubjectsWithContext(ctx *MessageContext, token string, userInfo, exam map[string]any) map[string]any {
	client := newSevenNetClient(token)
	return client.questionSubjectsWithContext(ctx, qtExamCommonForm(userInfo, exam))
}

func qtGetQuestionSubjectGradeWithContext(ctx *MessageContext, token string, userInfo, exam map[string]any, subject string, subjectCount int, compareClassAvg int) map[string]any {
	form := qtExamCommonForm(userInfo, exam)
	form.Set("subject", subject)
	form.Set("studentName", qtProfileStudentName(userInfo))
	form.Set("vip", qtBoolIntString(userInfo["isVip"]))
	form.Set("compareClassAvg", strconv.Itoa(compareClassAvg))
	form.Set("subjectCount", strconv.Itoa(subjectCount))
	client := newSevenNetClient(token)
	return client.questionSubjectGradeWithContext(ctx, form)
}

func qtGetQuestionAnswerCardURLWithContext(ctx *MessageContext, token string, userInfo, exam, subject map[string]any, isWatermark bool) map[string]any {
	form := qtExamCommonForm(userInfo, exam)
	form.Set("studentName", qtProfileStudentName(userInfo))
	form.Set("asiResponse", asString(subject["asiResponse"]))
	form.Set("isWatermark", strconv.FormatBool(isWatermark))
	client := newSevenNetClient(token)
	return client.questionAnswerCardURLWithContext(ctx, form)
}

func qtHashDigits(input string) string {
	sum := sha1.Sum([]byte(input))
	value := new(big.Int).SetBytes(sum[:]).Text(10)
	if len(value) >= qtHashDigitsLen {
		return value
	}
	return strings.Repeat("0", qtHashDigitsLen-len(value)) + value
}
