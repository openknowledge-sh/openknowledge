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
	validateInsight(rel, meta, result)
}

func validateInsight(rel string, meta ASTFrontmatter, result *Result) {
	normalizedRel := filepath.ToSlash(rel)
	inInsights := strings.HasPrefix(normalizedRel, "insights/") || strings.Contains(normalizedRel, "/insights/")
	if frontmatterString(meta, "type") != "Open Knowledge Insight" && !inInsights {
		return
	}
	add := func(message string) {
		result.Errors = append(result.Errors, Issue{Path: rel, Line: 1, Rule: "insight-contract", Message: message})
	}
	if frontmatterString(meta, "type") != "Open Knowledge Insight" {
		add("documents under insights/ must use type Open Knowledge Insight")
	}
	published, ok := meta.Data["okf_publish"].(bool)
	if !ok || published {
		add("Open Knowledge insights must declare okf_publish: false")
	}
	for _, key := range []string{"title", "okf_insight_id", "okf_insight_kind"} {
		if frontmatterString(meta, key) == "" {
			add(fmt.Sprintf("insight frontmatter must include non-empty %s", key))
		}
	}
	if result.SpecVersion == "0.2" && usesOKFV02InsightMetadata(meta) {
		validateOKFV02InsightMetadata(meta, add)
	} else {
		validateLegacyInsightMetadata(meta, add)
	}
	targets, ok := meta.Data["okf_insight_targets"].([]any)
	if !ok || len(targets) == 0 {
		add("okf_insight_targets must be a non-empty list")
		return
	}
	for _, target := range targets {
		value, ok := target.(string)
		clean := filepath.ToSlash(filepath.Clean(value))
		if !ok || strings.TrimSpace(value) == "" || filepath.IsAbs(value) || clean == ".." || strings.HasPrefix(clean, "../") {
			add("insight targets must be non-empty knowledge-base-relative paths")
			return
		}
	}
}

func usesOKFV02InsightMetadata(meta ASTFrontmatter) bool {
	if _, exists := meta.Data["generated"]; exists {
		return true
	}
	if _, exists := meta.Data["okf_insight_status"]; exists {
		return true
	}
	switch strings.ToLower(frontmatterString(meta, "status")) {
	case "draft", "stable", "deprecated":
		return true
	}
	return false
}

func validateOKFV02InsightMetadata(meta ASTFrontmatter, add func(string)) {
	status := strings.ToLower(frontmatterString(meta, "status"))
	switch status {
	case "draft", "stable", "deprecated":
	default:
		add("OKF 0.2 insight status must be draft, stable, or deprecated")
	}
	workflow := strings.ToLower(frontmatterString(meta, "okf_insight_status"))
	if workflow != "" && (workflow != "blocked" || status != "draft") {
		add("okf_insight_status may only be blocked when status is draft")
	}
	generated, ok := meta.Data["generated"].(map[string]any)
	if !ok {
		add("OKF 0.2 insights must include generated.by and generated.at")
	} else {
		if !okfActor(generated["by"]) {
			add("generated.by must identify an OKF actor")
		}
		if !okfDateTime(generated["at"]) {
			add("generated.at must be an ISO 8601 datetime")
		}
	}
	for _, legacy := range []string{"okf_insight_runtime", "okf_insight_created_at"} {
		if _, exists := meta.Data[legacy]; exists {
			add(fmt.Sprintf("OKF 0.2 insights must use generated instead of %s", legacy))
		}
	}
}

func validateLegacyInsightMetadata(meta ASTFrontmatter, add func(string)) {
	status := strings.ToLower(frontmatterString(meta, "status"))
	switch status {
	case "pending", "resolved", "dismissed", "blocked":
	default:
		add("legacy insight status must be pending, resolved, dismissed, or blocked")
	}
	for _, key := range []string{"okf_insight_runtime", "okf_insight_created_at"} {
		if frontmatterString(meta, key) == "" {
			add(fmt.Sprintf("legacy insight frontmatter must include non-empty %s", key))
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
