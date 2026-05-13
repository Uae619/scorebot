package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

type MemoryStore struct {
	mu sync.RWMutex

	users       map[string]map[string]any
	teachers    map[string]map[string]any
	claims      map[string]struct{}
	examContext map[string]map[string]any

	qtStudentExamCache map[string]cacheEntry[[]map[string]any]
	qtTeacherRuleCache map[string]cacheEntry[qtTeacherRuleRef]
	qtTeacherOverall   map[string]cacheEntry[string]
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:              map[string]map[string]any{},
		teachers:           map[string]map[string]any{},
		claims:             map[string]struct{}{},
		examContext:        map[string]map[string]any{},
		qtStudentExamCache: map[string]cacheEntry[[]map[string]any]{},
		qtTeacherRuleCache: map[string]cacheEntry[qtTeacherRuleRef]{},
		qtTeacherOverall:   map[string]cacheEntry[string]{},
	}
}

func cloneAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func (s *MemoryStore) ViewUser(userKey string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.users[userKey]
	if !ok {
		return map[string]any{"Return": false}
	}
	result := cloneAnyMap(data)
	result["Return"] = true
	return result
}

func (s *MemoryStore) NewUser(userKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[userKey]; !ok {
		s.users[userKey] = map[string]any{"qqid": userKey}
	}
}

func (s *MemoryStore) WriteUser(userKey string, data map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[userKey]; !ok {
		s.users[userKey] = map[string]any{"qqid": userKey}
	}
	for key, value := range data {
		s.users[userKey][key] = value
	}
}

func (s *MemoryStore) DeleteUser(userKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.users, userKey)
	delete(s.examContext, userKey)
	delete(s.qtStudentExamCache, userKey)
}

func (s *MemoryStore) ViewTeacher(school string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, key := range teacherSchoolKeyVariants(school) {
		if data, ok := s.teachers[key]; ok {
			result := cloneAnyMap(data)
			result["Return"] = true
			return result
		}
	}
	return map[string]any{"Return": false}
}

func (s *MemoryStore) ViewTeacherQT(school string) map[string]any {
	return s.ViewTeacher("QT-" + school)
}

func (s *MemoryStore) WriteTeacher(school string, data map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	candidates := teacherSchoolKeyVariants(school)
	if len(candidates) == 0 {
		return
	}
	key := candidates[0]
	if _, ok := s.teachers[key]; !ok {
		s.teachers[key] = map[string]any{"school": key}
	}
	for dataKey, value := range data {
		s.teachers[key][dataKey] = value
	}
}

func (s *MemoryStore) TryClaimMessage(ctx context.Context, msgCtx *MessageContext) (bool, error) {
	key := messageDedupKey(msgCtx)
	if key == "" {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.claims[key]; ok {
		return false, nil
	}
	s.claims[key] = struct{}{}
	return true, nil
}

func (s *MemoryStore) QueryRows(query string, args ...any) ([]map[string]any, error) {
	return nil, fmt.Errorf("memory store does not support SQL queries")
}

func (s *MemoryStore) QueryOne(query string, args ...any) (map[string]any, error) {
	return nil, fmt.Errorf("memory store does not support SQL queries")
}

func (s *MemoryStore) QueryRowsWithColumns(query string, args ...any) ([]string, []map[string]any, error) {
	return nil, nil, fmt.Errorf("memory store does not support SQL queries")
}

func (s *MemoryStore) TableColumnSet(tableName string) (map[string]struct{}, error) {
	return nil, fmt.Errorf("memory store does not support SQL schema inspection")
}

func (s *MemoryStore) InvalidateTableColumnCache(tableName string) {}

func (s *MemoryStore) WriteExamContext(userKey, examID string, subjectMap map[string]any) {
	if userKey == "" || examID == "" || len(subjectMap) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.examContext[userKey] = map[string]any{
		"exam":        examID,
		"subject_map": cloneAnyMap(subjectMap),
		"Return":      true,
	}
}

func (s *MemoryStore) ViewExamContext(userKey string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.examContext[userKey]
	if !ok {
		return map[string]any{"Return": false}
	}
	return cloneAnyMap(data)
}

func (s *MemoryStore) ViewQTStudentExamCache(userKey string) []map[string]any {
	s.mu.RLock()
	entry, ok := s.qtStudentExamCache[userKey]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}
	result := make([]map[string]any, 0, len(entry.value))
	for _, item := range entry.value {
		result = append(result, cloneAnyMap(item))
	}
	return result
}

func (s *MemoryStore) WriteQTStudentExamCache(userKey string, exams []map[string]any, ttl time.Duration) {
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
	s.qtStudentExamCache[userKey] = cacheEntry[[]map[string]any]{value: items, expiresAt: time.Now().Add(ttl)}
	s.mu.Unlock()
}

func (s *MemoryStore) DeleteQTStudentExamCache(userKey string) {
	s.mu.Lock()
	delete(s.qtStudentExamCache, userKey)
	s.mu.Unlock()
}

func (s *MemoryStore) ViewQTTeacherRuleCache(school, examGuid string) qtTeacherRuleRef {
	s.mu.RLock()
	entry, ok := s.qtTeacherRuleCache[school+"\x00"+examGuid]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return qtTeacherRuleRef{}
	}
	return entry.value
}

func (s *MemoryStore) WriteQTTeacherRuleCache(school, examGuid, ruleGuid string, ruleType int, ttl time.Duration) {
	if school == "" || examGuid == "" || ruleGuid == "" {
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	s.mu.Lock()
	s.qtTeacherRuleCache[school+"\x00"+examGuid] = cacheEntry[qtTeacherRuleRef]{
		value:     qtTeacherRuleRef{RuleGuid: ruleGuid, RuleType: ruleType},
		expiresAt: time.Now().Add(ttl),
	}
	s.mu.Unlock()
}

func (s *MemoryStore) DeleteQTTeacherRuleCache(school, examGuid string) {
	s.mu.Lock()
	delete(s.qtTeacherRuleCache, school+"\x00"+examGuid)
	s.mu.Unlock()
}

func (s *MemoryStore) ViewQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid string) string {
	s.mu.RLock()
	entry, ok := s.qtTeacherOverall[school+"\x00"+examRuCode+"\x00"+examGuid+"\x00"+ruleGuid]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return ""
	}
	return entry.value
}

func (s *MemoryStore) WriteQTTeacherOverallCache(school, examRuCode, examGuid, ruleGuid, payload string, ttl time.Duration) {
	if school == "" || examRuCode == "" || examGuid == "" || ruleGuid == "" || payload == "" {
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	s.mu.Lock()
	s.qtTeacherOverall[school+"\x00"+examRuCode+"\x00"+examGuid+"\x00"+ruleGuid] = cacheEntry[string]{
		value:     payload,
		expiresAt: time.Now().Add(ttl),
	}
	s.mu.Unlock()
}
