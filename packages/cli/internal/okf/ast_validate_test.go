package okf

import "testing"

func TestValidateASTValidatesParsedBundle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "concept.md", "---\ntype: Concept\ntitle: Concept\n---\n\n# Concept\n")

	ast, err := ParseAST(root)
	if err != nil {
		t.Fatal(err)
	}

	result := ValidateAST(ast)
	if result.Root != ast.Root {
		t.Fatalf("expected validation root %q, got %q", ast.Root, result.Root)
	}
	if result.Files != 2 || result.Indexes != 1 || result.Concepts != 1 {
		t.Fatalf("unexpected validation counts: %#v", result)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected valid AST, got errors: %#v", result.Errors)
	}
}
