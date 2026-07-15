package okf_test

import (
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
