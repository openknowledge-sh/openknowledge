package okf

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type BundleInfo struct {
	Root        string
	Metadata    BundleMetadata
	RootTitle   string
	HasIndex    bool
	HasMetadata bool
}

func ReadBundleInfo(root string) (BundleInfo, error) {
	info := BundleInfo{Root: root}
	content, err := os.ReadFile(filepath.Join(root, "index.md"))
	if os.IsNotExist(err) {
		return info, nil
	}
	if err != nil {
		return info, err
	}

	info.HasIndex = true
	meta, body, err := splitFrontmatter(string(content))
	if err != nil {
		return info, err
	}

	info.RootTitle = firstH1(body)
	if meta.has {
		info.Metadata = bundleMetadataFromFrontmatter(meta.values)
		info.HasMetadata = hasBundleMetadata(info.Metadata)
	}
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

func bundleMetadataFromFrontmatter(values map[string]string) BundleMetadata {
	metadata := BundleMetadata{
		Name:    strings.TrimSpace(values["okf_bundle_name"]),
		Title:   strings.TrimSpace(values["okf_bundle_title"]),
		Purpose: strings.TrimSpace(values["okf_bundle_purpose"]),
		Tags:    parseFlowStringList(values["okf_bundle_tags"]),
	}

	for key, value := range values {
		name, ok := strings.CutPrefix(key, "okf_bundle_entry_")
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		metadata.Entries = append(metadata.Entries, BundleEntry{
			Name: strings.TrimSpace(name),
			Path: strings.TrimSpace(value),
		})
	}
	sort.Slice(metadata.Entries, func(i, j int) bool {
		if metadata.Entries[i].Name == "default" {
			return true
		}
		if metadata.Entries[j].Name == "default" {
			return false
		}
		return metadata.Entries[i].Name < metadata.Entries[j].Name
	})
	return metadata
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
