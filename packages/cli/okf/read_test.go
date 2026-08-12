package okf_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/okf"
)

func TestPublicReadAPIExercisesCoreViews(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_name: sdk-test\n---\n\n# SDK Test\n\nRead the [guide](guide.md).\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Search Guide\ndescription: Public Go API example.\n---\n\n# Retrieval\n\nUse deterministic knowledge search.\n")

	validation, err := okf.ValidateWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if err := okf.RequireValidBundle(validation); err != nil {
		t.Fatal(err)
	}
	if validation.SchemaVersion != okf.MachineSchemaVersion || validation.SpecVersion != "0.1" {
		t.Fatalf("unexpected public validation identity: %#v", validation)
	}

	ast, err := okf.ParseASTWithVersion(root, "0.1")
	if err != nil || len(ast.Documents) != 2 {
		t.Fatalf("unexpected public AST: documents=%d err=%v", len(ast.Documents), err)
	}
	var warnings []okf.ASTFrontmatterWarning = ast.Documents[1].Frontmatter.Warnings
	_ = warnings
	bundle, err := okf.ParseBundleWithVersion(root, "0.1")
	if err != nil || len(bundle.Files) != 2 {
		t.Fatalf("unexpected public bundle: files=%d err=%v", len(bundle.Files), err)
	}
	listing, err := okf.ListWithVersion(root, "0.1")
	if err != nil || len(listing.Entries) != 2 {
		t.Fatalf("unexpected public listing: entries=%d err=%v", len(listing.Entries), err)
	}
	results, err := okf.SearchWithVersion(root, "0.1", okf.SearchOptions{Query: "deterministic search", Limit: 5})
	if err != nil || len(results.Results) == 0 || results.Results[0].Path != "guide.md" {
		t.Fatalf("unexpected public search: %#v err=%v", results, err)
	}
	context, err := okf.ResolveContextWithVersion(root, "0.1", okf.ContextOptions{Query: "deterministic search", Budget: 500, Limit: 5})
	var revision okf.RetrievalRevision = context.Revision
	if err != nil || len(context.Sources) == 0 || len(revision.IndexSHA256) != 64 || context.Sources[0].Locator == "" || !strings.Contains(context.Sources[0].Markdown, "deterministic knowledge search") {
		t.Fatalf("unexpected public context: %#v err=%v", context, err)
	}
	targets := []okf.FederatedTarget{{Name: "sdk", Root: root}}
	federatedMatches, err := okf.SearchFederated(targets, okf.SearchOptions{Query: "deterministic search", Limit: 5, Fuzzy: true})
	if err != nil || federatedMatches.Fusion.Method != "rrf" || len(federatedMatches.Results) == 0 || federatedMatches.Results[0].KnowledgeBase != "sdk" {
		t.Fatalf("unexpected public federated search: %#v err=%v", federatedMatches, err)
	}
	federatedContext, err := okf.ResolveFederatedContextWithVersion(targets, "0.1", okf.ContextOptions{Query: "deterministic search", Budget: 500, Limit: 5})
	if err != nil || len(federatedContext.Sources) == 0 || federatedContext.Sources[0].Source.Locator == "" {
		t.Fatalf("unexpected public federated context: %#v err=%v", federatedContext, err)
	}
	graph, err := okf.BuildGraphWithType(root, "0.1", okf.GraphTypeSearch)
	if err != nil || len(graph.Nodes) == 0 || graph.Type != okf.GraphTypeSearch {
		t.Fatalf("unexpected public graph: %#v err=%v", graph, err)
	}
	info, err := okf.ReadBundleInfo(root)
	if err != nil || info.Metadata.Name != "sdk-test" {
		t.Fatalf("unexpected public bundle info: %#v err=%v", info, err)
	}
}

func TestPublicContextIndexReusesImmutableSnapshot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n\nRead the guide.\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Search Guide\n---\n\n# Retrieval\n\nUse deterministic lexical search.\n")

	index, err := okf.BuildContextIndexWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	first := index.Search(okf.SearchOptions{Query: "deterministic lexical", Limit: 5, NoExpand: true})
	if len(first.Results) != 1 || first.Results[0].Path != "guide.md" {
		t.Fatalf("unexpected reusable index result: %#v", first)
	}
	context, err := index.Resolve(okf.ContextOptions{Query: "deterministic lexical", Budget: 500, Limit: 5, NoExpand: true})
	if err != nil || len(context.Sources) != 1 || context.Revision != first.Revision {
		t.Fatalf("expected search and context to share one revision: search=%#v context=%#v err=%v", first, context, err)
	}

	writeFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Search Guide\n---\n\n# Retrieval\n\nUse replacement canary evidence.\n")
	stale := index.Search(okf.SearchOptions{Query: "replacement canary", Limit: 5, NoExpand: true})
	if len(stale.Results) != 0 || stale.Revision != first.Revision {
		t.Fatalf("expected the reusable index to retain its immutable snapshot: %#v", stale)
	}
	fresh, err := okf.SearchWithVersion(root, "0.1", okf.SearchOptions{Query: "replacement canary", Limit: 5, NoExpand: true})
	if err != nil || len(fresh.Results) != 1 || fresh.Revision == first.Revision {
		t.Fatalf("expected a one-shot search to build a fresh revision: %#v err=%v", fresh, err)
	}
}

func TestPublicContextIndexReturnsIndependentIssues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "[Missing](missing.md)\n")
	index, err := okf.BuildContextIndexWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	first := index.Search(okf.SearchOptions{Query: "missing", Limit: 5})
	second := index.Search(okf.SearchOptions{Query: "missing", Limit: 5})
	if len(first.Issues) == 0 || len(second.Issues) == 0 {
		t.Fatalf("expected validation issues in both results: first=%#v second=%#v", first, second)
	}
	first.Issues[0].Message = "caller mutation"
	if second.Issues[0].Message == "caller mutation" {
		t.Fatal("result issues share mutable storage")
	}
}

func TestPublicContextIndexSupportsConcurrentSearch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n\nConcurrent retrieval.\n")
	index, err := okf.BuildContextIndexWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	errors := make(chan string, 8)
	for worker := range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 25 {
				if worker%2 == 0 {
					result := index.Search(okf.SearchOptions{Query: "concurrent retrieval", Limit: 5, NoExpand: true})
					if len(result.Results) != 1 || result.Results[0].Path != "index.md" {
						errors <- "unexpected concurrent search result"
						return
					}
					continue
				}
				result, err := index.Resolve(okf.ContextOptions{Query: "concurrent retrieval", Budget: 500, Limit: 5, NoExpand: true})
				if err != nil || len(result.Sources) != 1 || result.Sources[0].Path != "index.md" {
					errors <- "unexpected concurrent context result"
					return
				}
			}
		}()
	}
	wait.Wait()
	close(errors)
	for message := range errors {
		t.Fatal(message)
	}
}

func BenchmarkPublicContextIndexSearch(b *testing.B) {
	root := b.TempDir()
	for documentIndex := range 100 {
		content := "---\ntype: Guide\ntitle: Reference Guide\n---\n\n# Reference\n\nPortable knowledge retrieval.\n"
		if documentIndex%10 == 0 {
			content = "---\ntype: Runbook\ntitle: Deployment Guide\n---\n\n# Validation\n\nDeployment validation workflow.\n"
		}
		path := filepath.Join(root, "guides", fmt.Sprintf("%03d.md", documentIndex))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	index, err := okf.BuildContextIndexWithVersion(root, "0.1")
	if err != nil {
		b.Fatal(err)
	}
	options := okf.SearchOptions{Query: "deployment validation", Limit: 5, NoExpand: true}

	b.Run("reused_index", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			result := index.Search(options)
			if len(result.Results) == 0 {
				b.Fatal("expected search results")
			}
		}
	})
	b.Run("one_shot", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			result, err := okf.SearchWithVersion(root, "0.1", options)
			if err != nil || len(result.Results) == 0 {
				b.Fatalf("expected search results: %#v err=%v", result, err)
			}
		}
	})
}

func TestPublicConfigurationAndManifestHelpers(t *testing.T) {
	options := okf.ValidationOptions{}
	if err := okf.SetValidationRuleSeverity(&options, "link-target", "error"); err != nil {
		t.Fatal(err)
	}
	if options.Rules["link-target"] != okf.ValidationSeverityError {
		t.Fatalf("validation option was not applied: %#v", options)
	}
	v01Rules, err := okf.KnownValidationRulesForVersion("0.1")
	if err != nil {
		t.Fatal(err)
	}
	if okf.IsKnownValidationRuleForVersion("0.1", "okf-0.2-metadata") || !okf.IsKnownValidationRuleForVersion("0.2", "okf-0.2-metadata") {
		t.Fatalf("expected version-bound validation rule discovery: %v", v01Rules)
	}
	if !okf.IsKnownValidationRuleForVersion("0.1", "publish-metadata") || okf.IsValidationRuleOverrideableForVersion("0.1", "publish-metadata") {
		t.Fatal("expected the fixed publish-metadata rule to be part of the 0.1 profile")
	}
	if err := okf.SetValidationRuleSeverityForVersion(&options, "0.1", "okf-0.2-metadata", "warn"); err == nil {
		t.Fatal("expected 0.1 to reject the 0.2-only validation rule")
	}

	manifestJSON := `{"type":"openknowledge.bundle","version":1,"spec":"0.1","archive":"bundle.tar.gz","archiveSha256":"` + strings.Repeat("a", 64) + `","archiveFormat":"tar.gz"}`
	manifest, err := okf.DecodeBundleManifest([]byte(manifestJSON))
	if err != nil || manifest.Type != okf.BundleManifestType {
		t.Fatalf("unexpected public manifest result: %#v err=%v", manifest, err)
	}
	if versions := okf.SupportedSpecVersions(); len(versions) != 2 || versions[0] != "0.1" || versions[1] != okf.LatestSpecVersion {
		t.Fatalf("unexpected public spec registry: %v", versions)
	}
}

func TestPublicRegistryDiscoveryIsReadOnly(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "knowledge")
	writeFile(t, root, "index.md", "# Knowledge\n")
	secondRoot := filepath.Join(base, "second")
	writeFile(t, secondRoot, "index.md", "# Second\n")
	registryPath := filepath.Join(base, "registry.json")
	t.Setenv(okf.RegistryFileEnv, registryPath)
	stored := map[string]any{
		"schemaVersion": okf.RegistrySchemaVersion,
		"connections": map[string]any{
			secondRoot: map[string]any{"key": "zeta", "access": "read"},
			root:       map[string]any{"key": "docs", "access": "write"},
		},
	}
	content, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(registryPath, content, 0600); err != nil {
		t.Fatal(err)
	}

	path, err := okf.RegistryFile()
	if err != nil || path != registryPath {
		t.Fatalf("unexpected public registry path: %q err=%v", path, err)
	}
	entries, err := okf.RegistryEntries()
	if err != nil || len(entries) != 2 || entries[0].Name != "docs" || entries[1].Name != "zeta" || entries[0].Path != root || !okf.RegistryEntryCanWrite(entries[0]) {
		t.Fatalf("unexpected public registry inventory: %#v err=%v", entries, err)
	}
	entry, found, err := okf.ResolveRegistryEntry("docs")
	if err != nil || !found || entry != entries[0] {
		t.Fatalf("unexpected public key resolution: %#v found=%t err=%v", entry, found, err)
	}
	byPath, found, err := okf.ResolveRegistryTarget(root)
	if err != nil || !found || byPath != entry {
		t.Fatalf("unexpected public target resolution: %#v found=%t err=%v", byPath, found, err)
	}
	resolved, err := okf.ResolveKnowledgeRoot("docs")
	if err != nil || resolved != root {
		t.Fatalf("unexpected public knowledge-root resolution: %q err=%v", resolved, err)
	}
	canWrite, err := okf.RegistryPathCanWrite(filepath.Join(root, "index.md"))
	if err != nil || !canWrite {
		t.Fatalf("unexpected public path capability: canWrite=%t err=%v", canWrite, err)
	}
	if err := okf.RequireRegistryWriteAccess(filepath.Join(root, "index.md")); err != nil {
		t.Fatalf("unexpected public write guard: %v", err)
	}
	after, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(content) {
		t.Fatalf("read-only public registry API mutated storage:\nbefore=%s\nafter=%s", content, after)
	}
}

func writeFile(t *testing.T, root string, relative string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
