package okf

type astDocumentSummary struct {
	ID          string
	Path        string
	Kind        string
	Reserved    bool
	Type        string
	Title       string
	Description string
	Resource    string
}

func summarizeASTDocument(document astDocument, metadata astDocumentMetadata) astDocumentSummary {
	if document.Reserved {
		title := deriveTitle(document.Rel)
		if document.Kind == "index" {
			title = "Index"
		}
		if document.Kind == "log" {
			title = "Log"
		}

		return astDocumentSummary{
			ID:       document.ID,
			Path:     document.Rel,
			Kind:     document.Kind,
			Reserved: document.Reserved,
			Title:    title,
		}
	}

	title := metadata.Title
	if title == "" {
		title = deriveTitle(document.Rel)
	}

	return astDocumentSummary{
		ID:          document.ID,
		Path:        document.Rel,
		Kind:        document.Kind,
		Type:        metadata.Type,
		Title:       title,
		Description: metadata.Description,
		Resource:    metadata.Resource,
	}
}
