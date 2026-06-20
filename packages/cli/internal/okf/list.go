package okf

import (
	"path/filepath"
	"strings"
)

type ListResult struct {
	Root    string      `json:"root"`
	Entries []ListEntry `json:"entries"`
}

type ListEntry struct {
	ID          string  `json:"id"`
	Path        string  `json:"path"`
	Kind        string  `json:"kind"`
	Reserved    bool    `json:"reserved"`
	Type        string  `json:"type,omitempty"`
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Resource    string  `json:"resource,omitempty"`
	Issues      []Issue `json:"issues,omitempty"`
}

func List(root string) (ListResult, error) {
	return ListWithVersion(root, LatestSpecVersion)
}

func ListWithVersion(root string, version string) (ListResult, error) {
	validation, ast, err := parseAndValidateBundle(root, version)
	if err != nil {
		return ListResult{}, err
	}

	issues := append([]Issue{}, validation.Errors...)
	issues = append(issues, validation.Warnings...)
	return listInventoryFromAST(ast, issues)
}

func listInventoryFromAST(bundle astBundle, issues []Issue) (ListResult, error) {
	issuesByPath := groupIssuesByPath(issues)
	entries := make([]ListEntry, 0, len(bundle.Documents))
	for _, document := range bundle.Documents {
		if document.ReadErr != nil {
			return ListResult{}, document.ReadErr
		}
		if document.Reserved {
			entries = append(entries, attachIssues(reservedEntry(document), issuesByPath))
			continue
		}
		if document.FrontmatterErr != nil {
			entries = append(entries, attachIssues(conceptEntry(document, astDocumentMetadata{}), issuesByPath))
			continue
		}
		entries = append(entries, attachIssues(conceptEntry(document, document.Metadata), issuesByPath))
	}
	return ListResult{Root: bundle.Root, Entries: entries}, nil
}

func groupIssuesByPath(issues []Issue) map[string][]Issue {
	grouped := make(map[string][]Issue)
	for _, issue := range issues {
		grouped[issue.Path] = append(grouped[issue.Path], issue)
	}
	return grouped
}

func attachIssues(entry ListEntry, issuesByPath map[string][]Issue) ListEntry {
	entry.Issues = issuesByPath[entry.Path]
	return entry
}

func conceptEntry(document astDocument, metadata astDocumentMetadata) ListEntry {
	title := metadata.Title
	if title == "" {
		title = deriveTitle(document.Rel)
	}

	return ListEntry{
		ID:          document.ID,
		Path:        document.Rel,
		Kind:        document.Kind,
		Type:        metadata.Type,
		Title:       title,
		Description: metadata.Description,
		Resource:    metadata.Resource,
	}
}

func reservedEntry(document astDocument) ListEntry {
	title := deriveTitle(document.Rel)
	if document.Kind == "index" {
		title = "Index"
	}
	if document.Kind == "log" {
		title = "Log"
	}

	return ListEntry{
		ID:       document.ID,
		Path:     document.Rel,
		Kind:     document.Kind,
		Reserved: document.Reserved,
		Title:    title,
	}
}

func isReserved(path string) bool {
	_, _, reserved := classifyDocument(path)
	return reserved
}

func classifyDocument(rel string) (string, string, bool) {
	name := filepath.Base(rel)
	if strings.EqualFold(name, "index.md") {
		return trimMarkdownExtension(rel), "index", true
	}
	if strings.EqualFold(name, "log.md") {
		return trimMarkdownExtension(rel), "log", true
	}
	return trimMarkdownExtension(rel), "concept", false
}

func trimMarkdownExtension(path string) string {
	extension := filepath.Ext(path)
	if strings.EqualFold(extension, ".md") {
		return strings.TrimSuffix(path, extension)
	}
	return path
}

func deriveTitle(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	if base == "" {
		return "Untitled"
	}
	return strings.ToUpper(base[:1]) + base[1:]
}
