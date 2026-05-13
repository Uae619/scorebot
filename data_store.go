package main

import (
	"context"
	"time"
)

type DataStore interface {
	ViewUser(userKey string) map[string]any
	NewUser(userKey string)
	WriteUser(userKey string, data map[string]any)
	DeleteUser(userKey string)

	ViewTeacher(school string) map[string]any
	ViewTeacherQT(school string) map[string]any
	WriteTeacher(school string, data map[string]any)

	TryClaimMessage(ctx context.Context, msgCtx *MessageContext) (bool, error)

	QueryRows(query string, args ...any) ([]map[string]any, error)
	QueryOne(query string, args ...any) (map[string]any, error)
	QueryRowsWithColumns(query string, args ...any) ([]string, []map[string]any, error)
	TableColumnSet(tableName string) (map[string]struct{}, error)
	InvalidateTableColumnCache(tableName string)

	WriteExamContext(userKey, examID string, subjectMap map[string]any)
	ViewExamContext(userKey string) map[string]any

	ViewQTStudentExamCache(userKey string) []map[string]any
	WriteQTStudentExamCache(userKey string, exams []map[string]any, ttl time.Duration)
	DeleteQTStudentExamCache(userKey string)

	ViewQTTeacherRuleCache(school, examGuid string) qtTeacherRuleRef
	WriteQTTeacherRuleCache(school, examGuid, ruleGuid string, ruleType int, ttl time.Duration)
	DeleteQTTeacherRuleCache(school, examGuid string)

	ViewQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid string) string
	WriteQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid, payload string, ttl time.Duration)
}

type MySQLStore struct{}

func (MySQLStore) ViewUser(userKey string) map[string]any { return mysqlOpView(userKey) }
func (MySQLStore) NewUser(userKey string)                 { mysqlOpNew(userKey) }
func (MySQLStore) WriteUser(userKey string, data map[string]any) {
	mysqlOpWrite(userKey, data)
}
func (MySQLStore) DeleteUser(userKey string) { mysqlOpDelete(userKey) }

func (MySQLStore) ViewTeacher(school string) map[string]any   { return mysqlOpViewTeacher(school) }
func (MySQLStore) ViewTeacherQT(school string) map[string]any { return mysqlOpViewTeacherQT(school) }
func (MySQLStore) WriteTeacher(school string, data map[string]any) {
	mysqlOpWriteTeacher(school, data)
}

func (MySQLStore) TryClaimMessage(ctx context.Context, msgCtx *MessageContext) (bool, error) {
	return mysqlOpTryClaimMessage(ctx, msgCtx)
}

func (MySQLStore) QueryRows(query string, args ...any) ([]map[string]any, error) {
	return mysqlQueryRows(query, args...)
}
func (MySQLStore) QueryOne(query string, args ...any) (map[string]any, error) {
	return mysqlQueryOne(query, args...)
}
func (MySQLStore) QueryRowsWithColumns(query string, args ...any) ([]string, []map[string]any, error) {
	return mysqlQueryRowsWithColumns(query, args...)
}
func (MySQLStore) TableColumnSet(tableName string) (map[string]struct{}, error) {
	return mysqlGetTableColumnSet(tableName)
}
func (MySQLStore) InvalidateTableColumnCache(tableName string) {
	mysqlInvalidateTableColumnCache(tableName)
}

func (MySQLStore) WriteExamContext(userKey, examID string, subjectMap map[string]any) {
	mysqlOpWriteExamContext(userKey, examID, subjectMap)
}
func (MySQLStore) ViewExamContext(userKey string) map[string]any {
	return mysqlOpViewExamContext(userKey)
}

func (MySQLStore) ViewQTStudentExamCache(userKey string) []map[string]any {
	return mysqlOpViewQTStudentExamCache(userKey)
}
func (MySQLStore) WriteQTStudentExamCache(userKey string, exams []map[string]any, ttl time.Duration) {
	mysqlOpWriteQTStudentExamCache(userKey, exams, ttl)
}
func (MySQLStore) DeleteQTStudentExamCache(userKey string) {
	mysqlOpDeleteQTStudentExamCache(userKey)
}

func (MySQLStore) ViewQTTeacherRuleCache(school, examGuid string) qtTeacherRuleRef {
	return mysqlOpViewQTTeacherRuleCache(school, examGuid)
}
func (MySQLStore) WriteQTTeacherRuleCache(school, examGuid, ruleGuid string, ruleType int, ttl time.Duration) {
	mysqlOpWriteQTTeacherRuleCache(school, examGuid, ruleGuid, ruleType, ttl)
}
func (MySQLStore) DeleteQTTeacherRuleCache(school, examGuid string) {
	mysqlOpDeleteQTTeacherRuleCache(school, examGuid)
}

func (MySQLStore) ViewQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid string) string {
	return mysqlOpViewQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid)
}
func (MySQLStore) WriteQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid, payload string, ttl time.Duration) {
	mysqlOpWriteQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid, payload, ttl)
}

func opView(userKey string) map[string]any { return dataStore.ViewUser(userKey) }
func opNew(userKey string)                 { dataStore.NewUser(userKey) }
func opWrite(userKey string, data map[string]any) {
	dataStore.WriteUser(userKey, data)
}
func opDelete(userKey string) { dataStore.DeleteUser(userKey) }

func opViewTeacher(school string) map[string]any   { return dataStore.ViewTeacher(school) }
func opViewTeacherQT(school string) map[string]any { return dataStore.ViewTeacherQT(school) }
func opWriteTeacher(school string, data map[string]any) {
	dataStore.WriteTeacher(school, data)
}

func opTryClaimMessage(ctx context.Context, msgCtx *MessageContext) (bool, error) {
	return dataStore.TryClaimMessage(ctx, msgCtx)
}

func queryRows(query string, args ...any) ([]map[string]any, error) {
	return dataStore.QueryRows(query, args...)
}
func queryOne(query string, args ...any) (map[string]any, error) {
	return dataStore.QueryOne(query, args...)
}
func queryRowsWithColumns(query string, args ...any) ([]string, []map[string]any, error) {
	return dataStore.QueryRowsWithColumns(query, args...)
}
func getTableColumnSet(tableName string) (map[string]struct{}, error) {
	return dataStore.TableColumnSet(tableName)
}
func invalidateTableColumnCache(tableName string) {
	dataStore.InvalidateTableColumnCache(tableName)
}

func opWriteExamContext(userKey, examID string, subjectMap map[string]any) {
	dataStore.WriteExamContext(userKey, examID, subjectMap)
}
func opViewExamContext(userKey string) map[string]any {
	return dataStore.ViewExamContext(userKey)
}

func opViewQTStudentExamCache(userKey string) []map[string]any {
	return dataStore.ViewQTStudentExamCache(userKey)
}
func opWriteQTStudentExamCache(userKey string, exams []map[string]any, ttl time.Duration) {
	dataStore.WriteQTStudentExamCache(userKey, exams, ttl)
}
func opDeleteQTStudentExamCache(userKey string) {
	dataStore.DeleteQTStudentExamCache(userKey)
}

func opViewQTTeacherRuleCache(school, examGuid string) qtTeacherRuleRef {
	return dataStore.ViewQTTeacherRuleCache(school, examGuid)
}
func opWriteQTTeacherRuleCache(school, examGuid, ruleGuid string, ruleType int, ttl time.Duration) {
	dataStore.WriteQTTeacherRuleCache(school, examGuid, ruleGuid, ruleType, ttl)
}
func opDeleteQTTeacherRuleCache(school, examGuid string) {
	dataStore.DeleteQTTeacherRuleCache(school, examGuid)
}

func opViewQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid string) string {
	return dataStore.ViewQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid)
}
func opWriteQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid, payload string, ttl time.Duration) {
	dataStore.WriteQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid, payload, ttl)
}
