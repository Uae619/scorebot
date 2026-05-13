package main

import "strings"

func normalizeSchoolName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}
	if mapped, ok := schoolMapping[name]; ok {
		return mapped
	}
	return name
}

func schoolNameVariants(raw string) []string {
	canonical := normalizeSchoolName(raw)
	if canonical == "" {
		return nil
	}

	variants := []string{canonical}
	seen := map[string]struct{}{canonical: {}}
	for alias, mapped := range schoolMapping {
		if normalizeSchoolName(mapped) != canonical {
			continue
		}
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		if _, ok := seen[alias]; ok {
			continue
		}
		seen[alias] = struct{}{}
		variants = append(variants, alias)
	}
	return variants
}

func schoolNamesEquivalent(left, right string) bool {
	leftName := normalizeSchoolName(left)
	rightName := normalizeSchoolName(right)
	return leftName != "" && leftName == rightName
}

func normalizeQTBindableSchool(raw string) (string, bool) {
	school := normalizeSchoolName(raw)
	if school == "" {
		return "", false
	}
	_, ok := qtBindableSchools[school]
	return school, ok
}

func teacherSchoolKeyVariants(raw string) []string {
	school := strings.TrimSpace(raw)
	if school == "" {
		return nil
	}
	if strings.HasPrefix(school, "QT-") {
		names := schoolNameVariants(strings.TrimSpace(strings.TrimPrefix(school, "QT-")))
		variants := make([]string, 0, len(names))
		for _, name := range names {
			variants = append(variants, "QT-"+name)
		}
		return variants
	}
	return schoolNameVariants(school)
}
