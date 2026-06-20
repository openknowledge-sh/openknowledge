package okf

import "strings"

func validateMarkdownSyntax(rel string, content string, startLine int, result *Result) {
	if startLine < 1 {
		startLine = 1
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var fence *markdownFence
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		lineNumber := startLine + index
		trimmed := strings.TrimSpace(line)
		if marker, length, ok := markdownFenceMarker(trimmed); ok {
			if fence == nil {
				fence = &markdownFence{marker: marker, length: length, line: lineNumber}
				continue
			}
			if marker == fence.marker && length >= fence.length {
				fence = nil
			}
			continue
		}
		if fence != nil {
			continue
		}

		if countUnescapedByte(line, '`')%2 == 1 {
			result.Warnings = append(result.Warnings, Issue{
				Path:    rel,
				Line:    lineNumber,
				Rule:    "markdown-syntax",
				Message: "inline code span is not closed",
			})
		}

		for _, message := range markdownLinkSyntaxWarnings(line) {
			result.Warnings = append(result.Warnings, Issue{
				Path:    rel,
				Line:    lineNumber,
				Rule:    "markdown-syntax",
				Message: message,
			})
		}

		if index+1 < len(lines) && looksLikeTableRow(line) && looksLikeTableSeparator(lines[index+1]) {
			header := tableCells(line)
			separator := tableCells(lines[index+1])
			if len(header) != len(separator) {
				result.Warnings = append(result.Warnings, Issue{
					Path:    rel,
					Line:    lineNumber + 1,
					Rule:    "markdown-syntax",
					Message: "table separator column count does not match the header",
				})
			}
		}
	}

	if fence != nil {
		result.Warnings = append(result.Warnings, Issue{
			Path:    rel,
			Line:    fence.line,
			Rule:    "markdown-syntax",
			Message: "fenced code block is not closed",
		})
	}
}

type markdownFence struct {
	marker byte
	length int
	line   int
}

func markdownFenceMarker(line string) (byte, int, bool) {
	if len(line) < 3 {
		return 0, 0, false
	}
	marker := line[0]
	if marker != '`' && marker != '~' {
		return 0, 0, false
	}
	length := 0
	for length < len(line) && line[length] == marker {
		length++
	}
	if length < 3 {
		return 0, 0, false
	}
	return marker, length, true
}

func countUnescapedByte(line string, target byte) int {
	count := 0
	escaped := false
	for index := 0; index < len(line); index++ {
		char := line[index]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == target {
			count++
		}
	}
	return count
}

func markdownLinkSyntaxWarnings(line string) []string {
	var warnings []string
	segments := strings.Split(line, "`")
	for index, segment := range segments {
		if index%2 == 1 {
			continue
		}
		warnings = append(warnings, markdownLinkSegmentWarnings(segment)...)
	}
	return warnings
}

func markdownLinkSegmentWarnings(segment string) []string {
	var warnings []string
	offset := 0
	for offset < len(segment) {
		open := indexUnescapedByte(segment, '[', offset)
		if open < 0 {
			break
		}
		labelStart := open + 1
		if open > 0 && segment[open-1] == '!' {
			labelStart = open + 1
		}
		close := indexUnescapedByte(segment, ']', labelStart)
		if close < 0 {
			break
		}
		if close+1 >= len(segment) || segment[close+1] != '(' {
			offset = close + 1
			continue
		}
		targetStart := close + 2
		targetEnd := indexUnescapedByte(segment, ')', targetStart)
		if targetEnd < 0 {
			warnings = append(warnings, "Markdown link is missing closing ')'")
			break
		}
		if strings.TrimSpace(segment[labelStart:close]) == "" {
			warnings = append(warnings, "Markdown link label is empty")
		}
		if strings.TrimSpace(segment[targetStart:targetEnd]) == "" {
			warnings = append(warnings, "Markdown link target is empty")
		}
		offset = targetEnd + 1
	}
	return warnings
}

func indexUnescapedByte(line string, target byte, start int) int {
	escaped := false
	for index := start; index < len(line); index++ {
		char := line[index]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == target {
			return index
		}
	}
	return -1
}

func looksLikeTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.Contains(trimmed, "|") && !strings.HasPrefix(trimmed, "|---")
}

func looksLikeTableSeparator(line string) bool {
	cells := tableCells(line)
	return len(cells) > 0 && isTableSeparator(cells)
}
