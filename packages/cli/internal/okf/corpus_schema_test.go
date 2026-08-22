package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCorpusSchemaValidatesDocumentTypesPathsMetadataLinksAndMigrations(t *testing.T) {
	root := t.TempDir()
	writeCorpusTestFile(t, root, "index.md", `---
type: Index
okf_version: "0.2"
corpus_schema:
  version: "1"
  document_types:
    - id: Runbook
      aliases: [Run Book]
      paths: [runbooks/**]
      required: [owner, service]
    - id: Service
      paths: [services/**]
      required: [owner]
    - id: Migration
      paths: [migrations/**]
  link_predicates:
    - id: depends_on
      source_types: [Runbook]
      target_types: [Service]
  migrations:
    - from: "0"
      to: "1"
      document: migrations/corpus-v1.md
---
# Knowledge
`)
	writeCorpusTestFile(t, root, "runbooks/auth.md", `---
type: Run Book
owner: team:identity
service: auth
typed_links:
  - predicate: depends_on
    target: ../services/auth.md
---
# Auth runbook
`)
	writeCorpusTestFile(t, root, "services/auth.md", `---
type: Service
owner: team:identity
---
# Auth
`)
	writeCorpusTestFile(t, root, "migrations/corpus-v1.md", `---
type: Migration
---
# Corpus v1 migration
`)

	result, _, err := parseAndValidateASTBundle(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	if messages := corpusIssueMessages(result.Errors); len(messages) != 0 {
		t.Fatalf("valid corpus schema failed: %#v", messages)
	}
}

func TestCorpusSchemaFailsClosedForUndeclaredTypesAndInvalidTypedLinks(t *testing.T) {
	root := t.TempDir()
	writeCorpusTestFile(t, root, "index.md", `---
type: Index
corpus_schema:
  version: "1"
  document_types:
    - id: Runbook
      paths: [runbooks/**]
      required: [owner]
    - id: Service
      paths: [services/**]
  link_predicates:
    - id: depends_on
      source_types: [Runbook]
      target_types: [Service]
  migrations:
    - from: "0"
      to: "1"
      document: migrations/missing.md
---
# Knowledge
`)
	writeCorpusTestFile(t, root, "wrong.md", `---
type: Runbook
owner: ""
typed_links:
  - predicate: depends_on
    target: service.md
---
# Wrong path
`)
	writeCorpusTestFile(t, root, "service.md", `---
type: Unknown
---
# Unknown
`)

	result, _, err := parseAndValidateASTBundle(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	messages := strings.Join(corpusIssueMessages(result.Errors), "\n")
	for _, expected := range []string{
		`document type "Runbook" is not allowed at this path`,
		`requires non-empty frontmatter field "owner"`,
		`document type "Unknown" is not declared`,
		`does not allow target type ""`,
		`migration document "migrations/missing.md" does not exist`,
	} {
		if !strings.Contains(messages, expected) {
			t.Fatalf("missing %q in corpus issues:\n%s", expected, messages)
		}
	}
}

func TestCorpusSchemaRejectsAmbiguousAliasesAndUnknownFields(t *testing.T) {
	root := t.TempDir()
	writeCorpusTestFile(t, root, "index.md", `---
type: Index
corpus_schema:
  version: "1"
  unexpected: true
  document_types:
    - id: Guide
      aliases: [Shared]
      paths: [guides/**]
    - id: Policy
      aliases: [Shared]
      paths: [policies/**]
---
# Knowledge
`)
	result, _, err := parseAndValidateASTBundle(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	messages := strings.Join(corpusIssueMessages(result.Errors), "\n")
	if !strings.Contains(messages, `unsupported field "unexpected"`) || !strings.Contains(messages, `name or alias "Shared" is assigned to both`) {
		t.Fatalf("expected strict schema issues, got:\n%s", messages)
	}
}

func TestCorpusSchemaIsOptional(t *testing.T) {
	root := t.TempDir()
	writeCorpusTestFile(t, root, "index.md", "---\ntype: Index\n---\n# Knowledge\n")
	writeCorpusTestFile(t, root, "anywhere.md", "---\ntype: Anything\n---\n# Anything\n")
	result, _, err := parseAndValidateASTBundle(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if messages := corpusIssueMessages(result.Errors); len(messages) != 0 {
		t.Fatalf("inactive corpus schema must have no effect: %#v", messages)
	}
}

func writeCorpusTestFile(t *testing.T, root string, relative string, content string) {
	t.Helper()
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func corpusIssueMessages(issues []Issue) []string {
	messages := []string{}
	for _, issue := range issues {
		if issue.Rule == "corpus-schema" {
			messages = append(messages, issue.Message)
		}
	}
	return messages
}
