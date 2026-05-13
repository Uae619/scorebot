package main

import "testing"

func TestNormalizeContent(t *testing.T) {
	got := normalizeContent("／查询  123｜ &amp; 你好？")
	want := "/查询 123｜ & 你好?"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDataIntegrityCheckSubject(t *testing.T) {
	h := &CommandHandler{userdata: map[string]any{"exam": "2345678"}}
	got := h.dataIntegrityCheckSubject("/答题详情 6000000-123")
	if got["Return"] != true || got["subjectid"] != "6000000-123" {
		t.Fatalf("unexpected result: %+v", got)
	}

	got = h.dataIntegrityCheckSubject("/答题详情 6000000--123")
	if got["Return"] == true {
		t.Fatalf("unexpected double hyphen match: %+v", got)
	}
}
