package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerationManifestAndFilesystemPromotionAreContentBound(t *testing.T) {
	generation := t.TempDir()
	writeRuntimeTestFile(t, generation, "public/index.html", "<h1>Knowledge</h1>\n")
	writeRuntimeTestFile(t, generation, "source/index.md", "# Knowledge\n")
	writeRuntimeTestFile(t, generation, "search/index.md", "# Searchable knowledge\n")
	writeRuntimeTestFile(t, generation, "mcp/index.md", "# MCP knowledge\n")
	manifest, err := WriteGenerationManifest(generation, "wiki", "abc123", "0.1")
	if err != nil {
		t.Fatal(err)
	}
	store := FilesystemStore{Root: filepath.Join(t.TempDir(), "artifacts")}
	pointer, target, err := store.Publish(generation)
	if err != nil {
		t.Fatal(err)
	}
	if pointer.ContentDigest != manifest.ContentDigest || pointer.Generation != GenerationName(manifest) {
		t.Fatalf("unexpected pointer: %#v", pointer)
	}
	if _, activeTarget, err := store.Active("wiki"); err != nil || activeTarget != target {
		t.Fatalf("expected valid active generation, target=%q err=%v", activeTarget, err)
	}
	if info, err := os.Stat(target); err != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("expected public generation directory mode 0755, info=%v err=%v", info, err)
	}
	activePath := filepath.Join(store.Root, "wiki", ActivePointerFile)
	if info, err := os.Stat(activePath); err != nil || info.Mode().Perm() != 0644 {
		t.Fatalf("expected public active pointer mode 0644, info=%v err=%v", info, err)
	}
	if err := os.WriteFile(filepath.Join(target, "public", "index.html"), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Active("wiki"); err == nil {
		t.Fatal("expected tampered active generation to fail validation")
	}
}

func TestGenerationRejectsFilesOutsidePublicContract(t *testing.T) {
	generation := t.TempDir()
	writeRuntimeTestFile(t, generation, "public/index.html", "ok")
	writeRuntimeTestFile(t, generation, "agent-logs/raw.log", "secret")
	if _, err := BuildGenerationManifest(generation, "wiki", "abc123", "0.1"); err == nil {
		t.Fatal("expected private file outside public/source/search/mcp roots to be rejected")
	}
}

func TestGenerationManifestDecodingFailsClosed(t *testing.T) {
	generation := t.TempDir()
	writeRuntimeTestFile(t, generation, "public/index.html", "ok")
	if _, err := WriteGenerationManifest(generation, "wiki", "abc123", "0.1"); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(generation, GenerationManifestFile)
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := []byte("{\"type\":\"openknowledge.generation\",\"type\":\"openknowledge.generation\",\"version\":1}\n")
	if err := os.WriteFile(manifestPath, duplicate, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAndValidateGeneration(generation); err == nil {
		t.Fatal("expected duplicate manifest key to be rejected")
	}
	if err := os.WriteFile(manifestPath, append(content, []byte("{}")...), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAndValidateGeneration(generation); err == nil {
		t.Fatal("expected trailing manifest JSON to be rejected")
	}
}

func writeRuntimeTestFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
