package okf

import (
	"strings"
	"testing"
)

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

	if len(markdown.Sections) != 1 {
		t.Fatalf("expected one section, got %#v", markdown.Sections)
	}
	section := markdown.Sections[0]
	if section.Heading != "Home" || section.Level != 1 || section.Anchor != "home" || section.LineStart != 1 || section.LineEnd != 10 {
		t.Fatalf("unexpected Markdown section AST: %#v", section)
	}
	if len(section.Blocks) != 4 || len(section.Children) != 0 {
		t.Fatalf("expected section to own all blocks without children, got %#v", section)
	}
}

func TestParseASTMarkdownKeepsImageTitleOutOfTheTarget(t *testing.T) {
	markdown := ParseASTMarkdown(`![Architecture](assets/architecture.png "System overview")`, 7)
	if len(markdown.Links) != 1 {
		t.Fatalf("expected one image link, got %#v", markdown.Links)
	}
	image := markdown.Links[0]
	if !image.Image || image.Label != "Architecture" || image.Href != "assets/architecture.png" || image.Line != 7 {
		t.Fatalf("unexpected image link: %#v", image)
	}
}

func TestParseASTMarkdownCapturesLinkedImageAndDestination(t *testing.T) {
	markdown := ParseASTMarkdown(`[![Evidence coverage](assets/evidence.svg)](visualizations/evidence.md)`, 9)
	if len(markdown.Links) != 2 {
		t.Fatalf("expected destination link and nested image, got %#v", markdown.Links)
	}
	destination := markdown.Links[0]
	if destination.Image || destination.Label != "Evidence coverage" || destination.Href != "visualizations/evidence.md" || destination.Line != 9 {
		t.Fatalf("unexpected linked-image destination: %#v", destination)
	}
	image := markdown.Links[1]
	if !image.Image || image.Label != "Evidence coverage" || image.Href != "assets/evidence.svg" || image.Line != 9 {
		t.Fatalf("unexpected nested image: %#v", image)
	}
}

func TestParseASTMarkdownBuildsNestedSections(t *testing.T) {
	markdown := ParseASTMarkdown(strings.Join([]string{
		"Opening paragraph.",
		"",
		"# Guide",
		"Intro.",
		"",
		"## Install",
		"Install text.",
		"",
		"### CLI",
		"CLI text.",
		"",
		"# Reference",
		"Reference text.",
	}, "\n"), 1)

	if len(markdown.Sections) != 3 {
		t.Fatalf("expected top preamble and two h1 sections, got %#v", markdown.Sections)
	}
	top := markdown.Sections[0]
	if top.Heading != "Top" || top.Level != 0 || top.LineStart != 1 || top.LineEnd != 1 || len(top.Blocks) != 1 {
		t.Fatalf("unexpected top preamble section: %#v", top)
	}

	guide := markdown.Sections[1]
	if guide.Heading != "Guide" || guide.Level != 1 || guide.LineStart != 3 || guide.LineEnd != 4 {
		t.Fatalf("unexpected guide section: %#v", guide)
	}
	if len(guide.Children) != 1 {
		t.Fatalf("expected guide to have install child, got %#v", guide.Children)
	}
	install := guide.Children[0]
	if install.Heading != "Install" || install.Level != 2 || install.LineStart != 6 || install.LineEnd != 7 {
		t.Fatalf("unexpected install section: %#v", install)
	}
	if len(install.Children) != 1 {
		t.Fatalf("expected install to have CLI child, got %#v", install.Children)
	}
	cli := install.Children[0]
	if cli.Heading != "CLI" || cli.Level != 3 || cli.LineStart != 9 || cli.LineEnd != 10 {
		t.Fatalf("unexpected CLI section: %#v", cli)
	}

	reference := markdown.Sections[2]
	if reference.Heading != "Reference" || reference.Level != 1 || reference.LineStart != 12 || reference.LineEnd != 13 || len(reference.Children) != 0 {
		t.Fatalf("unexpected reference section: %#v", reference)
	}
}

func TestParseASTMarkdownBuildsCommonBlockNodes(t *testing.T) {
	markdown := ParseASTMarkdown(strings.Join([]string{
		"> Read [Quote](quote.md).",
		"> - Nested item",
		"",
		"---",
		"",
		"<!-- hidden -->",
		"<!-- okf-footer: agent-maintenance -->",
		"",
		"- Read [List](list.md)",
		"  continuation text.",
		"- Plain item",
		"",
		"| Name | Link |",
		"| --- | ---: |",
		"| Row | [Table](table.md) |",
	}, "\n"), 10)

	kinds := make([]string, 0, len(markdown.Blocks))
	for _, block := range markdown.Blocks {
		kinds = append(kinds, block.Kind)
	}
	expectedKinds := []string{"blockquote", "thematic-break", "html-comment", "agent-footer"}
	if strings.Join(kinds, ",") != strings.Join(expectedKinds, ",") {
		t.Fatalf("unexpected block kinds: %#v", kinds)
	}

	quote := markdown.Blocks[0]
	if quote.LineStart != 10 || quote.LineEnd != 11 || len(quote.Children) != 2 || len(quote.Links) != 1 {
		t.Fatalf("unexpected blockquote AST: %#v", quote)
	}
	footer := markdown.Blocks[3]
	if footer.Annotation == nil || footer.Annotation.Capability != "agent-context" || footer.LineEnd != 24 {
		t.Fatalf("unexpected legacy footer annotation: %#v", footer)
	}
	if len(footer.Children) != 2 {
		t.Fatalf("expected legacy footer content through end-of-file, got %#v", footer.Children)
	}
	list := footer.Children[0]
	if list.List == nil || list.List.Ordered || len(list.List.Items) != 2 {
		t.Fatalf("unexpected list AST: %#v", list)
	}
	if list.List.Items[0].Text != "Read [List](list.md) continuation text." || list.List.Items[0].LineStart != 18 || list.List.Items[0].LineEnd != 19 {
		t.Fatalf("unexpected wrapped list item AST: %#v", list.List.Items[0])
	}
	table := footer.Children[1]
	if table.Table == nil || strings.Join(table.Table.Header, ",") != "Name,Link" || len(table.Table.Rows) != 1 {
		t.Fatalf("unexpected table AST: %#v", table)
	}
	if len(table.Table.Alignments) != 2 || table.Table.Alignments[1] != "right" {
		t.Fatalf("unexpected table alignments: %#v", table.Table.Alignments)
	}

	if len(markdown.Links) != 1 || markdown.Links[0].Label != "Quote" {
		t.Fatalf("expected annotation links to stay out of the reader graph, got %#v", markdown.Links)
	}
}

func TestParseASTMarkdownBuildsBoundedAgentContextAnnotation(t *testing.T) {
	markdown := ParseASTMarkdown(strings.Join([]string{
		"# Page",
		"",
		"<!-- okf-annotation: agent-context -->",
		"## Maintenance notes",
		"See [Source](source.md).",
		"<!-- /okf-annotation -->",
		"",
		"Reader content.",
	}, "\n"), 5)

	if len(markdown.Diagnostics) != 0 {
		t.Fatalf("expected a valid annotation, got %#v", markdown.Diagnostics)
	}
	if len(markdown.Blocks) != 3 || markdown.Blocks[1].Kind != "annotation" {
		t.Fatalf("unexpected annotation blocks: %#v", markdown.Blocks)
	}
	annotation := markdown.Blocks[1]
	if annotation.LineStart != 7 || annotation.LineEnd != 10 || annotation.Annotation == nil || annotation.Annotation.Capability != "agent-context" {
		t.Fatalf("unexpected annotation metadata: %#v", annotation)
	}
	if len(annotation.Children) != 2 || annotation.Children[0].Kind != "heading" || annotation.Children[1].Kind != "paragraph" {
		t.Fatalf("expected parsed annotation children, got %#v", annotation.Children)
	}
	if len(markdown.Links) != 0 || len(markdown.Headings) != 1 {
		t.Fatalf("expected annotation structure to stay out of reader indexes, got links=%#v headings=%#v", markdown.Links, markdown.Headings)
	}
	if text := astMarkdownReaderText(markdown); strings.Contains(text, "Maintenance notes") || !strings.Contains(text, "Reader content") {
		t.Fatalf("unexpected reader text %q", text)
	}
}

func TestParseASTMarkdownDiagnosesInvalidAnnotations(t *testing.T) {
	markdown := ParseASTMarkdown(strings.Join([]string{
		"<!-- okf-annotation: unknown -->",
		"<!-- /okf-annotation -->",
		"<!-- okf-annotation: agent-context -->",
		"<!-- okf-annotation: agent-context -->",
	}, "\n"), 10)

	messages := make([]string, 0, len(markdown.Diagnostics))
	for _, diagnostic := range markdown.Diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, expected := range []string{
		"unknown OKF annotation capability",
		"closing marker has no matching opening marker",
		"OKF annotations cannot be nested",
		"OKF annotation is missing closing marker",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected diagnostic containing %q, got %#v", expected, markdown.Diagnostics)
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

func TestParseASTMarkdownRecordsSyntaxDiagnostics(t *testing.T) {
	markdown := ParseASTMarkdown("[Broken](missing.md\n\n```sh\necho ok", 4)

	if len(markdown.Diagnostics) != 2 {
		t.Fatalf("expected link and fence diagnostics, got %#v", markdown.Diagnostics)
	}
	link := markdown.Diagnostics[0]
	if link.Line != 4 || link.Message != "Markdown link is missing closing ')'" {
		t.Fatalf("unexpected link diagnostic: %#v", link)
	}
	fence := markdown.Diagnostics[1]
	if fence.Line != 6 || fence.Message != "fenced code block is not closed" {
		t.Fatalf("unexpected fence diagnostic: %#v", fence)
	}
}

func TestParseASTPopulatesResolvedLinksFromMarkdownAST(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead [Guide](guide.md).\n\n```markdown\n[Ignored](missing.md)\n```\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Guide\n---\n\n# Guide\n")

	ast, err := ParseAST(root)
	if err != nil {
		t.Fatal(err)
	}
	index := astDocumentByPath(t, ast, "index.md")

	if len(index.Markdown.Links) != 1 {
		t.Fatalf("expected Markdown AST to skip code-fence links, got %#v", index.Markdown.Links)
	}
	if len(index.Links) != 1 {
		t.Fatalf("expected resolved links from Markdown AST, got %#v", index.Links)
	}
	link := index.Links[0]
	if link.Label != "Guide" || link.Href != "guide.md" || link.Kind != "local" || link.TargetPath != "guide.md" || link.TargetID != "guide" || !link.Exists || link.Line != 3 {
		t.Fatalf("unexpected resolved link from Markdown AST: %#v", link)
	}
}

func astDocumentByPath(t *testing.T, bundle ASTBundle, path string) ASTDocument {
	t.Helper()
	for _, document := range bundle.Documents {
		if document.Rel == path {
			return document
		}
	}
	t.Fatalf("missing AST document %s in %#v", path, bundle.Documents)
	return ASTDocument{}
}
