package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type jsonCacheEntry[T any] struct {
	Value     T         `json:"value"`
	ExpiresAt time.Time `json:"expires_at"`
}

type jsonStoreData struct {
	Users              map[string]map[string]any                   `json:"users"`
	Teachers           map[string]map[string]any                   `json:"teachers"`
	Claims             map[string]struct{}                         `json:"claims"`
	ExamContext        map[string]map[string]any                   `json:"exam_context"`
	QTStudentExamCache map[string]jsonCacheEntry[[]map[string]any] `json:"qt_student_exam_cache"`
	QTTeacherRuleCache map[string]jsonCacheEntry[qtTeacherRuleRef] `json:"qt_teacher_rule_cache"`
	QTTeacherOverall   map[string]jsonCacheEntry[string]           `json:"qt_teacher_overall"`
}

type JSONStore struct {
	mu       sync.RWMutex
	data     jsonStoreData
	filePath string
}

func NewJSONStore() *JSONStore {
	path := os.Getenv("JSON_STORE_PATH")
	if path == "" {
		path = "data.json"
	}

	s := &JSONStore{
		filePath: path,
		data: jsonStoreData{
			Users:              map[string]map[string]any{},
			Teachers:           map[string]map[string]any{},
			Claims:             map[string]struct{}{},
			ExamContext:        map[string]map[string]any{},
			QTStudentExamCache: map[string]jsonCacheEntry[[]map[string]any]{},
			QTTeacherRuleCache: map[string]jsonCacheEntry[qtTeacherRuleRef]{},
			QTTeacherOverall:   map[string]jsonCacheEntry[string]{},
		},
	}

	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &s.data); err == nil {
			// Ensure maps are initialized for any nil fields after unmarshal
			if s.data.Users == nil {
				s.data.Users = map[string]map[string]any{}
			}
			if s.data.Teachers == nil {
				s.data.Teachers = map[string]map[string]any{}
			}
			if s.data.Claims == nil {
				s.data.Claims = map[string]struct{}{}
			}
			if s.data.ExamContext == nil {
				s.data.ExamContext = map[string]map[string]any{}
			}
			if s.data.QTStudentExamCache == nil {
				s.data.QTStudentExamCache = map[string]jsonCacheEntry[[]map[string]any]{}
			}
			if s.data.QTTeacherRuleCache == nil {
				s.data.QTTeacherRuleCache = map[string]jsonCacheEntry[qtTeacherRuleRef]{}
			}
			if s.data.QTTeacherOverall == nil {
				s.data.QTTeacherOverall = map[string]jsonCacheEntry[string]{}
			}
		}
		return s
	}
	s.save()
	return s
}

func (s *JSONStore) save() {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		logger.Printf("JSONStore save marshal error: %v", err)
		return
	}
	if err := os.WriteFile(s.filePath, raw, 0644); err != nil {
		logger.Printf("JSONStore save write error: %v", err)
	}
}

func (s *JSONStore) ViewUser(userKey string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.data.Users[userKey]
	if !ok {
		return map[string]any{"Return": false}
	}
	result := cloneAnyMap(data)
	result["Return"] = true
	return result
}

func (s *JSONStore) NewUser(userKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.Users[userKey]; !ok {
		s.data.Users[userKey] = map[string]any{"qqid": userKey}
		s.save()
	}
}

func (s *JSONStore) WriteUser(userKey string, data map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.Users[userKey]; !ok {
		s.data.Users[userKey] = map[string]any{"qqid": userKey}
	}
	for key, value := range data {
		s.data.Users[userKey][key] = value
	}
	s.save()
}

func (s *JSONStore) DeleteUser(userKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Users, userKey)
	delete(s.data.ExamContext, userKey)
	delete(s.data.QTStudentExamCache, userKey)
	s.save()
}

func (s *JSONStore) ViewTeacher(school string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, key := range teacherSchoolKeyVariants(school) {
		if data, ok := s.data.Teachers[key]; ok {
			result := cloneAnyMap(data)
			result["Return"] = true
			return result
		}
	}
	return map[string]any{"Return": false}
}

func (s *JSONStore) ViewTeacherQT(school string) map[string]any {
	return s.ViewTeacher("QT-" + school)
}

func (s *JSONStore) WriteTeacher(school string, data map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	candidates := teacherSchoolKeyVariants(school)
	if len(candidates) == 0 {
		return
	}
	key := candidates[0]
	if _, ok := s.data.Teachers[key]; !ok {
		s.data.Teachers[key] = map[string]any{"school": key}
	}
	for dataKey, value := range data {
		s.data.Teachers[key][dataKey] = value
	}
	s.save()
}

func (s *JSONStore) TryClaimMessage(ctx context.Context, msgCtx *MessageContext) (bool, error) {
	key := messageDedupKey(msgCtx)
	if key == "" {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.Claims[key]; ok {
		return false, nil
	}
	s.data.Claims[key] = struct{}{}
	s.save()
	return true, nil
}

func (s *JSONStore) QueryRows(query string, args ...any) ([]map[string]any, error) {
	return nil, fmt.Errorf("json store does not support SQL queries")
}

func (s *JSONStore) QueryOne(query string, args ...any) (map[string]any, error) {
	return nil, fmt.Errorf("json store does not support SQL queries")
}

func (s *JSONStore) QueryRowsWithColumns(query string, args ...any) ([]string, []map[string]any, error) {
	return nil, nil, fmt.Errorf("json store does not support SQL queries")
}

func (s *JSONStore) TableColumnSet(tableName string) (map[string]struct{}, error) {
	return nil, fmt.Errorf("json store does not support SQL schema inspection")
}

func (s *JSONStore) InvalidateTableColumnCache(tableName string) {}

func (s *JSONStore) WriteExamContext(userKey, examID string, subjectMap map[string]any) {
	if userKey == "" || examID == "" || len(subjectMap) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.ExamContext[userKey] = map[string]any{
		"exam":        examID,
		"subject_map": cloneAnyMap(subjectMap),
		"Return":      true,
	}
	s.save()
}

func (s *JSONStore) ViewExamContext(userKey string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.data.ExamContext[userKey]
	if !ok {
		return map[string]any{"Return": false}
	}
	return cloneAnyMap(data)
}

func (s *JSONStore) ViewQTStudentExamCache(userKey string) []map[string]any {
	s.mu.RLock()
	entry, ok := s.data.QTStudentExamCache[userKey]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil
	}
	result := make([]map[string]any, 0, len(entry.Value))
	for _, item := range entry.Value {
		result = append(result, cloneAnyMap(item))
	}
	return result
}

func (s *JSONStore) WriteQTStudentExamCache(userKey string, exams []map[string]any, ttl time.Duration) {
	if userKey == "" || len(exams) == 0 {
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	items := make([]map[string]any, 0, len(exams))
	for _, item := range exams {
		items = append(items, cloneAnyMap(item))
	}
	s.mu.Lock()
	s.data.QTStudentExamCache[userKey] = jsonCacheEntry[[]map[string]any]{Value: items, ExpiresAt: time.Now().Add(ttl)}
	s.mu.Unlock()
	s.save()
}

func (s *JSONStore) DeleteQTStudentExamCache(userKey string) {
	s.mu.Lock()
	delete(s.data.QTStudentExamCache, userKey)
	s.mu.Unlock()
	s.save()
}

func (s *JSONStore) ViewQTTeacherRuleCache(school, examGuid string) qtTeacherRuleRef {
	s.mu.RLock()
	entry, ok := s.data.QTTeacherRuleCache[school+"\x00"+examGuid]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.ExpiresAt) {
		return qtTeacherRuleRef{}
	}
	return entry.Value
}

func (s *JSONStore) WriteQTTeacherRuleCache(school, examGuid, ruleGuid string, ruleType int, ttl time.Duration) {
	if school == "" || examGuid == "" || ruleGuid == "" {
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	s.mu.Lock()
	s.data.QTTeacherRuleCache[school+"\x00"+examGuid] = jsonCacheEntry[qtTeacherRuleRef]{
		Value:     qtTeacherRuleRef{RuleGuid: ruleGuid, RuleType: ruleType},
		ExpiresAt: time.Now().Add(ttl),
	}
	s.mu.Unlock()
	s.save()
}

func (s *JSONStore) DeleteQTTeacherRuleCache(school, examGuid string) {
	s.mu.Lock()
	delete(s.data.QTTeacherRuleCache, school+"\x00"+examGuid)
	s.mu.Unlock()
	s.save()
}

func (s *JSONStore) ViewQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid string) string {
	s.mu.RLock()
	entry, ok := s.data.QTTeacherOverall[school+"\x00"+examRuCode+"\x00"+examGuid+"\x00"+ruleGuid]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.ExpiresAt) {
		return ""
	}
	return entry.Value
}

func (s *JSONStore) WriteQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid, payload string, ttl time.Duration) {
	if school == "" || examRuCode == "" || examGuid == "" || ruleGuid == "" || payload == "" {
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	s.mu.Lock()
	s.data.QTTeacherOverall[school+"\x00"+examRuCode+"\x00"+examGuid+"\x00"+ruleGuid] = jsonCacheEntry[string]{
		Value:     payload,
		ExpiresAt: time.Now().Add(ttl),
	}
	s.mu.Unlock()
	s.save()
}
