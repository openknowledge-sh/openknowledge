package okf

import (
	"reflect"
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
