package okf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func ReadBundleInfo(root string) (BundleInfo, error) {
	info := BundleInfo{Root: root}
	indexPath, err := ResolveBundlePath(root, "index.md")
	if errors.Is(err, os.ErrNotExist) {
		return info, nil
	}
	if err != nil {
		return info, err
	}
	document := parseASTDocumentFile(indexPath, "index.md")
	if document.ReadDiagnostic != nil && errors.Is(document.ReadDiagnostic, os.ErrNotExist) {
		return info, nil
	}
	if document.ReadDiagnostic != nil {
		return info, document.ReadDiagnostic
	}

	info.HasIndex = true
	if document.FrontmatterDiagnostic != nil {
		return info, document.FrontmatterDiagnostic
	}

	info.RootTitle = firstASTMarkdownH1(document.Markdown)
	info.Metadata = document.Metadata.Bundle
	info.HasMetadata = hasBundleMetadata(info.Metadata)
	return info, nil
}

func (info BundleInfo) DisplayName() string {
	for _, value := range []string{
		info.Metadata.Title,
		info.RootTitle,
		info.Metadata.Name,
		titleFromFileName(filepath.Base(filepath.Clean(info.Root))),
	} {
		value = strings.TrimSpace(value)
		if value != "" && value != "." && value != string(filepath.Separator) {
			return value
		}
	}
	return "Open Knowledge"
}

func (info BundleInfo) EntryNames() []string {
	names := make([]string, 0, len(info.Metadata.Entries))
	for _, entry := range info.Metadata.Entries {
		names = append(names, entry.Name)
	}
	return names
}

func (info BundleInfo) EntryPath(name string) (string, bool) {
	for _, entry := range info.Metadata.Entries {
		if entry.Name == name {
			return entry.Path, true
		}
	}
	return "", false
}

func ReadMarkdownDocumentInfo(path string, rel string) (MarkdownDocumentInfo, error) {
	info := MarkdownDocumentInfo{Path: rel}
	document := parseASTDocumentFile(path, rel)
	if document.ReadDiagnostic != nil {
		return info, document.ReadDiagnostic
	}
	if document.FrontmatterDiagnostic != nil {
		return info, document.FrontmatterDiagnostic
	}
	info.Type = strings.TrimSpace(document.Metadata.Type)
	info.Title = strings.TrimSpace(document.Metadata.Title)
	info.Description = strings.TrimSpace(document.Metadata.Description)
	info.Tags = document.Metadata.Tags
	info.UseWhen = document.Metadata.UseWhen
	return info, nil
}

func firstASTMarkdownH1(markdown ASTMarkdown) string {
	for _, heading := range markdown.Headings {
		if heading.Level == 1 {
			return strings.TrimSpace(heading.Text)
		}
	}
	return ""
}

func titleFromFileName(name string) string {
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	words := strings.Fields(name)
	for index, word := range words {
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
