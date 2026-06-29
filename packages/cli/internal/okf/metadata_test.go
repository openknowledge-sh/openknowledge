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

func TestReadBundleInfoUsesParsedMarkdownHeading(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "```md\n# Fenced Heading\n```\n\n# Real Heading\n")

	info, err := ReadBundleInfo(root)
	if err != nil {
		t.Fatal(err)
	}
	if info.RootTitle != "Real Heading" {
		t.Fatalf("expected parsed Markdown heading, got %#v", info)
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
