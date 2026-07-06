package okf

import (
	"reflect"
	"strings"
	"testing"
)

func TestSearchBundleRanksTitleAndBodyMatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead the incident playbook.\n")
	writeFile(t, root, "guides/incident.md", "---\ntype: Guide\ntitle: Incident Playbook\ndescription: Triage production alerts.\n---\n\n# Incident Response\n\nRun `openknowledge validate` before sharing updates.\n")
	writeFile(t, root, "notes/release.md", "---\ntype: Note\ntitle: Release Notes\n---\n\n# Release\n\nIncident details belong in the guide.\n")

	results, err := Search(root, SearchOptions{Query: "incident playbook", Limit: 5, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0].Path != "guides/incident.md" {
		t.Fatalf("expected guide title match first, got %#v", results)
	}
	if results[0].Snippet == "" {
		t.Fatalf("expected snippet in result: %#v", results[0])
	}
	if results[0].HighlightText != "Incident Playbook" {
		t.Fatalf("expected exact phrase highlight from title, got %#v", results[0])
	}
}

func TestSearchIndexFromASTMatchesBundleSearch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead the incident playbook.\n")
	writeFile(t, root, "guides/incident.md", "---\ntype: Guide\ntitle: Incident Playbook\ndescription: Triage production alerts.\n---\n\n# Incident Response\n\nRun `openknowledge validate` before sharing updates.\n")

	bundle, err := ParseBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	astIndex, err := buildSearchIndex(root)
	if err != nil {
		t.Fatal(err)
	}

	options := SearchOptions{Query: "incident playbook", Limit: 5, Fuzzy: true}
	bundleResults := SearchBundle(bundle, options)
	astResults := astIndex.Search(options)
	if len(bundleResults) == 0 || len(astResults) == 0 {
		t.Fatalf("expected search results from both paths, bundle=%#v ast=%#v", bundleResults, astResults)
	}
	if !reflect.DeepEqual(bundleResults[0], astResults[0]) {
		t.Fatalf("expected AST search to match bundle search, bundle=%#v ast=%#v", bundleResults[0], astResults[0])
	}
}

func TestSearchUsesASTBackedIndex(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead the incident playbook.\n")
	writeFile(t, root, "guides/incident.md", "---\ntype: Guide\ntitle: Incident Playbook\ndescription: Triage production alerts.\n---\n\n# Incident Response\n\nRun `openknowledge validate` before sharing updates.\n")

	bundle, err := ParseBundle(root)
	if err != nil {
		t.Fatal(err)
	}

	options := SearchOptions{Query: "incident playbook", Limit: 5, Fuzzy: true}
	bundleResults := SearchBundle(bundle, options)
	astResults, err := Search(root, options)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bundleResults, astResults) {
		t.Fatalf("expected AST-backed search to match bundle search, bundle=%#v ast=%#v", bundleResults, astResults)
	}
}

func TestSearchBundleSupportsFuzzyAndDiacriticInsensitiveMatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "guides/commands.md", "---\ntype: Guide\ntitle: Prikazy\n---\n\n# Prikazova Radka\n\nPříkazová řádka spouští validaci wiki.\n")

	diacriticResults, err := Search(root, SearchOptions{Query: "prikazova radka", Limit: 5, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(diacriticResults) == 0 || diacriticResults[0].Path != "guides/commands.md" {
		t.Fatalf("expected diacritic-insensitive match, got %#v", diacriticResults)
	}

	fuzzyResults, err := Search(root, SearchOptions{Query: "validaci", Limit: 5, Fuzzy: false})
	if err != nil {
		t.Fatal(err)
	}
	if len(fuzzyResults) == 0 {
		t.Fatal("expected exact normalized match before fuzzy check")
	}

	fuzzyResults, err = Search(root, SearchOptions{Query: "validace", Limit: 5, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(fuzzyResults) == 0 || fuzzyResults[0].Path != "guides/commands.md" {
		t.Fatalf("expected fuzzy match, got %#v", fuzzyResults)
	}
	if fuzzyResults[0].HighlightText != "validaci" {
		t.Fatalf("expected fuzzy highlight to use the visible matched token, got %#v", fuzzyResults[0])
	}
}

func TestSearchHighlightFallsBackToMatchedVisibleToken(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "customers/acme.md", "---\ntype: Customer\ntitle: ACME Account\n---\n\n# ACME\n\nThe onboarding playbook names the decision owner.\n")

	results, err := Search(root, SearchOptions{Query: "playbook decision", Limit: 5, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search result")
	}
	if results[0].HighlightText != "playbook" {
		t.Fatalf("expected first visible matched token as fallback highlight, got %#v", results[0])
	}
}

func TestSearchHighlightOmitsPathOnlyMatch(t *testing.T) {
	bundle := Bundle{Files: []BundleFile{{
		Path: "customers/acme.md",
		Kind: "concept",
	}}}

	results := SearchBundle(bundle, SearchOptions{Query: "acme", Limit: 5, Fuzzy: true})
	if len(results) == 0 {
		t.Fatal("expected path-only result")
	}
	if results[0].HighlightText != "" {
		t.Fatalf("expected path-only match to omit highlight text, got %#v", results[0])
	}
}

func TestSearchBundleRanksIndexMarkdownBelowRegularPages(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/index.md", "# Index\n\nShared ranking topic.\n")
	writeFile(t, root, "docs/topic.md", "---\ntype: Note\ntitle: Index\n---\n\n# Index\n\nShared ranking topic.\n")

	results, err := Search(root, SearchOptions{Query: "index", Limit: 5, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) < 2 {
		t.Fatalf("expected multiple search results, got %#v", results)
	}
	if results[0].Path != "docs/topic.md" {
		t.Fatalf("expected regular page to outrank index.md, got %#v", results)
	}
}

func TestSearchBundleReturnsNoResultsForBlankQuery(t *testing.T) {
	results := SearchBundle(Bundle{}, SearchOptions{Query: "   "})
	if len(results) != 0 {
		t.Fatalf("expected no blank-query results, got %#v", results)
	}
}

func TestSearchKnowledgeRanksHeadingChunksWithBM25(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nSee [Authentication](guides/auth.md).\n")
	writeFile(t, root, "guides/auth.md", strings.Join([]string{
		"---",
		"type: Guide",
		"title: MCP Guide",
		"description: Configure remote MCP access.",
		"---",
		"",
		"# MCP",
		"",
		"General MCP setup notes.",
		"",
		"## Authentication",
		"",
		"Use OAuth tokens for private MCP authentication.",
		"Store the issuer and audience beside the deployment checklist.",
		"",
		"## Deployment",
		"",
		"Deploy the HTTP server after authentication is configured.",
	}, "\n"))
	writeFile(t, root, "notes/long.md", "---\ntype: Note\ntitle: General Notes\n---\n\n# General\n\n"+strings.Repeat("authentication ", 80)+"MCP appears once in a broad note.\n")

	results, err := SearchKnowledge(root, SearchOptions{Query: "MCP authentication", Limit: 5, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Results) == 0 {
		t.Fatal("expected knowledge search results")
	}
	first := results.Results[0]
	if first.Path != "guides/auth.md" || first.Heading != "Authentication" {
		t.Fatalf("expected focused authentication chunk first, got %#v", results.Results)
	}
	if !reflect.DeepEqual(first.HeadingPath, []string{"MCP", "Authentication"}) {
		t.Fatalf("expected heading path metadata, got %#v", first)
	}
	if first.LineStart == 0 || first.LineEnd < first.LineStart {
		t.Fatalf("expected source line range on result, got %#v", first)
	}
	if !strings.Contains(first.Snippet, "OAuth tokens") {
		t.Fatalf("expected section snippet from matching chunk, got %#v", first)
	}
	if first.Score <= 0 {
		t.Fatalf("expected positive BM25-style score, got %#v", first)
	}
}

func TestSearchKnowledgeSkipsHeadingOnlyParentChunks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "workflows/feature-docs.md", strings.Join([]string{
		"---",
		"type: Workflow",
		"title: Feature Docs Workflow",
		"---",
		"",
		"# Feature Docs Workflow",
		"",
		"## Trigger",
		"",
		"Use this workflow when touching command documentation.",
	}, "\n"))

	results, err := SearchKnowledge(root, SearchOptions{Query: "feature docs workflow", Limit: 5, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Results) == 0 {
		t.Fatal("expected search result")
	}
	for _, result := range results.Results {
		if result.Path == "workflows/feature-docs.md" && result.Heading == "Feature Docs Workflow" {
			t.Fatalf("expected heading-only parent chunk to be omitted, got %#v", results.Results)
		}
	}
	first := results.Results[0]
	if first.Path != "workflows/feature-docs.md" || first.Heading != "Trigger" {
		t.Fatalf("expected first result to be content-bearing child section, got %#v", results.Results)
	}
	if !strings.Contains(first.Snippet, "touching command documentation") {
		t.Fatalf("expected child section snippet, got %#v", first)
	}
}

func TestSearchKnowledgeExpandsThroughGraphNeighbors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "runbook.md", "---\ntype: Runbook\ntitle: Deploy Runbook\n---\n\n# Deploy\n\nRun the deploy checklist and read [Rollback](rollback.md).\n")
	writeFile(t, root, "rollback.md", "---\ntype: Runbook\ntitle: Rollback Plan\n---\n\n# Rollback\n\nRestore the previous release when verification fails.\n")
	writeFile(t, root, "owners.md", "---\ntype: Team\ntitle: Owners\n---\n\n# Owners\n\nPlatform owns the [Runbook](runbook.md).\n")

	results, err := SearchKnowledge(root, SearchOptions{Query: "deploy checklist", Limit: 5, Fuzzy: true, ExpandGraph: true})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]SearchResult{}
	for _, result := range results.Results {
		if _, ok := byPath[result.Path]; !ok {
			byPath[result.Path] = result
		}
	}
	if result, ok := byPath["runbook.md"]; !ok || result.Neighbor {
		t.Fatalf("expected direct runbook match, got %#v", results.Results)
	}
	if result, ok := byPath["rollback.md"]; !ok || !result.Neighbor || result.Relation != "outgoing-link" {
		t.Fatalf("expected outgoing rollback neighbor, got %#v", results.Results)
	}
	if result, ok := byPath["owners.md"]; !ok || !result.Neighbor || result.Relation != "backlink" {
		t.Fatalf("expected backlink owner neighbor, got %#v", results.Results)
	}
}
