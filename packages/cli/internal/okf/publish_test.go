package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func enablePublicArtifactTest(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ValidationConfigFile), []byte("[publish]\nenabled = true\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestPublicArtifactsRequireExplicitBundleEnable(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n")

	if _, err := BuildPublicationSetWithVersion(root, "0.1"); err == nil || !strings.Contains(err.Error(), "[publish] enabled = true") {
		t.Fatalf("expected disabled-by-default publication refusal, got %v", err)
	}
}

func TestPublicationTargetsDefaultTrueAndFilterIndependently(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, root)
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "default.md", "---\ntype: Guide\n---\n\n# Default\n")
	writeFile(t, root, "controlled.md", "---\ntype: Guide\nokf_publish: true\nokf_targets:\n  viewer: true\n  search: false\n  mcp: false\n  llms: false\n  sitemap: false\n---\n\n# Controlled\n")
	writeFile(t, root, "private.md", "---\ntype: Guide\nokf_publish: false\nokf_targets:\n  viewer: true\n  search: true\n  mcp: true\n  llms: true\n  sitemap: true\n---\n\n# Private\n")

	all, err := BuildPublicationSetWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(all.Markdown, ",") != "controlled.md,default.md,index.md" {
		t.Fatalf("unexpected public source set: %#v", all.Markdown)
	}
	viewer, err := BuildPublicationSetForTargetWithVersion(root, "0.1", PublicationTargetViewer)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(viewer.Markdown, ",") != "controlled.md,default.md,index.md" {
		t.Fatalf("unexpected viewer set: %#v", viewer.Markdown)
	}
	for _, target := range []PublicationTarget{PublicationTargetSearch, PublicationTargetMCP, PublicationTargetLLMS, PublicationTargetSitemap} {
		set, err := BuildPublicationSetForTargetWithVersion(root, "0.1", target)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(set.Markdown, ",") != "default.md,index.md" {
			t.Fatalf("unexpected %s set: %#v", target, set.Markdown)
		}
	}
}

func TestPublicationTargetsFailClosedOnUnknownOrNonBooleanValues(t *testing.T) {
	for _, frontmatter := range []string{
		"okf_targets:\n  typo: true\n",
		"okf_targets:\n  search: yes\n",
		"okf_targets: false\n",
		"okf_publish: yes\n",
	} {
		root := t.TempDir()
		enablePublicArtifactTest(t, root)
		writeFile(t, root, "index.md", "# Home\n")
		writeFile(t, root, "bad.md", "---\ntype: Guide\n"+frontmatter+"---\n\n# Bad\n")
		if _, err := BuildPublicationSetForTargetWithVersion(root, "0.1", PublicationTargetSearch); err == nil {
			t.Fatalf("expected malformed publication metadata refusal for %q", frontmatter)
		}
		result, err := ValidateWithVersion(root, "0.1")
		if err != nil {
			t.Fatal(err)
		}
		if countRule(result.Errors, "publish-metadata") == 0 {
			t.Fatalf("expected publish-metadata validation error for %q: %#v", frontmatter, result.Errors)
		}
	}
}
