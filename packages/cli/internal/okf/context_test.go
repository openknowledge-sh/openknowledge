package okf

import (
	"strings"
	"testing"
)

func TestBuildContextIndexSplitsMarkdownSectionsWithLineRanges(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "guide.md", strings.Join([]string{
		"---",
		"type: Guide",
		"title: Setup Guide",
		"---",
		"",
		"Intro before headings.",
		"",
		"# Install",
		"Run setup.",
		"",
		"```md",
		"# Not a section",
		"```",
		"",
		"## Validate",
		"Run `openknowledge validate`.",
	}, "\n"))

	index, err := BuildContextIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Sections) != 3 {
		t.Fatalf("expected top, install, and validate sections, got %#v", index.Sections)
	}

	top := index.Sections[0]
	if top.ID != "guide#top" || top.LineStart != 5 || top.LineEnd != 7 || strings.Contains(top.Text, "type: Guide") {
		t.Fatalf("unexpected top section: %#v", top)
	}
	install := index.Sections[1]
	if install.ID != "guide#install" || install.Heading != "Install" || install.LineStart != 8 || install.LineEnd != 14 {
		t.Fatalf("unexpected install section: %#v", install)
	}
	validate := index.Sections[2]
	if validate.ID != "guide#validate" || validate.HeadingLevel != 2 || validate.LineStart != 15 || validate.LineEnd != 16 {
		t.Fatalf("unexpected validate section: %#v", validate)
	}
}

func TestContextIndexUsesParsedMarkdownHeadingBoundaries(t *testing.T) {
	document := ASTDocument{
		Rel:   "guide.md",
		ID:    "guide",
		Kind:  "concept",
		Body:  "# Raw Heading\n\nBody text.\n",
		Links: nil,
		Frontmatter: ASTFrontmatter{
			BodyLine: 1,
		},
		Markdown: ASTMarkdown{},
	}

	index := ContextIndexFromAST(Result{Root: "root"}, ASTBundle{Root: "root", Documents: []ASTDocument{document}})

	if len(index.Sections) != 1 {
		t.Fatalf("expected one top section from empty Markdown headings, got %#v", index.Sections)
	}
	if index.Sections[0].Heading != "Top" || index.Sections[0].HeadingLevel != 0 {
		t.Fatalf("expected context to trust parsed Markdown headings, got %#v", index.Sections[0])
	}
}

func TestResolveContextRanksHeadingMetadataAndBodyMatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nIncident material lives in the guide.\n")
	writeFile(t, root, "guides/incident.md", "---\ntype: Playbook\ntitle: Incident Playbook\ndescription: Triage production alerts.\n---\n\n# Response\n\nRun the escalation checklist.\n")
	writeFile(t, root, "notes/release.md", "---\ntype: Note\ntitle: Release Notes\n---\n\n# Release\n\nIncident details belong in release notes.\n")

	result, err := ResolveContext(root, ContextOptions{Query: "incident playbook", Budget: 1200})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) == 0 {
		t.Fatal("expected context results")
	}
	if result.Results[0].Path != "guides/incident.md" {
		t.Fatalf("expected title metadata match first, got %#v", result.Results)
	}
	if result.EstimatedTokens <= 0 || result.EstimatedTokens > result.Budget {
		t.Fatalf("unexpected token accounting: %#v", result)
	}
}

func TestResolveContextTrimsOversizedTopMatchToBudget(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Budget Guide\n---\n\n# Budget\n\n"+strings.Repeat("token budget details stay relevant\n", 40))

	result, err := ResolveContext(root, ContextOptions{Query: "budget", Budget: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected one trimmed result, got %#v", result.Results)
	}
	if result.Results[0].EstimatedTokens > 30 || result.EstimatedTokens > 30 {
		t.Fatalf("expected result to fit budget, got %#v", result)
	}
	if result.Results[0].LineEnd >= 44 {
		t.Fatalf("expected truncated line range, got %#v", result.Results[0])
	}
}

func TestResolveContextIncludesLinkedNeighborWithinBudget(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "runbook.md", "---\ntype: Runbook\ntitle: Deploy Runbook\n---\n\n# Deploy\n\nBefore deploy read [Rollback](rollback.md).\n")
	writeFile(t, root, "rollback.md", "---\ntype: Runbook\ntitle: Rollback\n---\n\n# Rollback\n\nRestore the previous release.\n")

	result, err := ResolveContext(root, ContextOptions{Query: "deploy", Budget: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) < 2 {
		t.Fatalf("expected linked neighbor result, got %#v", result.Results)
	}
	if result.Results[1].Path != "rollback.md" || !result.Results[1].Neighbor {
		t.Fatalf("expected rollback neighbor second, got %#v", result.Results)
	}
}
