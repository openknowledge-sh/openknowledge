package main

import (
	"os"
	"path/filepath"
	"testing"
)

func enablePublicArtifactTest(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "openknowledge.toml"), []byte("[publish]\nenabled = true\n"), 0644); err != nil {
		t.Fatal(err)
	}
}
