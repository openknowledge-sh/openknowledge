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
const RegistrySchemaVersion = "1"
const RegistryStorageSchemaID = "https://openknowledge.sh/schemas/cli/storage/v1/registry.schema.json"
const RemoteCacheSourceSchemaID = "https://openknowledge.sh/schemas/cli/storage/v1/cache-source.schema.json"
const MaxRegistryBytes int64 = 8 << 20

var registryNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// registryProcessLock complements the filesystem lock. Some operating systems
// scope advisory locks to a process instead of an individual file descriptor,
// so independent goroutines must also serialize registry transactions.
var registryProcessLock sync.Mutex

type Registry struct {
	SchemaVersion string                        `json:"schemaVersion,omitempty"`
	Connections   map[string]RegistryConnection `json:"connections"`
	Entries       []RegistryEntry               `json:"-"`
}

type RegistryEntry struct {
	Name    string         `json:"name"`
	Path    string         `json:"path"`
	Access  string         `json:"access,omitempty"`
	Managed bool           `json:"managed,omitempty"`
	Source  RegistrySource `json:"source,omitempty"`
}

type RegistryConnection struct {
	Name    string          `json:"key"`
	Access  string          `json:"access,omitempty"`
	Managed bool            `json:"managed,omitempty"`
	Source  *RegistrySource `json:"source,omitempty"`
}

type RegistrySource struct {
	Type          string `json:"type,omitempty"`
	URL           string `json:"url,omitempty"`
	Ref           string `json:"ref,omitempty"`
	ResolvedURL   string `json:"resolvedUrl,omitempty"`
	ManifestURL   string `json:"manifestUrl,omitempty"`
	ArchiveURL    string `json:"archiveUrl,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	ContentSHA256 string `json:"contentSha256,omitempty"`
	GitCommit     string `json:"gitCommit,omitempty"`
	GitRef        string `json:"gitRef,omitempty"`
	GitSubdir     string `json:"gitSubdir,omitempty"`
	Spec          string `json:"spec,omitempty"`
	FetchedAt     string `json:"fetchedAt,omitempty"`
	ManagedRoot   string `json:"managedRoot,omitempty"`
}

type RemoveRegistryOptions struct {
	RequireManaged bool
	ExpectedEntry  *RegistryEntry
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

	content, err := ReadFileAtMost(path, MaxRegistryBytes)
	if os.IsNotExist(err) {
		return Registry{}, nil
	}
	if err != nil {
		return Registry{}, err
	}

	var registry Registry
	if err := DecodeStrictJSON(content, &registry); err != nil {
		return Registry{}, fmt.Errorf("invalid Open Knowledge registry %s: %w", path, err)
	}
	if err := validateStoredRegistry(registry); err != nil {
		return Registry{}, fmt.Errorf("invalid Open Knowledge registry %s: %w", path, err)
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
	if access == "write" && registrySourceIsManaged(source) {
		return RegistryEntry{}, "", fmt.Errorf("managed remote connections are read-only")
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
	if registrySourceIsManaged(source) {
		if strings.TrimSpace(source.Type) == "" {
			source.Type = "unknown"
		}
		if strings.TrimSpace(source.ManagedRoot) == "" {
			source.ManagedRoot = absolute
		}
	}

	var connected RegistryEntry
	warning := ""
	err = mutateRegistry(func(registry *Registry) (bool, error) {
		if index := registryEntryIndexByPath(registry.Entries, absolute); index >= 0 {
			entry := registry.Entries[index]
			entrySource := source
			if registryEntryIsManaged(entry) && !registrySourceIsManaged(entrySource) {
				entrySource = entry.Source
			}
			if access == "write" && (registryEntryIsManaged(entry) || registrySourceIsManaged(entrySource)) {
				return false, fmt.Errorf("managed remote connections are read-only")
			}
			if explicitName && entry.Name != name {
				if existing := registryEntryIndexByName(registry.Entries, name); existing >= 0 && registry.Entries[existing].Path != absolute {
					return false, fmt.Errorf("connection key %q already points to %s", name, registry.Entries[existing].Path)
				}
				entry.Name = name
			}
			entry.Access = access
			entry.Managed = registrySourceIsManaged(entrySource)
			entry.Source = entrySource
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

		connected = RegistryEntry{Name: name, Path: absolute, Access: access, Managed: registrySourceIsManaged(source), Source: source}
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
		if options.ExpectedEntry != nil && entry != *options.ExpectedEntry {
			return false, fmt.Errorf("connection %q changed while it was being removed", entry.Name)
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

func ReplaceRegistryEntry(expected RegistryEntry, replacement RegistryEntry) (RegistryEntry, error) {
	if !validRegistryName(replacement.Name) {
		return RegistryEntry{}, fmt.Errorf("connection key must use letters, numbers, dots, underscores, or dashes and must not look like a path")
	}
	if replacement.Access == "" {
		replacement.Access = "read"
	}
	if replacement.Access != "read" && replacement.Access != "write" {
		return RegistryEntry{}, fmt.Errorf("access must be read or write")
	}
	if replacement.Access == "write" && registryEntryIsManaged(replacement) {
		return RegistryEntry{}, fmt.Errorf("managed remote connections are read-only")
	}
	absolute, err := absoluteDirectory(replacement.Path)
	if err != nil {
		return RegistryEntry{}, err
	}
	replacement.Path = absolute

	err = mutateRegistry(func(registry *Registry) (bool, error) {
		index := registryEntryIndexByPath(registry.Entries, expected.Path)
		if index < 0 || registry.Entries[index] != expected {
			return false, fmt.Errorf("connection %q changed while it was being replaced", expected.Name)
		}
		if registryEntryIsManaged(expected) && !registryEntryIsManaged(replacement) {
			return false, fmt.Errorf("managed source metadata cannot be removed during replacement")
		}
		if collision := registryEntryIndexByPath(registry.Entries, replacement.Path); collision >= 0 && collision != index {
			return false, fmt.Errorf("replacement path is already connected as %q", registry.Entries[collision].Name)
		}
		if collision := registryEntryIndexByName(registry.Entries, replacement.Name); collision >= 0 && collision != index {
			return false, fmt.Errorf("connection key %q already points to %s", replacement.Name, registry.Entries[collision].Path)
		}
		registry.Entries[index] = replacement
		sortRegistryEntries(registry.Entries)
		return true, nil
	})
	if err != nil {
		return RegistryEntry{}, err
	}
	return replacement, nil
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
	if err := validateStoredRegistry(registry); err != nil {
		return fmt.Errorf("refusing to write invalid Open Knowledge registry: %w", err)
	}
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

func validateStoredRegistry(registry Registry) error {
	if registry.SchemaVersion != "" && registry.SchemaVersion != RegistrySchemaVersion {
		return fmt.Errorf("unsupported registry schema version %q", registry.SchemaVersion)
	}
	current := registry.SchemaVersion == RegistrySchemaVersion
	if current && registry.Connections == nil {
		return fmt.Errorf("registry schema version %s requires connections", RegistrySchemaVersion)
	}
	names := make(map[string]string, len(registry.Connections))
	for storedPath, connection := range registry.Connections {
		if strings.TrimSpace(storedPath) == "" || storedPath != filepath.Clean(storedPath) || !filepath.IsAbs(storedPath) {
			return fmt.Errorf("connection path must be canonical and absolute: %q", storedPath)
		}
		if !validRegistryName(connection.Name) {
			return fmt.Errorf("connection at %s has invalid key %q", storedPath, connection.Name)
		}
		if previous, exists := names[connection.Name]; exists {
			return fmt.Errorf("connection key %q is duplicated for %s and %s", connection.Name, previous, storedPath)
		}
		names[connection.Name] = storedPath
		if current && connection.Access == "" {
			return fmt.Errorf("connection %q is missing access", connection.Name)
		}
		if connection.Access != "" && connection.Access != "read" && connection.Access != "write" {
			return fmt.Errorf("connection %q has invalid access %q", connection.Name, connection.Access)
		}
		if current && connection.Source != nil {
			source := *connection.Source
			if !connection.Managed {
				return fmt.Errorf("connection %q has source provenance but is not managed", connection.Name)
			}
			if source.Type != "" && source.Type != "manifest" && source.Type != "tar" && source.Type != "git" && source.Type != "unknown" {
				return fmt.Errorf("connection %q has invalid source type %q", connection.Name, source.Type)
			}
			if source.ManagedRoot != "" && (source.ManagedRoot != filepath.Clean(source.ManagedRoot) || !filepath.IsAbs(source.ManagedRoot)) {
				return fmt.Errorf("connection %q managed root must be canonical and absolute", connection.Name)
			}
			if source.ManagedRoot != "" {
				rel, err := filepath.Rel(source.ManagedRoot, storedPath)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return fmt.Errorf("connection %q path is outside its managed root", connection.Name)
				}
			}
		} else if current && connection.Managed {
			return fmt.Errorf("managed connection %q is missing source provenance", connection.Name)
		}
	}
	return nil
}

func normalizeRegistry(registry Registry) Registry {
	if len(registry.Connections) > 0 {
		registry.Entries = registry.Entries[:0]
		for path, connection := range registry.Connections {
			source := RegistrySource{}
			if connection.Source != nil {
				source = *connection.Source
			}
			registry.Entries = append(registry.Entries, RegistryEntry{
				Name:    connection.Name,
				Path:    path,
				Access:  connection.Access,
				Managed: connection.Managed,
				Source:  source,
			})
		}
	}
	if registry.Connections == nil {
		registry.Connections = map[string]RegistryConnection{}
	}
	for index := range registry.Entries {
		entry := &registry.Entries[index]
		entry.Managed = registryEntryIsManaged(*entry)
		if entry.Managed {
			if strings.TrimSpace(entry.Source.Type) == "" {
				entry.Source.Type = "unknown"
			}
			if strings.TrimSpace(entry.Source.ManagedRoot) == "" {
				entry.Source.ManagedRoot = entry.Path
			}
		}
		if entry.Access != "write" || entry.Managed {
			entry.Access = "read"
		}
	}
	return registry
}

// RegistryEntryCanWrite reports whether a connection grants local authoring
// access. Remote materializations are immutable cache generations regardless
// of legacy or forged registry access values.
func RegistryEntryCanWrite(entry RegistryEntry) bool {
	return entry.Access == "write" && !registryEntryIsManaged(entry)
}

// RequireRegistryWriteAccess refuses writes inside a registered read-only
// knowledge tree. Paths outside the registry are intentionally unaffected.
func RequireRegistryWriteAccess(path string) error {
	canWrite, entry, err := registryPathWriteAccess(path)
	if err != nil {
		return err
	}
	if canWrite {
		return nil
	}
	return fmt.Errorf("registered knowledge base %q is read-only; reconnect a local source with --access write to modify it", entry.Name)
}

// RegistryPathCanWrite reports whether a local authoring affordance may be
// exposed for a path. Unregistered paths remain writable.
func RegistryPathCanWrite(path string) (bool, error) {
	canWrite, _, err := registryPathWriteAccess(path)
	return canWrite, err
}

func registryPathWriteAccess(path string) (bool, RegistryEntry, error) {
	candidate, err := canonicalPathForRegistryAccess(path)
	if err != nil {
		return false, RegistryEntry{}, err
	}
	entries, err := RegistryEntries()
	if err != nil {
		return false, RegistryEntry{}, err
	}

	var matched *RegistryEntry
	matchedRootLength := -1
	for index := range entries {
		root, err := canonicalPathForRegistryAccess(entries[index].Path)
		if err != nil || !insideRoot(root, candidate) {
			continue
		}
		if len(root) > matchedRootLength {
			matched = &entries[index]
			matchedRootLength = len(root)
		}
	}
	if matched == nil || RegistryEntryCanWrite(*matched) {
		return true, RegistryEntry{}, nil
	}
	return false, *matched, nil
}

func canonicalPathForRegistryAccess(path string) (string, error) {
	expanded, err := ExpandUserPath(path)
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}

	current := absolute
	tail := []string{}
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for index := len(tail) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, tail[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		tail = append(tail, filepath.Base(current))
		current = parent
	}
}

func registrySourceIsManaged(source RegistrySource) bool {
	return strings.TrimSpace(source.Type) != "" ||
		strings.TrimSpace(source.URL) != "" ||
		strings.TrimSpace(source.ManagedRoot) != ""
}

func registryEntryIsManaged(entry RegistryEntry) bool {
	return entry.Managed || registrySourceIsManaged(entry.Source)
}

func registryForStorage(registry Registry) Registry {
	connections := make(map[string]RegistryConnection, len(registry.Entries))
	for _, entry := range registry.Entries {
		if strings.TrimSpace(entry.Path) == "" {
			continue
		}
		var source *RegistrySource
		if registrySourceIsManaged(entry.Source) {
			value := entry.Source
			source = &value
		}
		connections[entry.Path] = RegistryConnection{
			Name:    entry.Name,
			Access:  entry.Access,
			Managed: entry.Managed,
			Source:  source,
		}
	}
	return Registry{SchemaVersion: RegistrySchemaVersion, Connections: connections}
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
