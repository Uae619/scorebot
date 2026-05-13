package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	mu       sync.RWMutex
	db       *sql.DB
	filePath string
}

func NewSQLiteStore() *SQLiteStore {
	path := os.Getenv("SQLITE_STORE_PATH")
	if path == "" {
		path = "data.sqlite"
	}

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		logger.Printf("SQLiteStore open error: %v", err)
		return &SQLiteStore{filePath: path}
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &SQLiteStore{db: db, filePath: path}
	s.initTables()
	return s
}

func (s *SQLiteStore) initTables() {
	if s.db == nil {
		return
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS userdata (
			qqid TEXT NOT NULL PRIMARY KEY,
			mode TEXT,
			zh TEXT,
			pw TEXT,
			id INTEGER,
			school TEXT,
			xuehao TEXT,
			name TEXT,
			grade TEXT,
			banji TEXT,
			exam TEXT,
			token TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS teadata (
			school TEXT NOT NULL PRIMARY KEY,
			account TEXT,
			password TEXT,
			cookie TEXT,
			cookie_fx TEXT,
			cookie_js TEXT,
			tofenxi TEXT,
			login_mode TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS user_exam_context (
			qqid TEXT NOT NULL PRIMARY KEY,
			exam TEXT NOT NULL,
			subject_map TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS qqbot_message_dedup (
			message_key TEXT NOT NULL PRIMARY KEY,
			user_id TEXT NOT NULL,
			seq TEXT NOT NULL,
			message_id TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS qt_student_exam_cache (
			qqid TEXT NOT NULL PRIMARY KEY,
			payload TEXT NOT NULL,
			expires_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS qt_teacher_rule_cache (
			school TEXT NOT NULL,
			exam_guid TEXT NOT NULL,
			rule_guid TEXT NOT NULL,
			rule_type INTEGER NOT NULL DEFAULT 0,
			expires_at TEXT NOT NULL,
			PRIMARY KEY (school, exam_guid)
		)`,
		`CREATE TABLE IF NOT EXISTS qt_teacher_overall_cache (
			school TEXT NOT NULL,
			exam_ru_code TEXT NOT NULL,
			exam_guid TEXT NOT NULL,
			rule_guid TEXT NOT NULL,
			payload TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			PRIMARY KEY (school, exam_ru_code, exam_guid, rule_guid)
		)`,
	}
	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			logger.Printf("SQLiteStore init table error: %v", err)
		}
	}
}

func (s *SQLiteStore) ensureDB() bool {
	return s.db != nil
}

func (s *SQLiteStore) ViewUser(userKey string) map[string]any {
	if !s.ensureDB() {
		return map[string]any{"Return": false}
	}
	row := s.db.QueryRow(`
		SELECT qqid, mode, zh, pw, id, school, xuehao, name, grade, banji, exam, token
		FROM userdata WHERE qqid = ?`, userKey)

	var (
		qqid, mode, zh, pw, school, xuehao, name, grade, banji, exam, token sql.NullString
		id                                                                    sql.NullInt64
	)
	err := row.Scan(&qqid, &mode, &zh, &pw, &id, &school, &xuehao, &name, &grade, &banji, &exam, &token)
	if err == sql.ErrNoRows {
		return map[string]any{"Return": false}
	}
	if err != nil {
		logger.Printf("SQLiteStore ViewUser scan error: %v", err)
		return map[string]any{"Return": false}
	}
	return map[string]any{
		"qqid":   qqid.String,
		"mode":   nullableToAny(mode),
		"zh":     nullableToAny(zh),
		"pw":     nullableToAny(pw),
		"id":     nullableToAny(id),
		"school": nullableToAny(school),
		"xuehao": nullableToAny(xuehao),
		"name":   nullableToAny(name),
		"grade":  nullableToAny(grade),
		"banji":  nullableToAny(banji),
		"exam":   nullableToAny(exam),
		"token":  nullableToAny(token),
		"Return": true,
	}
}

func (s *SQLiteStore) NewUser(userKey string) {
	if !s.ensureDB() {
		return
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO userdata (qqid) VALUES (?)`, userKey)
	if err != nil {
		logger.Printf("SQLiteStore NewUser error: %v", err)
	}
}

func (s *SQLiteStore) WriteUser(userKey string, data map[string]any) {
	if !s.ensureDB() || len(data) == 0 {
		return
	}
	s.NewUser(userKey)
	parts := make([]string, 0, len(data))
	args := make([]any, 0, len(data)+1)
	for key, value := range data {
		parts = append(parts, fmt.Sprintf("%s = ?", key))
		args = append(args, value)
	}
	args = append(args, userKey)
	query := fmt.Sprintf("UPDATE userdata SET %s WHERE qqid = ?", strings.Join(parts, ", "))
	if _, err := s.db.Exec(query, args...); err != nil {
		logger.Printf("SQLiteStore WriteUser error: %v", err)
	}
}

func (s *SQLiteStore) DeleteUser(userKey string) {
	if !s.ensureDB() {
		return
	}
	s.db.Exec(`DELETE FROM userdata WHERE qqid = ?`, userKey)
	s.db.Exec(`DELETE FROM user_exam_context WHERE qqid = ?`, userKey)
}

func (s *SQLiteStore) ViewTeacher(school string) map[string]any {
	if !s.ensureDB() {
		return map[string]any{"Return": false}
	}
	candidates := teacherSchoolKeyVariants(school)
	if len(candidates) == 0 {
		return map[string]any{"Return": false}
	}
	args := make([]any, len(candidates))
	placeholders := make([]string, len(candidates))
	for i, c := range candidates {
		args[i] = c
		placeholders[i] = "?"
	}
	rows, err := s.db.Query(fmt.Sprintf(
		`SELECT school, account, password, cookie, cookie_fx, cookie_js, tofenxi, login_mode
		 FROM teadata WHERE school IN (%s)`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		logger.Printf("SQLiteStore ViewTeacher query error: %v", err)
		return map[string]any{"Return": false}
	}
	defer rows.Close()

	found := map[string]map[string]any{}
	for rows.Next() {
		var sc, account, password, cookie, cookieFX, cookieJS, tofenxi, loginMode sql.NullString
		if err := rows.Scan(&sc, &account, &password, &cookie, &cookieFX, &cookieJS, &tofenxi, &loginMode); err != nil {
			logger.Printf("SQLiteStore ViewTeacher scan error: %v", err)
			return map[string]any{"Return": false}
		}
		found[sc.String] = map[string]any{
			"school":     sc.String,
			"account":    nullableToAny(account),
			"password":   nullableToAny(password),
			"cookie":     nullableToAny(cookie),
			"cookie_fx":  nullableToAny(cookieFX),
			"cookie_js":  nullableToAny(cookieJS),
			"tofenxi":    nullableToAny(tofenxi),
			"login_mode": nullableToAny(loginMode),
			"Return":     true,
		}
	}
	for _, candidate := range candidates {
		if data, ok := found[candidate]; ok {
			return data
		}
	}
	return map[string]any{"Return": false}
}

func (s *SQLiteStore) ViewTeacherQT(school string) map[string]any {
	return s.ViewTeacher("QT-" + school)
}

func (s *SQLiteStore) WriteTeacher(school string, data map[string]any) {
	if !s.ensureDB() || len(data) == 0 {
		return
	}
	candidates := teacherSchoolKeyVariants(school)
	if len(candidates) == 0 {
		return
	}
	key := candidates[0]

	// Ensure row exists
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM teadata WHERE school = ?`, key).Scan(&exists); err != nil {
		logger.Printf("SQLiteStore WriteTeacher check error: %v", err)
		return
	}
	if exists == 0 {
		if _, err := s.db.Exec(`INSERT INTO teadata (school) VALUES (?)`, key); err != nil {
			logger.Printf("SQLiteStore WriteTeacher insert error: %v", err)
			return
		}
	}

	parts := make([]string, 0, len(data))
	args := make([]any, 0, len(data)+len(candidates))
	for dataKey, value := range data {
		parts = append(parts, fmt.Sprintf("%s = ?", dataKey))
		args = append(args, value)
	}
	for _, candidate := range candidates {
		args = append(args, candidate)
	}
	query := fmt.Sprintf(
		"UPDATE teadata SET %s WHERE school IN (%s)",
		strings.Join(parts, ", "),
		strings.TrimRight(strings.Repeat("?,", len(candidates)), ","),
	)
	if _, err := s.db.Exec(query, args...); err != nil {
		logger.Printf("SQLiteStore WriteTeacher error: %v", err)
	}
}

func (s *SQLiteStore) TryClaimMessage(ctx context.Context, msgCtx *MessageContext) (bool, error) {
	if !s.ensureDB() {
		return false, nil
	}
	messageKey := messageDedupKey(msgCtx)
	if messageKey == "" {
		return false, nil
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO qqbot_message_dedup (message_key, user_id, seq, message_id)
		VALUES (?, ?, ?, ?)`,
		messageKey, msgCtx.UserID, msgCtx.Seq, msgCtx.ID,
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

func (s *SQLiteStore) QueryRows(query string, args ...any) ([]map[string]any, error) {
	if !s.ensureDB() {
		return nil, fmt.Errorf("sqlite store not available")
	}
	rows, err := s.db.Query(query, args...)
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

func (s *SQLiteStore) QueryOne(query string, args ...any) (map[string]any, error) {
	rows, err := s.QueryRows(query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (s *SQLiteStore) QueryRowsWithColumns(query string, args ...any) ([]string, []map[string]any, error) {
	if !s.ensureDB() {
		return nil, nil, fmt.Errorf("sqlite store not available")
	}
	rows, err := s.db.Query(query, args...)
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
			if b, ok := values[i].([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = values[i]
			}
		}
		result = append(result, rowMap)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return columns, result, nil
}

func (s *SQLiteStore) TableColumnSet(tableName string) (map[string]struct{}, error) {
	if !s.ensureDB() {
		return nil, fmt.Errorf("sqlite store not available")
	}
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columnSet := make(map[string]struct{})
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defaultVal sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &pk); err != nil {
			return nil, err
		}
		columnSet[name] = struct{}{}
	}
	return columnSet, rows.Err()
}

func (s *SQLiteStore) InvalidateTableColumnCache(tableName string) {}

func (s *SQLiteStore) WriteExamContext(userKey, examID string, subjectMap map[string]any) {
	if !s.ensureDB() || userKey == "" || examID == "" || len(subjectMap) == 0 {
		return
	}
	raw, err := json.Marshal(subjectMap)
	if err != nil {
		logger.Printf("SQLiteStore WriteExamContext marshal error: %v", err)
		return
	}
	_, err = s.db.Exec(`
		INSERT INTO user_exam_context (qqid, exam, subject_map)
		VALUES (?, ?, ?)
		ON CONFLICT(qqid) DO UPDATE SET exam = excluded.exam, subject_map = excluded.subject_map`,
		userKey, examID, string(raw))
	if err != nil {
		logger.Printf("SQLiteStore WriteExamContext error: %v", err)
	}
}

func (s *SQLiteStore) ViewExamContext(userKey string) map[string]any {
	if !s.ensureDB() || userKey == "" {
		return map[string]any{"Return": false}
	}
	row := s.db.QueryRow(`SELECT exam, subject_map FROM user_exam_context WHERE qqid = ?`, userKey)
	var examID, subjectMapRaw string
	if err := row.Scan(&examID, &subjectMapRaw); err == sql.ErrNoRows {
		return map[string]any{"Return": false}
	} else if err != nil {
		logger.Printf("SQLiteStore ViewExamContext scan error: %v", err)
		return map[string]any{"Return": false}
	}
	subjectMap := map[string]any{}
	if err := json.Unmarshal([]byte(subjectMapRaw), &subjectMap); err != nil {
		logger.Printf("SQLiteStore ViewExamContext unmarshal error: %v", err)
		return map[string]any{"Return": false}
	}
	return map[string]any{
		"exam":        examID,
		"subject_map": subjectMap,
		"Return":      true,
	}
}

func (s *SQLiteStore) ViewQTStudentExamCache(userKey string) []map[string]any {
	if !s.ensureDB() || userKey == "" {
		return nil
	}
	var payload, expiresAtStr string
	row := s.db.QueryRow(`SELECT payload, expires_at FROM qt_student_exam_cache WHERE qqid = ?`, userKey)
	if err := row.Scan(&payload, &expiresAtStr); err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		logger.Printf("SQLiteStore ViewQTStudentExamCache scan error: %v", err)
		return nil
	}
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return nil
	}
	if time.Now().After(expiresAt) {
		return nil
	}
	items := []map[string]any{}
	if err := json.Unmarshal([]byte(payload), &items); err != nil {
		logger.Printf("SQLiteStore ViewQTStudentExamCache unmarshal error: %v", err)
		return nil
	}
	return items
}

func (s *SQLiteStore) WriteQTStudentExamCache(userKey string, exams []map[string]any, ttl time.Duration) {
	if !s.ensureDB() || userKey == "" || len(exams) == 0 {
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	raw, err := json.Marshal(exams)
	if err != nil {
		logger.Printf("SQLiteStore WriteQTStudentExamCache marshal error: %v", err)
		return
	}
	expiresAt := time.Now().Add(ttl).UTC().Format(time.RFC3339)
	_, err = s.db.Exec(`
		INSERT INTO qt_student_exam_cache (qqid, payload, expires_at)
		VALUES (?, ?, ?)
		ON CONFLICT(qqid) DO UPDATE SET payload = excluded.payload, expires_at = excluded.expires_at`,
		userKey, string(raw), expiresAt)
	if err != nil {
		logger.Printf("SQLiteStore WriteQTStudentExamCache error: %v", err)
	}
}

func (s *SQLiteStore) DeleteQTStudentExamCache(userKey string) {
	if !s.ensureDB() || userKey == "" {
		return
	}
	s.db.Exec(`DELETE FROM qt_student_exam_cache WHERE qqid = ?`, userKey)
}

func (s *SQLiteStore) ViewQTTeacherRuleCache(school, examGuid string) qtTeacherRuleRef {
	if !s.ensureDB() || school == "" || examGuid == "" {
		return qtTeacherRuleRef{}
	}
	var ruleGuid, expiresAtStr string
	var ruleType int
	row := s.db.QueryRow(`
		SELECT rule_guid, rule_type, expires_at
		FROM qt_teacher_rule_cache
		WHERE school = ? AND exam_guid = ?`, school, examGuid)
	if err := row.Scan(&ruleGuid, &ruleType, &expiresAtStr); err == sql.ErrNoRows {
		return qtTeacherRuleRef{}
	} else if err != nil {
		logger.Printf("SQLiteStore ViewQTTeacherRuleCache scan error: %v", err)
		return qtTeacherRuleRef{}
	}
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return qtTeacherRuleRef{}
	}
	if time.Now().After(expiresAt) {
		return qtTeacherRuleRef{}
	}
	return qtTeacherRuleRef{RuleGuid: ruleGuid, RuleType: ruleType}
}

func (s *SQLiteStore) WriteQTTeacherRuleCache(school, examGuid, ruleGuid string, ruleType int, ttl time.Duration) {
	if !s.ensureDB() || school == "" || examGuid == "" || ruleGuid == "" {
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	expiresAt := time.Now().Add(ttl).UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO qt_teacher_rule_cache (school, exam_guid, rule_guid, rule_type, expires_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(school, exam_guid) DO UPDATE SET
			rule_guid = excluded.rule_guid,
			rule_type = excluded.rule_type,
			expires_at = excluded.expires_at`,
		school, examGuid, ruleGuid, ruleType, expiresAt)
	if err != nil {
		logger.Printf("SQLiteStore WriteQTTeacherRuleCache error: %v", err)
	}
}

func (s *SQLiteStore) DeleteQTTeacherRuleCache(school, examGuid string) {
	if !s.ensureDB() || school == "" || examGuid == "" {
		return
	}
	s.db.Exec(`DELETE FROM qt_teacher_rule_cache WHERE school = ? AND exam_guid = ?`, school, examGuid)
}

func (s *SQLiteStore) ViewQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid string) string {
	if !s.ensureDB() || school == "" || examRuCode == "" || examGuid == "" || ruleGuid == "" {
		return ""
	}
	var payload, expiresAtStr string
	row := s.db.QueryRow(`
		SELECT payload, expires_at
		FROM qt_teacher_overall_cache
		WHERE school = ? AND exam_ru_code = ? AND exam_guid = ? AND rule_guid = ?`,
		school, examRuCode, examGuid, ruleGuid)
	if err := row.Scan(&payload, &expiresAtStr); err == sql.ErrNoRows {
		return ""
	} else if err != nil {
		logger.Printf("SQLiteStore ViewQTTeacherOverallCache scan error: %v", err)
		return ""
	}
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return ""
	}
	if time.Now().After(expiresAt) {
		return ""
	}
	return payload
}

func (s *SQLiteStore) WriteQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid, payload string, ttl time.Duration) {
	if !s.ensureDB() || school == "" || examRuCode == "" || examGuid == "" || ruleGuid == "" || payload == "" {
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	expiresAt := time.Now().Add(ttl).UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO qt_teacher_overall_cache (school, exam_ru_code, exam_guid, rule_guid, payload, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(school, exam_ru_code, exam_guid, rule_guid) DO UPDATE SET
			payload = excluded.payload,
			expires_at = excluded.expires_at`,
		school, examRuCode, examGuid, ruleGuid, payload, expiresAt)
	if err != nil {
		logger.Printf("SQLiteStore WriteQTTeacherOverallCache error: %v", err)
	}
}
