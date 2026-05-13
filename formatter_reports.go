package main

import (
	"fmt"
	"sort"
	"strings"
)

func textformatExamlist(examData map[string]any) string {
	if asInt(examData["code"]) != 0 {
		return asString(examData["msg"])
	}
	examList, _ := extractExamList(examData)
	if len(examList) == 0 {
		return "* 错误: 好分数账号内没有可查询的考试。请确认考试使用的平台是否为好分数、账号是否绑定了正确的学生、相应考试是否已有科目发布。"
	}
	lines := make([]string, 0, len(examList)+3)
	lines = append(lines, "【好分数考试列表】", "考试ID ✱ 考试名称")
	for _, item := range examList {
		record := asMap(item)
		examID := asInt(record["examId"])
		if examID == 0 {
			examID = asInt(record["examid"])
		}
		lines = append(lines, fmt.Sprintf("%d ✱ %s", examID, defaultString(asString(record["name"]), "未知")))
	}
	lines = append(lines, "➤回复：/考试详情 [考试ID] 以查询详细信息。")
	return strings.Join(lines, "\n")
}

func extractExamList(examData map[string]any) ([]any, string) {
	if list := asSlice(examData["data"]); list != nil {
		return list, "data"
	}

	data := asMap(examData["data"])
	candidates := []struct {
		key   string
		value any
	}{
		{key: "data.list", value: data["list"]},
		{key: "data.examList", value: data["examList"]},
		{key: "data.archives", value: data["archives"]},
		{key: "data.archiveList", value: data["archiveList"]},
		{key: "data.items", value: data["items"]},
		{key: "data.rows", value: data["rows"]},
		{key: "list", value: examData["list"]},
	}
	for _, candidate := range candidates {
		if list := asSlice(candidate.value); list != nil {
			return list, candidate.key
		}
	}

	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if list := asSlice(data[key]); list != nil {
			return list, "data." + key
		}
	}
	return nil, ""
}

func extractAnswerSheetURLs(subInfo map[string]any) ([]any, string) {
	data := asMap(subInfo["data"])
	candidates := []struct {
		key   string
		value any
	}{
		{key: "data.answerUrls", value: data["answerUrls"]},
		{key: "data.paperPic", value: data["paperPic"]},
		{key: "data.paperPicResize", value: data["paperPicResize"]},
		{key: "data.url", value: data["url"]},
		{key: "data.urlResize", value: data["urlResize"]},
		{key: "data.urls", value: data["urls"]},
		{key: "data.images", value: data["images"]},
		{key: "data.pictures", value: data["pictures"]},
		{key: "data.answerPictures", value: data["answerPictures"]},
		{key: "data.answerPictureUrls", value: data["answerPictureUrls"]},
	}
	for _, candidate := range candidates {
		if list := asSlice(candidate.value); list != nil {
			return list, candidate.key
		}
	}

	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if list := asSlice(data[key]); list != nil {
			return list, "data." + key
		}
	}
	return nil, ""
}

func textformatSublistFromTeacher(textDict map[string]string) string {
	keywords := []string{"总分", "语文", "数学", "英语", "日语", "俄语", "物理", "历史", "化学", "生物", "政治", "地理", "技术"}
	used := map[string]struct{}{}
	lines := make([]string, 0, len(textDict))
	for _, key := range keywords {
		if value, ok := textDict[key]; ok {
			lines = append(lines, value)
			used[key] = struct{}{}
		}
	}
	others := make([]string, 0)
	for key, value := range textDict {
		if _, ok := used[key]; ok {
			continue
		}
		others = append(others, value)
	}
	sort.Strings(others)
	lines = append(lines, others...)
	return strings.Join(lines, "\n")
}
