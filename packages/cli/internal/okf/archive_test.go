package okf

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateBundleManifestContract(t *testing.T) {
	valid := BundleManifest{
		Type:          BundleManifestType,
		Version:       BundleManifestVersion,
		Spec:          "0.1",
		Archive:       BundleArchiveRelPath,
		ArchiveSHA256: strings.Repeat("a", 64),
		ArchiveFormat: BundleArchiveFormat,
	}
	if spec, err := ValidateBundleManifest(valid); err != nil || spec != "0.1" {
		t.Fatalf("expected valid manifest contract, spec=%q err=%v", spec, err)
	}

	tests := []struct {
		name     string
		mutate   func(*BundleManifest)
		expected string
	}{
		{name: "type", mutate: func(manifest *BundleManifest) { manifest.Type = "bundle" }, expected: "manifest type"},
		{name: "version", mutate: func(manifest *BundleManifest) { manifest.Version = 2 }, expected: "manifest version"},
		{name: "missing spec", mutate: func(manifest *BundleManifest) { manifest.Spec = "" }, expected: "missing spec"},
		{name: "moving spec", mutate: func(manifest *BundleManifest) { manifest.Spec = "latest" }, expected: "concrete version"},
		{name: "unsupported spec", mutate: func(manifest *BundleManifest) { manifest.Spec = "9.9" }, expected: "unsupported OKF spec"},
		{name: "noncanonical spec", mutate: func(manifest *BundleManifest) { manifest.Spec = " 0.1 " }, expected: "canonical version"},
		{name: "missing archive", mutate: func(manifest *BundleManifest) { manifest.Archive = "" }, expected: "missing archive"},
		{name: "archive format", mutate: func(manifest *BundleManifest) { manifest.ArchiveFormat = "zip" }, expected: "archive format"},
		{name: "missing checksum", mutate: func(manifest *BundleManifest) { manifest.ArchiveSHA256 = "" }, expected: "64-character SHA-256"},
		{name: "invalid checksum", mutate: func(manifest *BundleManifest) { manifest.ArchiveSHA256 = strings.Repeat("z", 64) }, expected: "64-character SHA-256"},
		{name: "noncanonical checksum", mutate: func(manifest *BundleManifest) { manifest.ArchiveSHA256 = " " + strings.Repeat("a", 64) }, expected: "64-character SHA-256"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := valid
			test.mutate(&manifest)
			if _, err := ValidateBundleManifest(manifest); err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("expected %q error, got %v", test.expected, err)
			}
		})
	}
}

func TestBundleManifestForArchiveCannotProduceInvalidContract(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Bundle\n")
	if _, err := BundleManifestForArchive(root, "0.1", BundleArchiveRelPath, ""); err == nil || !strings.Contains(err.Error(), "archiveSha256") {
		t.Fatalf("expected producer to reject missing archive checksum, got %v", err)
	}
}

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

func TestExtractBundleArchiveEnforcesResourceLimitsAtomically(t *testing.T) {
	tests := []struct {
		name     string
		entries  map[string]string
		limits   ArchiveExtractionLimits
		expected string
	}{
		{
			name:     "entry count",
			entries:  map[string]string{"one.md": "one", "two.md": "two"},
			limits:   ArchiveExtractionLimits{MaxEntries: 1, MaxFileBytes: 10, MaxExtractedBytes: 20},
			expected: "maximum entry count",
		},
		{
			name:     "single file",
			entries:  map[string]string{"large.md": "0123456789"},
			limits:   ArchiveExtractionLimits{MaxEntries: 10, MaxFileBytes: 5, MaxExtractedBytes: 20},
			expected: "maximum file size",
		},
		{
			name:     "total bytes",
			entries:  map[string]string{"one.md": "1234", "two.md": "5678"},
			limits:   ArchiveExtractionLimits{MaxEntries: 10, MaxFileBytes: 10, MaxExtractedBytes: 6},
			expected: "maximum extracted size",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parent := t.TempDir()
			archivePath := filepath.Join(parent, "bundle.tar.gz")
			writeArchiveTestTarGzip(t, archivePath, test.entries)
			target := filepath.Join(parent, "out")
			err := ExtractBundleArchiveWithLimits(archivePath, target, test.limits)
			if err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("expected %q error, got %v", test.expected, err)
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("failed extraction must not publish target, stat error: %v", err)
			}
			matches, err := filepath.Glob(filepath.Join(parent, ".openknowledge-extract-*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != 0 {
				t.Fatalf("failed extraction left staging directories: %v", matches)
			}
		})
	}
}

func TestExtractBundleArchiveRefusesExistingTarget(t *testing.T) {
	parent := t.TempDir()
	archivePath := filepath.Join(parent, "bundle.tar.gz")
	writeArchiveTestTarGzip(t, archivePath, map[string]string{"index.md": "# Replacement\n"})
	target := filepath.Join(parent, "out")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "marker")
	if err := os.WriteFile(marker, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ExtractBundleArchive(archivePath, target); err == nil || !strings.Contains(err.Error(), "target already exists") {
		t.Fatalf("expected existing target refusal, got %v", err)
	}
	content, err := os.ReadFile(marker)
	if err != nil || string(content) != "original" {
		t.Fatalf("existing target must remain untouched, content=%q err=%v", content, err)
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
