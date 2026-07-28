package okf

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseASTMarkdown(f *testing.F) {
	for _, seed := range []string{
		"# Title\n\nA [link](guide.md#start).\n",
		"```go\nfmt.Println(\"hello\")\n```\n",
		"| A | B |\n| --- | --- |\n| 1 | 2 |\n",
		"\xff\xfe\x00",
	} {
		f.Add(seed, 1)
	}

	f.Fuzz(func(t *testing.T, body string, bodyLine int) {
		if len(body) > 1<<20 {
			t.Skip()
		}
		markdown := ParseASTMarkdown(body, bodyLine)
		if _, err := json.Marshal(markdown); err != nil {
			t.Fatalf("AST markdown is not JSON-compatible: %v", err)
		}
	})
}

func FuzzParseFrontmatterDocument(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("---\ntype: Guide\ntags: [docs, test]\n---\n# Guide\n"),
		[]byte("---\nvalue:\n  nested: true\n---\nBody\n"),
		[]byte("---\nunclosed: true\n"),
		{0xff, 0xfe, 0x00},
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		if len(content) > 1<<20 {
			t.Skip()
		}
		document, err := ParseFrontmatterDocument(content)
		if err != nil {
			return
		}
		if _, err := json.Marshal(document); err != nil {
			t.Fatalf("frontmatter document is not JSON-compatible: %v", err)
		}
	})
}

func FuzzDecodeStrictJSON(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{"schemaVersion":"1","value":true}`),
		[]byte(`{"duplicate":1,"duplicate":2}`),
		[]byte(`{"nested":[{"value":"ok"}]}`),
		[]byte(`null trailing`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		if len(content) > 1<<20 {
			t.Skip()
		}
		var value any
		if err := DecodeStrictJSON(content, &value); err != nil {
			return
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("strict JSON result cannot be encoded: %v", err)
		}
		var roundTrip any
		if err := DecodeStrictJSON(encoded, &roundTrip); err != nil {
			t.Fatalf("strict JSON result does not round trip: %v", err)
		}
	})
}

func FuzzExtractBundleArchive(f *testing.F) {
	f.Add([]byte("not an archive"))
	f.Add(fuzzArchiveSeed())

	f.Fuzz(func(t *testing.T, archive []byte) {
		if len(archive) > 2<<20 {
			t.Skip()
		}
		root := t.TempDir()
		archivePath := filepath.Join(root, "bundle.tar.gz")
		target := filepath.Join(root, "target")
		if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
			t.Fatal(err)
		}
		_ = ExtractBundleArchiveWithLimits(archivePath, target, ArchiveExtractionLimits{
			MaxEntries:        128,
			MaxFileBytes:      1 << 20,
			MaxExtractedBytes: 2 << 20,
		})
		if _, err := os.Stat(filepath.Join(root, "escape")); !os.IsNotExist(err) {
			t.Fatalf("archive escaped its target: %v", err)
		}
	})
}

func fuzzArchiveSeed() []byte {
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	tarWriter := tar.NewWriter(gzipWriter)
	content := []byte("# Seed\n")
	_ = tarWriter.WriteHeader(&tar.Header{
		Name: "index.md",
		Mode: 0o644,
		Size: int64(len(content)),
	})
	_, _ = tarWriter.Write(content)
	_ = tarWriter.Close()
	_ = gzipWriter.Close()
	return compressed.Bytes()
}
