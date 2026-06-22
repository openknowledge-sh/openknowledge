package okf

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractBundleArchiveRejectsPathTraversal(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "bad.tar.gz")
	writeArchiveTestTarGzip(t, archivePath, map[string]string{
		"../escaped.md": "# Escaped\n",
	})

	err := ExtractBundleArchive(archivePath, filepath.Join(t.TempDir(), "out"))
	if err == nil || !strings.Contains(err.Error(), "unsafe archive entry path") {
		t.Fatalf("expected unsafe archive path error, got %v", err)
	}
}

func TestExtractBundleArchiveRejectsSymlinks(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "bad-link.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "index.md",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	err = ExtractBundleArchive(archivePath, filepath.Join(t.TempDir(), "out"))
	if err == nil || !strings.Contains(err.Error(), "unsupported archive entry type") {
		t.Fatalf("expected unsupported symlink error, got %v", err)
	}
}

func writeArchiveTestTarGzip(t *testing.T, archivePath string, entries map[string]string) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range entries {
		data := []byte(content)
		if err := tarWriter.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0644,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
