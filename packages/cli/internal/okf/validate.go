package okf

import (
	"strings"
)

func Validate(root string) (Result, error) {
	return ValidateWithVersion(root, LatestSpecVersion)
}

func ValidateWithVersion(root string, version string) (Result, error) {
	result, _, err := parseAndValidateASTBundle(root, version)
	return result, err
}

func parseAndValidateASTBundle(root string, version string) (Result, astBundle, error) {
	bundle, err := parseBundleAST(root, version)
	if err != nil {
		return Result{}, astBundle{}, err
	}
	return validateASTBundle(bundle), bundle, nil
}

func validateASTBundle(bundle astBundle) Result {
	result := Result{Root: bundle.Root, SpecVersion: bundle.SpecVersion}
	for _, document := range bundle.Documents {
		result.Files++
		validateDocument(bundle.Root, document, &result)
	}

	sortIssues(result.Errors)
	sortIssues(result.Warnings)
	result.Checks = buildChecks(result)
	return result
}

func validateDocument(root string, document astDocument, result *Result) {
	rel := document.Rel

	switch document.Kind {
	case "index":
		result.Indexes++
	case "log":
		result.Logs++
	default:
		result.Concepts++
	}

	if document.ReadDiagnostic != nil {
		result.Errors = append(result.Errors, Issue{Path: rel, Rule: "bundle-read", Message: document.ReadDiagnostic.Message})
		return
	}
	if document.UTF8Diagnostic != nil {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: document.UTF8Diagnostic.Line, Rule: "utf-8", Message: document.UTF8Diagnostic.Message})
		return
	}

	if document.FrontmatterDiagnostic != nil {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: document.FrontmatterDiagnostic.Line, Rule: "frontmatter", Message: document.FrontmatterDiagnostic.Message})
	}
	validateFrontmatterFormatting(rel, document.Frontmatter, result)

	if document.FrontmatterDiagnostic == nil {
		switch document.Kind {
		case "index":
			validateIndex(rel, document.Frontmatter, result)
		case "log":
			validateLog(rel, document.Frontmatter, document.Content, result)
		default:
			validateConcept(rel, document.Frontmatter, result)
		}
		validateMarkdownSyntax(rel, document.Body, document.Frontmatter.BodyLine, result)
	}
	validateDocumentLinks(root, document, result)
}

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
