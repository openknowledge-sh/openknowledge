package okf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/gofrs/flock"
	"github.com/natefinch/atomic"
)

const RegistryFileEnv = "OPENKNOWLEDGE_REGISTRY_FILE"

var registryNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// registryProcessLock complements the filesystem lock. Some operating systems
// scope advisory locks to a process instead of an individual file descriptor,
// so independent goroutines must also serialize registry transactions.
var registryProcessLock sync.Mutex

type Registry struct {
	Connections map[string]RegistryConnection `json:"connections,omitempty"`
	Entries     []RegistryEntry               `json:"-"`
}

type RegistryEntry struct {
	Name    string         `json:"name"`
	Path    string         `json:"path"`
	Access  string         `json:"access,omitempty"`
	Managed bool           `json:"managed,omitempty"`
	Source  RegistrySource `json:"source,omitempty"`
}

type RegistryConnection struct {
	Name    string         `json:"key"`
	Access  string         `json:"access,omitempty"`
	Managed bool           `json:"managed,omitempty"`
	Source  RegistrySource `json:"source,omitempty"`
}

type RegistrySource struct {
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
	Ref  string `json:"ref,omitempty"`
}

type RemoveRegistryOptions struct {
	RequireManaged bool
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
	registry = normalizeRegistry(registry)
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

func ConnectRegistryEntry(name string, path string, access string, explicitName bool) (RegistryEntry, string, error) {
	return ConnectRegistryEntryWithSource(name, path, access, explicitName, RegistrySource{})
}

func ConnectRegistryEntryWithSource(name string, path string, access string, explicitName bool, source RegistrySource) (RegistryEntry, string, error) {
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

	var connected RegistryEntry
	warning := ""
	err = mutateRegistry(func(registry *Registry) (bool, error) {
		if index := registryEntryIndexByPath(registry.Entries, absolute); index >= 0 {
			entry := registry.Entries[index]
			if explicitName && entry.Name != name {
				if existing := registryEntryIndexByName(registry.Entries, name); existing >= 0 && registry.Entries[existing].Path != absolute {
					return false, fmt.Errorf("connection key %q already points to %s", name, registry.Entries[existing].Path)
				}
				entry.Name = name
			}
			entry.Access = access
			entry.Managed = source.URL != ""
			entry.Source = source
			registry.Entries[index] = entry
			sortRegistryEntries(registry.Entries)
			connected = entry
			return true, nil
		}

		if explicitName {
			if existing := registryEntryIndexByName(registry.Entries, name); existing >= 0 {
				return false, fmt.Errorf("connection key %q already points to %s", name, registry.Entries[existing].Path)
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

		connected = RegistryEntry{Name: name, Path: absolute, Access: access, Managed: source.URL != "", Source: source}
		registry.Entries = append(registry.Entries, connected)
		sortRegistryEntries(registry.Entries)
		return true, nil
	})
	if err != nil {
		return RegistryEntry{}, "", err
	}
	return connected, warning, nil
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

func ResolveRegistryTarget(target string) (RegistryEntry, bool, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return RegistryEntry{}, false, fmt.Errorf("connection key or path is required")
	}

	registry, err := LoadRegistry()
	if err != nil {
		return RegistryEntry{}, false, err
	}

	return resolveRegistryTargetIn(registry, target)
}

func resolveRegistryTargetIn(registry Registry, target string) (RegistryEntry, bool, error) {
	if !LooksLikePath(target) {
		if index := registryEntryIndexByName(registry.Entries, target); index >= 0 {
			return registry.Entries[index], true, nil
		}
		return RegistryEntry{}, false, nil
	}

	expanded, err := ExpandUserPath(target)
	if err != nil {
		return RegistryEntry{}, false, err
	}
	absolute, err := filepath.Abs(expanded)
	if err != nil {
		return RegistryEntry{}, false, err
	}
	if index := registryEntryIndexByPath(registry.Entries, absolute); index >= 0 {
		return registry.Entries[index], true, nil
	}
	return RegistryEntry{}, false, nil
}

func RemoveRegistryEntry(target string) (RegistryEntry, bool, error) {
	return RemoveRegistryEntryWithOptions(target, RemoveRegistryOptions{})
}

func RemoveRegistryEntryWithOptions(target string, options RemoveRegistryOptions) (RegistryEntry, bool, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return RegistryEntry{}, false, fmt.Errorf("connection key or path is required")
	}

	var removed RegistryEntry
	found := false
	err := mutateRegistry(func(registry *Registry) (bool, error) {
		entry, ok, err := resolveRegistryTargetIn(*registry, target)
		if err != nil || !ok {
			return false, err
		}
		if options.RequireManaged && !entry.Managed {
			return false, fmt.Errorf("refusing to delete non-managed files: %s", entry.Path)
		}

		index := registryEntryIndexByPath(registry.Entries, entry.Path)
		if index < 0 {
			return false, nil
		}
		registry.Entries = append(registry.Entries[:index], registry.Entries[index+1:]...)
		sortRegistryEntries(registry.Entries)
		removed = entry
		found = true
		return true, nil
	})
	if err != nil {
		return RegistryEntry{}, false, err
	}
	return removed, found, nil
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

func RegistryKeyFromNameForCache(name string) string {
	key := registryKeyFromName(name)
	if key == "knowledge" && strings.TrimSpace(name) == "" {
		return ""
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

func mutateRegistry(mutate func(*Registry) (bool, error)) (resultErr error) {
	registryProcessLock.Lock()
	defer registryProcessLock.Unlock()

	path, err := RegistryFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	lock := flock.New(path+".lock", flock.SetPermissions(0600))
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("lock registry: %w", err)
	}
	defer func() {
		if err := lock.Close(); err != nil && resultErr == nil {
			resultErr = fmt.Errorf("unlock registry: %w", err)
		}
	}()

	registry, err := LoadRegistry()
	if err != nil {
		return err
	}
	changed, err := mutate(&registry)
	if err != nil || !changed {
		return err
	}
	return saveRegistry(registry)
}

func saveRegistry(registry Registry) error {
	path, err := RegistryFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	registry = registryForStorage(registry)
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.Chmod(path, 0600); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := atomic.WriteFile(path, bytes.NewReader(data)); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func normalizeRegistry(registry Registry) Registry {
	if len(registry.Connections) > 0 {
		registry.Entries = registry.Entries[:0]
		for path, connection := range registry.Connections {
			registry.Entries = append(registry.Entries, RegistryEntry{
				Name:    connection.Name,
				Path:    path,
				Access:  connection.Access,
				Managed: connection.Managed,
				Source:  connection.Source,
			})
		}
	}
	if registry.Connections == nil {
		registry.Connections = map[string]RegistryConnection{}
	}
	return registry
}

func registryForStorage(registry Registry) Registry {
	connections := make(map[string]RegistryConnection, len(registry.Entries))
	for _, entry := range registry.Entries {
		if strings.TrimSpace(entry.Path) == "" {
			continue
		}
		connections[entry.Path] = RegistryConnection{
			Name:    entry.Name,
			Access:  entry.Access,
			Managed: entry.Managed,
			Source:  entry.Source,
		}
	}
	return Registry{Connections: connections}
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
