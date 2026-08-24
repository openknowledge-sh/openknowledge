package okf

import (
	"fmt"
	"strings"
)

const ValidationConfigFile = ".openknowledge.toml"

// legacyValidationConfigFile is intentionally never loaded. Keep it private so
// an old configuration cannot be exposed as a public asset during migration.
const legacyValidationConfigFile = "openknowledge.toml"

const (
	ValidationSeverityOff     = "off"
	ValidationSeverityWarning = "warning"
	ValidationSeverityError   = "error"
	ValidationProfileBundle   = "bundle"
	ValidationProfileOKF      = "okf"
)

var extensionValidationRules = map[string]bool{
	"publish-metadata": true,
	"insight-contract": true,
	"rule-catalog":     true,
	"claim-profile":    true,
	"corpus-schema":    true,
}

type ValidationOptions struct {
	ConfigPath string
	Rules      map[string]string
	// Profile selects either pure OKF validation or the Open Knowledge bundle
	// extensions. Empty means bundle for backward compatibility.
	Profile string
	// EvidenceRoot resolves generation-private evidence when a published
	// projection does not contain the canonical .openknowledge/evidence path.
	// It is a runtime validation input and is never loaded from project config.
	EvidenceRoot string
}

func LoadValidationOptions(root string) (ValidationOptions, error) {
	config, err := LoadProjectConfig(root)
	if err != nil {
		return ValidationOptions{}, err
	}
	return config.Validation, nil
}

func LoadValidationOptionsFile(path string) (ValidationOptions, error) {
	config, err := LoadProjectConfigFile(path)
	if err != nil {
		return ValidationOptions{}, err
	}
	return config.Validation, nil
}

func ParseValidationOptionsConfig(content string) (ValidationOptions, error) {
	config, err := ParseProjectConfig(content)
	if err != nil {
		return ValidationOptions{}, err
	}
	return config.Validation, nil
}

func MergeValidationOptions(base ValidationOptions, override ValidationOptions) ValidationOptions {
	merged := ValidationOptions{ConfigPath: base.ConfigPath, EvidenceRoot: base.EvidenceRoot, Profile: base.Profile}
	if strings.TrimSpace(override.EvidenceRoot) != "" {
		merged.EvidenceRoot = override.EvidenceRoot
	}
	if strings.TrimSpace(override.Profile) != "" {
		merged.Profile = override.Profile
	}
	for rule, severity := range base.Rules {
		merged = withValidationRuleSeverity(merged, rule, severity)
	}
	for rule, severity := range override.Rules {
		merged = withValidationRuleSeverity(merged, rule, severity)
	}
	return merged
}

func NormalizeValidationProfile(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", ValidationProfileBundle:
		return ValidationProfileBundle, nil
	case ValidationProfileOKF:
		return ValidationProfileOKF, nil
	default:
		return "", fmt.Errorf("validation profile must be bundle or okf")
	}
}

func ValidationProfileAllowsRule(profile string, rule string) bool {
	normalized, err := NormalizeValidationProfile(profile)
	if err != nil {
		return false
	}
	return normalized == ValidationProfileBundle || !extensionValidationRules[strings.TrimSpace(rule)]
}

func SetValidationRuleSeverity(options *ValidationOptions, rule string, severity string) error {
	return SetValidationRuleSeverityForVersion(options, LatestSpecVersion, rule, severity)
}

func setConfiguredValidationRuleSeverity(options *ValidationOptions, rule string, severity string) error {
	rule = strings.TrimSpace(rule)
	defined, overrideable := validationRuleAcrossProfiles(rule)
	if !defined {
		return fmt.Errorf("unknown validation rule %q", rule)
	}
	if !overrideable {
		return fmt.Errorf("validation rule %q has fixed severity", rule)
	}
	normalized, err := NormalizeValidationSeverity(severity)
	if err != nil {
		return err
	}
	*options = withValidationRuleSeverity(*options, rule, normalized)
	return nil
}

func SetValidationRuleSeverityForVersion(options *ValidationOptions, version string, rule string, severity string) error {
	rule = strings.TrimSpace(rule)
	profile, err := validationProfileForVersion(version)
	if err != nil {
		return err
	}
	definition, ok := profile.Rules[rule]
	if !ok {
		return fmt.Errorf("validation rule %q is not defined for OKF %s", rule, profile.Version)
	}
	if !definition.Overrideable {
		return fmt.Errorf("validation rule %q has fixed severity for OKF %s", rule, profile.Version)
	}
	normalized, err := NormalizeValidationSeverity(severity)
	if err != nil {
		return err
	}
	*options = withValidationRuleSeverity(*options, rule, normalized)
	return nil
}

func ParseValidationRuleOverride(value string) (string, string, error) {
	return ParseValidationRuleOverrideForVersion(LatestSpecVersion, value)
}

func ParseValidationRuleOverrideForVersion(version string, value string) (string, string, error) {
	rule, severity, ok := strings.Cut(value, "=")
	if !ok {
		return "", "", fmt.Errorf("validation rule override must use rule=off|warn|error: %s", value)
	}
	normalized, err := ParseValidationSeverity(severity)
	if err != nil {
		return "", "", err
	}
	rule = strings.TrimSpace(rule)
	profile, err := validationProfileForVersion(version)
	if err != nil {
		return "", "", err
	}
	definition, ok := profile.Rules[rule]
	if !ok {
		return "", "", fmt.Errorf("validation rule %q is not defined for OKF %s", rule, profile.Version)
	}
	if !definition.Overrideable {
		return "", "", fmt.Errorf("validation rule %q has fixed severity for OKF %s", rule, profile.Version)
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
	rules, _ := KnownValidationRulesForVersion(LatestSpecVersion)
	return rules
}

func KnownValidationRulesForVersion(version string) ([]string, error) {
	profile, err := validationProfileForVersion(version)
	if err != nil {
		return nil, err
	}
	return validationRules(profile), nil
}

func IsKnownValidationRule(rule string) bool {
	return IsKnownValidationRuleForVersion(LatestSpecVersion, rule)
}

func IsKnownValidationRuleForVersion(version string, rule string) bool {
	_, ok := validationRuleForVersion(version, rule)
	return ok
}

func IsValidationRuleOverrideableForVersion(version string, rule string) bool {
	definition, ok := validationRuleForVersion(version, rule)
	return ok && definition.Overrideable
}

func applyValidationOptions(result *Result, options ValidationOptions) error {
	profile, err := validationProfileForVersion(result.SpecVersion)
	if err != nil {
		return err
	}
	overrides, err := normalizedValidationRulesForVersion(profile, options)
	if err != nil {
		return err
	}
	var errors []Issue
	var warnings []Issue
	for _, issue := range result.Errors {
		severity, err := validationSeverityForIssue(profile, issue, overrides)
		if err != nil {
			return err
		}
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
		severity, err := validationSeverityForIssue(profile, issue, overrides)
		if err != nil {
			return err
		}
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

func normalizedValidationRulesForVersion(profile validationSpecProfile, options ValidationOptions) (map[string]string, error) {
	if len(options.Rules) == 0 {
		return nil, nil
	}
	rules := make(map[string]string, len(options.Rules))
	for rule, severity := range options.Rules {
		if !ValidationProfileAllowsRule(options.Profile, rule) {
			continue
		}
		definition, ok := profile.Rules[rule]
		if !ok {
			if defined, _ := validationRuleAcrossProfiles(rule); defined {
				continue
			}
			return nil, fmt.Errorf("unknown validation rule %q", rule)
		}
		if !definition.Overrideable {
			return nil, fmt.Errorf("validation rule %q has fixed severity for OKF %s", rule, profile.Version)
		}
		normalized, err := NormalizeValidationSeverity(severity)
		if err != nil {
			return nil, err
		}
		rules[rule] = normalized
	}
	return rules, nil
}

func validationSeverityForIssue(profile validationSpecProfile, issue Issue, overrides map[string]string) (string, error) {
	definition, ok := profile.Rules[issue.Rule]
	if !ok {
		return "", fmt.Errorf("validation rule %q is not defined for OKF %s", issue.Rule, profile.Version)
	}
	if definition.Overrideable && overrides != nil {
		if severity, ok := overrides[issue.Rule]; ok {
			return severity, nil
		}
	}
	return definition.DefaultSeverity, nil
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
