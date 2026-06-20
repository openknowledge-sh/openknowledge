package okf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const RegistryFileEnv = "OPENKNOWLEDGE_REGISTRY_FILE"

var registryNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Registry struct {
	Entries []RegistryEntry `json:"entries"`
}

type RegistryEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Access  string `json:"access,omitempty"`
	Managed bool   `json:"managed,omitempty"`
}

func RegistryFile() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(RegistryFileEnv)); configured != "" {
		return ExpandUserPath(configured)
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "openknowledge", "registry.json"), nil
}

func LoadRegistry() (Registry, error) {
	path, err := RegistryFile()
	if err != nil {
		return Registry{}, err
	}

	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Registry{}, nil
	}
	if err != nil {
		return Registry{}, err
	}

	var registry Registry
	if err := json.Unmarshal(content, &registry); err != nil {
		return Registry{}, err
	}
	sortRegistryEntries(registry.Entries)
	return registry, nil
}

func RegistryEntries() ([]RegistryEntry, error) {
	registry, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	entries := append([]RegistryEntry{}, registry.Entries...)
	sortRegistryEntries(entries)
	return entries, nil
}

func AddRegistryEntry(name string, path string) (RegistryEntry, error) {
	name = strings.TrimSpace(name)
	if !validRegistryName(name) {
		return RegistryEntry{}, fmt.Errorf("registry name must use letters, numbers, dots, underscores, or dashes and must not look like a path")
	}

	absolute, err := absoluteDirectory(path)
	if err != nil {
		return RegistryEntry{}, err
	}

	registry, err := LoadRegistry()
	if err != nil {
		return RegistryEntry{}, err
	}

	entry := RegistryEntry{Name: name, Path: absolute}
	replaced := false
	for index := range registry.Entries {
		if registry.Entries[index].Name == name {
			registry.Entries[index] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		registry.Entries = append(registry.Entries, entry)
	}
	sortRegistryEntries(registry.Entries)

	if err := saveRegistry(registry); err != nil {
		return RegistryEntry{}, err
	}
	return entry, nil
}

func ConnectRegistryEntry(name string, path string, access string, explicitName bool) (RegistryEntry, string, error) {
	name = strings.TrimSpace(name)
	access = strings.TrimSpace(access)
	if access == "" {
		access = "read"
	}
	if access != "read" && access != "write" {
		return RegistryEntry{}, "", fmt.Errorf("access must be read or write")
	}
	if explicitName {
		if !validRegistryName(name) {
			return RegistryEntry{}, "", fmt.Errorf("connection key must use letters, numbers, dots, underscores, or dashes and must not look like a path")
		}
	} else {
		name = registryKeyFromName(name)
	}

	absolute, err := absoluteDirectory(path)
	if err != nil {
		return RegistryEntry{}, "", err
	}

	registry, err := LoadRegistry()
	if err != nil {
		return RegistryEntry{}, "", err
	}

	if index := registryEntryIndexByPath(registry.Entries, absolute); index >= 0 {
		entry := registry.Entries[index]
		if explicitName && entry.Name != name {
			if existing := registryEntryIndexByName(registry.Entries, name); existing >= 0 && registry.Entries[existing].Path != absolute {
				return RegistryEntry{}, "", fmt.Errorf("connection key %q already points to %s", name, registry.Entries[existing].Path)
			}
			entry.Name = name
		}
		entry.Access = access
		registry.Entries[index] = entry
		sortRegistryEntries(registry.Entries)
		if err := saveRegistry(registry); err != nil {
			return RegistryEntry{}, "", err
		}
		return entry, "", nil
	}

	warning := ""
	if explicitName {
		if existing := registryEntryIndexByName(registry.Entries, name); existing >= 0 {
			return RegistryEntry{}, "", fmt.Errorf("connection key %q already points to %s", name, registry.Entries[existing].Path)
		}
	} else {
		base := name
		for suffix := 2; registryEntryIndexByName(registry.Entries, name) >= 0; suffix++ {
			name = fmt.Sprintf("%s-%d", base, suffix)
		}
		if name != base {
			warning = fmt.Sprintf("connection key %q already exists; using %q", base, name)
		}
	}

	entry := RegistryEntry{Name: name, Path: absolute, Access: access}
	registry.Entries = append(registry.Entries, entry)
	sortRegistryEntries(registry.Entries)

	if err := saveRegistry(registry); err != nil {
		return RegistryEntry{}, "", err
	}
	return entry, warning, nil
}

func ResolveRegistryEntry(name string) (RegistryEntry, bool, error) {
	registry, err := LoadRegistry()
	if err != nil {
		return RegistryEntry{}, false, err
	}
	for _, entry := range registry.Entries {
		if entry.Name == name {
			return entry, true, nil
		}
	}
	return RegistryEntry{}, false, nil
}

func ResolveKnowledgeRoot(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return ".", nil
	}

	expanded, err := ExpandUserPath(value)
	if err != nil {
		return "", err
	}
	if LooksLikePath(value) {
		return expanded, nil
	}
	if info, err := os.Stat(expanded); err == nil && info.IsDir() {
		return expanded, nil
	}

	entry, ok, err := ResolveRegistryEntry(value)
	if err != nil {
		return "", err
	}
	if ok {
		return entry.Path, nil
	}

	return expanded, nil
}

func ExpandUserPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func LooksLikePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return value == "." ||
		value == ".." ||
		strings.HasPrefix(value, "./") ||
		strings.HasPrefix(value, "../") ||
		strings.HasPrefix(value, "~/") ||
		strings.Contains(value, "/") ||
		strings.Contains(value, string(filepath.Separator)) ||
		filepath.IsAbs(value)
}

func validRegistryName(name string) bool {
	if !registryNamePattern.MatchString(name) {
		return false
	}
	return !LooksLikePath(name)
}

func registryKeyFromName(name string) string {
	name = strings.TrimSpace(name)
	if validRegistryName(name) {
		return name
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == '.' || r == '_':
			if builder.Len() > 0 {
				builder.WriteRune(r)
				lastDash = false
			}
		case r == '-' || strings.ContainsRune(" \t\n\r", r):
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		default:
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	key := strings.Trim(builder.String(), ".-_")
	if key == "" {
		return "knowledge"
	}
	if !validRegistryName(key) {
		return "knowledge"
	}
	return key
}

func absoluteDirectory(path string) (string, error) {
	expanded, err := ExpandUserPath(path)
	if err != nil {
		return "", err
	}
	if expanded == "" {
		return "", fmt.Errorf("path is required")
	}

	absolute, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", absolute)
	}
	return absolute, nil
}

func saveRegistry(registry Registry) error {
	path, err := RegistryFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func sortRegistryEntries(entries []RegistryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
}

func registryEntryIndexByName(entries []RegistryEntry, name string) int {
	for index, entry := range entries {
		if entry.Name == name {
			return index
		}
	}
	return -1
}

func registryEntryIndexByPath(entries []RegistryEntry, path string) int {
	for index, entry := range entries {
		if entry.Path == path {
			return index
		}
	}
	return -1
}
