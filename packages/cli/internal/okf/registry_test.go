package okf

import (
	"path/filepath"
	"testing"
)

func TestRegistryAddListAndResolve(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	root := t.TempDir()
	entry, err := AddRegistryEntry("personal", root)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Name != "personal" || entry.Path != root {
		t.Fatalf("unexpected registry entry: %#v", entry)
	}

	entries, err := RegistryEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0] != entry {
		t.Fatalf("unexpected registry entries: %#v", entries)
	}

	resolved, err := ResolveKnowledgeRoot("personal")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != root {
		t.Fatalf("expected alias to resolve to %s, got %s", root, resolved)
	}
}

func TestResolveKnowledgeRootKeepsExplicitPaths(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	root := t.TempDir()
	if _, err := AddRegistryEntry("personal", root); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveKnowledgeRoot("./personal")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "./personal" {
		t.Fatalf("expected explicit path to stay explicit, got %s", resolved)
	}
}

func TestRegistryRejectsPathLikeNames(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	if _, err := AddRegistryEntry("./personal", t.TempDir()); err == nil {
		t.Fatal("expected path-like registry name to fail")
	}
}
