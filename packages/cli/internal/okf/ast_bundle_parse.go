package okf

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// ParseAST returns the parsed OKF document model used as the source of truth
// for validation, linting, and exporter projections inside the CLI. This Go
// API is internal to the CLI package; normalized exporter DTOs and command
// output remain the stable external contracts.
func ParseAST(root string) (ASTBundle, error) {
	return ParseASTWithVersion(root, LatestSpecVersion)
}

// ParseASTWithVersion returns the parsed OKF document model for a specific
// supported spec version.
func ParseASTWithVersion(root string, version string) (ASTBundle, error) {
	return parseASTBundle(root, version)
}

func parseASTBundle(root string, version string) (ASTBundle, error) {
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
		SchemaVersion: MachineSchemaVersion,
		Root:          absolute,
		SpecVersion:   resolved,
		Documents:     documents,
	}, nil
}

func parseASTDocuments(root string) ([]ASTDocument, error) {
	var documents []ASTDocument
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
