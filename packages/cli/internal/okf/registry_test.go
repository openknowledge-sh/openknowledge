package okf

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
		Type:          "manifest",
		URL:           "https://example.test/wiki/",
		Ref:           "https://cdn.example.test/bundle.tar.gz",
		ResolvedURL:   "https://cdn.example.test/openknowledge.json",
		ManifestURL:   "https://cdn.example.test/openknowledge.json",
		ArchiveURL:    "https://cdn.example.test/bundle.tar.gz",
		SHA256:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ContentSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Spec:          "0.1",
		FetchedAt:     "2026-07-15T12:00:00Z",
		ManagedRoot:   root,
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
		SchemaVersion string                        `json:"schemaVersion"`
		Connections   map[string]RegistryConnection `json:"connections"`
	}
	if err := json.Unmarshal(content, &stored); err != nil {
		t.Fatal(err)
	}
	connection, ok := stored.Connections[root]
	if !ok {
		t.Fatalf("expected path-keyed connection for %s in %#v", root, stored.Connections)
	}
	if connection.Name != entry.Name || connection.Source == nil || *connection.Source != source || !connection.Managed {
		t.Fatalf("unexpected stored connection: %#v", connection)
	}
	if stored.SchemaVersion != RegistrySchemaVersion {
		t.Fatalf("expected versioned registry storage, got %q", stored.SchemaVersion)
	}
}

func TestLoadRegistryRejectsAmbiguousOrInvalidStorage(t *testing.T) {
	root := t.TempDir()
	otherRoot := t.TempDir()
	quotedRoot := fmt.Sprintf("%q", root)
	quotedOtherRoot := fmt.Sprintf("%q", otherRoot)
	valid := `{"schemaVersion":"1","connections":{` + quotedRoot + `:{"key":"docs","access":"read"}}}`
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	if err := os.WriteFile(registryFile, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if registry, err := LoadRegistry(); err != nil || len(registry.Entries) != 1 {
		t.Fatalf("expected exact v1 registry to load, registry=%#v err=%v", registry, err)
	}
	legacy := strings.Replace(valid, `"schemaVersion":"1",`, "", 1)
	if err := os.WriteFile(registryFile, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(); err != nil {
		t.Fatalf("expected legacy unversioned registry to remain readable: %v", err)
	}
	if _, _, err := ConnectRegistryEntry("other", otherRoot, "read", true); err != nil {
		t.Fatalf("expected legacy registry migration write: %v", err)
	}
	migrated, err := os.ReadFile(registryFile)
	if err != nil || !strings.Contains(string(migrated), `"schemaVersion": "1"`) {
		t.Fatalf("legacy registry was not migrated to v1, content=%q err=%v", migrated, err)
	}

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{name: "unknown top-level", content: strings.TrimSuffix(valid, "}") + `,"extra":true}`, expected: "unknown field"},
		{name: "duplicate top-level", content: `{"schemaVersion":"1","schemaVersion":"1","connections":{}}`, expected: "duplicate field"},
		{name: "unknown connection", content: `{"schemaVersion":"1","connections":{` + quotedRoot + `:{"key":"docs","extra":true}}}`, expected: "unknown field"},
		{name: "duplicate nested", content: `{"schemaVersion":"1","connections":{` + quotedRoot + `:{"key":"docs","key":"other"}}}`, expected: "duplicate field"},
		{name: "unsupported version", content: `{"schemaVersion":"2","connections":{}}`, expected: "unsupported registry schema version"},
		{name: "relative path", content: `{"schemaVersion":"1","connections":{"relative":{"key":"docs"}}}`, expected: "canonical and absolute"},
		{name: "duplicate logical key", content: `{"schemaVersion":"1","connections":{` + quotedRoot + `:{"key":"docs","access":"read"},` + quotedOtherRoot + `:{"key":"docs","access":"read"}}}`, expected: "duplicated"},
		{name: "invalid access", content: `{"schemaVersion":"1","connections":{` + quotedRoot + `:{"key":"docs","access":"owner"}}}`, expected: "invalid access"},
		{name: "trailing JSON", content: valid + `{}`, expected: "trailing JSON"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(registryFile, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			before := append([]byte(nil), []byte(test.content)...)
			if _, err := LoadRegistry(); err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("expected %q error, got %v", test.expected, err)
			}
			if _, _, err := ConnectRegistryEntry("other", t.TempDir(), "read", true); err == nil {
				t.Fatal("registry mutation must refuse corrupt storage")
			}
			after, err := os.ReadFile(registryFile)
			if err != nil || string(after) != string(before) {
				t.Fatalf("refused mutation changed corrupt registry, content=%q err=%v", after, err)
			}
		})
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

func TestRemoveRegistryEntryRejectsChangedExpectedSnapshot(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	root := t.TempDir()
	original, _, err := ConnectRegistryEntryWithSource("remote", root, "read", true, RegistrySource{Type: "git", URL: "https://example.test/repo.git"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ConnectRegistryEntryWithSource("remote", root, "read", true, RegistrySource{Type: "git", URL: "https://example.test/repo.git", GitCommit: strings.Repeat("a", 40)}); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := RemoveRegistryEntryWithOptions("remote", RemoveRegistryOptions{RequireManaged: true, ExpectedEntry: &original}); err == nil || ok || !strings.Contains(err.Error(), "changed while it was being removed") {
		t.Fatalf("expected changed snapshot refusal, ok=%t err=%v", ok, err)
	}
	remaining, ok, err := ResolveRegistryEntry("remote")
	if err != nil || !ok {
		t.Fatalf("expected changed entry to remain, ok=%t err=%v", ok, err)
	}
	if remaining.Access != "read" || remaining.Source.GitCommit == "" {
		t.Fatalf("expected latest entry to remain unchanged: %#v", remaining)
	}
}

func TestRegistryAccessCapabilities(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	root := t.TempDir()
	nested := filepath.Join(root, "editable")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ConnectRegistryEntry("readonly", root, "read", true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ConnectRegistryEntry("editable", nested, "write", true); err != nil {
		t.Fatal(err)
	}

	if err := RequireRegistryWriteAccess(filepath.Join(root, "notes", "new.md")); err == nil || !strings.Contains(err.Error(), `"readonly" is read-only`) {
		t.Fatalf("expected read-only root to refuse a new file, got %v", err)
	}
	if err := RequireRegistryWriteAccess(filepath.Join(nested, "new.md")); err != nil {
		t.Fatalf("expected most-specific writable connection to allow a write: %v", err)
	}
	if err := RequireRegistryWriteAccess(filepath.Join(t.TempDir(), "new.md")); err != nil {
		t.Fatalf("expected unregistered path to remain writable: %v", err)
	}

	aliasParent := t.TempDir()
	alias := filepath.Join(aliasParent, "readonly-link")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if err := RequireRegistryWriteAccess(filepath.Join(alias, "notes", "new.md")); err == nil {
		t.Fatal("expected a symlinked path into a read-only connection to remain protected")
	}
}

func TestManagedRegistryConnectionsAreAlwaysReadOnly(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)
	root := t.TempDir()
	source := RegistrySource{Type: "git", URL: "https://example.test/wiki.git", ManagedRoot: root}

	if _, _, err := ConnectRegistryEntryWithSource("remote", root, "write", true, source); err == nil || !strings.Contains(err.Error(), "managed remote connections are read-only") {
		t.Fatalf("expected remote write access to be rejected, got %v", err)
	}
	if entries, err := RegistryEntries(); err != nil || len(entries) != 0 {
		t.Fatalf("rejected remote connection must not mutate the registry: entries=%#v err=%v", entries, err)
	}

	legacy := Registry{Connections: map[string]RegistryConnection{
		root: {Name: "legacy", Access: "write", Source: &source},
	}}
	content, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registryFile, content, 0600); err != nil {
		t.Fatal(err)
	}
	entry, ok, err := ResolveRegistryEntry("legacy")
	if err != nil || !ok {
		t.Fatalf("expected legacy connection to load: ok=%t err=%v", ok, err)
	}
	if entry.Access != "read" || !entry.Managed || RegistryEntryCanWrite(entry) {
		t.Fatalf("expected legacy managed write access to fail closed: %#v", entry)
	}
}

func TestManagedRegistryConnectionCannotBeDowngradedThroughItsCachePath(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)
	root := t.TempDir()
	source := RegistrySource{Type: "git", URL: "https://example.test/wiki.git", ManagedRoot: root}
	original, _, err := ConnectRegistryEntryWithSource("remote", root, "read", true, source)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := ConnectRegistryEntry("local-name", root, "write", true); err == nil || !strings.Contains(err.Error(), "managed remote connections are read-only") {
		t.Fatalf("expected cache-path write reconnect to fail, got %v", err)
	}
	current, ok, err := ResolveRegistryEntry("remote")
	if err != nil || !ok || current != original {
		t.Fatalf("failed downgrade must preserve the original entry: ok=%t entry=%#v err=%v", ok, current, err)
	}

	reconnected, _, err := ConnectRegistryEntry("remote", root, "read", true)
	if err != nil {
		t.Fatal(err)
	}
	if reconnected.Source != source || !reconnected.Managed {
		t.Fatalf("read reconnect must preserve managed provenance: %#v", reconnected)
	}

	replacement := reconnected
	replacement.Managed = false
	replacement.Source = RegistrySource{}
	if _, err := ReplaceRegistryEntry(reconnected, replacement); err == nil || !strings.Contains(err.Error(), "managed source metadata cannot be removed") {
		t.Fatalf("expected managed replacement downgrade to fail, got %v", err)
	}
}

func TestReplaceRegistryEntryRequiresExactSnapshot(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)

	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	original, _, err := ConnectRegistryEntryWithSource("remote", firstRoot, "read", true, RegistrySource{Type: "git", URL: "https://example.test/repo.git", GitCommit: strings.Repeat("a", 40)})
	if err != nil {
		t.Fatal(err)
	}
	replacement := original
	replacement.Path = secondRoot
	replacement.Source.GitCommit = strings.Repeat("b", 40)
	replacement.Source.ManagedRoot = secondRoot
	replaced, err := ReplaceRegistryEntry(original, replacement)
	if err != nil || replaced != replacement {
		t.Fatalf("expected exact replacement, entry=%#v err=%v", replaced, err)
	}
	if _, err := ReplaceRegistryEntry(original, RegistryEntry{Name: "remote", Path: firstRoot, Access: "read"}); err == nil || !strings.Contains(err.Error(), "changed while it was being replaced") {
		t.Fatalf("expected stale replacement refusal, got %v", err)
	}
	current, ok, err := ResolveRegistryEntry("remote")
	if err != nil || !ok || current != replacement {
		t.Fatalf("expected replacement to remain, ok=%t entry=%#v err=%v", ok, current, err)
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
