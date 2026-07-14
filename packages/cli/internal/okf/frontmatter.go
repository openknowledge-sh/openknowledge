package okf

import (
	"fmt"
	"strings"
)

func splitFrontmatter(text string) (frontmatter, string, error) {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return frontmatter{bodyLine: 1}, normalized, nil
	}

	var warnings []frontmatterWarning
	if lines[0] != "---" {
		warnings = append(warnings, frontmatterWarning{
			line:    1,
			message: "frontmatter opening delimiter should be exactly ---",
		})
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t") == "---" {
			end = i
			if lines[i] != "---" {
				warnings = append(warnings, frontmatterWarning{
					line:    i + 1,
					message: "frontmatter closing delimiter should be exactly ---",
				})
			}
			break
		}
	}
	if end == -1 {
		return frontmatter{has: true}, "", fmt.Errorf("frontmatter block is not closed")
	}

	block := lines[1:end]
	values, keys, data, parseWarnings, err := parseYAMLFrontmatter(block, 2)
	warnings = append(warnings, parseWarnings...)
	body := strings.Join(lines[end+1:], "\n")
	return frontmatter{has: true, values: values, keys: keys, data: data, warnings: warnings, bodyLine: end + 2}, body, err
}

func ParseFrontmatterDocument(content []byte) (FrontmatterDocument, error) {
	meta, body, err := splitFrontmatter(string(content))
	document := FrontmatterDocument{
		Has:      meta.has,
		Values:   copyStringMap(meta.values),
		Data:     copyAnyMap(meta.data),
		Body:     body,
		BodyLine: meta.bodyLine,
		Warnings: exportedFrontmatterWarnings(meta.warnings),
	}
	if err != nil {
		return document, err
	}
	return document, nil
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func copyAnyMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	copied := make(map[string]any, len(values))
	for key, value := range values {
		copied[key] = copyFrontmatterValue(value)
	}
	return copied
}

func copyFrontmatterValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return copyAnyMap(typed)
	case []any:
		copied := make([]any, 0, len(typed))
		for _, item := range typed {
			copied = append(copied, copyFrontmatterValue(item))
		}
		return copied
	default:
		return typed
	}
}

func exportedFrontmatterWarnings(warnings []frontmatterWarning) []FrontmatterWarning {
	if len(warnings) == 0 {
		return nil
	}
	exported := make([]FrontmatterWarning, 0, len(warnings))
	for _, warning := range warnings {
		exported = append(exported, FrontmatterWarning{
			Line:    warning.line,
			Message: warning.message,
		})
	}
	return exported
}

func cleanScalar(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		return ""
	}
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}
