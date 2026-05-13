package main

import (
	"strings"
	"testing"
)

func TestQTEncodePassword(t *testing.T) {
	got := qtEncodePassword("123456")
	want := "MTIzNDU2e01UZ3lNalUyTURVME1qRjdjM3B2Ym1WOX0="
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestQTNormalizeGrade(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "g1", want: "高一", ok: true},
		{input: "A11", want: "高二", ok: true},
		{input: "高三", want: "高三", ok: true},
		{input: "a9", want: "", ok: false},
	}

	for _, tt := range tests {
		got, ok := qtNormalizeGrade(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("input=%s got=(%s,%v) want=(%s,%v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestQTAllowUnsupportedGradeForUser(t *testing.T) {
	if qtAllowUnsupportedGradeForUser("123") {
		t.Fatalf("unexpected unsupported grade bypass")
	}
}

func TestQTShouldRelogin(t *testing.T) {
	if !qtShouldRelogin(map[string]any{"code": 401, "msg": "您的账号在另一个地点登录，已被迫下线。"}) {
		t.Fatalf("expected relogin on forced logout")
	}
	if qtShouldRelogin(map[string]any{"code": 403, "msg": "密码错误"}) {
		t.Fatalf("unexpected relogin on password error")
	}
}

func TestQTHashDigits(t *testing.T) {
	got := qtHashDigits("20250619-0921-416d-c36c-adfe1f708352")
	if len(got) != qtHashDigitsLen {
		t.Fatalf("unexpected len: %d", len(got))
	}
	for _, ch := range got {
		if ch < '0' || ch > '9' {
			t.Fatalf("unexpected non-digit hash string: %q", got)
		}
	}
}

func TestBuildUniqueNumericPrefixes(t *testing.T) {
	got := buildUniqueNumericPrefixes(map[string]string{
		"a": "1234567",
		"b": "1234568",
		"c": "9876543",
	}, 6)

	if got["a"] != "1234567" || got["b"] != "1234568" || got["c"] != "987654" {
		t.Fatalf("unexpected prefixes: %+v", got)
	}
}

func TestQTEstimateRankRange(t *testing.T) {
	start, end, ok := qtEstimateRankRange("A1", 582)
	if !ok || start != 1 || end != 6 {
		t.Fatalf("got (%d,%d,%v)", start, end, ok)
	}

	start, end, ok = qtEstimateRankRange("E", 100)
	if !ok || start != 100 || end != 100 {
		t.Fatalf("got (%d,%d,%v)", start, end, ok)
	}
}

func TestQTBuildSubjectContext(t *testing.T) {
	context := qtBuildSubjectContext(map[string]any{"examGuid": "exam-1"}, map[string]any{
		"subjects": []any{
			map[string]any{"km": "总分", "question": map[string]any{"asiresponse": ""}},
			map[string]any{"km": "数学", "myScore": 95, "fullScore": 100, "question": map[string]any{"asiresponse": "asi-1"}},
			map[string]any{"km": "英语", "myScore": 88, "fullScore": 100, "question": map[string]any{"asiresponse": "asi-2"}},
			map[string]any{"km": "化学", "myScore": 77, "fullScore": 100, "question": map[string]any{"asiresponse": ""}},
		},
	})

	items := asMap(context["__items"])
	aliases := asMap(context["__aliases"])
	if len(items) != 2 {
		t.Fatalf("unexpected items: %+v", items)
	}
	if asString(aliases["数学"]) != "001" || asString(aliases["英语"]) != "002" {
		t.Fatalf("unexpected aliases: %+v", aliases)
	}
	if asInt(context["__subjectCount"]) != 2 {
		t.Fatalf("unexpected subject count: %+v", context)
	}
}

func TestQTRenderExamOverview(t *testing.T) {
	got := qtRenderExamOverview(
		map[string]any{"examName": "七天测试", "time": "2025-07-01", "score": "541.25"},
		"123456",
		map[string]any{
			"subjects": []any{
				map[string]any{"km": "总分", "myScore": "541.25", "fullScore": "600", "question": map[string]any{"asiresponse": ""}},
				map[string]any{"km": "数学", "myScore": "95", "fullScore": "100", "question": map[string]any{"asiresponse": "asi-1"}},
			},
		},
		map[string]any{
			"data": map[string]any{
				"report": map[string]any{"myScore": "541.25", "fullScore": "600", "total": 582, "grade": "A1"},
			},
		},
	)

	if !strings.Contains(got, "预估排名（可能有错误） 1-6") {
		t.Fatalf("unexpected overview: %s", got)
	}
	if strings.Contains(got, "学校排名") {
		t.Fatalf("unexpected school rank in overview: %s", got)
	}
	if !strings.Contains(got, "001 | 数学 | 95/100") {
		t.Fatalf("unexpected subject line: %s", got)
	}
}

func TestQTRenderExamOverviewHidesZeroTotal(t *testing.T) {
	got := qtRenderExamOverview(
		map[string]any{"examName": "七天测试", "time": "2026-03-28", "score": "0"},
		"002050",
		map[string]any{
			"subjects": []any{
				map[string]any{"km": "总分", "myScore": "0", "fullScore": "0", "question": map[string]any{"asiresponse": ""}},
				map[string]any{"km": "化学", "myScore": "73", "fullScore": "100", "question": map[string]any{"asiresponse": "asi-1"}},
			},
		},
		map[string]any{
			"data": map[string]any{
				"report": map[string]any{"myScore": "0", "fullScore": "0"},
			},
		},
	)

	if strings.Contains(got, "总分 0/0") {
		t.Fatalf("unexpected zero total: %s", got)
	}
	if !strings.Contains(got, "001 | 化学 | 73/100") {
		t.Fatalf("unexpected subject line: %s", got)
	}
}

func TestExtractAnswerSheetURLsAnswerUrls(t *testing.T) {
	got, source := extractAnswerSheetURLs(map[string]any{
		"data": map[string]any{
			"answerUrls": []any{"u1", "u2"},
		},
	})
	if len(got) != 2 || source != "data.answerUrls" {
		t.Fatalf("got len=%d source=%s", len(got), source)
	}
}

func TestQTRenderSubjectGradeDetail(t *testing.T) {
	got := qtRenderSubjectGradeDetail(
		map[string]any{"subject": "数学", "score": "95", "fullScore": "100"},
		map[string]any{
			"data": map[string]any{
				"report": map[string]any{"myScore": "95", "fullScore": "100", "total": 581, "grade": "A1"},
			},
		},
	)

	if !strings.Contains(got, " > 数学 分数 95/100") {
		t.Fatalf("unexpected detail: %s", got)
	}
	if !strings.Contains(got, "预估年级排名 1-6") {
		t.Fatalf("unexpected rank detail: %s", got)
	}
}
