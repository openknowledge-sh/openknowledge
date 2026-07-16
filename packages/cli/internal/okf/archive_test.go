package okf

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		{name: "uppercase checksum", mutate: func(manifest *BundleManifest) { manifest.ArchiveSHA256 = strings.Repeat("A", 64) }, expected: "64-character SHA-256"},
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

func TestDecodeBundleManifestRejectsAmbiguousOrExtendedJSON(t *testing.T) {
	checksum := strings.Repeat("a", 64)
	valid := `{"type":"openknowledge.bundle","version":1,"spec":"0.1","archive":"assets/openknowledge-bundle.tar.gz","archiveSha256":"` + checksum + `","archiveFormat":"tar.gz"}`
	manifest, err := DecodeBundleManifest([]byte(valid))
	if err != nil || manifest.ArchiveSHA256 != checksum {
		t.Fatalf("expected strict decoder to accept valid manifest, manifest=%#v err=%v", manifest, err)
	}

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{name: "unknown field", content: strings.TrimSuffix(valid, "}") + `,"extra":true}`, expected: "unknown field"},
		{name: "duplicate field", content: strings.Replace(valid, `"type":"openknowledge.bundle"`, `"type":"openknowledge.bundle","type":"openknowledge.bundle"`, 1), expected: "duplicate field"},
		{name: "trailing JSON", content: valid + `{}`, expected: "trailing JSON"},
		{name: "wrong format", content: strings.Replace(valid, `"archiveFormat":"tar.gz"`, `"archiveFormat":"zip"`, 1), expected: "archive format"},
		{name: "top-level array", content: `[]`, expected: "cannot unmarshal array"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeBundleManifest([]byte(test.content)); err == nil || !strings.Contains(err.Error(), test.expected) {
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

func TestWriteBundleTarGzipRejectsSymbolicLinks(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "bundle")
	writeFile(t, root, "index.md", "# Bundle\n")
	outside := filepath.Join(base, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "secret.txt")); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}

	out := filepath.Join(base, "bundle.tar.gz")
	if _, err := WriteBundleTarGzipWithVersion(root, out, "0.1", nil); err == nil || !strings.Contains(err.Error(), "symbolic links are not supported") {
		t.Fatalf("expected archive writer to reject symlink, got %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("refused archive must not be published, got %v", err)
	}
}

func TestWritePublishedBundleTarGzipUsesExplicitPublicAllowlist(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Bundle\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n")
	writeFile(t, root, "draft.md", "---\ntype: Draft\nokf_publish: false\n---\n\n# Draft\n")
	writeFile(t, root, "assets/public/logo.svg", "<svg/>\n")
	writeFile(t, root, "assets/private/diagram.svg", "<svg>private</svg>\n")
	writeFile(t, root, "secret.txt", "do not publish\n")
	writeFile(t, root, ".openknowledge/runtime.json", "{\"secret\":true}\n")
	writeFile(t, root, "openknowledge.toml", "[publish]\nenabled = true\nassets = [\"assets/public/**\", \"**/*.md\"]\n")

	out := filepath.Join(t.TempDir(), "published.tar.gz")
	if _, err := WritePublishedBundleTarGzipWithVersion(root, out, "0.1", nil); err != nil {
		t.Fatal(err)
	}
	extracted := filepath.Join(t.TempDir(), "bundle")
	if err := ExtractBundleArchive(out, extracted); err != nil {
		t.Fatal(err)
	}
	for _, included := range []string{"index.md", "guide.md", "assets/public/logo.svg"} {
		if _, err := os.Stat(filepath.Join(extracted, filepath.FromSlash(included))); err != nil {
			t.Fatalf("expected %s in public archive: %v", included, err)
		}
	}
	for _, excluded := range []string{"draft.md", "assets/private/diagram.svg", "secret.txt", ".openknowledge/runtime.json", "openknowledge.toml"} {
		if _, err := os.Stat(filepath.Join(extracted, filepath.FromSlash(excluded))); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be excluded from public archive, got %v", excluded, err)
		}
	}
}

func TestWriteBundleTarGzipRejectsInvalidBundleWithoutReplacingOutput(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Bundle\n")
	writeFile(t, root, "invalid.md", "# Missing required concept frontmatter\n")
	out := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(out, []byte("previous archive"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := WriteBundleTarGzipWithVersion(root, out, "0.1", nil); err == nil || !strings.Contains(err.Error(), "bundle validation failed") {
		t.Fatalf("expected invalid bundle refusal, got %v", err)
	}
	content, err := os.ReadFile(out)
	if err != nil || string(content) != "previous archive" {
		t.Fatalf("invalid export must preserve prior output, content=%q err=%v", content, err)
	}
}

func TestWriteBundleTarGzipIsReproducibleAcrossDestinationsAndHostMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Bundle\n")
	writeFile(t, root, "notes/guide.md", "---\ntype: Guide\n---\n\n# Guide\n")
	firstPath := filepath.Join(t.TempDir(), "first-name.tar.gz")
	first, err := WriteBundleTarGzipWithVersion(root, firstPath, "0.1", nil)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}

	changedTime := time.Date(2042, time.December, 31, 23, 59, 58, 0, time.FixedZone("test", 9*60*60))
	for _, rel := range []string{"index.md", "notes/guide.md"} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.Chtimes(path, changedTime, changedTime); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0600); err != nil {
			t.Fatal(err)
		}
	}
	secondPath := filepath.Join(t.TempDir(), "different-name.tgz")
	second, err := WriteBundleTarGzipWithVersion(root, secondPath, "0.1", nil)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("expected byte-identical archives, first=%s second=%s", first.SHA256, second.SHA256)
	}
	if first.SHA256 != second.SHA256 || first.Bytes != second.Bytes {
		t.Fatalf("expected stable archive identity, first=%#v second=%#v", first, second)
	}
}

func TestWriteBundleTarGzipUsesCanonicalHeaders(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Bundle\n")
	executable := filepath.Join(root, "tool.sh")
	writeFile(t, root, "tool.sh", "#!/bin/sh\n")
	if err := os.Chmod(executable, 0701); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if _, err := WriteBundleTarGzipWithVersion(root, out, "0.1", nil); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	if gzipReader.Name != "" || !gzipReader.ModTime.IsZero() || gzipReader.OS != 255 {
		t.Fatalf("unexpected gzip identity header: name=%q modTime=%s os=%d", gzipReader.Name, gzipReader.ModTime, gzipReader.OS)
	}

	tarReader := tar.NewReader(gzipReader)
	wantModes := map[string]int64{"index.md": 0644, "tool.sh": 0755}
	seen := make(map[string]bool)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		wantMode, ok := wantModes[header.Name]
		if !ok {
			t.Fatalf("unexpected archive entry %q", header.Name)
		}
		seen[header.Name] = true
		if header.Mode != wantMode || header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" {
			t.Fatalf("noncanonical tar identity header for %s: %#v", header.Name, header)
		}
		if !header.ModTime.Equal(time.Unix(0, 0)) || !header.AccessTime.IsZero() || !header.ChangeTime.IsZero() {
			t.Fatalf("noncanonical tar timestamps for %s: mtime=%s atime=%s ctime=%s", header.Name, header.ModTime, header.AccessTime, header.ChangeTime)
		}
	}
	if len(seen) != len(wantModes) {
		t.Fatalf("expected canonical headers for every file, saw %#v", seen)
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
