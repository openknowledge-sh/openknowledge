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

func TestContextIndexUsesParsedMarkdownSections(t *testing.T) {
	document := ASTDocument{
		Rel:   "guide.md",
		ID:    "guide",
		Kind:  "concept",
		Body:  "# Raw Heading\n\nBody text.\n",
		Links: nil,
		Frontmatter: ASTFrontmatter{
			BodyLine: 1,
		},
		Markdown: ASTMarkdown{
			Headings: []ASTMarkdownHeading{
				{Level: 1, Text: "Raw Heading", Anchor: "raw-heading", Line: 1},
			},
		},
	}

	index := ContextIndexFromAST(Result{Root: "root"}, ASTBundle{Root: "root", Documents: []ASTDocument{document}})

	if len(index.Sections) != 1 {
		t.Fatalf("expected one top section from empty Markdown sections, got %#v", index.Sections)
	}
	if index.Sections[0].Heading != "Top" || index.Sections[0].HeadingLevel != 0 {
		t.Fatalf("expected context to trust parsed Markdown sections, got %#v", index.Sections[0])
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
	if len(result.Sources) == 0 {
		t.Fatal("expected context results")
	}
	if result.Sources[0].Path != "guides/incident.md" {
		t.Fatalf("expected BM25 title metadata match first, got %#v", result.Sources)
	}
	if result.EstimatedTokens <= 0 || result.EstimatedTokens > result.Budget {
		t.Fatalf("unexpected token accounting: %#v", result)
	}
	if !strings.Contains(result.Sources[0].Markdown, "escalation checklist") || result.Sources[0].Relation != "direct" {
		t.Fatalf("expected source-preserving direct context, got %#v", result.Sources[0])
	}
	matches, err := SearchKnowledge(root, SearchOptions{Query: "incident playbook", Limit: 12, Fuzzy: true, NoExpand: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches.Results) == 0 || matches.Results[0].ID != result.Sources[0].ID || matches.Results[0].Score != result.Sources[0].Score {
		t.Fatalf("expected context and matches to share BM25 ranking, context=%#v matches=%#v", result.Sources, matches.Results)
	}
	limited, err := ResolveContext(root, ContextOptions{Query: "incident", Budget: 1200, Limit: 1, NoExpand: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(limited.Sources) != 1 || limited.Limit != 1 {
		t.Fatalf("expected context limit to cap selected sources, got %#v", limited)
	}
}

func TestResolveContextTrimsOversizedTopMatchToBudget(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Budget Guide\n---\n\n# Budget\n\n"+strings.Repeat("token budget details stay relevant\n", 40))

	result, err := ResolveContext(root, ContextOptions{Query: "budget", Budget: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected one trimmed result, got %#v", result.Sources)
	}
	if result.Sources[0].EstimatedTokens > 30 || result.EstimatedTokens > 30 {
		t.Fatalf("expected result to fit budget, got %#v", result)
	}
	if result.Sources[0].LineEnd >= 44 {
		t.Fatalf("expected truncated line range, got %#v", result.Sources[0])
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
	if len(result.Sources) < 2 {
		t.Fatalf("expected linked neighbor result, got %#v", result.Sources)
	}
	if result.Sources[1].Path != "rollback.md" || result.Sources[1].Relation != "outgoing-link" {
		t.Fatalf("expected rollback neighbor second, got %#v", result.Sources)
	}
	if !strings.Contains(result.Sources[1].Markdown, "Restore the previous release") {
		t.Fatalf("expected original rollback Markdown, got %#v", result.Sources[1])
	}

	directOnly, err := ResolveContext(root, ContextOptions{Query: "deploy", Budget: 500, NoExpand: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range directOnly.Sources {
		if source.Relation != "direct" {
			t.Fatalf("expected NoExpand to omit related sources, got %#v", directOnly.Sources)
		}
	}
}

func TestRetrievalRevisionAndLocatorsBindResultsToIndexedContent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "guides/auth.md", "---\ntype: Guide\ntitle: Authentication\n---\n\n# Authentication\n\nUse short-lived OAuth tokens.\n")

	first, err := ResolveContextWithVersion(root, "0.1", ContextOptions{Query: "OAuth tokens", Budget: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Sources) != 1 {
		t.Fatalf("expected one source, got %#v", first.Sources)
	}
	if first.Revision.SpecVersion != "0.1" || len(first.Revision.IndexSHA256) != 64 {
		t.Fatalf("expected concrete retrieval revision, got %#v", first.Revision)
	}
	source := first.Sources[0]
	if len(source.ContentSHA256) != 64 || !strings.Contains(source.Locator, first.Revision.IndexSHA256) || !strings.Contains(source.Locator, source.ContentSHA256) || !strings.Contains(source.Locator, "guides%2Fauth.md") {
		t.Fatalf("expected revision-bound source locator, got %#v", source)
	}

	repeated, err := SearchKnowledgeWithVersion(root, "0.1", SearchOptions{Query: "OAuth tokens", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if repeated.Revision != first.Revision || len(repeated.Results) != 1 || repeated.Results[0].Locator != source.Locator {
		t.Fatalf("expected context and matches to share revision identity: context=%#v matches=%#v", first, repeated)
	}

	writeFile(t, root, "guides/auth.md", "---\ntype: Guide\ntitle: Authentication\n---\n\n# Authentication\n\nUse rotated short-lived OAuth tokens.\n")
	changed, err := ResolveContextWithVersion(root, "0.1", ContextOptions{Query: "OAuth tokens", Budget: 500})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Revision.IndexSHA256 == first.Revision.IndexSHA256 || changed.Sources[0].ContentSHA256 == source.ContentSHA256 || changed.Sources[0].Locator == source.Locator {
		t.Fatalf("expected content edit to invalidate revision and locator: before=%#v after=%#v", first, changed)
	}
}
