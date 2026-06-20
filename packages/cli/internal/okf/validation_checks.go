package okf

import "fmt"

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
