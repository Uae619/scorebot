package main

import (
	"reflect"
	"sort"
	"testing"
)

func TestSchoolNameVariants(t *testing.T) {
	got := schoolNameVariants("00宁德市民族中学")
	want := []string{"宁德民中", "宁德市民族中学", "00宁德市民族中学"}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("variants = %v want %v", got, want)
	}

	got = schoolNameVariants("宁德民中")
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonical variants = %v want %v", got, want)
	}
}

func TestSchoolNamesEquivalent(t *testing.T) {
	if !schoolNamesEquivalent("宁德民中", "00宁德市民族中学") {
		t.Fatalf("expected aliases to be equivalent")
	}
	if schoolNamesEquivalent("宁德民中", "宁德一中") {
		t.Fatalf("unexpected equivalence")
	}
}

func TestNormalizeQTBindableSchool(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantSchool string
		wantOK     bool
	}{
		{
			name:       "canonical school",
			raw:        "宁德民中",
			wantSchool: "宁德民中",
			wantOK:     true,
		},
		{
			name:       "mapped school",
			raw:        "00宁德市民族中学",
			wantSchool: "宁德民中",
			wantOK:     true,
		},
		{
			name:       "unsupported school",
			raw:        "未开放学校",
			wantSchool: "未开放学校",
			wantOK:     false,
		},
		{
			name:       "empty school",
			raw:        " ",
			wantSchool: "",
			wantOK:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSchool, gotOK := normalizeQTBindableSchool(tt.raw)
			if gotSchool != tt.wantSchool || gotOK != tt.wantOK {
				t.Fatalf("normalizeQTBindableSchool(%q) = (%q, %v), want (%q, %v)", tt.raw, gotSchool, gotOK, tt.wantSchool, tt.wantOK)
			}
		})
	}
}

func TestTeacherSchoolKeyVariants(t *testing.T) {
	got := teacherSchoolKeyVariants("QT-00宁德市民族中学")
	want := []string{"QT-宁德民中", "QT-宁德市民族中学", "QT-00宁德市民族中学"}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("teacher variants = %v want %v", got, want)
	}
}
