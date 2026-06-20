package okf

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var logDateHeading = regexp.MustCompile(`^##\s+\d{4}-\d{2}-\d{2}\s*$`)

func validateFrontmatterFormatting(rel string, meta ASTFrontmatter, result *Result) {
	for _, warning := range meta.Warnings {
		result.Warnings = append(result.Warnings, Issue{
			Path:    rel,
			Line:    warning.Line,
			Rule:    "frontmatter-format",
			Message: warning.Message,
		})
	}
}

func validateIndex(rel string, meta ASTFrontmatter, result *Result) {
	if strings.EqualFold(rel, "index.md") {
		if meta.Has {
			if version := meta.Values["okf_version"]; version != "" && version != result.SpecVersion {
				result.Warnings = append(result.Warnings, Issue{Path: rel, Line: 1, Rule: "okf-version", Message: fmt.Sprintf("declares okf_version %q, validating against %s", version, result.SpecVersion)})
			}
		}
		return
	}

	if meta.Has && !hasOnlyIndexPublishMetadata(meta) {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "index-frontmatter", Message: "index.md frontmatter may only declare okf_publish metadata"})
	}
}

func hasOnlyIndexPublishMetadata(meta ASTFrontmatter) bool {
	if !meta.Has {
		return true
	}
	for key := range meta.Keys {
		if key != "okf_publish" {
			return false
		}
	}
	return true
}

func validateLog(rel string, meta ASTFrontmatter, content string, result *Result) {
	if meta.Has {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "log-frontmatter", Message: "log.md must not use concept frontmatter"})
	}

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") && !logDateHeading.MatchString(line) {
			result.Errors = append(result.Errors, Issue{Path: rel, Line: i + 1, Rule: "log-date", Message: "log date heading must use YYYY-MM-DD"})
		}
	}
}

func validateConcept(rel string, meta ASTFrontmatter, result *Result) {
	if !meta.Has {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "concept-frontmatter", Message: "concept document is missing YAML frontmatter"})
		return
	}

	if meta.Values["type"] == "" {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "concept-type", Message: "concept frontmatter must include non-empty type"})
	}
}

func validateDocumentLinks(root string, document ASTDocument, result *Result) {
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
