package okf

import (
	"fmt"
	"path/filepath"
	"strings"
)

func buildChecks(result Result, profile validationSpecProfile, bundle ASTBundle) []Check {
	specLabel := "OKF " + result.SpecVersion
	const coreGroup = "OKF core"
	const extensionGroup = "Open Knowledge extensions"
	checks := []Check{
		{
			Name:    "Bundle scan",
			Group:   coreGroup,
			Status:  "pass",
			Message: fmt.Sprintf("%s section 3; %d Markdown files scanned", specLabel, result.Files),
		},
		{
			Name:    "UTF-8 content",
			Group:   coreGroup,
			Status:  statusForErrorWarningRules(result.Errors, result.Warnings, []string{"utf-8"}, []string{"utf-8"}),
			Message: fmt.Sprintf("%s section 4; Markdown files must be valid UTF-8", specLabel),
		},
		{
			Name:    "Concept documents",
			Group:   coreGroup,
			Status:  statusForErrorWarningRules(result.Errors, result.Warnings, []string{"utf-8", "frontmatter", "concept-frontmatter", "concept-type"}, []string{"utf-8", "frontmatter", "concept-frontmatter", "concept-type"}),
			Message: fmt.Sprintf("%s %s; %d concepts require YAML frontmatter with non-empty type", specLabel, profile.ConceptSections, result.Concepts),
		},
		{
			Name:    "Reserved files",
			Group:   coreGroup,
			Status:  statusForErrorWarningRules(result.Errors, result.Warnings, []string{"index-frontmatter", "log-frontmatter"}, []string{"index-frontmatter", "log-frontmatter"}),
			Message: fmt.Sprintf("%s %s; %d indexes and %d logs follow reserved-file rules", specLabel, profile.ReservedSections, result.Indexes, result.Logs),
		},
		{
			Name:    "Log dates",
			Group:   coreGroup,
			Status:  statusForErrorWarningRules(result.Errors, result.Warnings, []string{"log-date"}, []string{"log-date"}),
			Message: specLabel + " " + profile.LogSection + "; log.md ## headings must use YYYY-MM-DD",
		},
		{
			Name:    "Frontmatter formatting",
			Group:   coreGroup,
			Status:  statusForErrorWarningRules(result.Errors, result.Warnings, []string{"frontmatter"}, []string{"frontmatter-format"}),
			Message: "YAML frontmatter should be parseable and consistently formatted",
		},
		{
			Name:    "Markdown syntax",
			Group:   coreGroup,
			Status:  statusForErrorWarningRules(result.Errors, result.Warnings, []string{"markdown-syntax"}, []string{"markdown-syntax"}),
			Message: "Markdown should parse without malformed links, code spans, tables, or fences",
		},
		{
			Name:    "Spec version",
			Group:   coreGroup,
			Status:  statusForErrorWarningRules(result.Errors, result.Warnings, []string{"okf-version"}, []string{"okf-version"}),
			Message: fmt.Sprintf("%s %s; root index.md may declare okf_version: %q", specLabel, profile.VersionSection, result.SpecVersion),
		},
		{
			Name:    "Link targets",
			Group:   coreGroup,
			Status:  statusForErrorWarningRules(result.Errors, result.Warnings, []string{"link-target"}, []string{"link-target"}),
			Message: "Local Markdown links should resolve inside the bundle",
		},
	}
	if _, ok := profile.Rules["okf-0.2-metadata"]; ok {
		checks = append(checks, Check{
			Name:    "OKF 0.2 metadata",
			Group:   coreGroup,
			Status:  statusForErrorWarningRules(result.Errors, result.Warnings, []string{"okf-0.2-metadata"}, []string{"okf-0.2-metadata"}),
			Message: "Optional provenance, trust, lifecycle, and attested-computation metadata should follow OKF 0.2",
		})
	}
	if result.Profile == ValidationProfileBundle {
		extensions := []struct {
			check  Check
			rule   string
			active bool
		}{
			{check: Check{Name: "Publish metadata", Group: extensionGroup, Status: statusForErrorWarningRules(result.Errors, result.Warnings, []string{"publish-metadata"}, []string{"publish-metadata"}), Message: "Publishing metadata must identify valid public projections"}, rule: "publish-metadata", active: bundleUsesFrontmatterKey(bundle, "okf_publish", "okf_targets")},
			{check: Check{Name: "Insights", Group: extensionGroup, Status: statusForErrorWarningRules(result.Errors, result.Warnings, []string{"insight-contract"}, []string{"insight-contract"}), Message: "Insight documents must follow the Open Knowledge insight contract"}, rule: "insight-contract", active: bundleUsesInsights(bundle)},
			{check: Check{Name: "Rule catalog", Group: extensionGroup, Status: statusForErrorWarningRules(result.Errors, result.Warnings, []string{"rule-catalog"}, []string{"rule-catalog"}), Message: "Custom rule documents under configured rule paths should define canonical IDs, summaries, and instruction bullets"}, rule: "rule-catalog", active: bundleUsesRuleCatalog(bundle)},
			{check: Check{Name: "Typed claims", Group: extensionGroup, Status: statusForErrorWarningRules(result.Errors, result.Warnings, []string{"claim-profile"}, []string{"claim-profile"}), Message: "Typed claims must pass ontology, evidence, lifecycle, time, relation, reference, and section binding checks"}, rule: "claim-profile", active: bundleUsesFrontmatterKey(bundle, ClaimProfileActivationKey, "claims", "claim_refs")},
			{check: Check{Name: "Corpus schema", Group: extensionGroup, Status: statusForErrorWarningRules(result.Errors, result.Warnings, []string{"corpus-schema"}, []string{"corpus-schema"}), Message: "Optional corpus schemas must enforce document types, paths, required metadata, typed links, and migration records"}, rule: "corpus-schema", active: rootIndexUsesFrontmatterKey(bundle, CorpusSchemaKey)},
		}
		for _, extension := range extensions {
			if extension.active || hasIssueRule(result.Errors, extension.rule) || hasIssueRule(result.Warnings, extension.rule) {
				checks = append(checks, extension.check)
			}
		}
	}
	return checks
}

func bundleUsesFrontmatterKey(bundle ASTBundle, keys ...string) bool {
	for _, document := range bundle.Documents {
		for _, key := range keys {
			if _, ok := document.Frontmatter.Data[key]; ok {
				return true
			}
		}
	}
	return false
}

func rootIndexUsesFrontmatterKey(bundle ASTBundle, key string) bool {
	index := rootIndexDocument(bundle)
	if index == nil {
		return false
	}
	_, ok := index.Frontmatter.Data[key]
	return ok
}

func bundleUsesInsights(bundle ASTBundle) bool {
	for _, document := range bundle.Documents {
		rel := filepath.ToSlash(document.Rel)
		inInsights := strings.HasPrefix(rel, "insights/") || strings.Contains(rel, "/insights/")
		if inInsights || frontmatterString(document.Frontmatter, "type") == "Open Knowledge Insight" {
			return true
		}
	}
	return false
}

func bundleUsesRuleCatalog(bundle ASTBundle) bool {
	config, err := LoadRuleCatalogConfig(bundle.Root)
	if err != nil {
		return true
	}
	if config.PathsConfigured || config.EnabledConfigured {
		return true
	}
	for _, document := range bundle.Documents {
		if isCustomRuleDocumentInPaths(document.Rel, rulePathsFromConfig(config)) {
			return true
		}
	}
	return false
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
