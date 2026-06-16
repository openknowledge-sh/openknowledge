package okf

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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

	return listInventory(validation.Root, validation.Errors)
}

func listInventory(absolute string, issues []Issue) (ListResult, error) {
	issuesByPath := groupIssuesByPath(issues)
	var entries []ListEntry
	err := filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isMarkdown(path) || isReserved(path) {
			if !isMarkdown(path) {
				return nil
			}
			rel := relPath(absolute, path)
			entries = append(entries, attachIssues(reservedEntry(rel), issuesByPath))
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel := relPath(absolute, path)
		meta, _, err := splitFrontmatter(string(content))
		if err != nil {
			entries = append(entries, attachIssues(conceptEntry(rel, frontmatter{}), issuesByPath))
			return nil
		}
		entries = append(entries, attachIssues(conceptEntry(rel, meta), issuesByPath))
		return nil
	})
	if err != nil {
		return ListResult{}, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
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
