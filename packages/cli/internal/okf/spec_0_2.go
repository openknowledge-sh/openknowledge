package okf

// normalizeASTDocumentsForSpec applies consumer-side compatibility rules that
// change the parsed representation without changing the source Markdown.
func normalizeASTDocumentsForSpec(documents []ASTDocument, version string) {
	if version != "0.2" {
		return
	}
	for index := range documents {
		verified, ok := documents[index].Frontmatter.Data["verified"].(map[string]any)
		if !ok {
			continue
		}
		documents[index].Frontmatter.Data["verified"] = []any{copyAnyMap(verified)}
	}
}
