package okf

import (
	"os"
	"path/filepath"
	"strings"
)

type Bundle struct {
	Root        string       `json:"root"`
	SpecVersion string       `json:"specVersion"`
	Files       []BundleFile `json:"files"`
	Issues      []Issue      `json:"issues,omitempty"`
}

type BundleFile struct {
	ID          string            `json:"id"`
	Path        string            `json:"path"`
	Kind        string            `json:"kind"`
	Reserved    bool              `json:"reserved,omitempty"`
	Type        string            `json:"type,omitempty"`
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Resource    string            `json:"resource,omitempty"`
	Frontmatter map[string]string `json:"frontmatter,omitempty"`
	Body        string            `json:"body"`
	Links       []Link            `json:"links,omitempty"`
	Issues      []Issue           `json:"issues,omitempty"`
}

type Link struct {
	Label      string `json:"label"`
	Href       string `json:"href"`
	Kind       string `json:"kind"`
	Line       int    `json:"line"`
	TargetPath string `json:"targetPath,omitempty"`
	TargetID   string `json:"targetId,omitempty"`
	Exists     bool   `json:"exists,omitempty"`
}

func ParseBundle(root string) (Bundle, error) {
	return ParseBundleWithVersion(root, LatestSpecVersion)
}

func ParseBundleWithVersion(root string, version string) (Bundle, error) {
	validation, err := ValidateWithVersion(root, version)
	if err != nil {
		return Bundle{}, err
	}

	issues := append([]Issue{}, validation.Errors...)
	issues = append(issues, validation.Warnings...)
	files, err := bundleFiles(validation.Root, issues)
	if err != nil {
		return Bundle{}, err
	}

	return Bundle{
		Root:        validation.Root,
		SpecVersion: validation.SpecVersion,
		Files:       files,
		Issues:      issues,
	}, nil
}

func bundleFiles(root string, issues []Issue) ([]BundleFile, error) {
	issuesByPath := groupIssuesByPath(issues)
	documents, err := parseMarkdownDocuments(root)
	if err != nil {
		return nil, err
	}

	files := make([]BundleFile, 0, len(documents))
	for _, document := range documents {
		files = append(files, bundleFile(root, document, issuesByPath[document.Rel]))
	}
	return files, nil
}

func bundleFile(root string, document parsedDocument, issues []Issue) BundleFile {
	entry := ListEntry{}
	if isReserved(document.Rel) {
		entry = reservedEntry(document.Rel)
	} else {
		entry = conceptEntry(document.Rel, document.Frontmatter)
	}

	return BundleFile{
		ID:          entry.ID,
		Path:        entry.Path,
		Kind:        entry.Kind,
		Reserved:    entry.Reserved,
		Type:        entry.Type,
		Title:       entry.Title,
		Description: entry.Description,
		Resource:    entry.Resource,
		Frontmatter: frontmatterValues(document.Frontmatter),
		Body:        document.Body,
		Links:       ExtractLinks(root, document.Rel, document.Content),
		Issues:      issues,
	}
}

func frontmatterValues(meta frontmatter) map[string]string {
	if !meta.has || len(meta.values) == 0 {
		return nil
	}

	values := make(map[string]string, len(meta.values))
	for key, value := range meta.values {
		values[key] = value
	}
	return values
}

func ShouldPublish(file BundleFile) bool {
	return strings.TrimSpace(strings.ToLower(file.Frontmatter["okf_publish"])) != "false"
}

func ExtractLinks(root string, rel string, content string) []Link {
	linkContent := maskFencedCode(content)
	var links []Link
	for _, match := range markdownLinkDetail.FindAllStringSubmatchIndex(linkContent, -1) {
		label := content[match[4]:match[5]]
		href := content[match[6]:match[7]]
		link := Link{
			Label: strings.TrimSpace(label),
			Href:  strings.TrimSpace(href),
			Kind:  linkKind(href),
			Line:  lineForOffset(content, match[0]),
		}

		targetRel := ""
		if link.Kind == "local" {
			targetRel = linkTargetRel(rel, href)
		}
		if targetRel != "" {
			link.Kind = "local"
			link.TargetPath = targetRel
			link.TargetID = trimMarkdownExtension(targetRel)
			targetPath := filepath.Join(root, filepath.FromSlash(targetRel))
			if insideRoot(root, targetPath) {
				if info, err := os.Stat(targetPath); err == nil {
					link.Exists = !info.IsDir()
				}
			}
		}

		links = append(links, link)
	}
	return links
}

func linkKind(href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "#") {
		return "anchor"
	}
	if shouldSkipLink(href) {
		return "external"
	}
	return "local"
}
