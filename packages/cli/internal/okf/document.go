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
	Content        string
	Frontmatter    frontmatter
	Body           string
	FrontmatterErr error
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
		if err != nil {
			return err
		}
		meta, body, frontmatterErr := splitFrontmatter(string(content))
		documents = append(documents, parsedDocument{
			Absolute:       path,
			Rel:            relPath(root, path),
			Content:        string(content),
			Frontmatter:    meta,
			Body:           body,
			FrontmatterErr: frontmatterErr,
		})
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
