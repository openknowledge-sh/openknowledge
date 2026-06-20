package okf

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type astDocument struct {
	Absolute          string
	Rel               string
	ID                string
	Kind              string
	Reserved          bool
	Raw               []byte
	Content           string
	Frontmatter       frontmatter
	FrontmatterValues map[string]string
	Metadata          astDocumentMetadata
	Body              string
	Links             []Link
	ReadErr           error
	FrontmatterErr    error
}

type astDocumentMetadata struct {
	Type        string
	Title       string
	Description string
	Resource    string
	Tags        []string
	UseWhen     []string
	Bundle      BundleMetadata
}

type astBundle struct {
	Root        string
	SpecVersion string
	Documents   []astDocument
}

func parseMarkdownDocuments(root string) ([]astDocument, error) {
	var documents []astDocument
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
		if !isMarkdown(path) {
			return nil
		}

		document := parseMarkdownDocumentFile(path, relPath(root, path))
		if document.ReadErr == nil {
			document.Links = ExtractLinks(root, document.Rel, document.Content)
		}
		documents = append(documents, document)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(documents, func(i, j int) bool {
		return documents[i].Rel < documents[j].Rel
	})
	return documents, nil
}

func parseMarkdownDocumentFile(path string, rel string) astDocument {
	content, err := os.ReadFile(path)
	id, kind, reserved := classifyDocument(rel)
	document := astDocument{
		Absolute: path,
		Rel:      rel,
		ID:       id,
		Kind:     kind,
		Reserved: reserved,
		Raw:      content,
		ReadErr:  err,
	}
	if err != nil {
		return document
	}

	meta, body, frontmatterErr := splitFrontmatter(string(content))
	document.Content = string(content)
	document.Frontmatter = meta
	document.FrontmatterValues = frontmatterValues(meta)
	document.Metadata = astDocumentMetadataFromValues(document.FrontmatterValues)
	document.Body = body
	document.FrontmatterErr = frontmatterErr
	return document
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

func astDocumentMetadataFromValues(values map[string]string) astDocumentMetadata {
	return astDocumentMetadata{
		Type:        values["type"],
		Title:       values["title"],
		Description: values["description"],
		Resource:    values["resource"],
		Tags:        parseFlowStringList(values["tags"]),
		UseWhen:     parseFlowStringList(values["use_when"]),
		Bundle:      bundleMetadataFromFrontmatter(values),
	}
}
