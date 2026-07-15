package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveBundlePathRejectsSymbolicLinks(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "bundle")
	writeFile(t, root, "index.md", "# Bundle\n")
	outside := filepath.Join(base, "outside.md")
	if err := os.WriteFile(outside, []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}

	if _, err := ResolveBundlePath(root, "linked.md"); err == nil || !strings.Contains(err.Error(), "symbolic links are not supported") {
		t.Fatalf("expected symlink refusal, got %v", err)
	}
	resolved, err := ResolveBundlePath(root, "index.md")
	if err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(resolved); err != nil || string(content) != "# Bundle\n" {
		t.Fatalf("expected normal bundle file, content=%q err=%v", content, err)
	}
}

func TestParseBundleRejectsNonMarkdownSymbolicLink(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "bundle")
	writeFile(t, root, "index.md", "# Bundle\n")
	outside := filepath.Join(base, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "asset.txt")); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}

	if _, err := ParseBundle(root); err == nil || !strings.Contains(err.Error(), "symbolic links are not supported") {
		t.Fatalf("expected bundle parser to reject asset symlink, got %v", err)
	}
}

func TestReadBundleInfoRejectsIndexSymbolicLink(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "bundle")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(base, "outside.md")
	if err := os.WriteFile(outside, []byte("---\nokf_bundle_name: stolen\n---\n\n# Outside\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "index.md")); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}

	if _, err := ReadBundleInfo(root); err == nil || !strings.Contains(err.Error(), "symbolic links are not supported") {
		t.Fatalf("expected bundle metadata read to reject index symlink, got %v", err)
	}
}
