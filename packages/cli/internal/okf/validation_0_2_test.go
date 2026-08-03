package okf

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestOKFV02SourceDiagnostics(t *testing.T) {
	tests := []struct {
		name     string
		meta     map[string]any
		expected []string
	}{
		{
			name:     "sources must be a list",
			meta:     map[string]any{"sources": "source"},
			expected: []string{"sources should be a YAML list"},
		},
		{
			name: "invalid sources and root usage window",
			meta: map[string]any{"sources": "source", "usage_window": "June"},
			expected: []string{
				"sources should be a YAML list",
				"usage_window should be a { from, to } mapping",
			},
		},
		{
			name:     "source must be a mapping",
			meta:     map[string]any{"sources": []any{"source"}},
			expected: []string{"sources[0] should be a mapping"},
		},
		{
			name: "source fields",
			meta: map[string]any{"sources": []any{map[string]any{
				"resource":      " ",
				"id":            7,
				"title":         "",
				"author":        []any{},
				"usage_count":   -1,
				"last_modified": "yesterday",
				"usage_window":  "June",
			}}},
			expected: []string{
				"sources[0].resource should be a non-empty string",
				"sources[0].id should be a non-empty string",
				"sources[0].title should be a non-empty string",
				"sources[0].author should be a non-empty string",
				"sources[0].usage_count should be a non-negative number",
				"sources[0].last_modified should use YYYY-MM-DD",
				"sources[0].usage_window should be a { from, to } mapping",
			},
		},
		{
			name: "duplicate source IDs",
			meta: map[string]any{"sources": []any{
				map[string]any{"id": "source", "resource": "https://example.test/one"},
				map[string]any{"id": "source", "resource": "https://example.test/two"},
			}},
			expected: []string{`sources id "source" should be unique`},
		},
		{
			name:     "root usage window without sources",
			meta:     map[string]any{"usage_window": "June"},
			expected: []string{"usage_window should be a { from, to } mapping"},
		},
		{
			name: "root usage window dates",
			meta: map[string]any{
				"sources":      []any{},
				"usage_window": map[string]any{"from": "June", "to": "July"},
			},
			expected: []string{"usage_window should contain from and to dates in YYYY-MM-DD form"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			messages := collectOKFV02Messages(func(add func(string)) {
				validateOKFV02Sources(test.meta, add)
			})
			assertOKFV02Messages(t, messages, test.expected)
		})
	}
}

func TestOKFV02GeneratedAndVerifiedDiagnostics(t *testing.T) {
	tests := []struct {
		name     string
		validate func(func(string))
		expected []string
	}{
		{
			name: "generated mapping",
			validate: func(add func(string)) {
				validateOKFV02Generated(map[string]any{"generated": []any{}}, add)
			},
			expected: []string{"generated should be a mapping"},
		},
		{
			name: "generated actor and time",
			validate: func(add func(string)) {
				validateOKFV02Generated(map[string]any{"generated": map[string]any{"by": "human:", "at": "soon"}}, add)
			},
			expected: []string{
				"generated.by should identify an actor as <producer>/<version>, human:<id>, or process:<id>",
				"generated.at should be an ISO 8601 datetime",
			},
		},
		{
			name: "verified list",
			validate: func(add func(string)) {
				validateOKFV02Verified(map[string]any{"verified": "event"}, add)
			},
			expected: []string{"verified should be a mapping or a list of mappings"},
		},
		{
			name: "verified event mapping",
			validate: func(add func(string)) {
				validateOKFV02Verified(map[string]any{"verified": []any{"event"}}, add)
			},
			expected: []string{"verified[0] should be a mapping"},
		},
		{
			name: "verified actor and time",
			validate: func(add func(string)) {
				validateOKFV02Verified(map[string]any{"verified": []any{map[string]any{"by": "agent version", "at": 42}}}, add)
			},
			expected: []string{
				"verified[0].by should identify an actor as <producer>/<version>, human:<id>, or process:<id>",
				"verified[0].at should be an ISO 8601 datetime",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertOKFV02Messages(t, collectOKFV02Messages(test.validate), test.expected)
		})
	}
}

func TestOKFV02LifecycleAndAttributionDiagnostics(t *testing.T) {
	lifecycleTests := []struct {
		name     string
		meta     map[string]any
		expected []string
	}{
		{
			name:     "status type",
			meta:     map[string]any{"status": 7},
			expected: []string{"status should be draft, stable, or deprecated"},
		},
		{
			name:     "status value",
			meta:     map[string]any{"status": "retired"},
			expected: []string{`status "retired" should be draft, stable, or deprecated`},
		},
		{
			name:     "stale date",
			meta:     map[string]any{"stale_after": "tomorrow"},
			expected: []string{"stale_after should use YYYY-MM-DD"},
		},
	}
	for _, test := range lifecycleTests {
		t.Run(test.name, func(t *testing.T) {
			messages := collectOKFV02Messages(func(add func(string)) {
				validateOKFV02Lifecycle(test.meta, add)
			})
			assertOKFV02Messages(t, messages, test.expected)
		})
	}

	messages := collectOKFV02Messages(func(add func(string)) {
		validateOKFV02Attribution(
			"Known[^known]. Missing[^missing]. Duplicate[^missing]. Empty[^ ].",
			map[string]struct{}{"known": {}},
			add,
		)
	})
	assertOKFV02Messages(t, messages, []string{`footnote "missing" should match a sources[].id`})
}

func TestOKFV02ComputationDiagnostics(t *testing.T) {
	tests := []struct {
		name     string
		document ASTDocument
		expected []string
	}{
		{
			name:     "runtime and computation required",
			document: okfV02ComputationDocument(map[string]any{}, "", ASTMarkdown{}),
			expected: []string{
				"Attested Computation runtime should be a non-empty string",
				"Attested Computation should define a computation path or an inline # Computation code block",
			},
		},
		{
			name: "parameters must be a list",
			document: okfV02ComputationDocument(map[string]any{
				"runtime": "bigquery", "computation": "query.sql", "parameters": "year",
			}, "", ASTMarkdown{}),
			expected: []string{"Attested Computation parameters should be a list"},
		},
		{
			name: "parameter entry mapping",
			document: okfV02ComputationDocument(map[string]any{
				"runtime": "bigquery", "computation": "query.sql", "parameters": []any{"year"},
			}, "", ASTMarkdown{}),
			expected: []string{"parameters[0] should be a mapping"},
		},
		{
			name: "parameter fields",
			document: okfV02ComputationDocument(map[string]any{
				"runtime": "bigquery", "computation": "query.sql",
				"parameters": []any{map[string]any{"name": "", "type": 7, "required": "yes"}},
			}, "", ASTMarkdown{}),
			expected: []string{
				"parameters[0].name should be a non-empty string",
				"parameters[0].type should be a non-empty string",
				"parameters[0].required should be a boolean",
			},
		},
		{
			name: "computation path",
			document: okfV02ComputationDocument(map[string]any{
				"runtime": "bigquery", "computation": "",
			}, "", ASTMarkdown{}),
			expected: []string{
				"Attested Computation computation should be a non-empty path string",
				"Attested Computation should define a computation path or an inline # Computation code block",
			},
		},
		{
			name: "external and inline are exclusive",
			document: okfV02ComputationDocument(map[string]any{
				"runtime": "bigquery", "computation": "query.sql",
			}, "# Computation\n\n```sql\nSELECT 1\n```\n", ASTMarkdown{}),
			expected: []string{"Attested Computation should use either computation path or an inline # Computation code block, not both"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			messages := collectOKFV02Messages(func(add func(string)) {
				validateOKFV02Computation(test.document, add)
			})
			assertOKFV02Messages(t, messages, test.expected)
		})
	}
}

func TestOKFV02ExecutorAndDateRangeDiagnostics(t *testing.T) {
	executorTests := []struct {
		name     string
		meta     map[string]any
		expected []string
	}{
		{
			name:     "executor mapping",
			meta:     map[string]any{"executor": "runner"},
			expected: []string{"executor should be a mapping"},
		},
		{
			name: "executor fields",
			meta: map[string]any{"executor": map[string]any{"resource": "", "receipt": []any{"hash", ""}}},
			expected: []string{
				"executor.resource should be a non-empty string",
				"executor.receipt should be a list of non-empty field names",
			},
		},
		{
			name:     "attester mapping",
			meta:     map[string]any{"attester": "reviewer"},
			expected: []string{"attester should be a mapping"},
		},
		{
			name:     "attester resource",
			meta:     map[string]any{"attester": map[string]any{"resource": 7}},
			expected: []string{"attester.resource should be a non-empty string"},
		},
	}
	for _, test := range executorTests {
		t.Run(test.name, func(t *testing.T) {
			messages := collectOKFV02Messages(func(add func(string)) {
				validateOKFV02Executor(test.meta, add)
			})
			assertOKFV02Messages(t, messages, test.expected)
		})
	}

	rangeTests := []struct {
		name     string
		value    any
		expected string
	}{
		{name: "mapping", value: "June", expected: "window should be a { from, to } mapping"},
		{name: "dates", value: map[string]any{"from": "June", "to": "July"}, expected: "window should contain from and to dates in YYYY-MM-DD form"},
	}
	for _, test := range rangeTests {
		t.Run("date range "+test.name, func(t *testing.T) {
			messages := collectOKFV02Messages(func(add func(string)) {
				validateOKFV02DateRange("window", test.value, add)
			})
			assertOKFV02Messages(t, messages, []string{test.expected})
		})
	}
}

func TestOKFV02InlineComputationForms(t *testing.T) {
	tests := []struct {
		name     string
		markdown ASTMarkdown
		body     string
		expected bool
	}{
		{
			name: "AST code block",
			markdown: ASTMarkdown{
				Headings:   []ASTMarkdownHeading{{Level: 1, Text: "Computation", Line: 1}},
				CodeBlocks: []ASTMarkdownCodeBlock{{LineStart: 3, LineEnd: 5}},
			},
			expected: true,
		},
		{
			name: "AST unrelated heading",
			markdown: ASTMarkdown{
				Headings:   []ASTMarkdownHeading{{Level: 2, Text: "Computation", Line: 1}},
				CodeBlocks: []ASTMarkdownCodeBlock{{LineStart: 3, LineEnd: 5}},
			},
			expected: false,
		},
		{
			name: "AST code block after next section",
			markdown: ASTMarkdown{
				Headings: []ASTMarkdownHeading{
					{Level: 1, Text: "Computation", Line: 1},
					{Level: 1, Text: "Notes", Line: 3},
				},
				CodeBlocks: []ASTMarkdownCodeBlock{{LineStart: 4, LineEnd: 6}},
			},
			expected: false,
		},
		{name: "fenced body", body: "# Computation\n\n~~~sql\nSELECT 1\n~~~\n", expected: true},
		{name: "indented body", body: "# Computation\n\n    SELECT 1\n", expected: true},
		{name: "heading without code", body: "# Computation\n\nExplain it.\n", expected: false},
		{name: "code after next section", body: "# Computation\n\nExplain it.\n\n# Notes\n\n```sql\nSELECT 1\n```\n", expected: false},
		{name: "wrong heading", body: "# Query\n\n```sql\nSELECT 1\n```\n", expected: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := okfHasInlineComputation(test.markdown, test.body); actual != test.expected {
				t.Fatalf("expected %t, got %t", test.expected, actual)
			}
		})
	}
}

func TestOKFV02ScalarShapes(t *testing.T) {
	actorTests := []struct {
		value    any
		expected bool
	}{
		{value: "agent/v1", expected: true},
		{value: "human:reviewer", expected: true},
		{value: "process:build", expected: true},
		{value: "human:", expected: false},
		{value: "agent/version/extra", expected: false},
		{value: "agent version", expected: false},
		{value: 7, expected: false},
	}
	for _, test := range actorTests {
		if actual := okfActor(test.value); actual != test.expected {
			t.Errorf("okfActor(%#v): expected %t, got %t", test.value, test.expected, actual)
		}
	}

	dateTests := []struct {
		value    any
		expected bool
	}{
		{value: "2026-08-03", expected: true},
		{value: "2026-02-30", expected: false},
		{value: 20260803, expected: false},
	}
	for _, test := range dateTests {
		if actual := okfDate(test.value); actual != test.expected {
			t.Errorf("okfDate(%#v): expected %t, got %t", test.value, test.expected, actual)
		}
	}

	dateTimeTests := []struct {
		value    any
		expected bool
	}{
		{value: "2026-08-03T12:00:00Z", expected: true},
		{value: "2026-08-03", expected: false},
		{value: 20260803, expected: false},
	}
	for _, test := range dateTimeTests {
		if actual := okfDateTime(test.value); actual != test.expected {
			t.Errorf("okfDateTime(%#v): expected %t, got %t", test.value, test.expected, actual)
		}
	}

	numberTests := []struct {
		value    any
		expected bool
	}{
		{value: int(0), expected: true},
		{value: int8(1), expected: true},
		{value: int16(1), expected: true},
		{value: int32(1), expected: true},
		{value: int64(1), expected: true},
		{value: uint(1), expected: true},
		{value: uint8(1), expected: true},
		{value: uint16(1), expected: true},
		{value: uint32(1), expected: true},
		{value: uint64(1), expected: true},
		{value: float32(1), expected: true},
		{value: float64(1), expected: true},
		{value: -1, expected: false},
		{value: math.NaN(), expected: false},
		{value: "1", expected: false},
	}
	for _, test := range numberTests {
		if actual := okfNonNegativeNumber(test.value); actual != test.expected {
			t.Errorf("okfNonNegativeNumber(%#v): expected %t, got %t", test.value, test.expected, actual)
		}
	}

	listTests := []struct {
		value    any
		expected bool
	}{
		{value: []any{}, expected: true},
		{value: []any{"hash", "signature"}, expected: true},
		{value: []any{"hash", ""}, expected: false},
		{value: []string{"hash"}, expected: false},
	}
	for _, test := range listTests {
		if actual := okfStringList(test.value); actual != test.expected {
			t.Errorf("okfStringList(%#v): expected %t, got %t", test.value, test.expected, actual)
		}
	}
}

func TestOKFV02MetadataRulePolicyOverrides(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "concept.md", "---\ntype: Note\ngenerated: []\n---\n\n# Concept\n")

	warningResult, err := ValidateWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(warningResult.Warnings) != 1 || warningResult.Warnings[0].Rule != "okf-0.2-metadata" || warningResult.Warnings[0].Severity != ValidationSeverityWarning {
		t.Fatalf("expected one default 0.2 metadata warning, got %#v", warningResult)
	}
	if statusForCheck(warningResult, "OKF 0.2 metadata") != "warn" {
		t.Fatalf("expected metadata check to warn, got %#v", warningResult.Checks)
	}

	errorResult, err := ValidateWithVersionAndOptions(root, "0.2", ValidationOptions{
		Rules: map[string]string{"okf-0.2-metadata": "error"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(errorResult.Errors) != 1 || len(errorResult.Warnings) != 0 || errorResult.Errors[0].Severity != ValidationSeverityError {
		t.Fatalf("expected the metadata warning to escalate, got %#v", errorResult)
	}
	if statusForCheck(errorResult, "OKF 0.2 metadata") != "fail" {
		t.Fatalf("expected escalated metadata check to fail, got %#v", errorResult.Checks)
	}

	offResult, err := ValidateWithVersionAndOptions(root, "0.2", ValidationOptions{
		Rules: map[string]string{"okf-0.2-metadata": "off"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offResult.Errors) != 0 || len(offResult.Warnings) != 0 || len(offResult.Issues) != 0 {
		t.Fatalf("expected the metadata warning to be disabled, got %#v", offResult)
	}
	if statusForCheck(offResult, "OKF 0.2 metadata") != "pass" {
		t.Fatalf("expected disabled metadata check to pass, got %#v", offResult.Checks)
	}

	legacyResult, err := ValidateWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if countRule(legacyResult.Warnings, "okf-0.2-metadata") != 0 || statusForCheck(legacyResult, "OKF 0.2 metadata") != "" {
		t.Fatalf("0.1 must not run or report the 0.2 checker: %#v", legacyResult)
	}
}

func TestInsightContractUsesVersionSpecificLifecycleAndGenerationMetadata(t *testing.T) {
	root := t.TempDir()
	v02 := `---
type: Open Knowledge Insight
title: Update guide
status: draft
okf_publish: false
okf_insight_id: current
okf_insight_kind: explicit
generated:
  by: process:openknowledge-cli
  at: 2026-08-03T12:00:00Z
okf_insight_targets: [guide.md]
---

# Update guide
`
	writeFile(t, root, "insights/current.md", v02)
	current, err := ValidateWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Errors) != 0 || len(current.Warnings) != 0 {
		t.Fatalf("expected clean OKF 0.2 insight, got errors=%#v warnings=%#v", current.Errors, current.Warnings)
	}
	legacyProfile, err := ValidateWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if countRule(legacyProfile.Errors, "insight-contract") == 0 {
		t.Fatalf("expected OKF 0.1 insight contract to reject 0.2 metadata: %#v", legacyProfile)
	}

	legacy := `---
type: Open Knowledge Insight
title: Update guide
status: pending
okf_publish: false
okf_insight_id: legacy
okf_insight_kind: docs
okf_insight_runtime: codex
okf_insight_created_at: 2026-08-03T12:00:00Z
okf_insight_targets: [guide.md]
---

# Update guide
`
	writeFile(t, root, "insights/current.md", legacy)
	compatible, err := ValidateWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(compatible.Errors) != 0 || countRule(compatible.Warnings, "okf-0.2-metadata") == 0 {
		t.Fatalf("expected legacy insight compatibility with a 0.2 migration warning: %#v", compatible)
	}
	legacyResult, err := ValidateWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyResult.Errors) != 0 || len(legacyResult.Warnings) != 0 {
		t.Fatalf("expected clean OKF 0.1 legacy insight, got errors=%#v warnings=%#v", legacyResult.Errors, legacyResult.Warnings)
	}
}

func TestOKFV02ConceptIssuesCarryStableIdentity(t *testing.T) {
	result := Result{}
	document := ASTDocument{
		Rel:         "concept.md",
		Frontmatter: ASTFrontmatter{Data: map[string]any{"generated": []any{}}},
	}
	validateOKFV02Concept(document, &result)
	if len(result.Warnings) != 1 {
		t.Fatalf("expected one warning, got %#v", result.Warnings)
	}
	issue := result.Warnings[0]
	if issue.Path != "concept.md" || issue.Line != 1 || issue.Rule != "okf-0.2-metadata" || issue.Message != "generated should be a mapping" {
		t.Fatalf("unexpected issue identity: %#v", issue)
	}
}

func collectOKFV02Messages(validate func(func(string))) []string {
	var messages []string
	validate(func(message string) {
		messages = append(messages, message)
	})
	return messages
}

func assertOKFV02Messages(t *testing.T, actual []string, expected []string) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected messages:\nactual:   %#v\nexpected: %#v", actual, expected)
	}
}

func okfV02ComputationDocument(meta map[string]any, body string, markdown ASTMarkdown) ASTDocument {
	return ASTDocument{
		Frontmatter: ASTFrontmatter{Data: meta},
		Metadata:    ASTDocumentMetadata{Type: "Attested Computation"},
		Body:        body,
		Markdown:    markdown,
	}
}

func TestOKFV02ValidDirectShapesProduceNoDiagnostics(t *testing.T) {
	meta := map[string]any{
		"sources": []any{map[string]any{
			"id":            "source",
			"resource":      "https://example.test/source",
			"title":         "Source",
			"author":        "team:data",
			"usage_count":   uint64(2),
			"last_modified": "2026-08-03",
			"usage_window":  map[string]any{"from": "2026-08-01", "to": "2026-08-03"},
		}},
		"usage_window": map[string]any{"from": "2026-08-01", "to": "2026-08-03"},
		"generated":    map[string]any{"by": "process:build", "at": "2026-08-03T12:00:00Z"},
		"verified":     []any{map[string]any{"by": "human:reviewer", "at": "2026-08-03T12:30:00Z"}},
		"status":       "deprecated",
		"stale_after":  "2026-12-31",
		"runtime":      "bigquery",
		"parameters":   []any{map[string]any{"name": "year", "type": "integer", "required": true}},
		"computation":  "query.sql",
		"executor":     map[string]any{"resource": "https://example.test/executor", "receipt": []any{"hash", "signature"}},
		"attester":     map[string]any{"resource": "https://example.test/attester"},
	}
	document := okfV02ComputationDocument(meta, "Supported by source.[^source]", ASTMarkdown{})
	document.Rel = "computation.md"
	result := Result{}
	validateOKFV02Concept(document, &result)
	if len(result.Warnings) != 0 {
		t.Fatalf("expected valid direct shapes to pass, got %#v", result.Warnings)
	}
}

func TestOKFV02ActorRejectsWhitespaceAndMalformedProducerVersions(t *testing.T) {
	invalid := []any{"", " ", "human:review er", "process:", "/v1", "agent/", "agent/v1/extra", "agent\tv1"}
	for _, value := range invalid {
		if okfActor(value) {
			t.Errorf("expected malformed actor %#v to fail", value)
		}
	}
	for _, value := range []string{"human:reviewer", "process:build", "agent/v1"} {
		if !okfActor(value) {
			t.Errorf("expected actor %q to pass", value)
		}
	}
}

func TestOKFV02NonEmptyString(t *testing.T) {
	for _, value := range []any{"value", " value "} {
		if !okfNonEmptyString(value) {
			t.Errorf("expected %#v to be non-empty", value)
		}
	}
	for _, value := range []any{"", " ", 7, nil} {
		if okfNonEmptyString(value) {
			t.Errorf("expected %#v to be empty or non-string", value)
		}
	}
}

func TestOKFV02NoMetadataIsValid(t *testing.T) {
	result := Result{}
	validateOKFV02Concept(ASTDocument{
		Rel:         "concept.md",
		Frontmatter: ASTFrontmatter{Data: map[string]any{}},
		Metadata:    ASTDocumentMetadata{Type: "Note"},
		Body:        strings.Repeat("body", 2),
	}, &result)
	if len(result.Warnings) != 0 {
		t.Fatalf("optional metadata must remain optional: %#v", result.Warnings)
	}
}
