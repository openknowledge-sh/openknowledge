package okf

func List(root string) (ListResult, error) {
	return ListWithVersion(root, LatestSpecVersion)
}

func ListWithVersion(root string, version string) (ListResult, error) {
	validation, ast, err := parseAndValidateASTBundle(root, version)
	if err != nil {
		return ListResult{}, err
	}

	return listInventoryFromAST(ast, issuesFromResult(validation))
}

func listInventoryFromAST(bundle astBundle, issues []Issue) (ListResult, error) {
	issuesByPath := groupIssuesByPath(issues)
	entries := make([]ListEntry, 0, len(bundle.Documents))
	for _, document := range bundle.Documents {
		if document.ReadDiagnostic != nil {
			return ListResult{}, document.ReadDiagnostic
		}
		metadata := document.Metadata
		if document.FrontmatterDiagnostic != nil {
			metadata = astDocumentMetadata{}
		}
		entries = append(entries, attachIssues(listEntryFromASTSummary(summarizeASTDocument(document, metadata)), issuesByPath))
	}
	return ListResult{Root: bundle.Root, Entries: entries}, nil
}

func attachIssues(entry ListEntry, issuesByPath map[string][]Issue) ListEntry {
	entry.Issues = issuesByPath[entry.Path]
	return entry
}

func listEntryFromASTSummary(summary astDocumentSummary) ListEntry {
	return ListEntry{
		ID:          summary.ID,
		Path:        summary.Path,
		Kind:        summary.Kind,
		Reserved:    summary.Reserved,
		Type:        summary.Type,
		Title:       summary.Title,
		Description: summary.Description,
		Resource:    summary.Resource,
	}
}
