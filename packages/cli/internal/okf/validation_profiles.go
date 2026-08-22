package okf

import (
	"fmt"
	"sort"
	"strings"
)

type validationRuleDefinition struct {
	DefaultSeverity string
	Overrideable    bool
}

type validationSpecProfile struct {
	Version               string
	Rules                 map[string]validationRuleDefinition
	ConceptSections       string
	ReservedSections      string
	LogSection            string
	VersionSection        string
	ValidateConceptExtras func(ASTDocument, *Result)
}

var validationSpecProfiles = map[string]validationSpecProfile{
	"0.1": {
		Version: "0.1",
		Rules: map[string]validationRuleDefinition{
			"bundle-read":         {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"claim-profile":       {DefaultSeverity: ValidationSeverityError},
			"corpus-schema":       {DefaultSeverity: ValidationSeverityError},
			"concept-frontmatter": {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"concept-type":        {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"frontmatter":         {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"frontmatter-format":  {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"index-frontmatter":   {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"insight-contract":    {DefaultSeverity: ValidationSeverityError},
			"link-target":         {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"log-date":            {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"log-frontmatter":     {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"markdown-syntax":     {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"okf-version":         {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"publish-metadata":    {DefaultSeverity: ValidationSeverityError},
			"rule-catalog":        {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"utf-8":               {DefaultSeverity: ValidationSeverityError, Overrideable: true},
		},
		ConceptSections:  "sections 4 and 9",
		ReservedSections: "sections 3.1, 6, and 7",
		LogSection:       "section 7",
		VersionSection:   "section 11",
	},
	"0.2": {
		Version: "0.2",
		Rules: map[string]validationRuleDefinition{
			"bundle-read":         {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"claim-profile":       {DefaultSeverity: ValidationSeverityError},
			"corpus-schema":       {DefaultSeverity: ValidationSeverityError},
			"concept-frontmatter": {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"concept-type":        {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"frontmatter":         {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"frontmatter-format":  {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"index-frontmatter":   {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"insight-contract":    {DefaultSeverity: ValidationSeverityError},
			"link-target":         {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"log-date":            {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"log-frontmatter":     {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"markdown-syntax":     {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"okf-0.2-metadata":    {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"okf-version":         {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"publish-metadata":    {DefaultSeverity: ValidationSeverityError},
			"rule-catalog":        {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"utf-8":               {DefaultSeverity: ValidationSeverityError, Overrideable: true},
		},
		ConceptSections:       "sections 4 and 11",
		ReservedSections:      "sections 3.1, 8, and 9",
		LogSection:            "section 9",
		VersionSection:        "section 12",
		ValidateConceptExtras: validateOKFV02Concept,
	},
}

func validationProfileForVersion(version string) (validationSpecProfile, error) {
	resolved, ok := ResolveSpecVersion(version)
	if !ok {
		return validationSpecProfile{}, fmt.Errorf("unsupported OKF spec version: %s", strings.TrimSpace(version))
	}
	profile, ok := validationSpecProfiles[resolved]
	if !ok {
		return validationSpecProfile{}, fmt.Errorf("validation profile is not available for OKF %s", resolved)
	}
	return profile, nil
}

func validationRuleForVersion(version string, rule string) (validationRuleDefinition, bool) {
	profile, err := validationProfileForVersion(version)
	if err != nil {
		return validationRuleDefinition{}, false
	}
	definition, ok := profile.Rules[strings.TrimSpace(rule)]
	return definition, ok
}

func validationRuleAcrossProfiles(rule string) (bool, bool) {
	rule = strings.TrimSpace(rule)
	defined := false
	overrideable := false
	for _, profile := range validationSpecProfiles {
		definition, ok := profile.Rules[rule]
		if !ok {
			continue
		}
		defined = true
		overrideable = overrideable || definition.Overrideable
	}
	return defined, overrideable
}

func validationRules(profile validationSpecProfile) []string {
	rules := make([]string, 0, len(profile.Rules))
	for rule := range profile.Rules {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	return rules
}
