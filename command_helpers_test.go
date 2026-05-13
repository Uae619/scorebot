package main

import (
	"strings"
	"testing"
)

func TestParseBindArgs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bindArgs
		wantErr bool
	}{
		{
			name:    "valid student",
			content: "/绑定账号 1 user@example.com pass123",
			want:    bindArgs{accountType: "1", username: "user@example.com", password: "pass123"},
		},
		{
			name:    "sanitize separators for all bind types",
			content: "/绑定账号 2 12345｜67890 pass|｜123",
			want:    bindArgs{accountType: "2", username: "1234567890", password: "pass123"},
		},
		{
			name:    "invalid version",
			content: "/绑定账号 9 user pass",
			wantErr: true,
		},
		{
			name:    "invalid field count",
			content: "/绑定账号 1 onlytwo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, errMsg := parseBindArgs(tt.content)
			if tt.wantErr {
				if errMsg == "" {
					t.Fatalf("expected error")
				}
				return
			}
			if errMsg != "" {
				t.Fatalf("unexpected error: %s", errMsg)
			}
			if got != tt.want {
				t.Fatalf("got %+v want %+v", got, tt.want)
			}
		})
	}
}

func TestNormalizeContentCommandPrefix(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "leading space before fullwidth slash",
			content: "  ／查询",
			want:    "/查询",
		},
		{
			name:    "space after halfwidth slash",
			content: "/ 查询",
			want:    "/查询",
		},
		{
			name:    "space after fullwidth slash",
			content: "／ 查询",
			want:    "/查询",
		},
		{
			name:    "ideographic space after fullwidth slash",
			content: "／　查询",
			want:    "/查询",
		},
		{
			name:    "keeps argument separator",
			content: "／ 绑定账号 1 user@example.com pass123",
			want:    "/绑定账号 1 user@example.com pass123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeContent(tt.content); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestParseExamID(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantID  string
		wantNum int
		wantErr bool
	}{
		{
			name:    "valid",
			content: "/考试详情 2345678",
			wantID:  "2345678",
			wantNum: 2345678,
		},
		{
			name:    "too small",
			content: "/考试详情 9999",
			wantErr: true,
		},
		{
			name:    "out of range",
			content: "/考试详情 5000000",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotNum, errMsg := parseExamID(tt.content)
			if tt.wantErr {
				if errMsg == "" {
					t.Fatalf("expected error")
				}
				return
			}
			if errMsg != "" {
				t.Fatalf("unexpected error: %s", errMsg)
			}
			if gotID != tt.wantID || gotNum != tt.wantNum {
				t.Fatalf("got (%s,%d) want (%s,%d)", gotID, gotNum, tt.wantID, tt.wantNum)
			}
		})
	}
}

func TestIsSixDigitExamSelector(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "six digit qt short id", content: "/考试详情 123456", want: true},
		{name: "six digit in optional brackets", content: "/考试详情 【123456】", want: true},
		{name: "hfs seven digit id", content: "/考试详情 2345678"},
		{name: "no selector", content: "/考试详情"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSixDigitExamSelector(tt.content); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestQTStudentReloginErrorMessageAddsPasswordHint(t *testing.T) {
	got := qtStudentReloginErrorMessage("密码错误")
	want := "密码错误\n" + messageQTPasswordChanged
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestQTStudentReloginErrorMessageLeavesOtherErrors(t *testing.T) {
	got := qtStudentReloginErrorMessage("网络异常")
	if got != "网络异常" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatExamDate(t *testing.T) {
	if got := formatExamDate(int64(1769788800000)); got != "2026/01/31" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderExamOverviewUsesGroupedCountLabel(t *testing.T) {
	view := renderExamOverview(map[string]any{
		"data": map[string]any{
			"examId":             2489847,
			"name":               "分组考试",
			"time":               int64(1769788800000),
			"score":              20,
			"scoreBeforeGrading": 20,
			"manfen":             40,
			"mode":               3,
			"gradeStuNum":        122,
			"classStuNum":        41,
			"papers":             []any{},
		},
	})
	if !strings.Contains(view.summary, "> 人数(分组) 校 122 班 41") {
		t.Fatalf("unexpected summary: %s", view.summary)
	}
}

func TestShouldReloginExamList(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		want     bool
	}{
		{name: "token expired code 3001", response: map[string]any{"code": 3001, "msg": "登录无效，请重新登录"}, want: true},
		{name: "retry flag", response: map[string]any{"retryRelogin": true}, want: true},
		{name: "other error", response: map[string]any{"code": 500, "msg": "内部错误"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReloginExamList(tt.response); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestShouldReloginExamOverview(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		want     bool
	}{
		{name: "token expired code 3001", response: map[string]any{"code": 3001, "msg": "登录无效，请重新登录"}, want: true},
		{name: "token expired code 1000", response: map[string]any{"code": 1000, "msg": "token已过期, 请重新登录"}, want: true},
		{name: "no data", response: map[string]any{"code": 0, "msg": "暂无数据"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReloginExamOverview(tt.response); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
