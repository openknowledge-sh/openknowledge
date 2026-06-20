package okf

func parseASTDocumentLinks(root string, document astDocument) astDocument {
	if document.ReadDiagnostic != nil {
		return document
	}
	document.Links = ExtractLinks(root, document.Rel, document.Content)
	return document
}
