package okf

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestASTBackedOutputsMatchPublicAPIs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n\nRead [Setup](guides/setup.md).\n")
	writeFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup Guide\ndescription: Install and validate.\nresource: file://setup\ntags: [setup, cli]\n---\n\n# Setup Guide\n\nRun `openknowledge validate`.\n\n## Checklist\n\nValidate the bundle.\n")

	parsed, err := ParseAST(root)
	if err != nil {
		t.Fatal(err)
	}
	validation := ValidateAST(parsed)
	issues := issuesFromResult(validation)

	directBundle, err := BundleFromAST(parsed, issues)
	if err != nil {
		t.Fatal(err)
	}
	publicBundle, err := ParseBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(publicBundle, directBundle) {
		t.Fatalf("expected public bundle to match direct AST bundle\npublic=%#v\ndirect=%#v", publicBundle, directBundle)
	}

	directList, err := listInventoryFromAST(parsed, issues)
	if err != nil {
		t.Fatal(err)
	}
	publicList, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(publicList, directList) {
		t.Fatalf("expected public list to match direct AST list\npublic=%#v\ndirect=%#v", publicList, directList)
	}

	searchOptions := SearchOptions{Query: "setup validate", Limit: 5, Fuzzy: true}
	directSearch := newSearchIndexFromAST(parsed).Search(searchOptions)
	publicSearch, err := Search(root, searchOptions)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(publicSearch, directSearch) {
		t.Fatalf("expected public search to match direct AST search\npublic=%#v\ndirect=%#v", publicSearch, directSearch)
	}

	directContext := contextIndexFromAST(validation, parsed)
	publicContext, err := BuildContextIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(publicContext, directContext) {
		t.Fatalf("expected public context index to match direct AST context index\npublic=%#v\ndirect=%#v", publicContext, directContext)
	}

	publicOut := filepath.Join(t.TempDir(), "public")
	directOut := filepath.Join(t.TempDir(), "direct")
	publicHTML, err := WriteHTML(root, publicOut)
	if err != nil {
		t.Fatal(err)
	}
	directHTML, err := writeHTMLFromAST(parsed, directOut, staticPageTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if publicHTML.Root != directHTML.Root || !reflect.DeepEqual(publicHTML.Written, directHTML.Written) {
		t.Fatalf("expected public HTML result to match direct AST HTML result\npublic=%#v\ndirect=%#v", publicHTML, directHTML)
	}
	for _, name := range publicHTML.Written {
		publicContent := readExportFile(t, publicOut, name)
		directContent := readExportFile(t, directOut, name)
		if publicContent != directContent {
			t.Fatalf("expected public HTML %s to match direct AST HTML", name)
		}
	}
}

func TestValidateASTMatchesValidateForMalformedDocuments(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "bad-frontmatter.md", "---\ntype: [broken\n---\n\n# Broken\n")
	if err := os.WriteFile(filepath.Join(root, "bad-utf8.md"), []byte{0xff, '\n'}, 0644); err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseAST(root)
	if err != nil {
		t.Fatal(err)
	}
	direct := ValidateAST(parsed)
	public, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(public, direct) {
		t.Fatalf("expected Validate to match direct ValidateAST\npublic=%#v\ndirect=%#v", public, direct)
	}
}

func TestValidationAndExporterEntrypointsParseThroughAST(t *testing.T) {
	files := map[string][]string{
		"validate.go": {"parseAndValidateASTBundle"},
		"bundle.go":   {"parseAndValidateASTBundle", "BundleFromAST"},
		"list.go":     {"parseAndValidateASTBundle", "listInventoryFromAST"},
		"search.go":   {"parseAndValidateASTBundle", "newSearchIndexFromAST"},
		"html.go":     {"parseAndValidateASTBundle", "writeHTMLFromAST"},
		"context.go":  {"parseAndValidateASTBundle", "contextIndexFromAST"},
	}
	forbidden := []string{
		"ExtractLinks",
		"parseASTDocumentContent",
		"parseASTDocumentFile",
		"parseASTDocumentLinks",
		"splitFrontmatter",
	}

	for name, required := range files {
		calls := calledFunctions(t, name)
		for _, function := range required {
			if !slices.Contains(calls, function) {
				t.Fatalf("%s should call %s; calls=%v", name, function, calls)
			}
		}
		for _, function := range forbidden {
			if slices.Contains(calls, function) {
				t.Fatalf("%s should consume parsed AST instead of calling %s directly", name, function)
			}
		}
	}
}

func calledFunctions(t *testing.T, path string) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]struct{}{}
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch function := call.Fun.(type) {
		case *ast.Ident:
			seen[function.Name] = struct{}{}
		case *ast.SelectorExpr:
			seen[function.Sel.Name] = struct{}{}
		}
		return true
	})

	calls := make([]string, 0, len(seen))
	for name := range seen {
		calls = append(calls, name)
	}
	slices.Sort(calls)
	return calls
}
