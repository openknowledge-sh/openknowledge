package okf

func parseASTDocumentLinks(root string, document ASTDocument) ASTDocument {
	if document.ReadDiagnostic != nil {
		return document
	}
	document.Links = ExtractLinks(root, document.Rel, document.Content)
	return document
}
