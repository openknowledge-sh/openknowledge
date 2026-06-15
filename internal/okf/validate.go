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
	for _, warning := range warnings {
		if warning.Rule == "okf-version" {
			return "warn"
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
