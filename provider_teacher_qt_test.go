package main

import (
	"strings"
	"testing"
)

func TestQTTeacherSelectTenantCode(t *testing.T) {
	tenants := []any{
		map[string]any{"Code": "3509139", "Name": "福鼎一中"},
		map[string]any{"Code": "352025015", "Name": "宁德市教师进修学院"},
	}

	if got := qtTeacherSelectTenantCode("QT-福鼎一中", 1, tenants); got != "3509139" {
		t.Fatalf("matched tenant code = %q", got)
	}
	if got := qtTeacherSelectTenantCode("QT-不存在的学校", 1, tenants); got != "" {
		t.Fatalf("unexpected fallback tenant code = %q", got)
	}

	roleTwoSingleTenant := []any{
		map[string]any{"Code": "3509138", "Name": "福鼎六中山前校区"},
	}
	if got := qtTeacherSelectTenantCode("QT-福鼎六中", 2, roleTwoSingleTenant); got != "3509138" {
		t.Fatalf("roleType=2 single tenant fallback code = %q", got)
	}

	roleTwoMultipleTenants := []any{
		map[string]any{"Code": "3509138", "Name": "福鼎六中山前校区"},
		map[string]any{"Code": "352025015", "Name": "宁德市教师进修学院"},
	}
	if got := qtTeacherSelectTenantCode("QT-福鼎六中", 2, roleTwoMultipleTenants); got != "3509138" {
		t.Fatalf("roleType=2 alias tenant code = %q", got)
	}

	aliasTenants := []any{
		map[string]any{"Code": "3509999", "Name": "00宁德市民族中学"},
	}
	if got := qtTeacherSelectTenantCode("QT-宁德民中", 1, aliasTenants); got != "3509999" {
		t.Fatalf("alias tenant code = %q", got)
	}
	if got := qtTeacherSelectTenantCode("QT-00宁德市民族中学", 1, []any{map[string]any{"Code": "3510000", "Name": "宁德民中"}}); got != "3510000" {
		t.Fatalf("reverse alias tenant code = %q", got)
	}
}

func TestQTTeacherTenantNames(t *testing.T) {
	got := qtTeacherTenantNames([]any{
		map[string]any{"Name": " 福鼎一中 "},
		map[string]any{"Code": "2"},
		map[string]any{"Name": "宁德市教师进修学院"},
	})
	if strings.Join(got, ",") != "福鼎一中,宁德市教师进修学院" {
		t.Fatalf("tenant names = %v", got)
	}
}

func TestQTTeacherSelectRule(t *testing.T) {
	data := map[string]any{
		"list": []any{
			map[string]any{"RuleGuid": "rule-1", "RuleType": 0, "IsDefault": false},
			map[string]any{"RuleGuid": "rule-2", "RuleType": 2, "IsDefault": true},
		},
	}
	if got := qtTeacherSelectRule(data); got.RuleGuid != "rule-2" || got.RuleType != 2 {
		t.Fatalf("got %+v", got)
	}

	fallback := map[string]any{
		"list": []any{
			map[string]any{"RuleGuid": "rule-a", "RuleType": 1, "IsDefault": false},
			map[string]any{"RuleGuid": "rule-b", "RuleType": 2, "IsDefault": false},
		},
	}
	if got := qtTeacherSelectRule(fallback); got.RuleGuid != "rule-a" || got.RuleType != 1 {
		t.Fatalf("fallback got %+v", got)
	}
}

func TestQTTeacherSelectSupplementalRule(t *testing.T) {
	data := map[string]any{
		"list": []any{
			map[string]any{"RuleGuid": "rule-default", "RuleType": 0, "IsDefault": true},
			map[string]any{"RuleGuid": "rule-group", "RuleType": 2, "IsDefault": false},
		},
	}
	if got := qtTeacherSelectSupplementalRule(data); got.RuleGuid != "rule-group" || got.RuleType != 2 {
		t.Fatalf("got %+v", got)
	}

	notDefaultZero := map[string]any{
		"list": []any{
			map[string]any{"RuleGuid": "rule-default", "RuleType": 0, "IsDefault": false},
			map[string]any{"RuleGuid": "rule-group", "RuleType": 2, "IsDefault": true},
		},
	}
	if got := qtTeacherSelectSupplementalRule(notDefaultZero); got.Valid() {
		t.Fatalf("unexpected supplemental rule %+v", got)
	}

	extraRuleType := map[string]any{
		"list": []any{
			map[string]any{"RuleGuid": "rule-default", "RuleType": 0, "IsDefault": true},
			map[string]any{"RuleGuid": "rule-group", "RuleType": 2, "IsDefault": false},
			map[string]any{"RuleGuid": "rule-other", "RuleType": 1, "IsDefault": false},
		},
	}
	if got := qtTeacherSelectSupplementalRule(extraRuleType); got.Valid() {
		t.Fatalf("unexpected supplemental rule with extra type %+v", got)
	}
}

func TestQTTeacherStatusDetection(t *testing.T) {
	if !qtTeacherShouldRelogin(map[string]any{"code": 401, "msg": "登录超时"}) {
		t.Fatalf("expected relogin")
	}
	if !qtTeacherIsForcedLogout(map[string]any{"code": 401, "msg": "您的账号在另一个地点登录，已被迫下线。"}) {
		t.Fatalf("expected forced logout")
	}
	if !qtTeacherIsCredentialInvalid(map[string]any{"code": 403, "msg": "密码错误"}) {
		t.Fatalf("expected invalid credential")
	}
	if !qtTeacherRuleDeleted(map[string]any{"code": 44001, "msg": "规则已删除"}) {
		t.Fatalf("expected rule deleted")
	}
	if !qtTeacherStudentNoPermission(map[string]any{"code": 500, "msg": "程序异常，请联系管理员"}) {
		t.Fatalf("expected permission denied")
	}
}

func TestQTTeacherLoginModeHelpers(t *testing.T) {
	if got := qtTeacherLoginModeFromAny(nil); got != qtTeacherLoginModeWeb {
		t.Fatalf("nil login mode = %q", got)
	}
	if got := qtTeacherLoginModeFromAny(""); got != qtTeacherLoginModeWeb {
		t.Fatalf("empty login mode = %q", got)
	}
	if got := qtTeacherLoginModeFromAny("app"); got != qtTeacherLoginModeApp {
		t.Fatalf("app login mode = %q", got)
	}
}

func TestQTTeacherEncryptAppCredential(t *testing.T) {
	got, err := qtTeacherEncryptAppCredential("12345")
	if err != nil {
		t.Fatalf("encrypt app credential error: %v", err)
	}
	if got != "CcouFWtDVDcE4rakxYsXcw==" {
		t.Fatalf("unexpected encrypted credential: %s", got)
	}
}

func TestQTTeacherDisplayScore(t *testing.T) {
	if got, ok := qtTeacherDisplayScore(map[string]any{"OriginalScoreText": "50", "FuFenScoreText": "73"}); !ok || got != "73(原50)" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	if got, ok := qtTeacherDisplayScore(map[string]any{"OriginalScoreText": "73", "FuFenScoreText": "-"}); !ok || got != "73" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	if _, ok := qtTeacherDisplayScore(map[string]any{"OriginalScoreText": "-"}); ok {
		t.Fatalf("expected hidden unreferenced subject")
	}
}

func TestQTTeacherPlaceholderTotalItem(t *testing.T) {
	if !qtTeacherIsPlaceholderTotalItem(map[string]any{
		"OriginalScoreText": "0",
		"PaperScoreText":    "0",
		"FuFenScoreText":    "-",
		"SchoolRank":        "-",
		"ClassRank":         "-",
	}) {
		t.Fatalf("expected placeholder total item to be hidden")
	}
	if qtTeacherIsPlaceholderTotalItem(map[string]any{
		"OriginalScoreText": "73",
		"PaperScoreText":    "73",
		"FuFenScoreText":    "-",
		"SchoolRank":        "10",
	}) {
		t.Fatalf("unexpected placeholder detection on valid total item")
	}
}

func TestQTTeacherHideExamLine(t *testing.T) {
	if !qtTeacherHideExamLine(
		map[string]any{"Avg": 0, "Max": 0},
		map[string]any{"Avg": 0, "Max": 0},
	) {
		t.Fatalf("expected hidden zero-only exam line")
	}
	if qtTeacherHideExamLine(
		map[string]any{"Avg": 58.3, "Max": 90},
		map[string]any{"Avg": 0, "Max": 0},
	) {
		t.Fatalf("unexpected hidden non-zero exam line")
	}
}

func TestQTTeacherSelectCombination(t *testing.T) {
	if got := qtTeacherSelectCombination(map[string]any{"SelectSubject": "物化生"}); got != 10 {
		t.Fatalf("physics combination got %d", got)
	}
	if got := qtTeacherSelectCombination(map[string]any{"SelectSubject": "历史政治地理"}); got != 20 {
		t.Fatalf("history combination got %d", got)
	}
	if got := qtTeacherSelectCombination(map[string]any{"SelectSubject": "-"}); got != 0 {
		t.Fatalf("empty combination got %d", got)
	}
}

func TestQTTeacherRenderAnalysis(t *testing.T) {
	overallResponse := map[string]any{
		"data": map[string]any{
			"Table": map[string]any{
				"Items": []any{
					map[string]any{
						"OrgName": "全校",
						"SingleAvgInfos": []any{
							map[string]any{"Km": "总分", "TotalStr": "679", "Avg": 568.57, "Max": 707.5},
							map[string]any{"Km": "化学", "TotalStr": "511", "Avg": 88.8, "Max": 99},
							map[string]any{"Km": "英语", "TotalStr": "0", "Avg": 0, "Max": 0},
						},
					},
					map[string]any{
						"OrgName": "高二1班",
						"SingleAvgInfos": []any{
							map[string]any{"Km": "总分", "TotalStr": "40", "Avg": 540.25, "Max": 649.5},
							map[string]any{"Km": "化学", "TotalStr": "40", "Avg": 86.6, "Max": 97},
							map[string]any{"Km": "英语", "TotalStr": "0", "Avg": 0, "Max": 0},
						},
					},
				},
			},
		},
	}

	studentResponse := map[string]any{
		"data": map[string]any{
			"Table": map[string]any{
				"Items": []any{
					map[string]any{
						"ClassName":      "高二1班",
						"SelectSubject":  "物化生",
						"TotalKmItem":    map[string]any{"OriginalScoreText": "617.5", "FuFenScoreText": "649.5", "SchoolRank": "41", "ClassRank": "1"},
						"JiFenKmItems":   []any{map[string]any{"Km": "化学", "OriginalScoreText": "95", "FuFenScoreText": "-", "UnionRank": "3", "SchoolRank": "1", "ClassRank": "1"}, map[string]any{"Km": "政治", "OriginalScoreText": "-", "FuFenScoreText": "-"}, map[string]any{"Km": "英语", "OriginalScoreText": "48", "FuFenScoreText": "-", "SchoolRank": "193", "ClassRank": "12"}},
						"NoJiFenKmItems": []any{},
					},
				},
			},
		},
	}

	text := qtTeacherRenderAnalysis(overallResponse, studentResponse)
	if !strings.Contains(text, "===== 考试数据 =====") || !strings.Contains(text, "===== 个人数据 =====") {
		t.Fatalf("unexpected text: %s", text)
	}
	if !strings.Contains(text, "- 总分 649.5(原617.5) [校41|班1]") {
		t.Fatalf("missing total score: %s", text)
	}
	if !strings.Contains(text, "考试人数 | 校 679 班 40") {
		t.Fatalf("missing total counts: %s", text)
	}
	if !strings.Contains(text, "平均分数 | 校 568.57 班 540.25") {
		t.Fatalf("missing total averages: %s", text)
	}
	if !strings.Contains(text, "- 化学 95 [联3|校1|班1]") {
		t.Fatalf("missing chemistry line: %s", text)
	}
	if strings.Contains(text, "政治") {
		t.Fatalf("unexpected hidden subject: %s", text)
	}
	if strings.Contains(text, "平均分数 | 校 0 班 0") || strings.Contains(text, "最高分数 | 校 0 班 0") {
		t.Fatalf("unexpected zero-only exam line: %s", text)
	}
	if strings.Contains(text, "===== 分组排名 =====") {
		t.Fatalf("unexpected group section: %s", text)
	}
}

func TestQTTeacherRenderGroupRankingWithTitle(t *testing.T) {
	groupResponse := map[string]any{
		"data": map[string]any{
			"Table": map[string]any{
				"Items": []any{
					map[string]any{
						"ClassName":    "高二1班",
						"TotalKmItem":  map[string]any{"OriginalScoreText": "617.5", "FuFenScoreText": "649.5", "CombinationRank": "4"},
						"JiFenKmItems": []any{map[string]any{"Km": "化学", "OriginalScoreText": "95", "FuFenScoreText": "-", "CombinationRank": "2"}},
					},
				},
			},
		},
	}

	text := qtTeacherRenderGroupRankingWithTitle(groupResponse, "赋分报告（物理组）")
	if !strings.Contains(text, "===== 赋分报告（物理组） =====") {
		t.Fatalf("missing title: %s", text)
	}
	if strings.Contains(text, "===== 考试数据 =====") || strings.Contains(text, "===== 个人数据 =====") {
		t.Fatalf("unexpected analysis sections: %s", text)
	}
	if !strings.Contains(text, "- 总分 649.5(原617.5) [组4]") || !strings.Contains(text, "- 化学 95 [组2]") {
		t.Fatalf("missing group content: %s", text)
	}
	if strings.Contains(text, "考试人数 |") || strings.Contains(text, "平均分数 |") {
		t.Fatalf("unexpected exam content: %s", text)
	}
}

func TestQTTeacherRenderPersonalAnalysis(t *testing.T) {
	studentResponse := map[string]any{
		"data": map[string]any{
			"Table": map[string]any{
				"Items": []any{
					map[string]any{
						"ClassName":    "高二1班",
						"TotalKmItem":  map[string]any{"OriginalScoreText": "617.5", "FuFenScoreText": "649.5", "SchoolRank": "41", "ClassRank": "1"},
						"JiFenKmItems": []any{map[string]any{"Km": "化学", "OriginalScoreText": "95", "FuFenScoreText": "-", "CombinationRank": "2"}},
					},
				},
			},
		},
	}

	text := qtTeacherRenderPersonalAnalysis(studentResponse)
	if strings.Contains(text, "===== 考试数据 =====") {
		t.Fatalf("unexpected exam section: %s", text)
	}
	if !strings.Contains(text, "===== 个人数据 =====") {
		t.Fatalf("missing personal section: %s", text)
	}
	if !strings.Contains(text, "- 化学 95 [组2]") {
		t.Fatalf("missing combination rank: %s", text)
	}
}

func TestQTTeacherFormatRankLineOrder(t *testing.T) {
	line := qtTeacherFormatRankLine(map[string]any{
		"UnionRank":       "8",
		"SchoolRank":      "5",
		"CombinationRank": "3",
		"ClassRank":       "1",
	})
	if line != " [联8|组3|校5|班1]" {
		t.Fatalf("unexpected rank order: %s", line)
	}
}

func TestQTTeacherRenderAnalysisHidesPlaceholderTotal(t *testing.T) {
	text := qtTeacherRenderAnalysis(
		map[string]any{
			"data": map[string]any{
				"Table": map[string]any{
					"Items": []any{
						map[string]any{
							"OrgName":        "全校",
							"SingleAvgInfos": []any{map[string]any{"Km": "语文", "Avg": 98.6, "Max": 122}},
						},
						map[string]any{
							"OrgName":        "高一1班",
							"SingleAvgInfos": []any{map[string]any{"Km": "语文", "Avg": 108.7, "Max": 121}},
						},
					},
				},
			},
		},
		map[string]any{
			"data": map[string]any{
				"Table": map[string]any{
					"Items": []any{
						map[string]any{
							"ClassName":    "高一1班",
							"TotalKmItem":  map[string]any{"OriginalScoreText": "0", "PaperScoreText": "0", "FuFenScoreText": "-", "SchoolRank": "-", "ClassRank": "-"},
							"JiFenKmItems": []any{map[string]any{"Km": "语文", "OriginalScoreText": "122", "FuFenScoreText": "-", "SchoolRank": "1", "ClassRank": "1"}},
						},
					},
				},
			},
		},
	)

	if strings.Contains(text, "- 总分 ") {
		t.Fatalf("unexpected placeholder total line: %s", text)
	}
	if !strings.Contains(text, "- 语文 122 [校1|班1]") {
		t.Fatalf("missing subject line: %s", text)
	}
}
