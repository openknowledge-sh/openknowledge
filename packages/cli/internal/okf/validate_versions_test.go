package okf

import (
	"path/filepath"
	"testing"
)

func TestValidateConformanceBySpecVersion(t *testing.T) {
	for _, version := range SupportedSpecVersions() {
		t.Run(version+"/valid_bundle", func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "index.md", "---\nokf_version: \""+version+"\"\n---\n\n# Bundle\n")
			writeFile(t, root, "log.md", "# Log\n\n## 2026-06-15\n\n* **Creation**: Created bundle.\n")
			writeFile(t, root, "concepts/table.md", "---\ntype: BigQuery Table\ntitle: Orders\n---\n\n# Schema\n")

			result, err := ValidateWithVersion(root, version)
			if err != nil {
				t.Fatal(err)
			}
			if result.SpecVersion != version {
				t.Fatalf("expected spec version %s, got %s", version, result.SpecVersion)
			}
			if len(result.Errors) != 0 {
				t.Fatalf("expected no errors, got %#v", result.Errors)
			}
			if statusForCheck(result, "Bundle scan") != "pass" {
				t.Fatalf("expected bundle scan to pass, got %#v", result.Checks)
			}
		})

		t.Run(version+"/scanner_includes_uppercase_markdown", func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "SETUP.MD", "---\ntype: Setup\ntitle: Setup\n---\n")

			result, err := ValidateWithVersion(root, version)
			if err != nil {
				t.Fatal(err)
			}
			if result.Concepts != 1 || result.Files != 1 {
				t.Fatalf("expected uppercase Markdown file to be scanned, got %#v", result)
			}
		})

		t.Run(version+"/missing_concept_type_fails", func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "concept.md", "---\ntitle: Missing Type\n---\n")

			result, err := ValidateWithVersion(root, version)
			if err != nil {
				t.Fatal(err)
			}
			if statusForCheck(result, "Concept documents") != "fail" {
				t.Fatalf("expected concept documents check to fail, got %#v", result.Checks)
			}
		})

		t.Run(version+"/reserved_files_fail", func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "docs/index.md", "---\ntype: Index\n---\n# Docs\n")
			writeFile(t, root, "log.md", "# Log\n\n## June 15 2026\n\n* Bad date.\n")

			result, err := ValidateWithVersion(root, version)
			if err != nil {
				t.Fatal(err)
			}
			if statusForCheck(result, "Reserved files") != "fail" || statusForCheck(result, "Log dates") != "fail" {
				t.Fatalf("expected reserved file and log date checks to fail, got %#v", result.Checks)
			}
		})

		t.Run(version+"/generated_scaffold_validates", func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "knowledge")

			_, err := NewProject(NewProjectOptions{Name: "Knowledge", Path: target})
			if err != nil {
				t.Fatal(err)
			}
			result, err := ValidateWithVersion(target, version)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Errors) != 0 {
				t.Fatalf("expected scaffold to validate against %s, got %#v", version, result.Errors)
			}
		})
	}
}

func TestValidateRejectsUnsupportedSpecVersion(t *testing.T) {
	_, err := ValidateWithVersion(t.TempDir(), "9.9")
	if err == nil {
		t.Fatal("expected unsupported spec version error")
	}
}
