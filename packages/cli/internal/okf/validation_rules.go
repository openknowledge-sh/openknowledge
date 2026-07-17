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
			if version := frontmatterString(meta, "okf_version"); version != "" && version != result.SpecVersion {
				result.Warnings = append(result.Warnings, Issue{Path: rel, Line: 1, Rule: "okf-version", Message: fmt.Sprintf("declares okf_version %q, validating against %s", version, result.SpecVersion)})
			}
		}
		validatePublicationMetadata(rel, meta, result)
		return
	}

	if meta.Has && !hasOnlyIndexPublishMetadata(meta) {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "index-frontmatter", Message: "index.md frontmatter may only declare okf_publish and okf_targets metadata"})
	}
	validatePublicationMetadata(rel, meta, result)
}

func hasOnlyIndexPublishMetadata(meta ASTFrontmatter) bool {
	if !meta.Has {
		return true
	}
	for key := range meta.Keys {
		if key != "okf_publish" && key != "okf_targets" {
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

	if frontmatterString(meta, "type") == "" {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "concept-type", Message: "concept frontmatter must include non-empty type"})
	}
	validatePublicationMetadata(rel, meta, result)
	validateSuggestion(rel, meta, result)
}

func validateSuggestion(rel string, meta ASTFrontmatter, result *Result) {
	normalizedRel := filepath.ToSlash(rel)
	inSuggestions := strings.HasPrefix(normalizedRel, "suggestions/") || strings.Contains(normalizedRel, "/suggestions/")
	if frontmatterString(meta, "type") != "Open Knowledge Suggestion" && !inSuggestions {
		return
	}
	add := func(message string) {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "suggestion-contract", Message: message})
	}
	if frontmatterString(meta, "type") != "Open Knowledge Suggestion" {
		add("documents under suggestions/ must use type Open Knowledge Suggestion")
	}
	published, ok := meta.Data["okf_publish"].(bool)
	if !ok || published {
		add("Open Knowledge suggestions must declare okf_publish: false")
	}
	status := strings.ToLower(frontmatterString(meta, "status"))
	switch status {
	case "pending", "applied", "dismissed", "blocked":
	default:
		add("suggestion status must be pending, applied, dismissed, or blocked")
	}
	for _, key := range []string{"title", "okf_suggestion_id", "okf_suggestion_kind", "okf_suggestion_runtime", "okf_suggestion_created_at", "okf_suggestion_base"} {
		if frontmatterString(meta, key) == "" {
			add(fmt.Sprintf("suggestion frontmatter must include non-empty %s", key))
		}
	}
	targets, ok := meta.Data["okf_suggestion_targets"].([]any)
	if !ok || len(targets) == 0 {
		add("okf_suggestion_targets must be a non-empty list")
		return
	}
	for _, target := range targets {
		value, ok := target.(string)
		clean := filepath.ToSlash(filepath.Clean(value))
		if !ok || strings.TrimSpace(value) == "" || filepath.IsAbs(value) || clean == ".." || strings.HasPrefix(clean, "../") {
			add("suggestion targets must be non-empty knowledge-base-relative paths")
			return
		}
	}
}

func validatePublicationMetadata(rel string, meta ASTFrontmatter, result *Result) {
	frontmatter := meta.Data
	if value, exists := frontmatter["okf_publish"]; exists {
		if _, ok := value.(bool); !ok {
			result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "publish-metadata", Message: "okf_publish must be a boolean"})
		}
	}
	if _, err := shouldPublishToTarget(frontmatter, PublicationTargetViewer); err != nil {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "publish-metadata", Message: err.Error()})
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
