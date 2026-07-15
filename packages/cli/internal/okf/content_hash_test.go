package okf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectorySHA256TracksBundleContentButIgnoresGitInternals(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Bundle\n")
	writeFile(t, root, "notes/a.md", "# A\n")
	first, err := DirectorySHA256(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := DirectorySHA256(root)
	if err != nil || second != first {
		t.Fatalf("expected deterministic digest %s, got %s err=%v", first, second, err)
	}

	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("changing internals"), 0600); err != nil {
		t.Fatal(err)
	}
	ignored, err := DirectorySHA256(root)
	if err != nil || ignored != first {
		t.Fatalf("expected .git to be excluded, got %s want %s err=%v", ignored, first, err)
	}

	writeFile(t, root, "notes/a.md", "# Changed\n")
	changed, err := DirectorySHA256(root)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("expected content mutation to change digest")
	}
}
