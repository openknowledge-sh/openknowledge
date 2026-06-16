package okf

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var logDateHeading = regexp.MustCompile(`^##\s+\d{4}-\d{2}-\d{2}\s*$`)
var markdownLink = regexp.MustCompile(`!?\[[^\]]*\]\(([^\s)]+)(?:\s+"[^"]*")?\)`)

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
	resolved, ok := ResolveSpecVersion(version)
	if !ok {
		return Result{}, fmt.Errorf("unsupported OKF spec version: %s", version)
	}

	absolute, err := filepath.Abs(root)
	if err != nil {
		return Result{}, err
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return Result{}, err
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("%s is not a directory", absolute)
	}

	result := Result{Root: absolute, SpecVersion: resolved}
	err = filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isMarkdown(path) {
			return nil
		}

		result.Files++
		validateFile(absolute, path, &result)
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	sortIssues(result.Errors)
	sortIssues(result.Warnings)
	result.Checks = buildChecks(result)
	return result, nil
}

func validateFile(root, path string, result *Result) {
	rel := relPath(root, path)
	name := strings.ToLower(filepath.Base(path))

	switch name {
	case "index.md":
		result.Indexes++
	case "log.md":
		result.Logs++
	default:
		result.Concepts++
	}

	content, err := os.ReadFile(path)
	if err != nil {
		result.Errors = append(result.Errors, Issue{Path: rel, Rule: "bundle-read", Message: err.Error()})
		return
	}

	meta, _, frontmatterErr := splitFrontmatter(string(content))
	if frontmatterErr != nil {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "frontmatter", Message: frontmatterErr.Error()})
	}

	switch name {
	case "index.md":
		validateIndex(rel, meta, result)
	case "log.md":
		validateLog(rel, meta, string(content), result)
	default:
		validateConcept(rel, meta, result)
	}
	validateLinks(root, rel, string(content), result)
}

func validateIndex(rel string, meta frontmatter, result *Result) {
	if strings.EqualFold(rel, "index.md") {
		if meta.has {
			for key := range meta.keys {
				if key != "okf_version" {
					result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "index-frontmatter", Message: "root index.md frontmatter may only declare okf_version"})
				}
			}
			if version := meta.values["okf_version"]; version != "" && version != result.SpecVersion {
				result.Warnings = append(result.Warnings, Issue{Path: rel, Line: 1, Rule: "okf-version", Message: fmt.Sprintf("declares okf_version %q, validating against %s", version, result.SpecVersion)})
			}
		}
		return
	}

	if meta.has {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "index-frontmatter", Message: "index.md must not use concept frontmatter"})
	}
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

func validateLinks(root string, rel string, content string, result *Result) {
	linkContent := maskFencedCode(content)
	for _, match := range markdownLink.FindAllStringSubmatchIndex(linkContent, -1) {
		href := linkContent[match[2]:match[3]]
		if shouldSkipLink(href) {
			continue
		}

		targetRel := linkTargetRel(rel, href)
		if targetRel == "" {
			continue
		}
		targetPath := filepath.Join(root, filepath.FromSlash(targetRel))
		if !insideRoot(root, targetPath) {
			result.Warnings = append(result.Warnings, Issue{
				Path:    rel,
				Line:    lineForOffset(content, match[0]),
				Rule:    "link-target",
				Message: fmt.Sprintf("link target escapes bundle root: %s", href),
			})
			continue
		}

		info, err := os.Stat(targetPath)
		if err == nil && info.IsDir() {
			_, err = os.Stat(filepath.Join(targetPath, "index.md"))
		}
		if err != nil {
			result.Warnings = append(result.Warnings, Issue{
				Path:    rel,
				Line:    lineForOffset(content, match[0]),
				Rule:    "link-target",
				Message: fmt.Sprintf("link target does not exist: %s", href),
			})
		}
	}
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
			Name:    "Concept documents",
			Status:  statusForRules(result.Errors, "frontmatter", "concept-frontmatter", "concept-type"),
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

func statusForRules(errors []Issue, rules ...string) string {
	for _, issue := range errors {
		for _, rule := range rules {
			if issue.Rule == rule {
				return "fail"
			}
		}
	}
	return "pass"
}

func versionStatus(warnings []Issue) string {
	return warningStatus(warnings, "okf-version")
}

func warningStatus(warnings []Issue, rules ...string) string {
	for _, warning := range warnings {
		for _, rule := range rules {
			if warning.Rule == rule {
				return "warn"
			}
		}
	}
	return "pass"
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
