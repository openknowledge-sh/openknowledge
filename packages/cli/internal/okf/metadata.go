package okf

import (
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
	content, err := os.ReadFile(filepath.Join(root, "index.md"))
	if err != nil && !os.IsNotExist(err) {
		return info, err
	}

	if err == nil {
		info.HasIndex = true
		_, body, err := splitFrontmatter(string(content))
		if err != nil {
			return info, err
		}
		info.RootTitle = firstH1(body)
	}

	config, err := ReadConfig(root)
	if err != nil {
		return info, err
	}
	info.Metadata = config.Bundle
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
	content, err := os.ReadFile(path)
	if err != nil {
		return info, err
	}
	meta, _, err := splitFrontmatter(string(content))
	if err != nil {
		return info, err
	}
	if !meta.has {
		return info, nil
	}
	info.Type = strings.TrimSpace(meta.values["type"])
	info.Title = strings.TrimSpace(meta.values["title"])
	info.Description = strings.TrimSpace(meta.values["description"])
	info.Tags = parseFlowStringList(meta.values["tags"])
	info.UseWhen = parseFlowStringList(meta.values["use_when"])
	return info, nil
}

func hasBundleMetadata(metadata BundleMetadata) bool {
	return metadata.Name != "" ||
		metadata.Title != "" ||
		metadata.Purpose != "" ||
		len(metadata.Tags) > 0 ||
		len(metadata.Entries) > 0
}

func parseFlowStringList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return compactStrings([]string{cleanFlowString(value)})
	}

	var values []string
	current := strings.Builder{}
	quote := rune(0)
	escaped := false
	for _, r := range strings.TrimSpace(value[1 : len(value)-1]) {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if quote == '"' && r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		if r == ',' {
			values = append(values, cleanFlowString(current.String()))
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	values = append(values, cleanFlowString(current.String()))
	return compactStrings(values)
}

func cleanFlowString(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
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
