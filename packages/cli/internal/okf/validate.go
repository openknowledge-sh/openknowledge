package okf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var logDateHeading = regexp.MustCompile(`^##\s+\d{4}-\d{2}-\d{2}\s*$`)
var markdownLinkDetail = regexp.MustCompile(`(!?)\[([^\]]*)\]\(([^\s)]+)(?:\s+"[^"]*")?\)`)

type Issue struct {
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Rule    string `json:"rule,omitempty"`
	Message string `json:"message"`
}

func (i Issue) String() string {
	if i.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", i.Path, i.Line, i.Message)
	}
	return fmt.Sprintf("%s: %s", i.Path, i.Message)
}

type Result struct {
	Root        string  `json:"root"`
	SpecVersion string  `json:"specVersion"`
	Files       int     `json:"files"`
	Concepts    int     `json:"concepts"`
	Indexes     int     `json:"indexes"`
	Logs        int     `json:"logs"`
	Checks      []Check `json:"checks"`
	Errors      []Issue `json:"errors"`
	Warnings    []Issue `json:"warnings"`
}

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func Validate(root string) (Result, error) {
	return ValidateWithVersion(root, LatestSpecVersion)
}

func ValidateWithVersion(root string, version string) (Result, error) {
	result, _, err := parseAndValidateBundle(root, version)
	return result, err
}

func parseAndValidateBundle(root string, version string) (Result, parsedBundle, error) {
	resolved, ok := ResolveSpecVersion(version)
	if !ok {
		return Result{}, parsedBundle{}, fmt.Errorf("unsupported OKF spec version: %s", version)
	}

	absolute, err := filepath.Abs(root)
	if err != nil {
		return Result{}, parsedBundle{}, err
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return Result{}, parsedBundle{}, err
	}
	if !info.IsDir() {
		return Result{}, parsedBundle{}, fmt.Errorf("%s is not a directory", absolute)
	}

	documents, err := parseMarkdownDocuments(absolute)
	if err != nil {
		return Result{}, parsedBundle{}, err
	}
	bundle := parsedBundle{
		Root:        absolute,
		SpecVersion: resolved,
		Documents:   documents,
	}
	return validateParsedBundle(bundle), bundle, nil
}

func validateParsedBundle(bundle parsedBundle) Result {
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

func validateDocument(root string, document parsedDocument, result *Result) {
	rel := document.Rel
	name := strings.ToLower(filepath.Base(document.Rel))

	switch name {
	case "index.md":
		result.Indexes++
	case "log.md":
		result.Logs++
	default:
		result.Concepts++
	}

	if document.ReadErr != nil {
		result.Errors = append(result.Errors, Issue{Path: rel, Rule: "bundle-read", Message: document.ReadErr.Error()})
		return
	}
	if !utf8.Valid(document.Raw) {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: invalidUTF8Line(document.Raw), Rule: "utf-8", Message: "Markdown file must be valid UTF-8"})
		return
	}

	if document.FrontmatterErr != nil {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: frontmatterErrorLine(document.FrontmatterErr), Rule: "frontmatter", Message: document.FrontmatterErr.Error()})
	}
	validateFrontmatterFormatting(rel, document.Frontmatter, result)

	if document.FrontmatterErr == nil {
		switch name {
		case "index.md":
			validateIndex(rel, document.Frontmatter, result)
		case "log.md":
			validateLog(rel, document.Frontmatter, document.Content, result)
		default:
			validateConcept(rel, document.Frontmatter, result)
		}
		validateMarkdownSyntax(rel, document.Body, document.Frontmatter.bodyLine, result)
	}
	validateDocumentLinks(root, document, result)
}

func frontmatterErrorLine(err error) int {
	var parseErr frontmatterParseError
	if errors.As(err, &parseErr) && parseErr.line > 0 {
		return parseErr.line
	}
	return 1
}

func invalidUTF8Line(content []byte) int {
	line := 1
	for len(content) > 0 {
		r, size := utf8.DecodeRune(content)
		if r == utf8.RuneError && size == 1 {
			return line
		}
		if content[0] == '\n' {
			line++
		}
		content = content[size:]
	}
	return line
}

func validateFrontmatterFormatting(rel string, meta frontmatter, result *Result) {
	for _, warning := range meta.warnings {
		result.Warnings = append(result.Warnings, Issue{
			Path:    rel,
			Line:    warning.line,
			Rule:    "frontmatter-format",
			Message: warning.message,
		})
	}
}

func validateIndex(rel string, meta frontmatter, result *Result) {
	if strings.EqualFold(rel, "index.md") {
		if meta.has {
			if version := meta.values["okf_version"]; version != "" && version != result.SpecVersion {
				result.Warnings = append(result.Warnings, Issue{Path: rel, Line: 1, Rule: "okf-version", Message: fmt.Sprintf("declares okf_version %q, validating against %s", version, result.SpecVersion)})
			}
		}
		return
	}

	if meta.has && !hasOnlyIndexPublishMetadata(meta) {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "index-frontmatter", Message: "index.md frontmatter may only declare okf_publish metadata"})
	}
}

func hasOnlyIndexPublishMetadata(meta frontmatter) bool {
	if !meta.has {
		return true
	}
	for key := range meta.keys {
		if key != "okf_publish" {
			return false
		}
	}
	return true
}

func validateLog(rel string, meta frontmatter, content string, result *Result) {
	if meta.has {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "log-frontmatter", Message: "log.md must not use concept frontmatter"})
	}

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") && !logDateHeading.MatchString(line) {
			result.Errors = append(result.Errors, Issue{Path: rel, Line: i + 1, Rule: "log-date", Message: "log date heading must use YYYY-MM-DD"})
		}
	}
}

func validateConcept(rel string, meta frontmatter, result *Result) {
	if !meta.has {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "concept-frontmatter", Message: "concept document is missing YAML frontmatter"})
		return
	}

	if meta.values["type"] == "" {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "concept-type", Message: "concept frontmatter must include non-empty type"})
	}
}

func validateDocumentLinks(root string, document parsedDocument, result *Result) {
	for _, link := range document.Links {
		if link.Kind != "local" || link.TargetPath == "" {
			continue
		}

		targetPath := filepath.Join(root, filepath.FromSlash(link.TargetPath))
		if !insideRoot(root, targetPath) {
			result.Warnings = append(result.Warnings, Issue{
				Path:    document.Rel,
				Line:    link.Line,
				Rule:    "link-target",
				Message: fmt.Sprintf("link target escapes bundle root: %s", link.Href),
			})
			continue
		}

		if !link.Exists {
			result.Warnings = append(result.Warnings, Issue{
				Path:    document.Rel,
				Line:    link.Line,
				Rule:    "link-target",
				Message: fmt.Sprintf("link target does not exist: %s", link.Href),
			})
		}
	}
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

func maskFencedCode(content string) string {
	lines := strings.SplitAfter(content, "\n")
	var builder strings.Builder
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			builder.WriteString(maskLinePreservingNewline(line))
			continue
		}
		if inFence {
			builder.WriteString(maskLinePreservingNewline(line))
			continue
		}
		builder.WriteString(line)
	}
	return builder.String()
}

func maskLinePreservingNewline(line string) string {
	var builder strings.Builder
	for _, r := range line {
		if r == '\n' || r == '\r' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte(' ')
		}
	}
	return builder.String()
}

func shouldSkipLink(href string) bool {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "//") {
		return true
	}
	if schemeIndex := strings.Index(href, ":"); schemeIndex > 0 {
		slashIndex := strings.Index(href, "/")
		if slashIndex < 0 || schemeIndex < slashIndex {
			return true
		}
	}
	return false
}

func linkTargetRel(sourceRel string, href string) string {
	target := strings.TrimSpace(href)
	if hash := strings.Index(target, "#"); hash >= 0 {
		target = target[:hash]
	}
	if query := strings.Index(target, "?"); query >= 0 {
		target = target[:query]
	}
	if target == "" {
		return ""
	}

	var clean string
	if strings.HasPrefix(target, "/") {
		clean = filepath.ToSlash(filepath.Clean(strings.TrimPrefix(target, "/")))
	} else {
		base := filepath.Dir(sourceRel)
		if base == "." {
			base = ""
		}
		clean = filepath.ToSlash(filepath.Clean(filepath.Join(base, target)))
	}
	if clean == "." {
		clean = ""
	}
	if strings.HasSuffix(target, "/") {
		clean = filepath.ToSlash(filepath.Join(clean, "index.md"))
	}
	return clean
}

func insideRoot(root string, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func lineForOffset(content string, offset int) int {
	line := 1
	for index := 0; index < offset && index < len(content); index++ {
		if content[index] == '\n' {
			line++
		}
	}
	return line
}

func buildChecks(result Result) []Check {
	specLabel := "OKF " + result.SpecVersion
	return []Check{
		{
			Name:    "Bundle scan",
			Status:  "pass",
			Message: fmt.Sprintf("%s section 3; %d Markdown files scanned", specLabel, result.Files),
		},
		{
			Name:    "UTF-8 content",
			Status:  statusForRules(result.Errors, "utf-8"),
			Message: fmt.Sprintf("%s section 4; Markdown files must be valid UTF-8", specLabel),
		},
		{
			Name:    "Concept documents",
			Status:  statusForRules(result.Errors, "utf-8", "frontmatter", "concept-frontmatter", "concept-type"),
			Message: fmt.Sprintf("%s sections 4 and 9; %d concepts require YAML frontmatter with non-empty type", specLabel, result.Concepts),
		},
		{
			Name:    "Reserved files",
			Status:  statusForRules(result.Errors, "index-frontmatter", "log-frontmatter"),
			Message: fmt.Sprintf("%s sections 3.1, 6, and 7; %d indexes and %d logs follow reserved-file rules", specLabel, result.Indexes, result.Logs),
		},
		{
			Name:    "Log dates",
			Status:  statusForRules(result.Errors, "log-date"),
			Message: specLabel + " section 7; log.md ## headings must use YYYY-MM-DD",
		},
		{
			Name:    "Frontmatter formatting",
			Status:  statusForErrorWarningRules(result.Errors, result.Warnings, []string{"frontmatter"}, []string{"frontmatter-format"}),
			Message: "YAML frontmatter should be parseable and consistently formatted",
		},
		{
			Name:    "Markdown syntax",
			Status:  warningStatus(result.Warnings, "markdown-syntax"),
			Message: "Markdown should parse without malformed links, code spans, tables, or fences",
		},
		{
			Name:    "Spec version",
			Status:  versionStatus(result.Warnings),
			Message: fmt.Sprintf("%s section 11; root index.md may declare okf_version: %q", specLabel, result.SpecVersion),
		},
		{
			Name:    "Link targets",
			Status:  warningStatus(result.Warnings, "link-target"),
			Message: "Local Markdown links should resolve inside the bundle",
		},
	}
}

func statusForErrorWarningRules(errors []Issue, warnings []Issue, errorRules []string, warningRules []string) string {
	if hasIssueRule(errors, errorRules...) {
		return "fail"
	}
	if hasIssueRule(warnings, warningRules...) {
		return "warn"
	}
	return "pass"
}

func statusForRules(errors []Issue, rules ...string) string {
	if hasIssueRule(errors, rules...) {
		return "fail"
	}
	return "pass"
}

func versionStatus(warnings []Issue) string {
	return warningStatus(warnings, "okf-version")
}

func warningStatus(warnings []Issue, rules ...string) string {
	if hasIssueRule(warnings, rules...) {
		return "warn"
	}
	return "pass"
}

func hasIssueRule(issues []Issue, rules ...string) bool {
	for _, issue := range issues {
		for _, rule := range rules {
			if issue.Rule == rule {
				return true
			}
		}
	}
	return false
}

func isMarkdown(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".md")
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func sortIssues(issues []Issue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Path == issues[j].Path {
			return issues[i].Line < issues[j].Line
		}
		return issues[i].Path < issues[j].Path
	})
}
