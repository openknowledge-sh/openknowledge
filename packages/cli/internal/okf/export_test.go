package okf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseBundleIncludesContentLinksAndIssues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n\nSee [Setup](guides/setup.md), [Missing](missing.md), [Top](#home), and [External](https://openknowledge.sh).\n")
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

func TestParseBundleTrimsMarkdownExtensionIDs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "guide.markdown", "---\ntype: Guide\ntitle: Guide\n---\n\n# Guide\n")

	bundle, err := ParseBundle(root)
	if err != nil {
		t.Fatal(err)
	}

	guide := bundleFileByPath(t, bundle, "guide.markdown")
	if guide.ID != "guide" {
		t.Fatalf("expected .markdown ID to trim extension, got %q", guide.ID)
	}
}

func TestParseBundlePreservesTypedFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\ndescription: |-\n  First line.\n  Second line.\nconfig: {mode: fast, retries: 2}\ntags: [docs, cli]\nenabled: false\n---\n\n# Guide\n")

	bundle, err := ParseBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	guide := bundleFileByPath(t, bundle, "guide.md")
	if guide.Description != "First line.\nSecond line." {
		t.Fatalf("expected decoded block-scalar metadata, got %q", guide.Description)
	}
	config, ok := guide.Frontmatter["config"].(map[string]any)
	if !ok || config["mode"] != "fast" || config["retries"] != 2 {
		t.Fatalf("expected typed nested frontmatter, got %#v", guide.Frontmatter)
	}
	tags, ok := guide.Frontmatter["tags"].([]any)
	if !ok || !reflect.DeepEqual(tags, []any{"docs", "cli"}) || guide.Frontmatter["enabled"] != false {
		t.Fatalf("expected typed sequence and boolean, got %#v", guide.Frontmatter)
	}

	payload, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"config":{"mode":"fast","retries":2}`, `"tags":["docs","cli"]`, `"enabled":false`} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("normalized JSON must preserve typed frontmatter %s: %s", expected, payload)
		}
	}
}

func TestLinksFromASTMarkdownMarksDirectoryIndexLinksExisting(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "guides/index.md", "# Guides\n")

	markdown := ParseASTMarkdown("[Guides](guides) and [Guides index](guides/).\n", 1)
	links := LinksFromASTMarkdown(root, "index.md", markdown)
	if len(links) != 2 {
		t.Fatalf("expected two links, got %#v", links)
	}
	if links[0].TargetPath != "guides" || !links[0].Exists {
		t.Fatalf("expected directory link to resolve through index.md, got %#v", links[0])
	}
	if links[1].TargetPath != "guides/index.md" || !links[1].Exists {
		t.Fatalf("expected trailing-slash directory link to resolve to index.md, got %#v", links[1])
	}
}

func TestBuildGraphUsesASTBackedLocalLinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead [Guide](guides/setup.md), [Missing](missing.md), and [Self](index.md).\n\n```md\n[Code](guides/setup.md)\n```\n")
	writeFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup\n---\n\n# Setup\n\nBack to [Home](../index.md).\n")

	graph, err := BuildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if graph.SpecVersion != LatestSpecVersion || len(graph.Nodes) != 2 {
		t.Fatalf("unexpected graph metadata or nodes: %#v", graph)
	}
	if len(graph.Edges) != 2 {
		t.Fatalf("expected two existing non-self local edges, got %#v", graph.Edges)
	}
	if graph.Edges[0].Source != "guides/setup.md" || graph.Edges[0].Target != "index.md" {
		t.Fatalf("expected sorted setup-to-index edge first, got %#v", graph.Edges)
	}
	if graph.Edges[1].Source != "index.md" || graph.Edges[1].Target != "guides/setup.md" || graph.Edges[1].Label != "Guide" {
		t.Fatalf("expected index-to-guide edge from Markdown AST, got %#v", graph.Edges)
	}
	if strings.Contains(graph.Edges[1].Href, "Code") {
		t.Fatalf("expected graph links to ignore fenced-code links, got %#v", graph.Edges)
	}
}

func TestBuildSearchGraphIncludesSectionChunks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead [Guide](guides/setup.md).\n")
	writeFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup\n---\n\n# Setup\n\nPrepare the bundle.\n\n## Validate\n\nRun `openknowledge validate`.\n")

	graph, err := BuildGraphWithType(root, LatestSpecVersion, GraphTypeSearch)
	if err != nil {
		t.Fatal(err)
	}
	if graph.Type != GraphTypeSearch {
		t.Fatalf("expected search graph type, got %#v", graph)
	}
	var setupChunk GraphNode
	for _, node := range graph.Nodes {
		if node.ID == "guides/setup#validate" {
			setupChunk = node
			break
		}
	}
	if setupChunk.ID == "" || setupChunk.Kind != "chunk" || setupChunk.Path != "guides/setup.md" || setupChunk.Heading != "Validate" {
		t.Fatalf("expected validate chunk node, got %#v in graph %#v", setupChunk, graph)
	}

	kinds := map[string]bool{}
	for _, edge := range graph.Edges {
		kinds[edge.Kind] = true
	}
	for _, expected := range []string{"contains", "local-link", "next"} {
		if !kinds[expected] {
			t.Fatalf("expected %s edge in search graph, got %#v", expected, graph.Edges)
		}
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

func TestWriteHTMLFromASTRendersParsedMarkdownTree(t *testing.T) {
	out := filepath.Join(t.TempDir(), "site")
	ast := ASTBundle{
		Root:        t.TempDir(),
		SpecVersion: LatestSpecVersion,
		Documents: []ASTDocument{{
			Rel:  "index.md",
			ID:   "index",
			Kind: "index",
			Body: "# Raw Body\n",
			Markdown: ASTMarkdown{
				Blocks: []ASTMarkdownBlock{{
					Kind:      "paragraph",
					LineStart: 1,
					LineEnd:   1,
					Text:      "Parsed **tree** body.",
				}},
			},
		}},
	}

	if _, err := WriteHTMLFromAST(ast, out, staticPageTemplate); err != nil {
		t.Fatal(err)
	}
	index := readExportFile(t, out, "index.html")
	if !strings.Contains(index, "<p>Parsed <strong>tree</strong> body.</p>") {
		t.Fatalf("expected HTML to render parsed Markdown tree:\n%s", index)
	}
	if strings.Contains(index, "<h1>Raw Body</h1>") {
		t.Fatalf("expected HTML not to render raw body:\n%s", index)
	}
}

func TestWritePlainHTMLRendersUnstyledPages(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "plain-site")
	writeFile(t, root, "index.md", "# Home\n\nRead [Setup](guides/setup.md).\n")
	writeFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup\n---\n\n# Setup\n\nBack to [Home](../index.md).\n")

	result, err := WritePlainHTML(root, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Written) != 2 {
		t.Fatalf("expected two written files, got %#v", result.Written)
	}

	index := readExportFile(t, out, "index.html")
	if !strings.Contains(index, "<h1>Home</h1>") || !strings.Contains(index, `href="guides/setup.html"`) {
		t.Fatalf("expected plain export to render markdown with rewritten links:\n%s", index)
	}
	for _, forbidden := range []string{"<style", "<script", "class=", "data-note-workspace", "<header", "Open Knowledge</a>"} {
		if strings.Contains(index, forbidden) {
			t.Fatalf("plain export should not include %q:\n%s", forbidden, index)
		}
	}

	setup := readExportFile(t, out, "guides/setup.html")
	if !strings.Contains(setup, "<title>Setup</title>") || !strings.Contains(setup, `href="../index.html"`) {
		t.Fatalf("expected nested plain export to keep title and relative links:\n%s", setup)
	}
}

func TestWriteHTMLSkipsUnpublishedPages(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeFile(t, root, "index.md", "# Home\n\nRead [Public](public.md) and [Draft](draft.md).\n")
	writeFile(t, root, "public.md", "---\ntype: Guide\n---\n\n# Public\n")
	writeFile(t, root, "draft.md", "---\ntype: Draft\nokf_publish: false\n---\n\n# Draft\n")
	writeFile(t, root, "examples/index.md", "---\nokf_publish: false\n---\n\n# Examples\n")

	result, err := WriteHTML(root, out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Written, ",") != "index.html,public.html" {
		t.Fatalf("expected only published files, got %#v", result.Written)
	}
	if _, err := os.Stat(filepath.Join(out, "draft.html")); !os.IsNotExist(err) {
		t.Fatalf("expected draft.html to be absent, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "examples", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected examples/index.html to be absent, got err=%v", err)
	}
}

func TestWriteHTMLRendersBlockquotesAndStrongText(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeFile(t, root, "index.md", "# Home\n\n> This is a pinned upstream copy.\n> It is unofficial tooling.\n\n**Version 0.1 - Draft**\n\nUse *standard markdown*.\n\n---\n\n1. First\n2. Second\n")

	if _, err := WriteHTML(root, out); err != nil {
		t.Fatal(err)
	}

	index := readExportFile(t, out, "index.html")
	if !strings.Contains(index, "<blockquote>") || strings.Contains(index, "&gt; This is a pinned upstream copy") {
		t.Fatalf("expected markdown blockquote to render as blockquote:\n%s", index)
	}
	if !strings.Contains(index, "<strong>Version 0.1 - Draft</strong>") || strings.Contains(index, "**Version") {
		t.Fatalf("expected strong markdown to render as strong text:\n%s", index)
	}
	if !strings.Contains(index, "<em>standard markdown</em>") || strings.Contains(index, "*standard markdown*") {
		t.Fatalf("expected emphasis markdown to render as em text:\n%s", index)
	}
	if !strings.Contains(index, "<hr>") || strings.Contains(index, "<p>---</p>") {
		t.Fatalf("expected thematic break markdown to render as hr:\n%s", index)
	}
	if !strings.Contains(index, "<ol>") || !strings.Contains(index, "<li>First</li>") || strings.Contains(index, "<p>1. First") {
		t.Fatalf("expected ordered list markdown to render as ol:\n%s", index)
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
