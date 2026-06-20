package okf

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadBundleInfoReadsRootMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", `---
okf_version: "0.1"
okf_bundle_name: accessibility
okf_bundle_title: "Accessibility Review"
okf_bundle_purpose: "Review UI accessibility."
okf_bundle_tags: ["accessibility", "ui", "review"]
okf_bundle_entry_review: "agents/review.md"
okf_bundle_entry_default: "agents/checker.md"
---

# Root Heading
`)

	info, err := ReadBundleInfo(root)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasIndex || !info.HasMetadata {
		t.Fatalf("expected index and metadata, got %#v", info)
	}
	if info.DisplayName() != "Accessibility Review" {
		t.Fatalf("unexpected display name: %s", info.DisplayName())
	}
	if !reflect.DeepEqual(info.Metadata.Tags, []string{"accessibility", "ui", "review"}) {
		t.Fatalf("unexpected tags: %#v", info.Metadata.Tags)
	}
	if !reflect.DeepEqual(info.EntryNames(), []string{"default", "review"}) {
		t.Fatalf("unexpected entry names: %#v", info.EntryNames())
	}
}

func TestReadBundleInfoFallsBackToFolderName(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project-memory")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}

	info, err := ReadBundleInfo(root)
	if err != nil {
		t.Fatal(err)
	}
	if info.DisplayName() != "Project Memory" {
		t.Fatalf("unexpected display name: %s", info.DisplayName())
	}
}
