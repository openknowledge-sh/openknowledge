package okf

import (
	"io/fs"
	"path/filepath"
	"sort"
)

func List(root string) (ListResult, error) {
	return ListWithVersion(root, LatestSpecVersion)
}

func ListWithVersion(root string, version string) (ListResult, error) {
	validation, ast, err := parseAndValidateASTBundle(root, version)
	if err != nil {
		return ListResult{}, err
	}

	listing, err := ListFromAST(ast, issuesFromResult(validation))
	if err != nil {
		return ListResult{}, err
	}
	assets, err := listAssetEntries(ast.Root, listing.Entries)
	if err != nil {
		return ListResult{}, err
	}
	listing.Entries = append(listing.Entries, assets...)
	sortListEntries(listing.Entries)
	return listing, nil
}

func ListFromAST(bundle ASTBundle, issues []Issue) (ListResult, error) {
	issuesByPath := groupIssuesByPath(issues)
	entries := make([]ListEntry, 0, len(bundle.Documents))
	for _, document := range bundle.Documents {
		if document.ReadDiagnostic != nil {
			return ListResult{}, document.ReadDiagnostic
		}
		metadata := document.Metadata
		if document.FrontmatterDiagnostic != nil {
			metadata = ASTDocumentMetadata{}
		}
		entries = append(entries, attachIssues(listEntryFromASTSummary(SummarizeASTDocument(document, metadata)), issuesByPath))
	}
	sortListEntries(entries)
	return ListResult{SchemaVersion: MachineSchemaVersion, Root: bundle.Root, Entries: entries}, nil
}

func listAssetEntries(root string, documents []ListEntry) ([]ListEntry, error) {
	documentPaths := make(map[string]struct{}, len(documents))
	for _, document := range documents {
		documentPaths[document.Path] = struct{}{}
	}
	var assets []ListEntry
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel := relPath(root, path)
		if _, ok := documentPaths[rel]; ok || isMarkdown(path) {
			return nil
		}
		assets = append(assets, ListEntry{
			ID:   rel,
			Path: rel,
			Kind: "asset",
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return assets, nil
}

func sortListEntries(entries []ListEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
}

func attachIssues(entry ListEntry, issuesByPath map[string][]Issue) ListEntry {
	entry.Issues = issuesByPath[entry.Path]
	return entry
}

func listEntryFromASTSummary(summary ASTDocumentSummary) ListEntry {
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
