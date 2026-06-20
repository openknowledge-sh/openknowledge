package okf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryConnectListAndResolve(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	root := t.TempDir()
	entry, warning, err := ConnectRegistryEntry("personal", root, "read", true)
	if err != nil {
		t.Fatal(err)
	}
	if warning != "" {
		t.Fatalf("did not expect connection warning, got %q", warning)
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
	if _, _, err := ConnectRegistryEntry("personal", root, "read", true); err != nil {
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

func TestRegistryRejectsPathLikeKeys(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	if _, _, err := ConnectRegistryEntry("./personal", t.TempDir(), "read", true); err == nil {
		t.Fatal("expected path-like registry key to fail")
	}
}

func TestConnectRegistryEntryAddsAccessAndAvoidsImplicitKeyCollision(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	firstRoot := t.TempDir()
	secondRoot := t.TempDir()

	first, warning, err := ConnectRegistryEntry("personal", firstRoot, "write", false)
	if err != nil {
		t.Fatal(err)
	}
	if warning != "" {
		t.Fatalf("did not expect first connection warning, got %q", warning)
	}
	if first.Name != "personal" || first.Path != firstRoot || first.Access != "write" {
		t.Fatalf("unexpected first connection: %#v", first)
	}

	second, warning, err := ConnectRegistryEntry("personal", secondRoot, "read", false)
	if err != nil {
		t.Fatal(err)
	}
	if second.Name != "personal-2" || second.Path != secondRoot || second.Access != "read" {
		t.Fatalf("unexpected second connection: %#v", second)
	}
	if warning == "" {
		t.Fatal("expected implicit key collision warning")
	}
}

func TestConnectRegistryEntryRejectsExplicitKeyCollision(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	if _, _, err := ConnectRegistryEntry("personal", t.TempDir(), "read", true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ConnectRegistryEntry("personal", t.TempDir(), "read", true); err == nil {
		t.Fatal("expected explicit key collision to fail")
	}
}

func TestRemoveRegistryEntryByNameAndPath(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	first, _, err := ConnectRegistryEntry("first", firstRoot, "read", true)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := ConnectRegistryEntry("second", secondRoot, "read", true)
	if err != nil {
		t.Fatal(err)
	}

	removed, ok, err := RemoveRegistryEntry("first")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || removed != first {
		t.Fatalf("unexpected removed entry by name: ok=%v entry=%#v", ok, removed)
	}

	removed, ok, err = RemoveRegistryEntry(second.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || removed != second {
		t.Fatalf("unexpected removed entry by path: ok=%v entry=%#v", ok, removed)
	}

	entries, err := RegistryEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty registry, got %#v", entries)
	}
}

func TestRegistrySavesPathKeyedConnections(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	root := t.TempDir()
	source := RegistrySource{Type: "git", URL: "https://example.com/wiki.git"}
	entry, _, err := ConnectRegistryEntryWithSource("personal", root, "read", true, source)
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatal(err)
	}
	var stored struct {
		Connections map[string]RegistryConnection `json:"connections"`
	}
	if err := json.Unmarshal(content, &stored); err != nil {
		t.Fatal(err)
	}
	connection, ok := stored.Connections[root]
	if !ok {
		t.Fatalf("expected path-keyed connection for %s in %#v", root, stored.Connections)
	}
	if connection.Name != entry.Name || connection.Source.URL != source.URL || !connection.Managed {
		t.Fatalf("unexpected stored connection: %#v", connection)
	}
}
