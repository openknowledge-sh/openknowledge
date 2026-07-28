package okf

import (
	"reflect"
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

func TestContextIndexUsesCanonicalAnchorsAndAliases(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n\nOverview.\n\n#### Repeat\n\nLower-level detail.\n\n## Repeat\n\nCanonical duplicate content.\n\n## Empty Parent\n\n### Child\n\nChild content.\n")

	index, err := BuildContextIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.searchCorpus.documents) != len(index.Sections) {
		t.Fatalf("expected query-ready BM25 corpus to be built once with the context index: sections=%d documents=%d", len(index.Sections), len(index.searchCorpus.documents))
	}
	byID := map[string]ContextSection{}
	for _, section := range index.Sections {
		byID[section.ID] = section
	}
	if _, ok := byID["guide#repeat-2"]; !ok {
		t.Fatalf("expected chunk ID to preserve the canonical duplicate anchor, got %#v", index.Sections)
	}
	child, ok := byID["guide#child"]
	if !ok || !reflect.DeepEqual(child.Anchors, []string{"child", "empty-parent"}) {
		t.Fatalf("expected heading-only parent anchor to resolve to its content-bearing child, got %#v", child)
	}
	guide := byID["guide#guide"]
	if !testContainsString(guide.Anchors, "repeat") {
		t.Fatalf("expected lower-level heading anchor on its owning H1-H3 chunk, got %#v", guide)
	}
}

func testContainsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
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

func TestResolveContextKeepsHierarchicalEvidenceTogether(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "events.md", strings.Join([]string{
		"---",
		"type: Guide",
		"title: Application Events",
		"---",
		"",
		"# Events",
		"",
		"Application lifecycle reference.",
		"",
		"## Alternative Events",
		"",
		"The recommended way to handle startup and shutdown is lifespan. Event handlers are no longer called when lifespan is used; choose one approach, not both.",
		"",
		"### Startup and shutdown together",
		"",
		"Configure startup and shutdown handlers together.",
	}, "\n"))

	result, err := ResolveContext(root, ContextOptions{
		Query:  "startup shutdown recommended lifespan not both",
		Budget: 800,
		Limit:  4,
	})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]ContextSource{}
	for _, source := range result.Sources {
		byID[source.ID] = source
	}
	if _, ok := byID["events#alternative-events"]; !ok {
		t.Fatalf("expected parent explanation in context, got %#v", result.Sources)
	}
	if _, ok := byID["events#startup-and-shutdown-together"]; !ok {
		t.Fatalf("expected focused child section in context, got %#v", result.Sources)
	}
}

func TestResolveContextIncludesRelevantChildrenOfSelectedParent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "response.md", strings.Join([]string{
		"---",
		"type: Guide",
		"title: Response Model",
		"---",
		"",
		"# Response Model",
		"",
		"Return type reference.",
		"",
		"## Return Type and Data Filtering",
		"",
		"Filter or remove private response data while keeping a return type.",
		"",
		"### Type Annotations and Tooling",
		"",
		"The input model adds a password field.",
		"",
		"### FastAPI Data Filtering",
		"",
		"The response includes only the fields declared in the type.",
	}, "\n"))

	result, err := ResolveContext(root, ContextOptions{
		Query:  "response model filter remove password fields return type",
		Budget: 800,
		Limit:  5,
	})
	if err != nil {
		t.Fatal(err)
	}
	var markdown strings.Builder
	for _, source := range result.Sources {
		markdown.WriteString(source.Markdown)
		markdown.WriteByte('\n')
	}
	for _, expected := range []string{
		"Filter or remove private response data",
		"password field",
		"only the fields declared in the type",
	} {
		if !strings.Contains(markdown.String(), expected) {
			t.Fatalf("expected hierarchical evidence %q, got %#v", expected, result.Sources)
		}
	}
}

func TestResolveContextLabelsNonLexicalHierarchyAndNoExpandOmitsIt(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "websockets.md", strings.Join([]string{
		"---",
		"type: Guide",
		"title: Socket Guide",
		"---",
		"",
		"# Sockets",
		"",
		"Socket reference.",
		"",
		"## Using Depends",
		"",
		"Import Depends, Security, Cookie, Header, Path, and Query.",
		"",
		"### Try with dependencies",
		"",
		"Run the browser trial for dependencies.",
	}, "\n"))

	result, err := ResolveContext(root, ContextOptions{
		Query:  "dependencies browser trial",
		Budget: 500,
		Limit:  4,
	})
	if err != nil {
		t.Fatal(err)
	}
	parentFound := false
	for _, source := range result.Sources {
		if source.ID == "websockets#using-depends" {
			parentFound = true
			if source.Relation != "document-context" {
				t.Fatalf("expected explicit document-context relation, got %#v", source)
			}
		}
	}
	if !parentFound {
		t.Fatalf("expected non-lexical parent context, got %#v", result.Sources)
	}

	directOnly, err := ResolveContext(root, ContextOptions{
		Query:    "dependencies browser trial",
		Budget:   500,
		Limit:    4,
		NoExpand: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range directOnly.Sources {
		if source.Relation == "document-context" || source.ID == "websockets#using-depends" {
			t.Fatalf("expected --no-expand to omit structural context, got %#v", directOnly.Sources)
		}
	}
}

func TestPrioritizeContextResultsAddsUncoveredQueryEvidence(t *testing.T) {
	sections := []ContextSection{
		{ID: "release#local", Path: "release.md", Title: "Release", Heading: "Test locally", HeadingPath: []string{"Release", "Test locally"}, Text: "Test the build system locally."},
		{ID: "deep#one", Path: "deep.md", Title: "Build", Heading: "Build internals", HeadingPath: []string{"Build internals"}, Text: "Release build implementation details."},
		{ID: "deep#two", Path: "deep.md", Title: "Build", Heading: "Package internals", HeadingPath: []string{"Package internals"}, Text: "Release package implementation details."},
		{ID: "deep#three", Path: "deep.md", Title: "Build", Heading: "Workflow internals", HeadingPath: []string{"Workflow internals"}, Text: "Release workflow implementation details."},
		{ID: "deep#four", Path: "deep.md", Title: "Build", Heading: "Artifact internals", HeadingPath: []string{"Artifact internals"}, Text: "Release artifact implementation details."},
		{ID: "deep#five", Path: "deep.md", Title: "Build", Heading: "Signing internals", HeadingPath: []string{"Signing internals"}, Text: "Release signing implementation details."},
		{ID: "release#staging", Path: "release.md", Title: "Release", Heading: "Bumping packages", HeadingPath: []string{"Release", "Bumping packages"}, Text: "Use staging while avoiding an actual release."},
	}
	direct := []SearchResult{
		{ID: "release#local", Path: "release.md", Score: 100},
		{ID: "deep#one", Path: "deep.md", Score: 90},
		{ID: "deep#two", Path: "deep.md", Score: 80},
		{ID: "deep#three", Path: "deep.md", Score: 70},
		{ID: "deep#four", Path: "deep.md", Score: 60},
		{ID: "deep#five", Path: "deep.md", Score: 50},
		{ID: "release#staging", Path: "release.md", Score: 20},
	}

	prioritized := prioritizeContextResults(sections, direct, "test release build staging avoid actual", 6, false)
	if len(prioritized) < 6 || prioritized[0].ID != "release#local" || prioritized[5].ID != "release#staging" {
		t.Fatalf("expected uncovered staging evidence in the prioritized prefix, got %#v", prioritized)
	}
}

func TestPackContextTruncatesPrioritizedSectionInsteadOfSkippingIt(t *testing.T) {
	sections := []ContextSection{
		{ID: "guide#summary", Path: "guide.md", Heading: "Summary", Text: "Short summary.", EstimatedTokens: 4},
		{ID: "guide#details", Path: "guide.md", Heading: "Details", Text: "Decisive option.\n" + strings.Repeat("More detail.\n", 40), EstimatedTokens: 130},
		{ID: "other#small", Path: "other.md", Heading: "Small", Text: "Lower ranked note.", EstimatedTokens: 5},
	}
	direct := []SearchResult{
		{ID: "guide#summary", Path: "guide.md", Score: 100},
		{ID: "guide#details", Path: "guide.md", Score: 90, contextPriority: 1},
		{ID: "other#small", Path: "other.md", Score: 80},
	}

	sources := packContextSources(sections, direct, nil, 30, 3)
	if len(sources) != 2 || sources[1].ID != "guide#details" {
		t.Fatalf("expected the prioritized oversized source to be the final source, got %#v", sources)
	}
	if !strings.Contains(sources[1].Markdown, "Decisive option.") || sources[1].EstimatedTokens > 26 {
		t.Fatalf("expected useful truncated evidence within the remaining budget, got %#v", sources[1])
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
