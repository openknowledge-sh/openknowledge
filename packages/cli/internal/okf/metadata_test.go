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
---

# Root Heading
`)
	writeFile(t, root, ConfigFile, `[bundle]
name = "accessibility"
title = "Accessibility Review"
purpose = "Review UI accessibility."
tags = ["accessibility", "ui", "review"]

[bundle.entries]
review = "agents/review.md"
default = "agents/checker.md"
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

func TestReadConfigReadsBundleMetadataAndPublishExcludes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ConfigFile, `[bundle]
name = "accessibility"
title = "Accessibility Review"
purpose = "Review UI accessibility."
tags = ["accessibility", "ui", "review", "ui"]

[bundle.entries]
review = "agents/review.md"
default = "agents/checker.md"

[publish]
exclude = ["drafts/index.md", "./archive/old.md"]
`)

	config, err := ReadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if config.Bundle.Name != "accessibility" || config.Bundle.Title != "Accessibility Review" || config.Bundle.Purpose != "Review UI accessibility." {
		t.Fatalf("unexpected bundle metadata: %#v", config.Bundle)
	}
	if !reflect.DeepEqual(config.Bundle.Tags, []string{"accessibility", "ui", "review"}) {
		t.Fatalf("unexpected tags: %#v", config.Bundle.Tags)
	}
	if !reflect.DeepEqual(config.Bundle.Entries, []BundleEntry{{Name: "default", Path: "agents/checker.md"}, {Name: "review", Path: "agents/review.md"}}) {
		t.Fatalf("unexpected entries: %#v", config.Bundle.Entries)
	}
	for _, path := range []string{"drafts/index.md", "archive/old.md"} {
		if _, ok := config.PublishExclude[path]; !ok {
			t.Fatalf("expected publish exclusion for %s in %#v", path, config.PublishExclude)
		}
	}
}

func TestReadConfigRejectsPublishExclusionOutsideBundle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ConfigFile, "[publish]\nexclude = [\"../outside.md\"]\n")

	if _, err := ReadConfig(root); err == nil {
		t.Fatal("expected outside publish exclusion to fail")
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

func TestReadMarkdownDocumentInfoReadsAgentEntrypointMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "agents/review.md", `---
type: Agent Entrypoint
title: Accessibility Review
description: Review UI accessibility.
tags: [accessibility, review]
use_when: ["reviewing UI", "checking forms"]
---

# Review
`)

	path := filepath.Join(root, "agents", "review.md")
	info, err := ReadMarkdownDocumentInfo(path, "agents/review.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.Type != "Agent Entrypoint" || info.Title != "Accessibility Review" || info.Description != "Review UI accessibility." {
		t.Fatalf("unexpected document info: %#v", info)
	}
	if !reflect.DeepEqual(info.Tags, []string{"accessibility", "review"}) {
		t.Fatalf("unexpected tags: %#v", info.Tags)
	}
	if !reflect.DeepEqual(info.UseWhen, []string{"reviewing UI", "checking forms"}) {
		t.Fatalf("unexpected use_when: %#v", info.UseWhen)
	}
}
