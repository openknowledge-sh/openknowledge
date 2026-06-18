package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBundleIncludesContentLinksAndIssues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n\nSee [Setup](guides/setup.md), [Missing](missing.md), [Top](#home), and [External](https://example.com).\n")
	writeFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup Guide\ndescription: How to set up the bundle.\nresource: file://setup\n---\n\n# Setup\n\nRun `openknowledge validate`.\n")

	bundle, err := ParseBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.SpecVersion != LatestSpecVersion {
		t.Fatalf("unexpected spec version: %s", bundle.SpecVersion)
	}
	if len(bundle.Files) != 2 {
		t.Fatalf("expected two files, got %#v", bundle.Files)
	}
	if len(bundle.Issues) != 1 || bundle.Issues[0].Rule != "link-target" {
		t.Fatalf("expected broken link warning in bundle issues, got %#v", bundle.Issues)
	}

	index := bundleFileByPath(t, bundle, "index.md")
	if strings.Contains(index.Body, "okf_version") {
		t.Fatalf("expected frontmatter to be stripped from body: %q", index.Body)
	}
	if len(index.Links) != 4 {
		t.Fatalf("expected four links, got %#v", index.Links)
	}
	if index.Links[0].Kind != "local" || index.Links[0].TargetID != "guides/setup" || !index.Links[0].Exists {
		t.Fatalf("unexpected resolved local link: %#v", index.Links[0])
	}
	if index.Links[1].Kind != "local" || index.Links[1].TargetPath != "missing.md" || index.Links[1].Exists {
		t.Fatalf("unexpected missing local link: %#v", index.Links[1])
	}
	if index.Links[2].Kind != "anchor" || index.Links[3].Kind != "external" {
		t.Fatalf("unexpected non-local links: %#v", index.Links)
	}

	setup := bundleFileByPath(t, bundle, "guides/setup.md")
	if setup.Type != "Guide" || setup.Title != "Setup Guide" || setup.Description == "" || setup.Resource == "" {
		t.Fatalf("expected concept metadata, got %#v", setup)
	}
	if setup.Frontmatter["type"] != "Guide" {
		t.Fatalf("expected frontmatter values in JSON model, got %#v", setup.Frontmatter)
	}
}

func TestWriteHTMLRendersPagesAndRewritesMarkdownLinks(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeFile(t, root, "index.md", "# Home\n\nRead [Setup](guides/setup.md).\n")
	writeFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup\n---\n\n# Setup\n\nBack to [Home](../index.md).\n")

	result, err := WriteHTML(root, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Written) != 2 {
		t.Fatalf("expected two written files, got %#v", result.Written)
	}

	index := readExportFile(t, out, "index.html")
	if !strings.Contains(index, "<h1>Home</h1>") {
		t.Fatalf("expected rendered index heading:\n%s", index)
	}
	if !strings.Contains(index, `href="guides/setup.html"`) {
		t.Fatalf("expected markdown link rewritten in index:\n%s", index)
	}

	setup := readExportFile(t, out, "guides/setup.html")
	if strings.Contains(setup, "type: Guide") {
		t.Fatalf("expected frontmatter to be stripped from HTML:\n%s", setup)
	}
	if !strings.Contains(setup, "<h1>Setup</h1>") {
		t.Fatalf("expected rendered setup heading:\n%s", setup)
	}
	if !strings.Contains(setup, `href="../index.html"`) {
		t.Fatalf("expected parent markdown link rewritten in nested page:\n%s", setup)
	}
}

func bundleFileByPath(t *testing.T, bundle Bundle, path string) BundleFile {
	t.Helper()
	for _, file := range bundle.Files {
		if file.Path == path {
			return file
		}
	}
	t.Fatalf("missing bundle file %s in %#v", path, bundle.Files)
	return BundleFile{}
}

func readExportFile(t *testing.T, root string, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
