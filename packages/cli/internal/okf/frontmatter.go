package okf

import (
	"fmt"
	"strings"
)

type frontmatter struct {
	has      bool
	values   map[string]string
	keys     map[string]struct{}
	warnings []frontmatterWarning
	bodyLine int
}

type frontmatterWarning struct {
	line    int
	message string
}

type frontmatterParseError struct {
	line    int
	message string
}

func (e frontmatterParseError) Error() string {
	return e.message
}

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
		if strings.TrimSpace(lines[i]) == "---" {
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

	values, keys, parseWarnings, err := parseFrontmatter(lines[1:end], 2)
	warnings = append(warnings, parseWarnings...)
	body := strings.Join(lines[end+1:], "\n")
	return frontmatter{has: true, values: values, keys: keys, warnings: warnings, bodyLine: end + 2}, body, err
}

func parseFrontmatter(lines []string, startLine int) (map[string]string, map[string]struct{}, []frontmatterWarning, error) {
	values := make(map[string]string)
	keys := make(map[string]struct{})
	var warnings []frontmatterWarning

	for index, raw := range lines {
		line := startLine + index
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(raw, "\t") {
			warnings = append(warnings, frontmatterWarning{
				line:    line,
				message: "frontmatter indentation should use spaces, not tabs",
			})
			continue
		}
		if strings.HasPrefix(raw, " ") || strings.HasPrefix(trimmed, "- ") {
			continue
		}

		colonIndex := strings.Index(raw, ":")
		if colonIndex <= 0 {
			return values, keys, warnings, frontmatterParseError{
				line:    line,
				message: fmt.Sprintf("frontmatter line is not a top-level key: %q", raw),
			}
		}

		key := strings.TrimSpace(raw[:colonIndex])
		if key == "" {
			return values, keys, warnings, frontmatterParseError{
				line:    line,
				message: "frontmatter key is empty",
			}
		}
		if _, exists := keys[key]; exists {
			warnings = append(warnings, frontmatterWarning{
				line:    line,
				message: fmt.Sprintf("frontmatter key %q is repeated; later value wins", key),
			})
		}
		if err := validateFrontmatterScalar(raw[colonIndex+1:], line); err != nil {
			return values, keys, warnings, err
		}

		value := cleanScalar(raw[colonIndex+1:])
		values[key] = value
		keys[key] = struct{}{}
	}

	return values, keys, warnings, nil
}

func validateFrontmatterScalar(rawValue string, line int) error {
	if rawValue != "" && !strings.HasPrefix(rawValue, " ") && !strings.HasPrefix(rawValue, "\t") {
		return frontmatterParseError{
			line:    line,
			message: "frontmatter key must use YAML spacing: key: value",
		}
	}

	value := strings.TrimSpace(rawValue)
	if value == "" || strings.HasPrefix(value, "#") {
		return nil
	}

	if strings.HasPrefix(value, `"`) && !hasClosingScalarQuote(value, '"') {
		return frontmatterParseError{
			line:    line,
			message: "frontmatter double-quoted scalar is not closed",
		}
	}
	if strings.HasPrefix(value, `'`) && !hasClosingScalarQuote(value, '\'') {
		return frontmatterParseError{
			line:    line,
			message: "frontmatter single-quoted scalar is not closed",
		}
	}
	if strings.HasPrefix(value, "[") && !strings.Contains(value, "]") {
		return frontmatterParseError{
			line:    line,
			message: "frontmatter flow sequence is not closed",
		}
	}
	if strings.HasPrefix(value, "{") && !strings.Contains(value, "}") {
		return frontmatterParseError{
			line:    line,
			message: "frontmatter flow mapping is not closed",
		}
	}
	return nil
}

func hasClosingScalarQuote(value string, quote byte) bool {
	escaped := false
	for index := 1; index < len(value); index++ {
		char := value[index]
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && char == '\\' {
			escaped = true
			continue
		}
		if char == quote {
			rest := strings.TrimSpace(value[index+1:])
			return rest == "" || strings.HasPrefix(rest, "#")
		}
	}
	return false
}

func cleanScalar(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		return ""
	}
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}
