package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type AppConfig struct {
	AppID        string
	ClientSecret string
	DBUser       string
	DBPassword   string
	DBHost       string
	DBPort       string
	DBName       string
	MoonEndpoint    string
	MoonBearerToken string
	MoonGroupID     string
}

func loadConfig() AppConfig {
	return AppConfig{
		AppID:        os.Getenv("qqbot_appId"),
		ClientSecret: os.Getenv("qqbot_clientSecret"),
		DBUser:       os.Getenv("DB_USER"),
		DBPassword:   os.Getenv("DB_PASSWORD"),
		DBHost:       os.Getenv("DB_HOST"),
		DBPort:       os.Getenv("DB_PORT"),
		DBName:       os.Getenv("DB_NAME"),
		MoonEndpoint:    os.Getenv("MOON_NOTIFY_ENDPOINT"),
		MoonBearerToken: os.Getenv("MOON_NOTIFY_BEARER_TOKEN"),
		MoonGroupID:     os.Getenv("MOON_NOTIFY_GROUP_ID"),
	}
}

var (
	appConfig = loadConfig()

	dbOnce sync.Once
	dbConn *sql.DB
	dbErr  error

	tableColumnCacheMu sync.RWMutex
	tableColumnCache   = map[string]map[string]struct{}{}
)

type NullableBool struct {
	Bool  bool
	Valid bool
}

func (n *NullableBool) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		n.Bool = false
		n.Valid = false
		return nil
	case bool:
		n.Bool = v
		n.Valid = true
		return nil
	case int64:
		n.Bool = v != 0
		n.Valid = true
		return nil
	case []byte:
		return n.parseString(string(v))
	case string:
		return n.parseString(v)
	default:
		return fmt.Errorf("unsupported bool value type %T", value)
	}
}

func (n *NullableBool) parseString(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		n.Bool = false
		n.Valid = false
	case "1", "true", "t", "yes", "y", "on":
		n.Bool = true
		n.Valid = true
	case "0", "false", "f", "no", "n", "off":
		n.Bool = false
		n.Valid = true
	default:
		return fmt.Errorf("invalid bool string %q", value)
	}
	return nil
}

type UserData struct {
	QQID   string
	Mode   sql.NullString
	ZH     sql.NullString
	PW     sql.NullString
	ID     sql.NullInt64
	School sql.NullString
	Xuehao sql.NullString
	Name   sql.NullString
	Grade  sql.NullString
	Banji  sql.NullString
	Exam   sql.NullString
	Token  sql.NullString
}

type TeacherData struct {
	School    string
	Account   sql.NullString
	Password  sql.NullString
	Cookie    sql.NullString
	CookieFX  sql.NullString
	CookieJS  sql.NullString
	ToFenxi   sql.NullString
	LoginMode sql.NullString
}

func dsn() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		appConfig.DBUser,
		appConfig.DBPassword,
		appConfig.DBHost,
		appConfig.DBPort,
		appConfig.DBName,
	)
}

func getDB() (*sql.DB, error) {
	dbOnce.Do(func() {
		dbConn, dbErr = sql.Open("mysql", dsn())
		if dbErr != nil {
			return
		}
		dbConn.SetMaxOpenConns(4)
		dbConn.SetMaxIdleConns(2)
		dbConn.SetConnMaxLifetime(30 * time.Minute)
		dbConn.SetConnMaxIdleTime(5 * time.Minute)
		dbErr = dbConn.Ping()
	})
	return dbConn, dbErr
}

func nullableToAny(v any) any {
	switch n := v.(type) {
	case sql.NullString:
		if n.Valid {
			return n.String
		}
		return nil
	case sql.NullInt64:
		if n.Valid {
			return n.Int64
		}
		return nil
	case sql.NullBool:
		if n.Valid {
			return n.Bool
		}
		return nil
	case NullableBool:
		if n.Valid {
			return n.Bool
		}
		return nil
	default:
		return v
	}
}

func mysqlOpView(qqid string) map[string]any {
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpView db error: %v", err)
		return map[string]any{"Return": false}
	}

	row := db.QueryRow(`
		SELECT qqid, mode, zh, pw, id, school, xuehao, name, grade, banji, exam, token
		FROM userdata
		WHERE qqid = ?`, qqid)

	var data UserData
	err = row.Scan(
		&data.QQID,
		&data.Mode,
		&data.ZH,
		&data.PW,
		&data.ID,
		&data.School,
		&data.Xuehao,
		&data.Name,
		&data.Grade,
		&data.Banji,
		&data.Exam,
		&data.Token,
	)
	if err == sql.ErrNoRows {
		return map[string]any{"Return": false}
	}
	if err != nil {
		log.Printf("mysqlOpView scan error: %v", err)
		return map[string]any{"Return": false}
	}

	return map[string]any{
		"qqid":   data.QQID,
		"mode":   nullableToAny(data.Mode),
		"zh":     nullableToAny(data.ZH),
		"pw":     nullableToAny(data.PW),
		"id":     nullableToAny(data.ID),
		"school": nullableToAny(data.School),
		"xuehao": nullableToAny(data.Xuehao),
		"name":   nullableToAny(data.Name),
		"grade":  nullableToAny(data.Grade),
		"banji":  nullableToAny(data.Banji),
		"exam":   nullableToAny(data.Exam),
		"token":  nullableToAny(data.Token),
		"Return": true,
	}
}

func mysqlOpViewTeacher(school string) map[string]any {
	candidates := teacherSchoolKeyVariants(school)
	if len(candidates) == 0 {
		return map[string]any{"Return": false}
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpViewTeacher db error: %v", err)
		return map[string]any{"Return": false}
	}

	args := make([]any, 0, len(candidates))
	for _, candidate := range candidates {
		args = append(args, candidate)
	}
	rows, err := db.Query(fmt.Sprintf(`
		SELECT school, account, password, cookie, cookie_fx, cookie_js, tofenxi, login_mode
		FROM teadata
		WHERE school IN (%s)`, strings.TrimRight(strings.Repeat("?,", len(candidates)), ",")), args...)
	if err != nil {
		log.Printf("mysqlOpViewTeacher query error: %v", err)
		return map[string]any{"Return": false}
	}
	defer rows.Close()

	found := make(map[string]TeacherData, len(candidates))
	for rows.Next() {
		var data TeacherData
		if err := rows.Scan(
			&data.School,
			&data.Account,
			&data.Password,
			&data.Cookie,
			&data.CookieFX,
			&data.CookieJS,
			&data.ToFenxi,
			&data.LoginMode,
		); err != nil {
			log.Printf("mysqlOpViewTeacher scan error: %v", err)
			return map[string]any{"Return": false}
		}
		found[data.School] = data
	}
	if err := rows.Err(); err != nil {
		log.Printf("mysqlOpViewTeacher rows error: %v", err)
		return map[string]any{"Return": false}
	}

	for _, candidate := range candidates {
		data, ok := found[candidate]
		if !ok {
			continue
		}
		return map[string]any{
			"school":     data.School,
			"account":    nullableToAny(data.Account),
			"password":   nullableToAny(data.Password),
			"cookie":     nullableToAny(data.Cookie),
			"cookie_fx":  nullableToAny(data.CookieFX),
			"cookie_js":  nullableToAny(data.CookieJS),
			"tofenxi":    nullableToAny(data.ToFenxi),
			"login_mode": nullableToAny(data.LoginMode),
			"Return":     true,
		}
	}
	return map[string]any{"Return": false}
}

func mysqlOpViewTeacherQT(school string) map[string]any {
	return mysqlOpViewTeacher("QT-" + school)
}

func mysqlOpNew(qqid string) {
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpNew db error: %v", err)
		return
	}
	_, err = db.Exec(`
		INSERT INTO userdata (qqid) VALUES (?)
		ON DUPLICATE KEY UPDATE qqid = VALUES(qqid)`, qqid)
	if err != nil {
		log.Printf("mysqlOpNew exec error: %v", err)
	}
}

func buildUpdateSQL(table string, keyColumn string, keyValue any, data map[string]any) (string, []any) {
	parts := make([]string, 0, len(data))
	args := make([]any, 0, len(data)+1)
	for key, value := range data {
		parts = append(parts, fmt.Sprintf("`%s` = ?", key))
		args = append(args, value)
	}
	args = append(args, keyValue)
	return fmt.Sprintf("UPDATE %s SET %s WHERE `%s` = ?", table, strings.Join(parts, ", "), keyColumn), args
}

func mysqlOpWrite(qqid string, data map[string]any) {
	if len(data) == 0 {
		return
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpWrite db error: %v", err)
		return
	}
	query, args := buildUpdateSQL("userdata", "qqid", qqid, data)
	if _, err := db.Exec(query, args...); err != nil {
		log.Printf("mysqlOpWrite exec error: %v", err)
	}
}

func mysqlOpWriteTeacher(school string, data map[string]any) {
	if len(data) == 0 {
		return
	}
	candidates := teacherSchoolKeyVariants(school)
	if len(candidates) == 0 {
		return
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpWriteTeacher db error: %v", err)
		return
	}

	parts := make([]string, 0, len(data))
	args := make([]any, 0, len(data)+len(candidates))
	for key, value := range data {
		parts = append(parts, fmt.Sprintf("`%s` = ?", key))
		args = append(args, value)
	}
	for _, candidate := range candidates {
		args = append(args, candidate)
	}
	query := fmt.Sprintf(
		"UPDATE teadata SET %s WHERE `school` IN (%s)",
		strings.Join(parts, ", "),
		strings.TrimRight(strings.Repeat("?,", len(candidates)), ","),
	)
	if _, err := db.Exec(query, args...); err != nil {
		log.Printf("mysqlOpWriteTeacher exec error: %v", err)
	}
}

func mysqlOpDelete(qqid string) {
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpDelete db error: %v", err)
		return
	}
	if _, err := db.Exec(`DELETE FROM userdata WHERE qqid = ?`, qqid); err != nil {
		log.Printf("mysqlOpDelete exec error: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM user_exam_context WHERE qqid = ?`, qqid); err != nil {
		log.Printf("mysqlOpDelete exam context exec error: %v", err)
	}
}

func mysqlOpTryClaimMessage(ctx context.Context, msgCtx *MessageContext) (bool, error) {
	messageKey := messageDedupKey(msgCtx)
	if messageKey == "" {
		return false, nil
	}
	db, err := getDB()
	if err != nil {
		return false, err
	}
	result, err := db.ExecContext(requestContext(ctx), `
		INSERT IGNORE INTO qqbot_message_dedup (message_key, user_id, seq, message_id)
		VALUES (?, ?, ?, ?)`,
		messageKey,
		msgCtx.UserID,
		msgCtx.Seq,
		msgCtx.ID,
	)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected == 1, nil
}

func mysqlQueryRows(query string, args ...any) ([]map[string]any, error) {
	db, err := getDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		scans := make([]any, len(columns))
		for i := range values {
			scans[i] = &values[i]
		}
		if err := rows.Scan(scans...); err != nil {
			return nil, err
		}

		item := make(map[string]any, len(columns))
		for i, col := range columns {
			switch v := values[i].(type) {
			case []byte:
				item[col] = string(v)
			default:
				item[col] = v
			}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func mysqlQueryOne(query string, args ...any) (map[string]any, error) {
	rows, err := mysqlQueryRows(query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func mysqlQueryRowsWithColumns(query string, args ...any) ([]string, []map[string]any, error) {
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlQueryRowsWithColumns db error: %v", err)
		return nil, nil, err
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	result := make([]map[string]any, 0)
	for rows.Next() {
		// 为 Scan 准备容器
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, err
		}

		rowMap := make(map[string]any, len(columns))
		for i, col := range columns {
			v := values[i]
			// 常见驱动会把文本返回成 []byte，这里转成 string，避免后续格式化问题
			if b, ok := v.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = v
			}
		}
		result = append(result, rowMap)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return columns, result, nil
}

func mysqlGetTableColumnSet(tableName string) (map[string]struct{}, error) {
	tableColumnCacheMu.RLock()
	if cached, ok := tableColumnCache[tableName]; ok {
		tableColumnCacheMu.RUnlock()
		return cloneColumnSet(cached), nil
	}
	tableColumnCacheMu.RUnlock()

	columnSet, err := loadTableColumnSet(tableName)
	if err != nil {
		return nil, err
	}

	tableColumnCacheMu.Lock()
	if cached, ok := tableColumnCache[tableName]; ok {
		tableColumnCacheMu.Unlock()
		return cloneColumnSet(cached), nil
	}
	tableColumnCache[tableName] = cloneColumnSet(columnSet)
	tableColumnCacheMu.Unlock()
	return cloneColumnSet(columnSet), nil
}

func loadTableColumnSet(tableName string) (map[string]struct{}, error) {
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlGetTableColumnSet db error: %v", err)
		return nil, err
	}

	rows, err := db.Query(fmt.Sprintf("SHOW COLUMNS FROM `%s`", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columnSet := make(map[string]struct{})
	for rows.Next() {
		var field string
		var columnType string
		var nullable string
		var key string
		var defaultValue sql.NullString
		var extra string
		if err := rows.Scan(&field, &columnType, &nullable, &key, &defaultValue, &extra); err != nil {
			return nil, err
		}
		columnSet[field] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columnSet, nil
}

func cloneColumnSet(src map[string]struct{}) map[string]struct{} {
	dst := make(map[string]struct{}, len(src))
	for key := range src {
		dst[key] = struct{}{}
	}
	return dst
}

func mysqlInvalidateTableColumnCache(tableName string) {
	tableColumnCacheMu.Lock()
	delete(tableColumnCache, tableName)
	tableColumnCacheMu.Unlock()
}

func mysqlOpWriteExamContext(qqid, examID string, subjectMap map[string]any) {
	if qqid == "" || examID == "" || len(subjectMap) == 0 {
		return
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpWriteExamContext db error: %v", err)
		return
	}
	raw, err := json.Marshal(subjectMap)
	if err != nil {
		log.Printf("mysqlOpWriteExamContext marshal error: %v", err)
		return
	}
	_, err = db.Exec(`
		INSERT INTO user_exam_context (qqid, exam, subject_map)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE exam = VALUES(exam), subject_map = VALUES(subject_map)`,
		qqid, examID, string(raw))
	if err != nil {
		log.Printf("mysqlOpWriteExamContext exec error: %v", err)
	}
}

func mysqlOpViewExamContext(qqid string) map[string]any {
	if qqid == "" {
		return map[string]any{"Return": false}
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpViewExamContext db error: %v", err)
		return map[string]any{"Return": false}
	}

	row := db.QueryRow(`SELECT exam, subject_map FROM user_exam_context WHERE qqid = ?`, qqid)
	var examID string
	var subjectMapRaw string
	if err := row.Scan(&examID, &subjectMapRaw); err == sql.ErrNoRows {
		return map[string]any{"Return": false}
	} else if err != nil {
		log.Printf("mysqlOpViewExamContext scan error: %v", err)
		return map[string]any{"Return": false}
	}

	subjectMap := map[string]any{}
	if err := json.Unmarshal([]byte(subjectMapRaw), &subjectMap); err != nil {
		log.Printf("mysqlOpViewExamContext unmarshal error: %v", err)
		return map[string]any{"Return": false}
	}

	return map[string]any{
		"exam":        examID,
		"subject_map": subjectMap,
		"Return":      true,
	}
}

func mysqlOpViewQTStudentExamCache(qqid string) []map[string]any {
	if qqid == "" {
		return nil
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpViewQTStudentExamCache db error: %v", err)
		return nil
	}

	var payload string
	var expiresAt time.Time
	row := db.QueryRow(`
		SELECT payload, expires_at
		FROM qt_student_exam_cache
		WHERE qqid = ?`, qqid)
	if err := row.Scan(&payload, &expiresAt); err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		log.Printf("mysqlOpViewQTStudentExamCache scan error: %v", err)
		return nil
	}
	if time.Now().After(expiresAt) {
		return nil
	}

	items := []map[string]any{}
	if err := json.Unmarshal([]byte(payload), &items); err != nil {
		log.Printf("mysqlOpViewQTStudentExamCache unmarshal error: %v", err)
		return nil
	}
	return items
}

func mysqlOpWriteQTStudentExamCache(qqid string, exams []map[string]any, ttl time.Duration) {
	if qqid == "" || len(exams) == 0 {
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpWriteQTStudentExamCache db error: %v", err)
		return
	}
	raw, err := json.Marshal(exams)
	if err != nil {
		log.Printf("mysqlOpWriteQTStudentExamCache marshal error: %v", err)
		return
	}
	expiresAt := time.Now().Add(ttl)
	_, err = db.Exec(`
		INSERT INTO qt_student_exam_cache (qqid, payload, expires_at)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE payload = VALUES(payload), expires_at = VALUES(expires_at)`,
		qqid, string(raw), expiresAt)
	if err != nil {
		log.Printf("mysqlOpWriteQTStudentExamCache exec error: %v", err)
	}
}

func mysqlOpDeleteQTStudentExamCache(qqid string) {
	if qqid == "" {
		return
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpDeleteQTStudentExamCache db error: %v", err)
		return
	}
	if _, err := db.Exec(`
		DELETE FROM qt_student_exam_cache
		WHERE qqid = ?`, qqid); err != nil {
		log.Printf("mysqlOpDeleteQTStudentExamCache exec error: %v", err)
	}
}

func mysqlOpViewQTTeacherRuleCache(school, examGuid string) qtTeacherRuleRef {
	if school == "" || examGuid == "" {
		return qtTeacherRuleRef{}
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpViewQTTeacherRuleCache db error: %v", err)
		return qtTeacherRuleRef{}
	}

	var ruleGuid string
	var ruleType int
	var expiresAt time.Time
	row := db.QueryRow(`
		SELECT rule_guid, rule_type, expires_at
		FROM qt_teacher_rule_cache
		WHERE school = ? AND exam_guid = ?`, school, examGuid)
	if err := row.Scan(&ruleGuid, &ruleType, &expiresAt); err == sql.ErrNoRows {
		return qtTeacherRuleRef{}
	} else if err != nil {
		log.Printf("mysqlOpViewQTTeacherRuleCache scan error: %v", err)
		return qtTeacherRuleRef{}
	}
	if time.Now().After(expiresAt) {
		return qtTeacherRuleRef{}
	}
	return qtTeacherRuleRef{RuleGuid: ruleGuid, RuleType: ruleType}
}

func mysqlOpWriteQTTeacherRuleCache(school, examGuid, ruleGuid string, ruleType int, ttl time.Duration) {
	if school == "" || examGuid == "" || ruleGuid == "" {
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpWriteQTTeacherRuleCache db error: %v", err)
		return
	}
	expiresAt := time.Now().Add(ttl)
	_, err = db.Exec(`
		INSERT INTO qt_teacher_rule_cache (school, exam_guid, rule_guid, rule_type, expires_at)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE rule_guid = VALUES(rule_guid), rule_type = VALUES(rule_type), expires_at = VALUES(expires_at)`,
		school, examGuid, ruleGuid, ruleType, expiresAt)
	if err != nil {
		log.Printf("mysqlOpWriteQTTeacherRuleCache exec error: %v", err)
	}
}

func mysqlOpDeleteQTTeacherRuleCache(school, examGuid string) {
	if school == "" || examGuid == "" {
		return
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpDeleteQTTeacherRuleCache db error: %v", err)
		return
	}
	if _, err := db.Exec(`
		DELETE FROM qt_teacher_rule_cache
		WHERE school = ? AND exam_guid = ?`, school, examGuid); err != nil {
		log.Printf("mysqlOpDeleteQTTeacherRuleCache exec error: %v", err)
	}
}

func mysqlOpViewQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid string) string {
	if school == "" || examRuCode == "" || examGuid == "" || ruleGuid == "" {
		return ""
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpViewQTTeacherOverallCache db error: %v", err)
		return ""
	}

	var payload string
	var expiresAt time.Time
	row := db.QueryRow(`
		SELECT payload, expires_at
		FROM qt_teacher_overall_cache
		WHERE school = ? AND exam_ru_code = ? AND exam_guid = ? AND rule_guid = ?`,
		school, examRuCode, examGuid, ruleGuid)
	if err := row.Scan(&payload, &expiresAt); err == sql.ErrNoRows {
		return ""
	} else if err != nil {
		log.Printf("mysqlOpViewQTTeacherOverallCache scan error: %v", err)
		return ""
	}
	if time.Now().After(expiresAt) {
		return ""
	}
	return payload
}

func mysqlOpWriteQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid, payload string, ttl time.Duration) {
	if school == "" || examRuCode == "" || examGuid == "" || ruleGuid == "" || payload == "" {
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	db, err := getDB()
	if err != nil {
		log.Printf("mysqlOpWriteQTTeacherOverallCache db error: %v", err)
		return
	}
	expiresAt := time.Now().Add(ttl)
	_, err = db.Exec(`
		INSERT INTO qt_teacher_overall_cache (school, exam_ru_code, exam_guid, rule_guid, payload, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE payload = VALUES(payload), expires_at = VALUES(expires_at)`,
		school, examRuCode, examGuid, ruleGuid, payload, expiresAt)
	if err != nil {
		log.Printf("mysqlOpWriteQTTeacherOverallCache exec error: %v", err)
	}
}
