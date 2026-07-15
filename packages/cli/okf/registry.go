package okf

import core "github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"

// RegistryFile returns the active user registry path without creating or
// modifying it.
func RegistryFile() (string, error) {
	return core.RegistryFile()
}

// RegistryEntries loads the strict bounded registry and returns its sorted,
// normalized connection inventory.
func RegistryEntries() ([]RegistryEntry, error) {
	return core.RegistryEntries()
}

// ResolveRegistryEntry resolves one exact registry key.
func ResolveRegistryEntry(name string) (RegistryEntry, bool, error) {
	return core.ResolveRegistryEntry(name)
}

// ResolveRegistryTarget resolves either a registry key or connected path.
func ResolveRegistryTarget(target string) (RegistryEntry, bool, error) {
	return core.ResolveRegistryTarget(target)
}

// ResolveKnowledgeRoot resolves a registry key or local path to a normalized
// local knowledge root.
func ResolveKnowledgeRoot(value string) (string, error) {
	return core.ResolveKnowledgeRoot(value)
}

// RegistryEntryCanWrite reports the effective authoring capability of an
// already loaded registry entry.
func RegistryEntryCanWrite(entry RegistryEntry) bool {
	return core.RegistryEntryCanWrite(entry)
}

// RegistryPathCanWrite reports the effective authoring capability for a local
// path. Unregistered paths remain writable.
func RegistryPathCanWrite(path string) (bool, error) {
	return core.RegistryPathCanWrite(path)
}

// RequireRegistryWriteAccess returns an error when path is inside a registered
// read-only knowledge base. It performs no mutation.
func RequireRegistryWriteAccess(path string) error {
	return core.RequireRegistryWriteAccess(path)
}
