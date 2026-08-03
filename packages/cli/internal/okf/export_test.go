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
	writeFile(t, root, "index.md", "---\nokf_version: \""+LatestSpecVersion+"\"\n---\n\n# Home\n\nSee [Setup](guides/setup.md), [Missing](missing.md), [Top](#home), and [External](https://openknowledge.sh).\n")
	writeFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup Guide\ndescription: How to set up the bundle.\nresource: file://setup\n---\n\n# Setup\n\nRun `openknowledge validate`.\n")

	bundle, err := ParseBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.SpecVersion != LatestSpecVersion {
		t.Fatalf("unexpected spec version: %s", bundle.SpecVersion)
	}
	if bundle.SchemaVersion != MachineSchemaVersion {
		t.Fatalf("unexpected machine schema version: %s", bundle.SchemaVersion)
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

func TestPlainHTMLRejectsInvalidBundleBeforeWriting(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, root)
	out := filepath.Join(t.TempDir(), "site")
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "invalid.md", "# Missing required concept frontmatter\n")
	writeFile(t, out, "sentinel.txt", "previous generation\n")

	if _, err := WritePlainHTMLWithVersion(root, out, "0.1"); err == nil || !strings.Contains(err.Error(), "bundle validation failed") {
		t.Fatalf("expected invalid plain HTML refusal, got %v", err)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "sentinel.txt" {
		t.Fatalf("invalid plain export must not write partial output: %#v", entries)
	}
}

func TestPlainHTMLReplacesWholeGeneration(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, root)
	out := filepath.Join(t.TempDir(), "site")
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "old.md", "---\ntype: Note\n---\n\n# Old\n")
	if _, err := WritePlainHTMLWithVersion(root, out, "0.1"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "old.md")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "new.md", "---\ntype: Note\n---\n\n# New\n")
	if _, err := WritePlainHTMLWithVersion(root, out, "0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "new.html")); err != nil {
		t.Fatalf("expected new plain generation page: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "old.html")); !os.IsNotExist(err) {
		t.Fatalf("expected stale plain page to be removed, got %v", err)
	}
}

func TestPlainHTMLRejectsOutputContainingSource(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "bundle")
	writeFile(t, root, "index.md", "# Home\n")
	enablePublicArtifactTest(t, root)

	for _, out := range []string{root, parent} {
		if _, err := WritePlainHTMLWithVersion(root, out, "0.1"); err == nil || !strings.Contains(err.Error(), "must not contain the source bundle") {
			t.Fatalf("expected unsafe output %s to be rejected, got %v", out, err)
		}
	}
	if content, err := os.ReadFile(filepath.Join(root, "index.md")); err != nil || string(content) != "# Home\n" {
		t.Fatalf("unsafe export must preserve the source bundle, content=%q err=%v", content, err)
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

func TestLinksFromASTMarkdownPreservesTargetAnchors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n")

	markdown := ParseASTMarkdown("[Encoded](guide.md#release%20notes) and [Local](#home).\n", 1)
	links := LinksFromASTMarkdown(root, "index.md", markdown)
	if len(links) != 2 {
		t.Fatalf("expected two links, got %#v", links)
	}
	if links[0].Kind != "local" || links[0].TargetPath != "guide.md" || links[0].TargetAnchor != "release notes" || !links[0].Exists {
		t.Fatalf("expected decoded cross-file target anchor, got %#v", links[0])
	}
	if links[1].Kind != "anchor" || links[1].TargetPath != "index.md" || links[1].TargetID != "index" || links[1].TargetAnchor != "home" || !links[1].Exists {
		t.Fatalf("expected same-document anchor target, got %#v", links[1])
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

func TestOKFV02ListAndGraphSurfaceTrustAndProvenance(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Bundle\n")
	writeFile(t, root, "revenue.md", `---
type: Attested Computation
title: Revenue
runtime: python3
parameters:
  - { name: year, type: integer, required: true }
computation: https://example.test/revenue.py
executor: { resource: https://example.test/executor, receipt: [stdout, sha256] }
attester: { resource: https://example.test/attester }
verified: { by: human:reviewer, at: 2026-08-03T09:00:00Z }
status: stable
stale_after: 2026-08-04
sources:
  - id: policy
    resource: https://example.test/policy
    title: Revenue policy
---

# Revenue

Supported by policy.[^policy]

[^policy]: Revenue policy
`)

	listing, err := ListWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	var revenue ListEntry
	for _, entry := range listing.Entries {
		if entry.Path == "revenue.md" {
			revenue = entry
			break
		}
	}
	if revenue.OKF02 == nil || revenue.OKF02.TrustTier != OKFV02TrustHumanReviewed || !revenue.OKF02.Stale || revenue.OKF02.Computation == nil {
		t.Fatalf("expected dedicated OKF 0.2 list signals, got %#v", revenue)
	}

	graph, err := BuildGraphWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	var conceptSignals *OKFV02Signals
	kinds := map[string]bool{}
	resourceNodes := 0
	for _, node := range graph.Nodes {
		if node.Path == "revenue.md" {
			conceptSignals = node.OKF02
		}
		if node.Kind == "resource" {
			resourceNodes++
		}
	}
	for _, edge := range graph.Edges {
		if edge.Source == "revenue.md" {
			kinds[edge.Kind] = true
		}
	}
	if conceptSignals == nil || conceptSignals.TrustTier != OKFV02TrustHumanReviewed {
		t.Fatalf("expected graph concept signals, got %#v", conceptSignals)
	}
	for _, kind := range []string{"source", "computation", "executor", "attester"} {
		if !kinds[kind] {
			t.Fatalf("expected %s provenance edge, got %#v", kind, graph.Edges)
		}
	}
	if resourceNodes != 4 {
		t.Fatalf("expected four external resource nodes, got %d in %#v", resourceNodes, graph.Nodes)
	}
}

func TestBuildSourceGraphPreservesParallelLinkOccurrences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead [Setup](guide.md#setup).\n\nRead [Recovery](guide.md#recovery).\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n\n## Setup\n\nPrepare.\n\n## Recovery\n\nRestore.\n")

	graph, err := BuildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Edges) != 2 {
		t.Fatalf("expected both authored links to remain separate, got %#v", graph.Edges)
	}
	if graph.Edges[0].Label != "Setup" || graph.Edges[0].TargetAnchor != "setup" || graph.Edges[1].Label != "Recovery" || graph.Edges[1].TargetAnchor != "recovery" || graph.Edges[0].Line == graph.Edges[1].Line {
		t.Fatalf("expected occurrence metadata on parallel edges, got %#v", graph.Edges)
	}
}

func TestBuildSourceGraphResolvesDirectoryLinksToIndex(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead [Guides](guides).\n")
	writeFile(t, root, "guides/index.md", "# Guides\n")

	graph, err := BuildGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Edges) != 1 || graph.Edges[0].Source != "index.md" || graph.Edges[0].Target != "guides/index.md" {
		t.Fatalf("expected directory link to target its index node, got %#v", graph.Edges)
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

func TestBuildSearchGraphResolvesFragmentsToOwningChunks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead [Rollback](guide.md#rollback), [Parent](guide.md#operations), and [Missing](guide.md#missing).\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n\nIntroduction.\n\n## Operations\n\n### Recovery\n\nRecovery overview. Read [Guide](#guide).\n\n#### Rollback\n\nRestore the release.\n")

	graph, err := BuildGraphWithType(root, LatestSpecVersion, GraphTypeSearch)
	if err != nil {
		t.Fatal(err)
	}
	var anchored []GraphEdge
	for _, edge := range graph.Edges {
		if edge.Kind == "local-link" && edge.Source == "index#home" {
			anchored = append(anchored, edge)
		}
	}
	if len(anchored) != 2 {
		t.Fatalf("expected resolved rollback and parent edges but no missing-fragment fallback, got %#v", anchored)
	}
	for _, edge := range anchored {
		if edge.Target != "guide#recovery" || (edge.TargetAnchor != "rollback" && edge.TargetAnchor != "operations") {
			t.Fatalf("expected lower-level and heading-only anchors to select the recovery chunk, got %#v", anchored)
		}
	}
	var sameDocument bool
	for _, edge := range graph.Edges {
		if edge.Kind == "local-link" && edge.Source == "guide#recovery" && edge.Target == "guide#guide" && edge.TargetAnchor == "guide" {
			sameDocument = true
			break
		}
	}
	if !sameDocument {
		t.Fatalf("expected a pure fragment to create an exact same-document chunk edge, got %#v", graph.Edges)
	}
}

func TestWriteHTMLRendersPagesAndRewritesMarkdownLinks(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, root)
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
	enablePublicArtifactTest(t, root)
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

func TestWritePlainHTMLOKFV02RendersMetadataAndSourceReferences(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, root)
	out := filepath.Join(t.TempDir(), "plain-site")
	writeFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n")
	writeFile(t, root, "policy.md", `---
type: Policy
title: Policy
sources:
  - id: statute
    resource: https://example.test/statute
    title: Statute
---

# Policy

Follow the statute.[^statute]

[^statute]: Statute source
`)

	if _, err := WritePlainHTMLWithVersion(root, out, "0.2"); err != nil {
		t.Fatal(err)
	}
	page := readExportFile(t, out, "policy.html")
	for _, expected := range []string{
		"<summary>Metadata</summary>",
		`id="ok-source-statute"`,
		`href="https://example.test/statute"`,
		`href="#ok-source-statute"`,
	} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected %q in OKF 0.2 plain HTML:\n%s", expected, page)
		}
	}
	if strings.Contains(page, "Statute source") {
		t.Fatalf("source footnote definition should resolve to structured metadata:\n%s", page)
	}
}

func TestPlainFrontmatterLinksOnlyLocalAndHTTPResources(t *testing.T) {
	rendered := renderPlainFrontmatter(map[string]any{
		"resource":    "javascript:alert(1)",
		"computation": "scripts/compute.py",
		"sources": []any{map[string]any{
			"resource": "https://example.test/source",
		}},
	}, "concepts/result.md")
	if strings.Contains(rendered, `href="javascript:`) {
		t.Fatalf("plain frontmatter must not link unsafe URI schemes:\n%s", rendered)
	}
	for _, expected := range []string{`href="scripts/compute.py"`, `href="https://example.test/source"`} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected safe metadata link %q:\n%s", expected, rendered)
		}
	}
}

func TestWriteHTMLSkipsUnpublishedPages(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, root)
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
	enablePublicArtifactTest(t, root)
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
