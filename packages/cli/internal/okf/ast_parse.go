package okf

import (
	"os"
)

func parseASTDocumentFile(path string, rel string) ASTDocument {
	content, err := os.ReadFile(path)
	id, kind, reserved := classifyDocument(rel)
	document := ASTDocument{
		Absolute:       path,
		Rel:            rel,
		ID:             id,
		Kind:           kind,
		Reserved:       reserved,
		ReadDiagnostic: astReadDiagnostic(err),
	}
	if err != nil {
		return document
	}

	return parseASTDocumentContent(document, content)
}

func parseASTDocumentContent(document ASTDocument, content []byte) ASTDocument {
	document.UTF8Diagnostic = astUTF8Diagnostic(content)
	meta, body, frontmatterErr := splitFrontmatter(string(content))
	document.Content = string(content)
	document.Frontmatter = astFrontmatterFromParse(meta)
	document.Metadata = astDocumentMetadataFromFrontmatter(document.Frontmatter)
	document.Body = body
	document.Markdown = ParseASTMarkdown(body, document.Frontmatter.BodyLine)
	document.FrontmatterDiagnostic = astFrontmatterDiagnostic(frontmatterErr)
	return document
}
