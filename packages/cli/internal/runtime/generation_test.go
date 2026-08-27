package runtime

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestGenerationManifestAndFilesystemPromotionAreContentBound(t *testing.T) {
	generation := t.TempDir()
	writeRuntimeTestFile(t, generation, "public/index.html", "<h1>Knowledge</h1>\n")
	writeRuntimeTestFile(t, generation, "source/index.md", "# Knowledge\n")
	writeRuntimeTestFile(t, generation, "search/index.md", "# Searchable knowledge\n")
	writeRuntimeTestFile(t, generation, "mcp/index.md", "# MCP knowledge\n")
	manifest, err := WriteGenerationManifestWithChecks(generation, "wiki", "abc123", "0.1", []string{"Verify", "Knowledge Eval"})
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
	if len(manifest.Checks) != 2 || manifest.Checks[0] != "Knowledge Eval" || manifest.Checks[1] != "Verify" {
		t.Fatalf("generation did not bind sorted successful checks: %#v", manifest.Checks)
	}
	if manifest.Health != GenerationHealthPassing {
		t.Fatalf("generation did not record passing health: %#v", manifest)
	}
	if _, activeTarget, err := store.Active("wiki"); err != nil || activeTarget != target {
		t.Fatalf("expected valid active generation, target=%q err=%v", activeTarget, err)
	}
	if info, err := os.Stat(target); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0755) {
		t.Fatalf("expected public generation directory mode 0755, info=%v err=%v", info, err)
	}
	activePath := filepath.Join(store.Root, "wiki", ActivePointerFile)
	if info, err := os.Stat(activePath); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0644) {
		t.Fatalf("expected public active pointer mode 0644, info=%v err=%v", info, err)
	}
	if err := os.WriteFile(filepath.Join(target, "public", "index.html"), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Active("wiki"); err == nil {
		t.Fatal("expected tampered active generation to fail validation")
	}
}

func TestGenerationHealthIsValidatedAndContentBound(t *testing.T) {
	generation := t.TempDir()
	writeRuntimeTestFile(t, generation, "public/index.html", "<h1>Knowledge</h1>\n")
	passing, err := BuildGenerationManifestWithHealth(generation, "wiki", "abc123", "0.2", []string{"Verify"}, GenerationHealthPassing)
	if err != nil {
		t.Fatal(err)
	}
	degraded, err := BuildGenerationManifestWithHealth(generation, "wiki", "abc123", "0.2", []string{}, GenerationHealthDegraded)
	if err != nil {
		t.Fatal(err)
	}
	if passing.ContentDigest == degraded.ContentDigest || GenerationName(passing) == GenerationName(degraded) {
		t.Fatal("generation health and check outcome must change immutable generation identity")
	}
	if _, err := BuildGenerationManifestWithHealth(generation, "wiki", "abc123", "0.2", nil, "unknown"); err == nil {
		t.Fatal("invalid generation health was accepted")
	}
}

func TestGenerationManifestSatisfiesPublishedSchema(t *testing.T) {
	generation := t.TempDir()
	writeRuntimeTestFile(t, generation, "public/index.html", "ok")
	writeRuntimeTestFile(t, generation, "evidence/sha256/receipt.json", "{}")
	manifest, err := WriteGenerationManifestWithHealth(generation, "wiki", "abc123", "0.2", []string{"Verify"}, GenerationHealthDegraded)
	if err != nil {
		t.Fatal(err)
	}
	schemaContent, err := os.ReadFile(filepath.Join("..", "..", "schemas", "runtime", "v1", "generation.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	schemaDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaContent))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://openknowledge.sh/schemas/cli/runtime/v1/generation.schema.json", schemaDocument); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("https://openknowledge.sh/schemas/cli/runtime/v1/generation.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("runtime generation does not satisfy its published schema: %v", err)
	}
}

func TestFilesystemStoreStagesPinsListsAndRollsBackGenerations(t *testing.T) {
	store := FilesystemStore{Root: filepath.Join(t.TempDir(), "artifacts")}
	makeGeneration := func(commit string, body string) GenerationManifest {
		root := t.TempDir()
		writeRuntimeTestFile(t, root, "public/index.html", body)
		writeRuntimeTestFile(t, root, "source/index.md", body)
		writeRuntimeTestFile(t, root, "search/index.md", body)
		manifest, err := WriteGenerationManifest(root, "wiki", commit, "0.1")
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := store.Stage(root); err != nil {
			t.Fatal(err)
		}
		return manifest
	}
	first := makeGeneration("first", "first")
	second := makeGeneration("second", "second")
	releases, err := store.Releases("wiki")
	if err != nil || len(releases) != 2 || releases[0].Active || releases[1].Active {
		t.Fatalf("unexpected staged releases: %#v err=%v", releases, err)
	}
	firstName := GenerationName(first)
	secondName := GenerationName(second)
	if pointer, _, err := store.Pin("wiki", firstName); err != nil || pointer.Generation != firstName || pointer.PreviousGeneration != "" {
		t.Fatalf("unexpected first pin: %#v err=%v", pointer, err)
	}
	if pointer, _, err := store.Pin("wiki", secondName); err != nil || pointer.Generation != secondName || pointer.PreviousGeneration != firstName {
		t.Fatalf("unexpected second pin: %#v err=%v", pointer, err)
	}
	pointer, _, err := store.Rollback("wiki", "")
	if err != nil || pointer.Generation != firstName || pointer.PreviousGeneration != secondName {
		t.Fatalf("unexpected implicit rollback: %#v err=%v", pointer, err)
	}
	releases, err = store.Releases("wiki")
	if err != nil {
		t.Fatal(err)
	}
	active := ""
	for _, release := range releases {
		if release.Active {
			active = release.Name
		}
	}
	if active != firstName {
		t.Fatalf("active release after rollback = %q, want %q", active, firstName)
	}
}

func TestGenerationRejectsFilesOutsidePublicContract(t *testing.T) {
	generation := t.TempDir()
	writeRuntimeTestFile(t, generation, "public/index.html", "ok")
	writeRuntimeTestFile(t, generation, "agent-logs/raw.log", "secret")
	if _, err := BuildGenerationManifest(generation, "wiki", "abc123", "0.1"); err == nil {
		t.Fatal("expected private file outside public/source/search/mcp/evidence roots to be rejected")
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
