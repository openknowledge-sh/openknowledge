package okf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestImportStatusDetectsChangesOnBothSides(t *testing.T) {
	source, root := t.TempDir(), t.TempDir()
	content := []byte("# Original\n")
	if err := os.MkdirAll(filepath.Join(root, ".openknowledge"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, source, "doc.md", string(content))
	writeFile(t, root, "copy.md", string(content))
	manifest := setupImportManifest{Mode: "copy", Source: source, Documents: []setupImportManifestDocument{{Source: "doc.md", Target: "copy.md", SourceSHA256: setupContentSHA256(content), TargetSHA256: setupContentSHA256(content)}}}
	data, _ := json.Marshal(manifest)
	writeFile(t, root, ".openknowledge/import.json", string(data))
	status, err := InspectSetupImport(root)
	if err != nil || status.SourceChanged != 0 || status.CopyChanged != 0 {
		t.Fatalf("initial status: %#v %v", status, err)
	}
	writeFile(t, source, "doc.md", "# New source\n")
	writeFile(t, root, "copy.md", "# Edited copy\n")
	status, err = InspectSetupImport(root)
	if err != nil || status.SourceChanged != 1 || status.CopyChanged != 1 {
		t.Fatalf("changed status: %#v %v", status, err)
	}
	if err := os.Remove(filepath.Join(source, "doc.md")); err != nil {
		t.Fatal(err)
	}
	status, err = InspectSetupImport(root)
	if err != nil || status.Unavailable != 1 {
		t.Fatalf("missing status: %#v %v", status, err)
	}
}
