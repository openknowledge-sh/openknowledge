package okf

import "testing"

func TestParseASTMarkdownBuildsStructuralMarkdownTree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead [Guide](guide.md).\n\n```mermaid\ngraph TD\n  A-->B\n```\n\n![Diagram](diagram.png)\n")

	ast, err := ParseAST(root)
	if err != nil {
		t.Fatal(err)
	}
	document := ast.Documents[0]
	markdown := document.Markdown

	if document.Body == "" {
		t.Fatal("expected raw body to remain available")
	}
	if len(markdown.Headings) != 1 {
		t.Fatalf("expected one heading, got %#v", markdown.Headings)
	}
	heading := markdown.Headings[0]
	if heading.Level != 1 || heading.Text != "Home" || heading.Anchor != "home" || heading.Line != 1 {
		t.Fatalf("unexpected heading AST: %#v", heading)
	}

	if len(markdown.Links) != 2 {
		t.Fatalf("expected paragraph link and image link, got %#v", markdown.Links)
	}
	if markdown.Links[0].Label != "Guide" || markdown.Links[0].Href != "guide.md" || markdown.Links[0].Kind != "local" || markdown.Links[0].Line != 3 {
		t.Fatalf("unexpected Markdown link AST: %#v", markdown.Links[0])
	}
	if !markdown.Links[1].Image || markdown.Links[1].Label != "Diagram" || markdown.Links[1].Href != "diagram.png" || markdown.Links[1].Line != 10 {
		t.Fatalf("unexpected Markdown image AST: %#v", markdown.Links[1])
	}

	if len(markdown.CodeBlocks) != 1 {
		t.Fatalf("expected one code block, got %#v", markdown.CodeBlocks)
	}
	code := markdown.CodeBlocks[0]
	if code.Language != "mermaid" || !code.Mermaid || code.Text != "graph TD\n  A-->B" || code.LineStart != 5 || code.LineEnd != 8 {
		t.Fatalf("unexpected code block AST: %#v", code)
	}

	if len(markdown.Blocks) != 4 {
		t.Fatalf("expected heading, paragraph, code, paragraph blocks, got %#v", markdown.Blocks)
	}
	for index, kind := range []string{"heading", "paragraph", "code", "paragraph"} {
		if markdown.Blocks[index].Kind != kind {
			t.Fatalf("expected block %d to be %q, got %#v", index, kind, markdown.Blocks[index])
		}
	}
}

func TestParseASTMarkdownUsesDocumentLineNumbersAfterFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Guide\n---\n\n# Guide\n\nSee [Home](index.md).\n")

	ast, err := ParseAST(root)
	if err != nil {
		t.Fatal(err)
	}
	markdown := ast.Documents[0].Markdown

	if len(markdown.Headings) != 1 || markdown.Headings[0].Line != 6 {
		t.Fatalf("expected heading to use source line 6, got %#v", markdown.Headings)
	}
	if len(markdown.Links) != 1 || markdown.Links[0].Line != 8 {
		t.Fatalf("expected link to use source line 8, got %#v", markdown.Links)
	}
}
