package okf

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const ValidationConfigFile = "openknowledge.toml"

const (
	ValidationSeverityOff     = "off"
	ValidationSeverityWarning = "warning"
	ValidationSeverityError   = "error"
)

type ValidationOptions struct {
	ConfigPath string
	Rules      map[string]string
}

var knownValidationRules = map[string]struct{}{
	"bundle-read":         {},
	"concept-frontmatter": {},
	"concept-type":        {},
	"frontmatter":         {},
	"frontmatter-format":  {},
	"index-frontmatter":   {},
	"link-target":         {},
	"log-date":            {},
	"log-frontmatter":     {},
	"markdown-syntax":     {},
	"okf-version":         {},
	"rule-catalog":        {},
	"utf-8":               {},
}

func LoadValidationOptions(root string) (ValidationOptions, error) {
	path := filepath.Join(root, ValidationConfigFile)
	options, err := LoadValidationOptionsFile(path)
	if err != nil {
		return ValidationOptions{}, err
	}
	if len(options.Rules) == 0 {
		return ValidationOptions{}, nil
	}
	options.ConfigPath = path
	return options, nil
}

func LoadValidationOptionsFile(path string) (ValidationOptions, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ValidationOptions{}, nil
		}
		return ValidationOptions{}, err
	}
	options, err := ParseValidationOptionsConfig(string(content))
	if err != nil {
		return ValidationOptions{}, fmt.Errorf("%s: %w", path, err)
	}
	if len(options.Rules) > 0 {
		options.ConfigPath = path
	}
	return options, nil
}

func ParseValidationOptionsConfig(content string) (ValidationOptions, error) {
	options := ValidationOptions{}
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
		if section != "validation.rules" {
			continue
		}
		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return ValidationOptions{}, fmt.Errorf("%d expected key = value in [validation.rules]", lineNumber)
		}
		rule := strings.TrimSpace(key)
		severity, err := ParseValidationSeverity(rawValue)
		if err != nil {
			return ValidationOptions{}, fmt.Errorf("%d %w", lineNumber, err)
		}
		if err := SetValidationRuleSeverity(&options, rule, severity); err != nil {
			return ValidationOptions{}, fmt.Errorf("%d %w", lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return ValidationOptions{}, err
	}
	return options, nil
}

func MergeValidationOptions(base ValidationOptions, override ValidationOptions) ValidationOptions {
	merged := ValidationOptions{ConfigPath: base.ConfigPath}
	for rule, severity := range base.Rules {
		merged = withValidationRuleSeverity(merged, rule, severity)
	}
	for rule, severity := range override.Rules {
		merged = withValidationRuleSeverity(merged, rule, severity)
	}
	return merged
}

func SetValidationRuleSeverity(options *ValidationOptions, rule string, severity string) error {
	rule = strings.TrimSpace(rule)
	if !IsKnownValidationRule(rule) {
		return fmt.Errorf("unknown validation rule %q", rule)
	}
	normalized, err := NormalizeValidationSeverity(severity)
	if err != nil {
		return err
	}
	*options = withValidationRuleSeverity(*options, rule, normalized)
	return nil
}

func ParseValidationRuleOverride(value string) (string, string, error) {
	rule, severity, ok := strings.Cut(value, "=")
	if !ok {
		return "", "", fmt.Errorf("validation rule override must use rule=off|warn|error: %s", value)
	}
	normalized, err := ParseValidationSeverity(severity)
	if err != nil {
		return "", "", err
	}
	rule = strings.TrimSpace(rule)
	if !IsKnownValidationRule(rule) {
		return "", "", fmt.Errorf("unknown validation rule %q", rule)
	}
	return rule, normalized, nil
}

func ParseValidationSeverity(value string) (string, error) {
	value, err := parseValidationTomlStringValue(strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	return NormalizeValidationSeverity(value)
}

func NormalizeValidationSeverity(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ValidationSeverityOff, "ignore", "ignored", "none":
		return ValidationSeverityOff, nil
	case "warn", "warning", "warnings":
		return ValidationSeverityWarning, nil
	case ValidationSeverityError, "err", "errors":
		return ValidationSeverityError, nil
	default:
		return "", fmt.Errorf("validation severity must be off, warn, or error")
	}
}

func KnownValidationRules() []string {
	rules := make([]string, 0, len(knownValidationRules))
	for rule := range knownValidationRules {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	return rules
}

func IsKnownValidationRule(rule string) bool {
	_, ok := knownValidationRules[rule]
	return ok
}

func applyValidationOptions(result *Result, options ValidationOptions) error {
	overrides, err := normalizedValidationRules(options)
	if err != nil {
		return err
	}
	var errors []Issue
	var warnings []Issue
	for _, issue := range result.Errors {
		severity := validationSeverityForIssue(issue, ValidationSeverityError, overrides)
		switch severity {
		case ValidationSeverityError:
			errors = append(errors, issueWithSeverity(issue, ValidationSeverityError))
		case ValidationSeverityWarning:
			warnings = append(warnings, issueWithSeverity(issue, ValidationSeverityWarning))
		case ValidationSeverityOff:
		default:
			return fmt.Errorf("unsupported validation severity %q", severity)
		}
	}
	for _, issue := range result.Warnings {
		severity := validationSeverityForIssue(issue, ValidationSeverityWarning, overrides)
		switch severity {
		case ValidationSeverityError:
			errors = append(errors, issueWithSeverity(issue, ValidationSeverityError))
		case ValidationSeverityWarning:
			warnings = append(warnings, issueWithSeverity(issue, ValidationSeverityWarning))
		case ValidationSeverityOff:
		default:
			return fmt.Errorf("unsupported validation severity %q", severity)
		}
	}
	sortIssues(errors)
	sortIssues(warnings)
	result.Errors = errors
	result.Warnings = warnings
	result.Policy = ValidationPolicyReport{
		ConfigPath: options.ConfigPath,
		Overrides:  overrides,
	}
	return nil
}

func buildValidationSummary(result Result) ValidationSummary {
	status := "pass"
	if len(result.Errors) > 0 {
		status = "fail"
	} else if len(result.Warnings) > 0 {
		status = "warn"
	}
	return ValidationSummary{
		Status:       status,
		ErrorCount:   len(result.Errors),
		WarningCount: len(result.Warnings),
		IssueCount:   len(result.Errors) + len(result.Warnings),
	}
}

func normalizedValidationRules(options ValidationOptions) (map[string]string, error) {
	if len(options.Rules) == 0 {
		return nil, nil
	}
	rules := make(map[string]string, len(options.Rules))
	for rule, severity := range options.Rules {
		if !IsKnownValidationRule(rule) {
			return nil, fmt.Errorf("unknown validation rule %q", rule)
		}
		normalized, err := NormalizeValidationSeverity(severity)
		if err != nil {
			return nil, err
		}
		rules[rule] = normalized
	}
	return rules, nil
}

func validationSeverityForIssue(issue Issue, fallback string, overrides map[string]string) string {
	if overrides == nil {
		return fallback
	}
	if severity, ok := overrides[issue.Rule]; ok {
		return severity
	}
	return fallback
}

func issueWithSeverity(issue Issue, severity string) Issue {
	issue.Severity = severity
	return issue
}

func nonNilIssues(issues []Issue) []Issue {
	if issues == nil {
		return []Issue{}
	}
	return issues
}

func withValidationRuleSeverity(options ValidationOptions, rule string, severity string) ValidationOptions {
	if options.Rules == nil {
		options.Rules = map[string]string{}
	}
	options.Rules[rule] = severity
	return options
}

func stripValidationTomlComment(line string) string {
	inSingle := false
	inDouble := false
	escaped := false
	for index, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if inDouble && r == '\\' {
			escaped = true
			continue
		}
		switch r {
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:index]
			}
		}
	}
	return line
}

func parseValidationTomlStringValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			return strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(value, `"`), `"`), `\"`, `"`), nil
		}
		if strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`) {
			return strings.TrimSuffix(strings.TrimPrefix(value, `'`), `'`), nil
		}
	}
	if strings.ContainsAny(value, " \t") {
		return "", fmt.Errorf("expected string or bare severity value")
	}
	return value, nil
}
