package okf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type BundleInfo struct {
	Root        string
	Metadata    BundleMetadata
	RootTitle   string
	HasIndex    bool
	HasMetadata bool
}

type MarkdownDocumentInfo struct {
	Path        string
	Type        string
	Title       string
	Description string
	Tags        []string
	UseWhen     []string
}

func ReadBundleInfo(root string) (BundleInfo, error) {
	info := BundleInfo{Root: root}
	document := parseASTDocumentFile(filepath.Join(root, "index.md"), "index.md")
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

	info.RootTitle = firstH1(document.Body)
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

func firstH1(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "# ") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "# "))
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
