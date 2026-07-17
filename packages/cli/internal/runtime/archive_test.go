package runtime

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractDirectoryArchiveRejectsTraversalAndLinks(t *testing.T) {
	for _, test := range []struct {
		name     string
		header   tar.Header
		contents string
		want     string
	}{
		{name: "traversal", header: tar.Header{Name: "../secret", Typeflag: tar.TypeReg, Size: 1, Mode: 0644}, contents: "x", want: "escapes destination"},
		{name: "symlink", header: tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0777}, want: "links or unsupported"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var archive bytes.Buffer
			gzipWriter := gzip.NewWriter(&archive)
			tarWriter := tar.NewWriter(gzipWriter)
			if err := tarWriter.WriteHeader(&test.header); err != nil {
				t.Fatal(err)
			}
			if _, err := tarWriter.Write([]byte(test.contents)); err != nil {
				t.Fatal(err)
			}
			if err := tarWriter.Close(); err != nil {
				t.Fatal(err)
			}
			if err := gzipWriter.Close(); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), "out")
			err := ExtractDirectoryArchive(bytes.NewReader(archive.Bytes()), destination, 1<<20)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q refusal, got %v", test.want, err)
			}
			if _, err := os.Stat(filepath.Join(destination, "link")); !os.IsNotExist(err) {
				t.Fatalf("unsafe entry was materialized: %v", err)
			}
		})
	}
}
