package okf

import "strings"

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

func ParseBundle(root string) (Bundle, error) {
	return ParseBundleWithVersion(root, LatestSpecVersion)
}

func ParseBundleWithVersion(root string, version string) (Bundle, error) {
	validation, ast, err := parseAndValidateBundle(root, version)
	if err != nil {
		return Bundle{}, err
	}

	return bundleFromAST(validation, ast)
}

func bundleFromAST(validation Result, ast astBundle) (Bundle, error) {
	issues := issuesFromResult(validation)
	files, err := bundleFilesFromAST(ast, issues)
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

func bundleFilesFromAST(bundle astBundle, issues []Issue) ([]BundleFile, error) {
	issuesByPath := groupIssuesByPath(issues)
	files := make([]BundleFile, 0, len(bundle.Documents))
	for _, document := range bundle.Documents {
		if document.ReadErr != nil {
			return nil, document.ReadErr
		}
		files = append(files, bundleFile(document, issuesByPath[document.Rel]))
	}
	return files, nil
}

func bundleFile(document astDocument, issues []Issue) BundleFile {
	summary := summarizeASTDocument(document, document.Metadata)

	return BundleFile{
		ID:          summary.ID,
		Path:        summary.Path,
		Kind:        summary.Kind,
		Reserved:    summary.Reserved,
		Type:        summary.Type,
		Title:       summary.Title,
		Description: summary.Description,
		Resource:    summary.Resource,
		Frontmatter: document.FrontmatterValues,
		Body:        document.Body,
		Links:       document.Links,
		Issues:      issues,
	}
}

func ShouldPublish(file BundleFile) bool {
	return strings.TrimSpace(strings.ToLower(file.Frontmatter["okf_publish"])) != "false"
}
