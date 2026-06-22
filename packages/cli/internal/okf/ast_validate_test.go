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

func TestValidateASTUsesMarkdownDiagnostics(t *testing.T) {
	document := ASTDocument{
		Rel:  "index.md",
		ID:   "index",
		Kind: "index",
		Body: "[Broken](missing.md\n",
		Frontmatter: ASTFrontmatter{
			BodyLine: 1,
		},
		Markdown: ASTMarkdown{},
	}
	bundle := ASTBundle{
		Root:        "root",
		SpecVersion: LatestSpecVersion,
		Documents:   []ASTDocument{document},
	}

	result := ValidateAST(bundle)
	if countRule(result.Warnings, "markdown-syntax") != 0 {
		t.Fatalf("expected no raw-body Markdown syntax scan, got %#v", result.Warnings)
	}

	bundle.Documents[0].Markdown.Diagnostics = []ASTDiagnostic{
		{Line: 1, Message: "Markdown link is missing closing ')'"},
	}
	result = ValidateAST(bundle)
	if countRule(result.Warnings, "markdown-syntax") != 1 {
		t.Fatalf("expected Markdown diagnostics to drive validation, got %#v", result.Warnings)
	}
	if result.Warnings[0].Path != "index.md" || result.Warnings[0].Line != 1 {
		t.Fatalf("unexpected Markdown diagnostic issue: %#v", result.Warnings[0])
	}
}
