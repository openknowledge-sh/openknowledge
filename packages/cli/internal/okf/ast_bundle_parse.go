package okf

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

func parseBundleAST(root string, version string) (ASTBundle, error) {
	resolved, ok := ResolveSpecVersion(version)
	if !ok {
		return ASTBundle{}, fmt.Errorf("unsupported OKF spec version: %s", version)
	}

	absolute, err := filepath.Abs(root)
	if err != nil {
		return ASTBundle{}, err
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return ASTBundle{}, err
	}
	if !info.IsDir() {
		return ASTBundle{}, fmt.Errorf("%s is not a directory", absolute)
	}

	documents, err := parseASTDocuments(absolute)
	if err != nil {
		return ASTBundle{}, err
	}
	return ASTBundle{
		Root:        absolute,
		SpecVersion: resolved,
		Documents:   documents,
	}, nil
}

func parseASTDocuments(root string) ([]astDocument, error) {
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

		document := parseASTDocumentLinks(root, parseASTDocumentFile(path, relPath(root, path)))
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
