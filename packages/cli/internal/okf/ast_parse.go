package okf

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

func parseBundleAST(root string, version string) (astBundle, error) {
	resolved, ok := ResolveSpecVersion(version)
	if !ok {
		return astBundle{}, fmt.Errorf("unsupported OKF spec version: %s", version)
	}

	absolute, err := filepath.Abs(root)
	if err != nil {
		return astBundle{}, err
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return astBundle{}, err
	}
	if !info.IsDir() {
		return astBundle{}, fmt.Errorf("%s is not a directory", absolute)
	}

	documents, err := parseASTDocuments(absolute)
	if err != nil {
		return astBundle{}, err
	}
	return astBundle{
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

func parseASTDocumentFile(path string, rel string) astDocument {
	content, err := os.ReadFile(path)
	id, kind, reserved := classifyDocument(rel)
	document := astDocument{
		Absolute:       path,
		Rel:            rel,
		ID:             id,
		Kind:           kind,
		Reserved:       reserved,
		ReadDiagnostic: astReadDiagnostic(err),
	}
	if err != nil {
		return document
	}

	return parseASTDocumentContent(document, content)
}

func parseASTDocumentContent(document astDocument, content []byte) astDocument {
	document.UTF8Diagnostic = astUTF8Diagnostic(content)
	meta, body, frontmatterErr := splitFrontmatter(string(content))
	document.Content = string(content)
	document.Frontmatter = astFrontmatterFromParse(meta)
	document.Metadata = astDocumentMetadataFromValues(document.Frontmatter.Values)
	document.Body = body
	document.FrontmatterDiagnostic = astFrontmatterDiagnostic(frontmatterErr)
	return document
}

func parseASTDocumentLinks(root string, document astDocument) astDocument {
	if document.ReadDiagnostic != nil {
		return document
	}
	document.Links = ExtractLinks(root, document.Rel, document.Content)
	return document
}
