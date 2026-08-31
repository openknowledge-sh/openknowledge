package runtime

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestRuntimeIndexCacheRestoresSearchAndPrunesOnlyStaleGenerations(t *testing.T) {
	projection := t.TempDir()
	writeRuntimeTestFile(t, projection, "index.md", "# Cached knowledge\n\nPersistent search needle.\n")
	index, err := okf.BuildContextIndexWithVersion(projection, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	cache := IndexCache{Root: filepath.Join(t.TempDir(), "indexes")}
	digest := strings.Repeat("a", 64)
	path, err := cache.Store("wiki", "generation-1", digest, IndexTargetSearch, index)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("unexpected private cache mode: info=%v err=%v", info, err)
	}
	restored, err := cache.Load("wiki", "generation-1", digest, "0.1", IndexTargetSearch, projection)
	if err != nil {
		t.Fatal(err)
	}
	if results := restored.Search(okf.SearchOptions{Query: "persistent needle", Limit: 5}); len(results.Results) != 1 || results.Results[0].Path != "index.md" {
		t.Fatalf("restored index did not search cached sections: %#v", results)
	}
	if _, err := cache.Store("wiki", "stale-generation", digest, IndexTargetSearch, index); err != nil {
		t.Fatal(err)
	}
	candidates, err := cache.Prune("wiki", map[string]bool{"generation-1": true}, false)
	if err != nil || len(candidates) != 1 || candidates[0] != "stale-generation" {
		t.Fatalf("unexpected prune preview: %#v err=%v", candidates, err)
	}
	if _, err := os.Stat(filepath.Join(cache.Root, "wiki", "stale-generation")); err != nil {
		t.Fatalf("dry-run removed stale cache: %v", err)
	}
	if _, err := cache.Prune("wiki", map[string]bool{"generation-1": true}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache.Root, "wiki", "stale-generation")); !os.IsNotExist(err) {
		t.Fatalf("applied prune retained stale cache: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content = []byte(strings.Replace(string(content), "Persistent search needle", "Tampered search needle", 1))
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Load("wiki", "generation-1", digest, "0.1", IndexTargetSearch, projection); err == nil || !strings.Contains(err.Error(), "payload digest mismatch") {
		t.Fatalf("expected tampered cache refusal, got %v", err)
	}
}
