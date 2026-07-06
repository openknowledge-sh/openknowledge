package okf

import (
	"fmt"
	"strconv"
	"strings"
)

type structuredFrontmatterLine struct {
	line   int
	indent int
	text   string
}

type structuredFrontmatterParser struct {
	lines []structuredFrontmatterLine
	index int
}

func parseStructuredFrontmatter(rawLines []string, startLine int) (map[string]any, error) {
	lines, err := structuredFrontmatterLines(rawLines, startLine)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}
	parser := structuredFrontmatterParser{lines: lines}
	values, err := parser.parseMap(lines[0].indent)
	if err != nil {
		return nil, err
	}
	if parser.index < len(parser.lines) {
		line := parser.lines[parser.index]
		return values, frontmatterParseError{
			line:    line.line,
			message: fmt.Sprintf("frontmatter line has unexpected indentation: %q", line.text),
		}
	}
	return values, nil
}

func structuredFrontmatterLines(rawLines []string, startLine int) ([]structuredFrontmatterLine, error) {
	lines := make([]structuredFrontmatterLine, 0, len(rawLines))
	for index, raw := range rawLines {
		lineNumber := startLine + index
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(raw, "\t") {
			return nil, frontmatterParseError{
				line:    lineNumber,
				message: "frontmatter indentation should use spaces, not tabs",
			}
		}
		indent := 0
		for indent < len(raw) && raw[indent] == ' ' {
			indent++
		}
		text := strings.TrimRight(raw[indent:], " \t")
		if text == "" || strings.HasPrefix(strings.TrimSpace(text), "#") {
			continue
		}
		lines = append(lines, structuredFrontmatterLine{
			line:   lineNumber,
			indent: indent,
			text:   text,
		})
	}
	return lines, nil
}

func (parser *structuredFrontmatterParser) parseMap(indent int) (map[string]any, error) {
	values := map[string]any{}
	for parser.index < len(parser.lines) {
		line := parser.lines[parser.index]
		if line.indent < indent {
			break
		}
		if line.indent > indent {
			return values, frontmatterParseError{
				line:    line.line,
				message: fmt.Sprintf("frontmatter line has unexpected indentation: %q", line.text),
			}
		}
		if strings.HasPrefix(line.text, "- ") {
			break
		}

		key, rawValue, err := splitStructuredKeyValue(line)
		if err != nil {
			return values, err
		}
		if rawValue == "" {
			parser.index++
			if parser.index >= len(parser.lines) || parser.lines[parser.index].indent <= indent {
				values[key] = ""
				continue
			}
			childIndent := parser.lines[parser.index].indent
			if strings.HasPrefix(parser.lines[parser.index].text, "- ") {
				child, err := parser.parseList(childIndent)
				if err != nil {
					return values, err
				}
				values[key] = child
				continue
			}
			child, err := parser.parseMap(childIndent)
			if err != nil {
				return values, err
			}
			values[key] = child
			continue
		}

		value, err := parseStructuredScalar(rawValue, line.line)
		if err != nil {
			return values, err
		}
		values[key] = value
		parser.index++
	}
	return values, nil
}

func (parser *structuredFrontmatterParser) parseList(indent int) ([]any, error) {
	var values []any
	for parser.index < len(parser.lines) {
		line := parser.lines[parser.index]
		if line.indent < indent {
			break
		}
		if line.indent > indent {
			return values, frontmatterParseError{
				line:    line.line,
				message: fmt.Sprintf("frontmatter line has unexpected indentation: %q", line.text),
			}
		}
		if !strings.HasPrefix(line.text, "- ") {
			break
		}

		item := strings.TrimSpace(strings.TrimPrefix(line.text, "- "))
		if item == "" {
			parser.index++
			if parser.index >= len(parser.lines) || parser.lines[parser.index].indent <= indent {
				values = append(values, "")
				continue
			}
			childIndent := parser.lines[parser.index].indent
			if strings.HasPrefix(parser.lines[parser.index].text, "- ") {
				child, err := parser.parseList(childIndent)
				if err != nil {
					return values, err
				}
				values = append(values, child)
				continue
			}
			child, err := parser.parseMap(childIndent)
			if err != nil {
				return values, err
			}
			values = append(values, child)
			continue
		}

		if key, rawValue, ok := splitInlineMapItem(item); ok {
			entry := map[string]any{}
			value, err := parseStructuredScalar(rawValue, line.line)
			if err != nil {
				return values, err
			}
			entry[key] = value
			parser.index++
			if parser.index < len(parser.lines) && parser.lines[parser.index].indent > indent {
				childIndent := parser.lines[parser.index].indent
				child, err := parser.parseMap(childIndent)
				if err != nil {
					return values, err
				}
				for childKey, childValue := range child {
					entry[childKey] = childValue
				}
			}
			values = append(values, entry)
			continue
		}

		value, err := parseStructuredScalar(item, line.line)
		if err != nil {
			return values, err
		}
		values = append(values, value)
		parser.index++
	}
	return values, nil
}

func splitStructuredKeyValue(line structuredFrontmatterLine) (string, string, error) {
	colonIndex := findUnquotedRune(line.text, ':')
	if colonIndex <= 0 {
		return "", "", frontmatterParseError{
			line:    line.line,
			message: fmt.Sprintf("frontmatter line is not a mapping entry: %q", line.text),
		}
	}
	key := strings.TrimSpace(line.text[:colonIndex])
	if key == "" {
		return "", "", frontmatterParseError{
			line:    line.line,
			message: "frontmatter key is empty",
		}
	}
	if strings.ContainsAny(key, "[]{}#,") {
		return "", "", frontmatterParseError{
			line:    line.line,
			message: fmt.Sprintf("frontmatter key is not supported: %q", key),
		}
	}
	after := line.text[colonIndex+1:]
	if after != "" && !strings.HasPrefix(after, " ") && !strings.HasPrefix(after, "\t") {
		return "", "", frontmatterParseError{
			line:    line.line,
			message: "frontmatter key must use YAML spacing: key: value",
		}
	}
	return key, strings.TrimSpace(line.text[colonIndex+1:]), nil
}

func splitInlineMapItem(item string) (string, string, bool) {
	colonIndex := findUnquotedRune(item, ':')
	if colonIndex <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(item[:colonIndex])
	if key == "" || strings.ContainsAny(key, " []{}#,") {
		return "", "", false
	}
	after := item[colonIndex+1:]
	if after != "" && !strings.HasPrefix(after, " ") && !strings.HasPrefix(after, "\t") {
		return "", "", false
	}
	return key, strings.TrimSpace(after), true
}

func parseStructuredScalar(raw string, line int) (any, error) {
	value := stripStructuredComment(strings.TrimSpace(raw))
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "[") {
		return parseStructuredFlowSequence(value, line)
	}
	if strings.HasPrefix(value, "{") {
		return nil, frontmatterParseError{
			line:    line,
			message: "frontmatter flow mappings are not supported",
		}
	}
	if strings.HasPrefix(value, `"`) || strings.HasPrefix(value, `'`) {
		return parseStructuredQuotedScalar(value, line)
	}
	switch strings.ToLower(value) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null", "~":
		return nil, nil
	}
	if integer, err := strconv.Atoi(value); err == nil {
		return integer, nil
	}
	return value, nil
}

func parseStructuredFlowSequence(value string, line int) ([]any, error) {
	if !strings.HasSuffix(value, "]") {
		return nil, frontmatterParseError{
			line:    line,
			message: "frontmatter flow sequence is not closed",
		}
	}
	content := strings.TrimSpace(value[1 : len(value)-1])
	if content == "" {
		return nil, nil
	}
	parts, err := splitStructuredCommaList(content, line)
	if err != nil {
		return nil, err
	}
	values := make([]any, 0, len(parts))
	for _, part := range parts {
		parsed, err := parseStructuredScalar(part, line)
		if err != nil {
			return nil, err
		}
		values = append(values, parsed)
	}
	return values, nil
}

func parseStructuredQuotedScalar(value string, line int) (string, error) {
	quote := value[0]
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
		if char != quote {
			continue
		}
		rest := strings.TrimSpace(value[index+1:])
		if rest != "" && !strings.HasPrefix(rest, "#") {
			return "", frontmatterParseError{
				line:    line,
				message: "frontmatter quoted scalar has trailing content",
			}
		}
		inner := value[:index+1]
		if quote == '"' {
			unquoted, err := strconv.Unquote(inner)
			if err != nil {
				return "", frontmatterParseError{
					line:    line,
					message: "frontmatter double-quoted scalar is not valid",
				}
			}
			return unquoted, nil
		}
		return strings.ReplaceAll(value[1:index], "''", "'"), nil
	}
	if quote == '"' {
		return "", frontmatterParseError{line: line, message: "frontmatter double-quoted scalar is not closed"}
	}
	return "", frontmatterParseError{line: line, message: "frontmatter single-quoted scalar is not closed"}
}

func splitStructuredCommaList(value string, line int) ([]string, error) {
	var parts []string
	start := 0
	quote := byte(0)
	escaped := false
	depth := 0
	for index := 0; index < len(value); index++ {
		char := value[index]
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if quote == '"' && char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '"', '\'':
			quote = char
		case '[':
			depth++
		case ']':
			if depth == 0 {
				return nil, frontmatterParseError{line: line, message: "frontmatter flow sequence has an unexpected ]"}
			}
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(value[start:index]))
				start = index + 1
			}
		}
	}
	if quote != 0 {
		return nil, frontmatterParseError{line: line, message: "frontmatter quoted scalar is not closed"}
	}
	if depth != 0 {
		return nil, frontmatterParseError{line: line, message: "frontmatter flow sequence is not closed"}
	}
	parts = append(parts, strings.TrimSpace(value[start:]))
	return parts, nil
}

func stripStructuredComment(value string) string {
	quote := byte(0)
	escaped := false
	for index := 0; index < len(value); index++ {
		char := value[index]
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if quote == '"' && char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '"' || char == '\'' {
			quote = char
			continue
		}
		if char == '#' && (index == 0 || value[index-1] == ' ' || value[index-1] == '\t') {
			return strings.TrimSpace(value[:index])
		}
	}
	return strings.TrimSpace(value)
}

func findUnquotedRune(value string, needle byte) int {
	quote := byte(0)
	escaped := false
	for index := 0; index < len(value); index++ {
		char := value[index]
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if quote == '"' && char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '"' || char == '\'' {
			quote = char
			continue
		}
		if char == needle {
			return index
		}
	}
	return -1
}
