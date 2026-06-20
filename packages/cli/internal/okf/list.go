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
	validation, err := ValidateWithVersion(root, version)
	if err != nil {
		return ListResult{}, err
	}

	issues := append([]Issue{}, validation.Errors...)
	issues = append(issues, validation.Warnings...)
	return listInventory(validation.Root, issues)
}

func listInventory(absolute string, issues []Issue) (ListResult, error) {
	issuesByPath := groupIssuesByPath(issues)
	documents, err := parseMarkdownDocuments(absolute)
	if err != nil {
		return ListResult{}, err
	}

	entries := make([]ListEntry, 0, len(documents))
	for _, document := range documents {
		if isReserved(document.Rel) {
			entries = append(entries, attachIssues(reservedEntry(document.Rel), issuesByPath))
			continue
		}
		if document.FrontmatterErr != nil {
			entries = append(entries, attachIssues(conceptEntry(document.Rel, frontmatter{}), issuesByPath))
			continue
		}
		entries = append(entries, attachIssues(conceptEntry(document.Rel, document.Frontmatter), issuesByPath))
	}
	return ListResult{Root: absolute, Entries: entries}, nil
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

func conceptEntry(rel string, meta frontmatter) ListEntry {
	title := meta.values["title"]
	if title == "" {
		title = deriveTitle(rel)
	}

	return ListEntry{
		ID:          trimMarkdownExtension(rel),
		Path:        rel,
		Kind:        "concept",
		Type:        meta.values["type"],
		Title:       title,
		Description: meta.values["description"],
		Resource:    meta.values["resource"],
	}
}

func reservedEntry(rel string) ListEntry {
	name := filepath.Base(rel)
	kind := "reserved"
	title := deriveTitle(rel)
	if strings.EqualFold(name, "index.md") {
		kind = "index"
		title = "Index"
	}
	if strings.EqualFold(name, "log.md") {
		kind = "log"
		title = "Log"
	}

	return ListEntry{
		ID:       trimMarkdownExtension(rel),
		Path:     rel,
		Kind:     kind,
		Reserved: true,
		Title:    title,
	}
}

func isReserved(path string) bool {
	name := filepath.Base(path)
	return strings.EqualFold(name, "index.md") || strings.EqualFold(name, "log.md")
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
