package okf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDirectoryAtomicallyPublishesCompleteGeneration(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "site")
	writeFile(t, target, "stale.html", "old\n")

	absolute, err := WriteDirectoryAtomically(target, func(staging string) error {
		writeFile(t, staging, "index.html", "new\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if absolute != target {
		t.Fatalf("expected absolute target %s, got %s", target, absolute)
	}
	if content, err := os.ReadFile(filepath.Join(target, "index.html")); err != nil || string(content) != "new\n" {
		t.Fatalf("expected new generation, content=%q err=%v", content, err)
	}
	if _, err := os.Stat(filepath.Join(target, "stale.html")); !os.IsNotExist(err) {
		t.Fatalf("expected stale generation file to disappear, got %v", err)
	}
	assertNoOutputStagingPaths(t, parent)
}

func TestWriteDirectoryAtomicallyPreservesPreviousGenerationOnFailure(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "site")
	writeFile(t, target, "index.html", "old\n")

	expected := errors.New("injected generation failure")
	if _, err := WriteDirectoryAtomically(target, func(staging string) error {
		writeFile(t, staging, "index.html", "partial\n")
		return expected
	}); !errors.Is(err, expected) {
		t.Fatalf("expected injected failure, got %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(target, "index.html")); err != nil || string(content) != "old\n" {
		t.Fatalf("expected old generation to remain, content=%q err=%v", content, err)
	}
	assertNoOutputStagingPaths(t, parent)
}

func TestValidateHTMLOutputBoundaryRejectsOutputContainingSource(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "bundle")
	writeFile(t, root, "index.md", "# Home\n")

	for _, out := range []string{root, parent} {
		err := ValidateHTMLOutputBoundary(root, out)
		if err == nil || !strings.Contains(err.Error(), "must not contain the source bundle") {
			t.Fatalf("expected unsafe output %s to be rejected, got %v", out, err)
		}
	}

	if err := ValidateHTMLOutputBoundary(root, filepath.Join(root, "site")); err != nil {
		t.Fatalf("expected an output nested inside the source bundle to be allowed: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(root, "index.md")); err != nil || string(content) != "# Home\n" {
		t.Fatalf("boundary validation must not modify the source, content=%q err=%v", content, err)
	}
}

func assertNoOutputStagingPaths(t *testing.T, parent string) {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "site" {
			t.Fatalf("unexpected output staging path left behind: %s", entry.Name())
		}
	}
}
