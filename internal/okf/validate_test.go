package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateMinimalBundle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Bundle\n")
	writeFile(t, root, "log.md", "# Log\n\n## 2026-06-15\n\n* **Creation**: Created bundle.\n")
	writeFile(t, root, "concepts/table.md", "---\ntype: BigQuery Table\ntitle: Orders\n---\n\n# Schema\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %#v", result.Errors)
	}
	if result.Concepts != 1 || result.Indexes != 1 || result.Logs != 1 {
		t.Fatalf("unexpected counts: %#v", result)
	}
}

func TestValidateConceptRequiresType(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "concept.md", "---\ntitle: Missing Type\n---\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected one error, got %#v", result.Errors)
	}
	if statusForCheck(result, "Concept documents") != "fail" {
		t.Fatalf("expected concept documents check to fail, got %#v", result.Checks)
	}
}

func TestValidateUppercaseMarkdownExtension(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "SETUP.MD", "---\ntype: Setup\ntitle: Setup\n---\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %#v", result.Errors)
	}
	if result.Concepts != 1 {
		t.Fatalf("expected uppercase .MD to count as concept, got %#v", result)
	}

	listing, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 1 || listing.Entries[0].ID != "SETUP" {
		t.Fatalf("expected list to include uppercase .MD concept, got %#v", listing.Entries)
	}
}

func TestValidateReservedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/index.md", "---\ntype: Index\n---\n# Docs\n")
	writeFile(t, root, "log.md", "# Log\n\n## June 15 2026\n\n* Bad date.\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("expected two errors, got %#v", result.Errors)
	}
}

func TestListIncludesConceptsAndReservedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "notes/example-note.md", "---\ntype: Reference\ndescription: Example description.\n---\n")
	writeFile(t, root, "index.md", "# Index\n")
	writeFile(t, root, "notes/log.md", "# Log\n")

	listing, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 3 {
		t.Fatalf("expected concept plus reserved files, got %#v", listing.Entries)
	}
	if listing.Entries[0].Path != "index.md" || !listing.Entries[0].Reserved || listing.Entries[0].Kind != "index" {
		t.Fatalf("unexpected root index entry: %#v", listing.Entries[0])
	}
	if listing.Entries[1].ID != "notes/example-note" || listing.Entries[1].Title != "Example note" {
		t.Fatalf("unexpected concept: %#v", listing.Entries[1])
	}
	if listing.Entries[2].Path != "notes/log.md" || !listing.Entries[2].Reserved || listing.Entries[2].Kind != "log" {
		t.Fatalf("unexpected log entry: %#v", listing.Entries[2])
	}
}

func TestListAnnotatesInvalidBundle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "bad.md", "# Missing frontmatter\n")

	listing, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 1 {
		t.Fatalf("expected one listed entry, got %#v", listing.Entries)
	}
	if len(listing.Entries[0].Issues) != 1 {
		t.Fatalf("expected inline issue for invalid file, got %#v", listing.Entries[0])
	}
	if !strings.Contains(listing.Entries[0].Issues[0].Message, "frontmatter") {
		t.Fatalf("unexpected inline issue: %#v", listing.Entries[0].Issues[0])
	}
}

func TestNewProjectCreatesValidBundle(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "my-knowledge-base")

	result, err := NewProject(NewProjectOptions{Name: "My Knowledge Base", Path: target})
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "My Knowledge Base" {
		t.Fatalf("unexpected name: %s", result.Name)
	}
	if result.SetupPath != filepath.Join(target, "SETUP.MD") {
		t.Fatalf("unexpected setup path: %s", result.SetupPath)
	}

	for _, name := range []string{"index.md", "log.md", "AGENTS.md", "SETUP.MD", "SPEC.md", "concepts/index.md", "wiki/index.md", "raw/index.md"} {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
	agents, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "type: Agent Rules") || !strings.Contains(string(agents), "Open Knowledge wiki") {
		t.Fatalf("generated AGENTS.md does not contain expected starter rules")
	}
	spec, err := os.ReadFile(filepath.Join(target, "SPEC.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(spec), "type: Specification") || !strings.Contains(string(spec), "Open Knowledge Format") {
		t.Fatalf("generated SPEC.md does not contain expected spec content")
	}

	validation, err := Validate(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(validation.Errors) != 0 {
		t.Fatalf("expected generated project to validate, got %#v", validation.Errors)
	}
	if validation.Concepts != 3 {
		t.Fatalf("expected AGENTS.md, SETUP.MD and SPEC.md to count as concepts, got %#v", validation)
	}
}

func TestNewProjectRefusesNonEmptyDirectory(t *testing.T) {
	target := t.TempDir()
	writeFile(t, target, "existing.txt", "already here\n")

	_, err := NewProject(NewProjectOptions{Name: "Existing", Path: target})
	if err == nil {
		t.Fatal("expected error for non-empty directory")
	}
}

func TestLatestSpecIsEmbedded(t *testing.T) {
	if LatestSpecVersion != "0.1" {
		t.Fatalf("unexpected latest spec version: %s", LatestSpecVersion)
	}
	if !strings.Contains(LatestSpec(), "Open Knowledge Format") {
		t.Fatal("expected embedded latest spec content")
	}
	if Spec("unknown") != "" {
		t.Fatal("expected unknown spec version to return empty content")
	}
}

func statusForCheck(result Result, name string) string {
	for _, check := range result.Checks {
		if check.Name == name {
			return check.Status
		}
	}
	return ""
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
