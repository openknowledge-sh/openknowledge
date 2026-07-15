package okf_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	if err != nil || len(context.Sources) == 0 || !strings.Contains(context.Sources[0].Markdown, "deterministic knowledge search") {
		t.Fatalf("unexpected public context: %#v err=%v", context, err)
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

func TestPublicConfigurationAndManifestHelpers(t *testing.T) {
	options := okf.ValidationOptions{}
	if err := okf.SetValidationRuleSeverity(&options, "link-target", "error"); err != nil {
		t.Fatal(err)
	}
	if options.Rules["link-target"] != okf.ValidationSeverityError {
		t.Fatalf("validation option was not applied: %#v", options)
	}

	manifestJSON := `{"type":"openknowledge.bundle","version":1,"spec":"0.1","archive":"bundle.tar.gz","archiveSha256":"` + strings.Repeat("a", 64) + `","archiveFormat":"tar.gz"}`
	manifest, err := okf.DecodeBundleManifest([]byte(manifestJSON))
	if err != nil || manifest.Type != okf.BundleManifestType {
		t.Fatalf("unexpected public manifest result: %#v err=%v", manifest, err)
	}
	if versions := okf.SupportedSpecVersions(); len(versions) != 1 || versions[0] != okf.LatestSpecVersion {
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
