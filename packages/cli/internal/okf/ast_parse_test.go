package okf

import "testing"

func TestParseASTReturnsDocumentsAndMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead the guide.\n")
	writeFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup\ndescription: Install the CLI.\n---\n\n# Setup\n")

	ast, err := ParseAST(root)
	if err != nil {
		t.Fatal(err)
	}

	if ast.SpecVersion != LatestSpecVersion {
		t.Fatalf("expected latest spec version, got %q", ast.SpecVersion)
	}
	if len(ast.Documents) != 2 {
		t.Fatalf("expected two AST documents, got %#v", ast.Documents)
	}
	if ast.Documents[0].Rel != "guides/setup.md" || ast.Documents[1].Rel != "index.md" {
		t.Fatalf("expected sorted AST documents, got %#v", ast.Documents)
	}

	guide := ast.Documents[0]
	if guide.Metadata.Type != "Guide" || guide.Metadata.Title != "Setup" || guide.Metadata.Description != "Install the CLI." {
		t.Fatalf("unexpected AST metadata: %#v", guide.Metadata)
	}
	if guide.Body != "\n# Setup\n" {
		t.Fatalf("unexpected AST body: %q", guide.Body)
	}
}
