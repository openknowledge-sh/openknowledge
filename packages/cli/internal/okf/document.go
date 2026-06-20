package okf

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type parsedDocument struct {
	Absolute       string
	Rel            string
	Raw            []byte
	Content        string
	Frontmatter    frontmatter
	Body           string
	ReadErr        error
	FrontmatterErr error
}

type parsedBundle struct {
	Root        string
	SpecVersion string
	Documents   []parsedDocument
}

func parseMarkdownDocuments(root string) ([]parsedDocument, error) {
	var documents []parsedDocument
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

		content, err := os.ReadFile(path)
		document := parsedDocument{
			Absolute: path,
			Rel:      relPath(root, path),
			Raw:      content,
			ReadErr:  err,
		}
		if err != nil {
			documents = append(documents, document)
			return nil
		}
		meta, body, frontmatterErr := splitFrontmatter(string(content))
		document.Content = string(content)
		document.Frontmatter = meta
		document.Body = body
		document.FrontmatterErr = frontmatterErr
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
