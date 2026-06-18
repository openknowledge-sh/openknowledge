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
	Name string `json:"name"`
	Path string `json:"path"`
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
