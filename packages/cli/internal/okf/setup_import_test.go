package okf

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverMarkdownAcceptsPlainFilesAndRespectsGitIgnore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Product\n\nUseful knowledge.\n")
	writeFile(t, root, "docs/runbook.md", "# Runbook\n\nDeploy safely.\n")
	writeFile(t, root, "ignored/private.md", "# Private\n")
	writeFile(t, root, "generated/automatic.md", "# Generated\n")
	writeFile(t, root, ".gitignore", "ignored/\n")
	command := exec.Command("git", "init", "--quiet")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.Symlink(filepath.Join(root, "README.md"), filepath.Join(root, "linked.md")); err != nil {
		t.Fatal(err)
	}

	discovery, err := DiscoverMarkdown(root, MarkdownDiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.Documents) != 2 {
		t.Fatalf("documents=%#v", discovery.Documents)
	}
	if discovery.Documents[0].Path != "README.md" || discovery.Documents[0].Title != "Product" || discovery.Documents[0].HasFrontmatter {
		t.Fatalf("unexpected root document: %#v", discovery.Documents[0])
	}
	if discovery.Documents[1].Path != "docs/runbook.md" {
		t.Fatalf("unexpected nested document: %#v", discovery.Documents[1])
	}
	if len(discovery.Warnings) != 1 || !strings.Contains(discovery.Warnings[0], "linked.md") {
		t.Fatalf("warnings=%#v", discovery.Warnings)
	}
}

func TestDiscoverMarkdownRespectsParentRepositoryGitIgnore(t *testing.T) {
	repository := t.TempDir()
	docs := filepath.Join(repository, "docs")
	writeFile(t, docs, "guide.md", "# Guide\n")
	writeFile(t, docs, "generated/private.md", "# Generated\n")
	writeFile(t, repository, ".gitignore", "docs/generated/\n")
	command := exec.Command("git", "init", "--quiet")
	command.Dir = repository
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}

	discovery, err := DiscoverMarkdown(docs, MarkdownDiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.Documents) != 1 || discovery.Documents[0].Path != "guide.md" {
		t.Fatalf("documents=%#v", discovery.Documents)
	}
}

func TestDiscoverMarkdownAcceptsAbsoluteSelectionInsideRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Root\n")
	writeFile(t, root, "docs/guide.md", "# Guide\n")
	discovery, err := DiscoverMarkdown(root, MarkdownDiscoveryOptions{Include: []string{filepath.Join(root, "docs")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.Documents) != 1 || discovery.Documents[0].Path != "docs/guide.md" {
		t.Fatalf("documents=%#v", discovery.Documents)
	}
	if _, err := DiscoverMarkdown(root, MarkdownDiscoveryOptions{Include: []string{filepath.Dir(root)}}); err == nil || !strings.Contains(err.Error(), "escapes discovery root") {
		t.Fatalf("err=%v", err)
	}
}

func TestSetupImportCopyCreatesValidSearchableBundleWithoutChangingSources(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "Wiki")
	writeFile(t, source, "README.md", "# Product Guide\n\nDeploy with the release workflow.\n")
	writeFile(t, source, "docs/runbook.md", "---\ntitle: Deploy Runbook\n---\n\n# Deploy\n\nRollback safely.\n")
	original, err := os.ReadFile(filepath.Join(source, "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	plan, err := BuildSetupImportPlan(SetupImportOptions{
		Mode: SetupImportCopy, Source: source, Target: target, Name: "Product", SpecVersion: "latest", Rules: []string{"project", "writing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Documents != 2 || plan.Summary.Move != 0 || plan.Summary.Delete != 0 || plan.Summary.AddFrontmatter != 1 || plan.Summary.CompleteFrontmatter != 1 {
		t.Fatalf("summary=%#v", plan.Summary)
	}
	result, err := ApplySetupImportPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Documents != 2 || result.SearchHits == 0 {
		t.Fatalf("result=%#v", result)
	}
	after, err := os.ReadFile(filepath.Join(source, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("managed copy changed its source")
	}
	imported, err := os.ReadFile(filepath.Join(target, "imported", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(imported), "---\ntype: Document\ntitle: \"Product Guide\"") {
		t.Fatalf("unexpected imported content:\n%s", imported)
	}
	if _, err := os.Stat(filepath.Join(target, ".openknowledge", "import.json")); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(target, ".openknowledge", "import.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `"sourceSha256"`) || !strings.Contains(string(manifest), `"targetSha256"`) || strings.Contains(string(manifest), `"include"`) {
		t.Fatalf("unexpected provenance manifest:\n%s", manifest)
	}
}

func TestSetupImportCopyPreservesValidExistingFrontmatterExactly(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "Wiki")
	original := "---\ntype: Guide\ntitle: Deployment\nowner: team:platform\n---\n\n# Deployment\n\nRelease safely.\n"
	writeFile(t, source, "guide.md", original)
	plan, err := BuildSetupImportPlan(SetupImportOptions{Mode: SetupImportCopy, Source: source, Target: target, SpecVersion: "latest", Rules: []string{"project", "writing"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Preserve != 1 {
		t.Fatalf("summary=%#v", plan.Summary)
	}
	if _, err := ApplySetupImportPlan(plan); err != nil {
		t.Fatal(err)
	}
	imported, err := os.ReadFile(filepath.Join(target, "imported", "guide.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(imported) != original {
		t.Fatalf("valid frontmatter changed:\n%s", imported)
	}
}

func TestSetupImportInPlaceAddsMetadataWithoutMovingFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "guide.md", "# Guide\n\nLocal knowledge.\n")
	plan, err := BuildSetupImportPlan(SetupImportOptions{
		Mode: SetupImportInPlace, Source: root, Target: root, Name: "Docs", SpecVersion: "latest", Rules: []string{"project", "writing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Update != 1 || plan.Summary.Move != 0 || plan.Summary.Delete != 0 {
		t.Fatalf("summary=%#v", plan.Summary)
	}
	if _, err := ApplySetupImportPlan(plan); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "guide.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(content), "---\ntype: Document\ntitle: \"Guide\"") {
		t.Fatalf("unexpected adopted content:\n%s", content)
	}
	if _, err := os.Stat(filepath.Join(root, "index.md")); err != nil {
		t.Fatal(err)
	}
}

func TestSetupImportInPlaceAdoptsExistingRootIndexWithoutReplacingItsBody(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Existing documentation\n\nKeep this introduction.\n")
	writeFile(t, root, "guide.md", "# Guide\n\nLocal knowledge.\n")
	plan, err := BuildSetupImportPlan(SetupImportOptions{
		Mode: SetupImportInPlace, Source: root, Target: root, Name: "Docs", SpecVersion: "latest", Rules: []string{"project", "writing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplySetupImportPlan(plan); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `okf_version: "`+LatestSpecVersion+`"`) || !strings.Contains(string(content), "Keep this introduction.") {
		t.Fatalf("unexpected adopted index:\n%s", content)
	}
}

func TestSetupImportRejectsMalformedOrChangedInput(t *testing.T) {
	t.Run("malformed frontmatter", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "bad.md", "---\ntype: [broken\n---\n\n# Bad\n")
		_, err := BuildSetupImportPlan(SetupImportOptions{Mode: SetupImportInPlace, Source: root, Target: root, SpecVersion: "latest"})
		if err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("changed after plan", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "guide.md", "# Guide\n")
		plan, err := BuildSetupImportPlan(SetupImportOptions{Mode: SetupImportInPlace, Source: root, Target: root, SpecVersion: "latest"})
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, "guide.md", "# Changed\n")
		_, err = ApplySetupImportPlan(plan)
		if err == nil || !strings.Contains(err.Error(), "changed after planning") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("copy source changed after plan", func(t *testing.T) {
		root := t.TempDir()
		source := filepath.Join(root, "source")
		target := filepath.Join(root, "Wiki")
		writeFile(t, source, "guide.md", "# Guide\n")
		plan, err := BuildSetupImportPlan(SetupImportOptions{Mode: SetupImportCopy, Source: source, Target: target, SpecVersion: "latest"})
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, source, "guide.md", "# Changed\n")
		_, err = ApplySetupImportPlan(plan)
		if err == nil || !strings.Contains(err.Error(), "source changed after planning") {
			t.Fatalf("err=%v", err)
		}
		if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
			t.Fatalf("failed preflight wrote target: %v", statErr)
		}
	})
}

func TestSetupImportInPlaceRejectsPartialCorpusSelection(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "one.md", "# One\n")
	writeFile(t, root, "two.md", "# Two\n")
	_, err := BuildSetupImportPlan(SetupImportOptions{Mode: SetupImportInPlace, Source: root, Target: root, SpecVersion: "latest", Include: []string{"one.md"}})
	if err == nil || !strings.Contains(err.Error(), "one complete directory") {
		t.Fatalf("err=%v", err)
	}
}
