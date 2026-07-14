package okf

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const CustomRulesDir = "rules"

var ruleIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type RuleCatalogConfig struct {
	ConfigPath        string
	Paths             []string
	Enabled           []string
	PathsConfigured   bool
	EnabledConfigured bool
}

type RuleReviewOptions struct {
	Wiki  string
	Rules []string
	All   bool
}

func RuleSetsForWiki(wiki string) ([]RuleSet, error) {
	ruleSets, _, err := ruleSetsAndConfigForWiki(wiki)
	return ruleSets, err
}

func ruleSetsAndConfigForWiki(wiki string) ([]RuleSet, RuleCatalogConfig, error) {
	wiki = strings.TrimSpace(wiki)
	if wiki == "" {
		wiki = DefaultRulesWiki
	}
	config, err := LoadRuleCatalogConfig(wiki)
	if err != nil {
		return nil, RuleCatalogConfig{}, err
	}
	ruleSets := append([]RuleSet{}, RuleSets()...)
	custom, err := customRuleSetsWithConfig(wiki, config)
	if err != nil {
		return nil, RuleCatalogConfig{}, err
	}
	ruleSets = append(ruleSets, custom...)
	return ruleSets, config, nil
}

func ResolveRuleSetsForWiki(wiki string, ids []string) ([]RuleSet, error) {
	ruleSets, config, err := ruleSetsAndConfigForWiki(wiki)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 && len(config.Enabled) > 0 {
		ids = config.Enabled
	}
	return resolveRuleSetsFromCatalog(ruleSets, ids)
}

func RenderRulesListForWiki(wiki string) (string, error) {
	ruleSets, err := RuleSetsForWiki(wiki)
	if err != nil {
		return "", err
	}
	return renderRulesList(ruleSets), nil
}

func RenderRuleReviewPrompt(options RuleReviewOptions) (string, error) {
	wiki := strings.TrimSpace(options.Wiki)
	if wiki == "" {
		wiki = DefaultRulesWiki
	}

	var ruleSets []RuleSet
	var err error
	if options.All {
		ruleSets, err = RuleSetsForWiki(wiki)
	} else {
		ruleSets, err = ResolveRuleSetsForWiki(wiki, options.Rules)
	}
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("# Open Knowledge Rule Review\n\n")
	builder.WriteString("You are reviewing whether this workspace follows its Open Knowledge maintenance rules.\n\n")
	builder.WriteString(fmt.Sprintf("Wiki path: %s\n\n", markdownCode(wiki)))
	builder.WriteString("This is an advisory AI review, not deterministic validation. Run deterministic validation first:\n")
	builder.WriteString(fmt.Sprintf("- `openknowledge validate %q`\n\n", wiki))
	builder.WriteString("Review scope:\n")
	builder.WriteString("- Inspect the working tree, recent diffs, existing agent instructions, and only the wiki pages relevant to the selected rules.\n")
	builder.WriteString("- Treat missing evidence as uncertainty, not proof.\n")
	builder.WriteString("- Do not invent facts, secrets, source files, automations, or user intent.\n")
	builder.WriteString("- Do not edit files unless the user explicitly asks for fixes after the review.\n\n")
	builder.WriteString("Rules to review:\n")
	for _, ruleSet := range ruleSets {
		builder.WriteString(fmt.Sprintf("- %s: %s\n", ruleSet.ID, ruleSet.Summary))
	}
	builder.WriteByte('\n')

	for _, ruleSet := range ruleSets {
		builder.WriteString(fmt.Sprintf("## %s\n\n", ruleSet.ID))
		builder.WriteString(fmt.Sprintf("Summary: %s\n\n", ruleSet.Summary))
		if len(ruleSet.Rules) > 0 {
			builder.WriteString("Instructions:\n")
			for _, rule := range ruleSet.Rules {
				builder.WriteString("- ")
				builder.WriteString(rule)
				builder.WriteByte('\n')
			}
			builder.WriteByte('\n')
		}
		if strings.TrimSpace(ruleSet.ReviewPrompt) != "" {
			builder.WriteString("Review focus:\n")
			builder.WriteString(strings.TrimSpace(ruleSet.ReviewPrompt))
			builder.WriteString("\n\n")
		}
		builder.WriteString("Suggested evidence:\n")
		evidence := ruleSet.ReviewEvidence
		if len(evidence) == 0 {
			evidence = defaultRuleReviewEvidence(wiki)
		}
		for _, item := range evidence {
			builder.WriteString("- ")
			builder.WriteString(item)
			builder.WriteByte('\n')
		}
		builder.WriteByte('\n')
	}

	builder.WriteString("Report format:\n")
	builder.WriteString("- Lead with findings, ordered by severity.\n")
	builder.WriteString("- For each finding, include the rule ID, evidence, impact, and a concrete suggested fix.\n")
	builder.WriteString("- If no issues are found, say that clearly and list any evidence you could not inspect.\n")
	return builder.String(), nil
}

func LoadRuleCatalogConfig(root string) (RuleCatalogConfig, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = DefaultRulesWiki
	}
	path := filepath.Join(root, ValidationConfigFile)
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultRuleCatalogConfig(), nil
		}
		return RuleCatalogConfig{}, err
	}
	config, err := ParseRuleCatalogConfig(string(content))
	if err != nil {
		return RuleCatalogConfig{}, fmt.Errorf("%s: %w", path, err)
	}
	config.ConfigPath = path
	return withRuleCatalogDefaults(config), nil
}

func ParseRuleCatalogConfig(content string) (RuleCatalogConfig, error) {
	config := RuleCatalogConfig{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	section := ""
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(stripValidationTomlComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		if section != "rules" {
			continue
		}
		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return RuleCatalogConfig{}, fmt.Errorf("%d expected key = value in [rules]", lineNumber)
		}
		values, err := parseRuleConfigStringList(rawValue)
		if err != nil {
			return RuleCatalogConfig{}, fmt.Errorf("%d %w", lineNumber, err)
		}
		switch strings.TrimSpace(key) {
		case "paths":
			paths, err := normalizeRulePaths(values)
			if err != nil {
				return RuleCatalogConfig{}, fmt.Errorf("%d %w", lineNumber, err)
			}
			config.Paths = paths
			config.PathsConfigured = true
		case "enabled":
			enabled, err := normalizeConfiguredRuleIDs(values)
			if err != nil {
				return RuleCatalogConfig{}, fmt.Errorf("%d %w", lineNumber, err)
			}
			config.Enabled = enabled
			config.EnabledConfigured = true
		default:
			return RuleCatalogConfig{}, fmt.Errorf("%d unknown [rules] key %q", lineNumber, strings.TrimSpace(key))
		}
	}
	if err := scanner.Err(); err != nil {
		return RuleCatalogConfig{}, err
	}
	return withRuleCatalogDefaults(config), nil
}

func defaultRuleCatalogConfig() RuleCatalogConfig {
	return RuleCatalogConfig{Paths: []string{CustomRulesDir}}
}

func withRuleCatalogDefaults(config RuleCatalogConfig) RuleCatalogConfig {
	if len(config.Paths) == 0 {
		config.Paths = []string{CustomRulesDir}
	}
	return config
}

func loadRuleCatalogConfigForValidation(root string) (RuleCatalogConfig, []Issue) {
	config, err := LoadRuleCatalogConfig(root)
	if err == nil {
		issues := validateConfiguredRulePaths(root, config)
		return config, issues
	}
	return defaultRuleCatalogConfig(), []Issue{{
		Path:    ValidationConfigFile,
		Rule:    "rule-catalog",
		Message: "rules configuration is invalid: " + err.Error(),
	}}
}

func validateConfiguredRulePaths(root string, config RuleCatalogConfig) []Issue {
	if !config.PathsConfigured {
		return nil
	}
	var issues []Issue
	for _, rulePath := range config.Paths {
		path := filepath.Join(root, filepath.FromSlash(rulePath))
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			issues = append(issues, Issue{
				Path:    ValidationConfigFile,
				Rule:    "rule-catalog",
				Message: fmt.Sprintf("configured rules path does not exist: %s", rulePath),
			})
			continue
		}
		if err != nil {
			issues = append(issues, Issue{
				Path:    ValidationConfigFile,
				Rule:    "rule-catalog",
				Message: fmt.Sprintf("configured rules path could not be inspected: %s (%v)", rulePath, err),
			})
			continue
		}
		if !info.IsDir() {
			issues = append(issues, Issue{
				Path:    ValidationConfigFile,
				Rule:    "rule-catalog",
				Message: fmt.Sprintf("configured rules path is not a directory: %s", rulePath),
			})
		}
	}
	return issues
}

func parseRuleConfigStringList(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("expected string or string array")
	}
	if strings.HasPrefix(raw, "[") || strings.HasSuffix(raw, "]") {
		if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
			return nil, fmt.Errorf("expected TOML string array")
		}
		return parseFlowStringList(raw), nil
	}
	value, err := parseValidationTomlStringValue(raw)
	if err != nil {
		return nil, err
	}
	return compactStrings([]string{value}), nil
}

func normalizeRulePaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("rules.paths must include at least one path")
	}
	var normalized []string
	seen := map[string]struct{}{}
	for _, raw := range paths {
		path, err := normalizeRulePath(raw)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		normalized = append(normalized, path)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("rules.paths must include at least one path")
	}
	return normalized, nil
}

func normalizeRulePath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return "", fmt.Errorf("rules.paths entries must not be empty")
	}
	if strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("rules.paths entries must be relative paths")
	}
	cleaned := path.Clean(value)
	cleaned = strings.TrimPrefix(cleaned, "./")
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("rules.paths entries must not point at the bundle root")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("rules.paths entries must stay inside the bundle")
	}
	return cleaned, nil
}

func normalizeConfiguredRuleIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("rules.enabled must include at least one rule ID")
	}
	var normalized []string
	seen := map[string]struct{}{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, fmt.Errorf("rules.enabled entries must not be empty")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

func CustomRuleSets(wiki string) ([]RuleSet, error) {
	config, err := LoadRuleCatalogConfig(wiki)
	if err != nil {
		return nil, err
	}
	return customRuleSetsWithConfig(wiki, config)
}

func customRuleSetsWithConfig(wiki string, config RuleCatalogConfig) ([]RuleSet, error) {
	wiki = strings.TrimSpace(wiki)
	if wiki == "" {
		wiki = DefaultRulesWiki
	}
	absolute, err := filepath.Abs(wiki)
	if err != nil {
		return nil, err
	}

	var documents []ASTDocument
	seenFiles := map[string]struct{}{}
	for _, rulePath := range rulePathsFromConfig(config) {
		rulesDir := filepath.Join(absolute, filepath.FromSlash(rulePath))
		info, err := os.Stat(rulesDir)
		if os.IsNotExist(err) {
			if config.PathsConfigured {
				return nil, ruleCatalogError([]Issue{{
					Path:    ValidationConfigFile,
					Rule:    "rule-catalog",
					Message: fmt.Sprintf("configured rules path does not exist: %s", rulePath),
				}})
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, ruleCatalogError([]Issue{{
				Path:    ValidationConfigFile,
				Rule:    "rule-catalog",
				Message: fmt.Sprintf("configured rules path is not a directory: %s", rulePath),
			}})
		}
		if err := filepath.WalkDir(rulesDir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !isMarkdown(path) {
				return nil
			}
			rel := relPath(absolute, path)
			if !isCustomRuleDocumentInPaths(rel, []string{rulePath}) {
				return nil
			}
			if _, ok := seenFiles[rel]; ok {
				return nil
			}
			seenFiles[rel] = struct{}{}
			documents = append(documents, parseASTDocumentFile(path, rel))
			return nil
		}); err != nil {
			return nil, err
		}
	}

	ruleSets, issues := ruleSetsFromCustomRuleDocuments(documents)
	if len(issues) > 0 {
		return nil, ruleCatalogError(issues)
	}
	return ruleSets, nil
}

func RenderRulesList() string {
	return renderRulesList(RuleSets())
}

func ResolveRuleSets(ids []string) ([]RuleSet, error) {
	return resolveRuleSetsFromCatalog(RuleSets(), ids)
}

func ValidateRuleCatalog(bundle ASTBundle) []Issue {
	config, configIssues := loadRuleCatalogConfigForValidation(bundle.Root)
	var documents []ASTDocument
	for _, document := range bundle.Documents {
		if isCustomRuleDocumentInPaths(document.Rel, rulePathsFromConfig(config)) {
			documents = append(documents, document)
		}
	}
	customRuleSets, issues := ruleSetsFromCustomRuleDocuments(documents)
	issues = append(configIssues, issues...)
	if len(config.Enabled) > 0 {
		allRuleSets := append([]RuleSet{}, RuleSets()...)
		allRuleSets = append(allRuleSets, customRuleSets...)
		if _, err := resolveRuleSetsFromCatalog(allRuleSets, config.Enabled); err != nil {
			issues = append(issues, Issue{
				Path:    ValidationConfigFile,
				Rule:    "rule-catalog",
				Message: "configured rules.enabled is invalid: " + err.Error(),
			})
		}
	}
	sortIssues(issues)
	return issues
}

func ruleSetsFromCustomRuleDocuments(documents []ASTDocument) ([]RuleSet, []Issue) {
	var ruleSets []RuleSet
	var issues []Issue
	builtinIDs := builtinRuleIDs()
	seen := map[string]string{}

	for _, document := range documents {
		ruleSet, documentIssues := customRuleSetFromDocument(document)
		issues = append(issues, documentIssues...)
		if ruleSet.ID == "" || !validRuleID(ruleSet.ID) {
			continue
		}
		if _, ok := builtinIDs[ruleSet.ID]; ok {
			issues = append(issues, Issue{
				Path:    document.Rel,
				Line:    1,
				Rule:    "rule-catalog",
				Message: fmt.Sprintf("custom rule %q conflicts with a built-in rule ID", ruleSet.ID),
			})
			continue
		}
		if firstPath, ok := seen[ruleSet.ID]; ok {
			issues = append(issues, Issue{
				Path:    document.Rel,
				Line:    1,
				Rule:    "rule-catalog",
				Message: fmt.Sprintf("custom rule %q is already defined in %s", ruleSet.ID, firstPath),
			})
			continue
		}
		seen[ruleSet.ID] = document.Rel
		ruleSets = append(ruleSets, ruleSet)
	}

	sort.Slice(ruleSets, func(i, j int) bool {
		return ruleSets[i].ID < ruleSets[j].ID
	})
	sortIssues(issues)
	return ruleSets, issues
}

func customRuleSetFromDocument(document ASTDocument) (RuleSet, []Issue) {
	if document.ReadDiagnostic != nil {
		return RuleSet{}, []Issue{{
			Path:    document.Rel,
			Rule:    "rule-catalog",
			Message: "custom rule document could not be read: " + document.ReadDiagnostic.Message,
		}}
	}
	if document.UTF8Diagnostic != nil {
		return RuleSet{}, []Issue{{
			Path:    document.Rel,
			Line:    document.UTF8Diagnostic.Line,
			Rule:    "rule-catalog",
			Message: "custom rule document must be valid UTF-8: " + document.UTF8Diagnostic.Message,
		}}
	}
	if document.FrontmatterDiagnostic != nil {
		return RuleSet{}, []Issue{{
			Path:    document.Rel,
			Line:    document.FrontmatterDiagnostic.Line,
			Rule:    "rule-catalog",
			Message: "custom rule document frontmatter must be parseable: " + document.FrontmatterDiagnostic.Message,
		}}
	}

	meta := document.Frontmatter
	var issues []Issue
	if strings.ToLower(frontmatterString(meta, "type")) != "rule" {
		issues = append(issues, Issue{
			Path:    document.Rel,
			Line:    1,
			Rule:    "rule-catalog",
			Message: "custom rule documents must declare type: Rule",
		})
	}

	id := frontmatterString(meta, "rule_id")
	if id == "" {
		issues = append(issues, Issue{
			Path:    document.Rel,
			Line:    1,
			Rule:    "rule-catalog",
			Message: "custom rule document must declare rule_id",
		})
	} else if !validRuleID(id) {
		issues = append(issues, Issue{
			Path:    document.Rel,
			Line:    1,
			Rule:    "rule-catalog",
			Message: fmt.Sprintf("custom rule ID %q must use lowercase letters, numbers, and dashes, and start with a letter", id),
		})
	}

	summary := firstNonEmpty(frontmatterString(meta, "rule_summary"), frontmatterString(meta, "description"))
	if summary == "" {
		issues = append(issues, Issue{
			Path:    document.Rel,
			Line:    1,
			Rule:    "rule-catalog",
			Message: "custom rule document must declare rule_summary or description",
		})
	}

	rules := parseRuleInstructions(document.Body)
	if len(rules) == 0 {
		issues = append(issues, Issue{
			Path:    document.Rel,
			Line:    meta.BodyLine,
			Rule:    "rule-catalog",
			Message: "custom rule document must include at least one instruction bullet, preferably under ## Instructions",
		})
	}

	label := firstNonEmpty(frontmatterString(meta, "rule_label"), frontmatterString(meta, "title"), labelFromRuleID(id))
	return RuleSet{
		ID:             id,
		Label:          label,
		Summary:        summary,
		Rules:          rules,
		ReviewPrompt:   frontmatterString(meta, "rule_review_prompt"),
		ReviewEvidence: frontmatterStringList(meta, "rule_review_evidence"),
	}, issues
}

func renderRulesList(ruleSets []RuleSet) string {
	var builder strings.Builder
	builder.WriteString("openknowledge rules prints maintenance instructions for AI agents.\n\n")
	builder.WriteString("Use it when you already have, or plan to create, an Open Knowledge wiki and want\n")
	builder.WriteString("Codex, Claude, Cursor, or another coding agent to keep it up to date.\n\n")
	builder.WriteString("The command does not edit files. It prints a Markdown block you can paste into\n")
	builder.WriteString("AGENTS.md, CLAUDE.md, Cursor rules, or any project instruction file.\n\n")
	builder.WriteString("Custom rules can be added as OKF Markdown files under rules/ in the selected wiki.\n\n")
	builder.WriteString("Usage:\n")
	builder.WriteString("  openknowledge rules docs,changelog --path Wiki\n")
	builder.WriteString("  openknowledge rules apply docs,changelog --path Wiki --file AGENTS.md\n")
	builder.WriteString("  openknowledge setup --rules docs,changelog\n\n")
	builder.WriteString("Available rules:\n\n")
	for _, ruleSet := range ruleSets {
		builder.WriteString(fmt.Sprintf("  %-14s %s\n", ruleSet.ID, ruleSet.Summary))
	}
	return builder.String()
}

func resolveRuleSetsFromCatalog(ruleSets []RuleSet, ids []string) ([]RuleSet, error) {
	if len(ids) == 0 {
		ids = []string{"project"}
	}
	byID := map[string]RuleSet{}
	for _, ruleSet := range ruleSets {
		byID[ruleSet.ID] = ruleSet
	}
	resolved := make([]RuleSet, 0, len(ids))
	seen := map[string]struct{}{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, fmt.Errorf("rule must not be empty")
		}
		ruleSet, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown rule %q; run openknowledge rules --list", id)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		resolved = append(resolved, ruleSet)
	}
	return resolved, nil
}

func isCustomRuleDocument(rel string) bool {
	return isCustomRuleDocumentInPaths(rel, []string{CustomRulesDir})
}

func isCustomRuleDocumentInPaths(rel string, rulePaths []string) bool {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if !isMarkdown(rel) {
		return false
	}
	base := strings.ToLower(filepath.Base(rel))
	if base == "index.md" || base == "log.md" {
		return false
	}
	for _, rulePath := range rulePaths {
		prefix := strings.Trim(strings.TrimSpace(filepath.ToSlash(rulePath)), "/")
		if prefix == "" {
			continue
		}
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
			return true
		}
	}
	return false
}

func rulePathsFromConfig(config RuleCatalogConfig) []string {
	if len(config.Paths) == 0 {
		return []string{CustomRulesDir}
	}
	return config.Paths
}

func validRuleID(id string) bool {
	return ruleIDPattern.MatchString(strings.TrimSpace(id))
}

func builtinRuleIDs() map[string]struct{} {
	ids := map[string]struct{}{}
	for _, ruleSet := range RuleSets() {
		ids[ruleSet.ID] = struct{}{}
	}
	return ids
}

func parseRuleInstructions(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")
	var rules []string
	inInstructions := false
	sawInstructionsHeading := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			heading := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
			if heading == "instructions" || heading == "rules" {
				inInstructions = true
				sawInstructionsHeading = true
				continue
			}
			if sawInstructionsHeading {
				inInstructions = false
			}
			continue
		}
		if sawInstructionsHeading && !inInstructions {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			rule := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if rule != "" {
				rules = append(rules, rule)
			}
		}
	}
	return compactStrings(rules)
}

func labelFromRuleID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	parts := strings.Split(id, "-")
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func ruleCatalogError(issues []Issue) error {
	if len(issues) == 0 {
		return nil
	}
	messages := make([]string, 0, len(issues))
	for _, issue := range issues {
		messages = append(messages, issue.String())
	}
	return fmt.Errorf("custom rule catalog is invalid: %s", strings.Join(messages, "; "))
}

func defaultRuleReviewEvidence(wiki string) []string {
	return []string{
		"git status --short",
		"git diff --stat",
		"git diff",
		fmt.Sprintf("%s/index.md", strings.TrimRight(wiki, "/")),
		"AGENTS.md, CLAUDE.md, or Cursor project rules when present",
	}
}
