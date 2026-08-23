package okf

import "strings"

func searchSectionMatchesFilters(section ContextSection, filters SearchFilters) bool {
	if !searchFilterContains(filters.Types, section.Type) {
		return false
	}
	if len(filters.Tags) == 0 {
		return true
	}
	values, ok := section.FrontmatterData["tags"]
	if !ok {
		return false
	}
	for _, tag := range searchFilterValues(values) {
		if searchFilterContains(filters.Tags, tag) {
			return true
		}
	}
	return false
}

func searchFilterContains(required []string, actual string) bool {
	if len(required) == 0 {
		return true
	}
	actual = strings.ToLower(strings.TrimSpace(actual))
	for _, value := range required {
		if actual == strings.ToLower(strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

func searchFilterValues(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return values
	case []string:
		return append([]string{}, typed...)
	default:
		return nil
	}
}
