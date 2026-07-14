package okf

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseFrontmatterDocumentSupportsNestedDataAndScalarValues(t *testing.T) {
	content := []byte(`---
id: weekly-docs-audit
enabled: true
schedule:
  cron: "0 9 * * MON"
  timezone: Europe/Prague
agent:
  command: codex
  args:
    - exec
    - --model
    - gpt-5
verify:
  commands:
    - go test ./...
    - openknowledge validate Wiki
tags: [docs, cli]
---

Audit docs.
`)

	document, err := ParseFrontmatterDocument(content)
	if err != nil {
		t.Fatal(err)
	}
	if !document.Has {
		t.Fatal("expected frontmatter")
	}
	if document.Body != "\nAudit docs.\n" {
		t.Fatalf("unexpected body: %q", document.Body)
	}
	if document.Values["id"] != "weekly-docs-audit" || document.Values["agent"] != "" {
		t.Fatalf("unexpected scalar compatibility values: %#v", document.Values)
	}
	if document.BodyLine != 19 {
		t.Fatalf("unexpected body line: %d", document.BodyLine)
	}

	expected := map[string]any{
		"id":      "weekly-docs-audit",
		"enabled": true,
		"schedule": map[string]any{
			"cron":     "0 9 * * MON",
			"timezone": "Europe/Prague",
		},
		"agent": map[string]any{
			"command": "codex",
			"args":    []any{"exec", "--model", "gpt-5"},
		},
		"verify": map[string]any{
			"commands": []any{"go test ./...", "openknowledge validate Wiki"},
		},
		"tags": []any{"docs", "cli"},
	}
	if !reflect.DeepEqual(document.Data, expected) {
		t.Fatalf("unexpected structured data\nwant=%#v\ngot=%#v", expected, document.Data)
	}
}

func TestParseFrontmatterDocumentReturnsStructuredErrors(t *testing.T) {
	_, err := ParseFrontmatterDocument([]byte("---\nagent:\n    - codex\n  command: codex\n---\n"))
	if err == nil {
		t.Fatal("expected structured frontmatter error")
	}
}

func TestParseFrontmatterDocumentSupportsCompleteYAMLCollectionsAndBlockScalars(t *testing.T) {
	content := []byte(`---
description: |-
  First line.
  ---
  Last line.
summary: >-
  Folded
  text.
config: {mode: fast, retry: {count: 3}}
tags:
  - docs
  - cli
enabled: false
ratio: 1.5
missing: null
published: "false"
date: 2026-07-15
---

# Body
`)

	document, err := ParseFrontmatterDocument(content)
	if err != nil {
		t.Fatal(err)
	}
	if document.Body != "\n# Body\n" {
		t.Fatalf("indented block-scalar delimiter must not close frontmatter: %q", document.Body)
	}
	if document.Values["description"] != "First line.\n---\nLast line." || document.Values["summary"] != "Folded text." {
		t.Fatalf("expected decoded block scalars, got %#v", document.Values)
	}
	if document.Values["config"] != "" || document.Values["tags"] != "[docs, cli]" {
		t.Fatalf("unexpected compatibility projection: %#v", document.Values)
	}

	config, ok := document.Data["config"].(map[string]any)
	if !ok || config["mode"] != "fast" {
		t.Fatalf("expected typed flow mapping, got %#v", document.Data["config"])
	}
	retry, ok := config["retry"].(map[string]any)
	if !ok || retry["count"] != 3 {
		t.Fatalf("expected nested flow mapping, got %#v", config["retry"])
	}
	if document.Data["enabled"] != false || document.Data["ratio"] != 1.5 || document.Data["missing"] != nil || document.Data["published"] != "false" {
		t.Fatalf("expected typed YAML scalars, got %#v", document.Data)
	}
	if document.Data["date"] != "2026-07-15" {
		t.Fatalf("timestamps must remain JSON-compatible strings, got %#v", document.Data["date"])
	}
	if _, err := json.Marshal(document.Data); err != nil {
		t.Fatalf("frontmatter data must be JSON-compatible: %v", err)
	}
}

func TestParseFrontmatterDocumentRejectsNonMappingRootAndReportsAbsoluteLine(t *testing.T) {
	_, err := ParseFrontmatterDocument([]byte("---\n- docs\n- cli\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "must be a YAML mapping") {
		t.Fatalf("expected non-mapping root error, got %v", err)
	}

	content := []byte("---\nconfig:\n  values: [one, two\n---\n")
	meta, _, parseErr := splitFrontmatter(string(content))
	if parseErr == nil {
		t.Fatal("expected malformed nested YAML error")
	}
	diagnostic := astFrontmatterDiagnostic(parseErr)
	if diagnostic.Line != 2 {
		t.Fatalf("expected absolute YAML error line 2, got %#v", diagnostic)
	}
	if len(meta.data) != 0 {
		t.Fatalf("invalid YAML must not produce partial typed data: %#v", meta.data)
	}
}

func TestParseFrontmatterDocumentWarnsForNestedDuplicateKeysAndUsesLaterValue(t *testing.T) {
	document, err := ParseFrontmatterDocument([]byte("---\nconfig:\n  mode: slow\n  mode: fast\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Warnings) != 1 || document.Warnings[0].Line != 4 {
		t.Fatalf("expected duplicate warning on later key, got %#v", document.Warnings)
	}
	config, ok := document.Data["config"].(map[string]any)
	if !ok || config["mode"] != "fast" {
		t.Fatalf("expected later duplicate value to win, got %#v", document.Data)
	}
}
