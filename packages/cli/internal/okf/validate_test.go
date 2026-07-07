package okf

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestValidateRootIndexAllowsBundleMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_name: \"accessibility\"\nokf_bundle_title: \"Accessibility Review\"\nokf_bundle_tags: [\"accessibility\", \"review\"]\nokf_bundle_entry_default: \"agents/checker.md\"\ncustom_root_key: \"allowed\"\n---\n\n# Bundle\n")
	writeFile(t, root, "concept.md", "---\ntype: Concept\n---\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected root bundle metadata to validate, got %#v", result.Errors)
	}
}

func TestValidateIndexAllowsPublishMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Bundle\n")
	writeFile(t, root, "docs/index.md", "---\nokf_publish: false\n---\n\n# Docs\n")
	writeFile(t, root, "concept.md", "---\ntype: Concept\n---\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected index publish metadata to validate, got %#v", result.Errors)
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

func TestValidateRejectsInvalidUTF8Markdown(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "concept.md")
	if err := os.WriteFile(path, []byte{'-', '-', '-', '\n', 't', 'y', 'p', 'e', ':', ' ', 'C', 'o', 'n', 'c', 'e', 'p', 't', '\n', '-', '-', '-', '\n', '\n', '#', ' ', 0xff, '\n'}, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected one invalid UTF-8 error, got %#v", result.Errors)
	}
	if result.Errors[0].Rule != "utf-8" || result.Errors[0].Line != 5 {
		t.Fatalf("unexpected UTF-8 error: %#v", result.Errors[0])
	}
	if statusForCheck(result, "UTF-8 content") != "fail" {
		t.Fatalf("expected UTF-8 check to fail, got %#v", result.Checks)
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

func TestValidateWarnsForBrokenLocalLinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Index\n\n[Concept](concepts/good.md)\n[Section](#section)\n[External](https://openknowledge.sh)\n[Missing](missing.md)\n[Missing directory](references/)\n")
	writeFile(t, root, "log.md", "# Log\n\n## 2026-06-16\n\n* Created.\n")
	writeFile(t, root, "concepts/good.md", "---\ntype: Concept\ntitle: Good\n---\n\n# Good\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %#v", result.Errors)
	}
	if statusForCheck(result, "Link targets") != "warn" {
		t.Fatalf("expected link targets check to warn, got %#v", result.Checks)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("expected two broken-link warnings, got %#v", result.Warnings)
	}
	if result.Warnings[0].Rule != "link-target" || !strings.Contains(result.Warnings[0].Message, "missing.md") {
		t.Fatalf("unexpected first warning: %#v", result.Warnings[0])
	}

	listing, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	var index ListEntry
	for _, entry := range listing.Entries {
		if entry.Path == "index.md" {
			index = entry
			break
		}
	}
	if len(index.Issues) != 2 {
		t.Fatalf("expected list to include link warnings on index.md, got %#v", index.Issues)
	}
}

func TestValidateOptionsEscalateAndDisableRules(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Index\n\n[Missing](missing.md)\n")

	result, err := ValidateWithVersionAndOptions(root, LatestSpecVersion, ValidationOptions{
		Rules: map[string]string{"link-target": "error"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 1 || result.Errors[0].Rule != "link-target" || result.Errors[0].Severity != ValidationSeverityError {
		t.Fatalf("expected link target warning to be escalated, got %#v", result.Errors)
	}
	if result.Summary.Status != "fail" || result.Summary.ErrorCount != 1 || result.Summary.IssueCount != 1 {
		t.Fatalf("unexpected validation summary: %#v", result.Summary)
	}
	if statusForCheck(result, "Link targets") != "fail" {
		t.Fatalf("expected link target check to fail, got %#v", result.Checks)
	}

	result, err = ValidateWithVersionAndOptions(root, LatestSpecVersion, ValidationOptions{
		Rules: map[string]string{"link-target": "off"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 || len(result.Warnings) != 0 || len(result.Issues) != 0 {
		t.Fatalf("expected disabled link target rule to suppress issue, got %#v", result)
	}
	if result.Summary.Status != "pass" {
		t.Fatalf("expected disabled rule to pass validation, got %#v", result.Summary)
	}
}

func TestParseValidationOptionsConfig(t *testing.T) {
	options, err := ParseValidationOptionsConfig("[validation.rules]\nlink-target = \"error\"\nmarkdown-syntax = 'off'\n")
	if err != nil {
		t.Fatal(err)
	}
	if options.Rules["link-target"] != ValidationSeverityError || options.Rules["markdown-syntax"] != ValidationSeverityOff {
		t.Fatalf("unexpected validation options: %#v", options.Rules)
	}

	if _, err := ParseValidationOptionsConfig("[validation.rules]\nmissing-rule = \"warn\"\n"); err == nil {
		t.Fatal("expected unknown validation rule to fail")
	}
}

func TestValidateAcceptsDirectoryLinksToIndex(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Index\n\n[Guides](guides) and [Guides index](guides/).\n")
	writeFile(t, root, "log.md", "# Log\n\n## 2026-06-16\n\n* Created.\n")
	writeFile(t, root, "guides/index.md", "# Guides\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings for directory links with index.md, got %#v", result.Warnings)
	}
	if statusForCheck(result, "Link targets") != "pass" {
		t.Fatalf("expected link targets check to pass, got %#v", result.Checks)
	}
}

func TestValidateIgnoresLinksInsideFencedCode(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Index\n\n```markdown\n[Example](missing.md)\n```\n")
	writeFile(t, root, "log.md", "# Log\n\n## 2026-06-16\n\n* Created.\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings for example links inside fenced code, got %#v", result.Warnings)
	}
	if statusForCheck(result, "Link targets") != "pass" {
		t.Fatalf("expected link targets check to pass, got %#v", result.Checks)
	}
}

func TestValidateWarnsForMarkdownSyntax(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Index\n\n[Broken](missing.md\n\n```sh\necho ok\n")
	writeFile(t, root, "log.md", "# Log\n\n## 2026-06-16\n\n* Created.\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected markdown syntax issues to be warnings, got errors %#v", result.Errors)
	}
	if statusForCheck(result, "Markdown syntax") != "warn" {
		t.Fatalf("expected markdown syntax check to warn, got %#v", result.Checks)
	}
	if countRule(result.Warnings, "markdown-syntax") != 2 {
		t.Fatalf("expected two markdown syntax warnings, got %#v", result.Warnings)
	}
}

func TestValidateMarkdownSyntaxIgnoresFrontmatterValues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "concept.md", "---\ntype: Concept\ntitle: \"Use `code\"\n---\n\n# Concept\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if countRule(result.Warnings, "markdown-syntax") != 0 {
		t.Fatalf("expected no markdown syntax warnings from frontmatter values, got %#v", result.Warnings)
	}
}

func TestValidateWarnsForFrontmatterFormatting(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "concept.md", " ---\ntype: Concept\ntype: Duplicate\n--- \n\n# Concept\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected frontmatter formatting issues to be warnings, got errors %#v", result.Errors)
	}
	if statusForCheck(result, "Frontmatter formatting") != "warn" {
		t.Fatalf("expected frontmatter formatting check to warn, got %#v", result.Checks)
	}
	if countRule(result.Warnings, "frontmatter-format") != 3 {
		t.Fatalf("expected three frontmatter formatting warnings, got %#v", result.Warnings)
	}
}

func TestValidateErrorsForUnparseableFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "concept.md", "---\ntype:Concept\n---\n")

	result, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected one frontmatter parse error, got %#v", result.Errors)
	}
	if result.Errors[0].Rule != "frontmatter" || result.Errors[0].Line != 2 {
		t.Fatalf("unexpected frontmatter parse error: %#v", result.Errors[0])
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

	expectedCreated := []string{
		"index.md",
		"log.md",
		"AGENTS.md",
		"SETUP.MD",
		"SPEC.md",
	}
	if !reflect.DeepEqual(result.Created, expectedCreated) {
		t.Fatalf("unexpected created paths: %#v", result.Created)
	}

	for _, name := range []string{
		"index.md",
		"log.md",
		"AGENTS.md",
		"SETUP.MD",
		"SPEC.md",
	} {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
	for _, name := range []string{
		"concepts",
		"projects",
		"workflows",
		"skills",
		"automations",
		"references",
		"decisions",
		"wiki",
		"raw",
	} {
		if _, err := os.Stat(filepath.Join(target, name)); !os.IsNotExist(err) {
			t.Fatalf("expected optional scaffold path %s not to exist, got err=%v", name, err)
		}
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

func TestNewProjectCanSkipAgentAndSetupDocs(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "source-wiki")

	result, err := NewProject(NewProjectOptions{
		Name:           "Source Wiki",
		Path:           target,
		SkipAgentRules: true,
		SkipSetup:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SetupPath != "" {
		t.Fatalf("expected no setup path, got %s", result.SetupPath)
	}

	expectedCreated := []string{
		"index.md",
		"log.md",
		"SPEC.md",
	}
	if !reflect.DeepEqual(result.Created, expectedCreated) {
		t.Fatalf("unexpected created paths: %#v", result.Created)
	}

	for _, name := range []string{"AGENTS.md", "SETUP.MD"} {
		if _, err := os.Stat(filepath.Join(target, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s not to exist, got err=%v", name, err)
		}
	}

	content, err := os.ReadFile(filepath.Join(target, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	index := string(content)
	for _, unexpected := range []string{"AGENTS.md", "SETUP.MD"} {
		if strings.Contains(index, unexpected) {
			t.Fatalf("did not expect generated index to include %q:\n%s", unexpected, index)
		}
	}

	validation, err := Validate(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(validation.Errors) != 0 {
		t.Fatalf("expected generated project to validate, got %#v", validation.Errors)
	}
	if validation.Concepts != 1 {
		t.Fatalf("expected only SPEC.md to count as a concept, got %#v", validation)
	}
}

func TestNewProjectWritesOptionalBundleMetadata(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "accessibility")

	_, err := NewProject(NewProjectOptions{
		Name: "Accessibility Review",
		Path: target,
		BundleMetadata: BundleMetadata{
			Name:    "accessibility",
			Title:   "Accessibility Review",
			Purpose: "Accessibility review guidance for UI, HTML, ARIA, keyboard navigation, and design systems.",
			Tags:    []string{"accessibility", "ui", "review", "ui"},
			Entries: []BundleEntry{
				{Name: "review", Path: "agents/accessibility-review.md"},
				{Name: "default", Path: "agents/accessibility-checker.md"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(target, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	index := string(content)
	required := []string{
		`okf_bundle_name: "accessibility"`,
		`okf_bundle_title: "Accessibility Review"`,
		`okf_bundle_purpose: "Accessibility review guidance for UI, HTML, ARIA, keyboard navigation, and design systems."`,
		`okf_bundle_tags: ["accessibility", "ui", "review"]`,
		`okf_bundle_entry_default: "agents/accessibility-checker.md"`,
		`okf_bundle_entry_review: "agents/accessibility-review.md"`,
	}
	for _, expected := range required {
		if !strings.Contains(index, expected) {
			t.Fatalf("expected generated index.md to include %q:\n%s", expected, index)
		}
	}

	validation, err := Validate(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(validation.Errors) != 0 {
		t.Fatalf("expected generated project with bundle metadata to validate, got %#v", validation.Errors)
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

func countRule(issues []Issue, rule string) int {
	count := 0
	for _, issue := range issues {
		if issue.Rule == rule {
			count++
		}
	}
	return count
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
