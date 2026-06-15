package okf

import (
	"fmt"
	"strings"
)

type frontmatter struct {
	has    bool
	values map[string]string
	keys   map[string]struct{}
}

func splitFrontmatter(text string) (frontmatter, string, error) {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return frontmatter{}, normalized, nil
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return frontmatter{has: true}, "", fmt.Errorf("frontmatter block is not closed")
	}

	values, keys, err := parseFrontmatter(lines[1:end])
	body := strings.Join(lines[end+1:], "\n")
	return frontmatter{has: true, values: values, keys: keys}, body, err
}

func parseFrontmatter(lines []string) (map[string]string, map[string]struct{}, error) {
	values := make(map[string]string)
	keys := make(map[string]struct{})

	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") || strings.HasPrefix(trimmed, "- ") {
			continue
		}

		index := strings.Index(raw, ":")
		if index <= 0 {
			return values, keys, fmt.Errorf("frontmatter line is not a top-level key: %q", raw)
		}

		key := strings.TrimSpace(raw[:index])
		if key == "" {
			return values, keys, fmt.Errorf("frontmatter key is empty")
		}

		value := cleanScalar(raw[index+1:])
		values[key] = value
		keys[key] = struct{}{}
	}

	return values, keys, nil
}

func cleanScalar(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		return ""
	}
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}
