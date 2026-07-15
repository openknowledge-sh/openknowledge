package okf

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
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
	source := RegistrySource{
		Type:        "manifest",
		URL:         "https://example.test/wiki/",
		Ref:         "https://cdn.example.test/bundle.tar.gz",
		ResolvedURL: "https://cdn.example.test/openknowledge.json",
		ManifestURL: "https://cdn.example.test/openknowledge.json",
		ArchiveURL:  "https://cdn.example.test/bundle.tar.gz",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Spec:        "0.1",
		FetchedAt:   "2026-07-15T12:00:00Z",
		ManagedRoot: root,
	}
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
	if connection.Name != entry.Name || connection.Source != source || !connection.Managed {
		t.Fatalf("unexpected stored connection: %#v", connection)
	}
}

func TestRemoveRegistryEntryRequireManagedIsTransactional(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	root := t.TempDir()
	entry, _, err := ConnectRegistryEntry("personal", root, "read", true)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok, err := RemoveRegistryEntryWithOptions("personal", RemoveRegistryOptions{RequireManaged: true}); err == nil || ok {
		t.Fatalf("expected managed-only removal to refuse local entry, ok=%t err=%v", ok, err)
	}
	remaining, ok, err := ResolveRegistryEntry("personal")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || remaining != entry {
		t.Fatalf("expected refused entry to remain unchanged, ok=%t entry=%#v", ok, remaining)
	}
}

func TestRegistryConcurrentProcessesPreserveEveryConnection(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)
	const processCount = 16

	commands := make([]*exec.Cmd, 0, processCount)
	for index := 0; index < processCount; index++ {
		root := filepath.Join(t.TempDir(), "bundle")
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		command := exec.Command(os.Args[0], "-test.run=^TestRegistryConnectHelper$")
		command.Env = append(os.Environ(),
			"OPENKNOWLEDGE_REGISTRY_TEST_HELPER=1",
			RegistryFileEnv+"="+registryFile,
			fmt.Sprintf("OPENKNOWLEDGE_REGISTRY_TEST_NAME=bundle-%02d", index),
			"OPENKNOWLEDGE_REGISTRY_TEST_PATH="+root,
		)
		commands = append(commands, command)
	}

	var waitGroup sync.WaitGroup
	errors := make(chan error, processCount)
	for _, command := range commands {
		waitGroup.Add(1)
		go func(command *exec.Cmd) {
			defer waitGroup.Done()
			if output, err := command.CombinedOutput(); err != nil {
				errors <- fmt.Errorf("helper failed: %w\n%s", err, output)
			}
		}(command)
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if t.Failed() {
		return
	}

	content, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatal(err)
	}
	var stored Registry
	if err := json.Unmarshal(content, &stored); err != nil {
		t.Fatalf("registry must remain valid JSON: %v\n%s", err, content)
	}
	entries, err := RegistryEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != processCount {
		t.Fatalf("expected %d concurrent entries, got %d: %#v", processCount, len(entries), entries)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(registryFile)
		if err != nil {
			t.Fatal(err)
		}
		if permissions := info.Mode().Perm(); permissions != 0600 {
			t.Fatalf("expected owner-only registry permissions, got %04o", permissions)
		}
	}

	matches, err := filepath.Glob(registryFile + "*")
	if err != nil {
		t.Fatal(err)
	}
	for _, match := range matches {
		if match != registryFile && match != registryFile+".lock" {
			t.Fatalf("unexpected registry temporary file left behind: %s", match)
		}
	}
}

func TestRegistryConnectHelper(t *testing.T) {
	if os.Getenv("OPENKNOWLEDGE_REGISTRY_TEST_HELPER") != "1" {
		return
	}
	name := os.Getenv("OPENKNOWLEDGE_REGISTRY_TEST_NAME")
	path := os.Getenv("OPENKNOWLEDGE_REGISTRY_TEST_PATH")
	if _, _, err := ConnectRegistryEntry(name, path, "read", true); err != nil {
		t.Fatal(err)
	}
}
