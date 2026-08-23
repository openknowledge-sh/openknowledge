package okf

import (
	"fmt"
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

func TestSearchExcludesAgentContextAnnotations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", strings.Join([]string{
		"# Home",
		"",
		"Reader-visible guidance.",
		"",
		"<!-- okf-annotation: agent-context -->",
		"Private maintenance canary phrase.",
		"<!-- /okf-annotation -->",
	}, "\n"))

	results, err := Search(root, SearchOptions{Query: "canary phrase", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected agent context to be excluded from search, got %#v", results)
	}
	sectionResults, err := SearchKnowledge(root, SearchOptions{Query: "canary phrase", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(sectionResults.Results) != 0 {
		t.Fatalf("expected agent context to be excluded from section search, got %#v", sectionResults.Results)
	}

	results, err = Search(root, SearchOptions{Query: "visible guidance", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Path != "index.md" {
		t.Fatalf("expected reader content to remain searchable, got %#v", results)
	}
	sectionResults, err = SearchKnowledge(root, SearchOptions{Query: "visible guidance", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(sectionResults.Results) != 1 || sectionResults.Results[0].Path != "index.md" {
		t.Fatalf("expected reader content in section search, got %#v", sectionResults.Results)
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

func TestSearchKnowledgeAddsDeterministicVectorCandidates(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "policy.md", "---\ntype: Policy\ntitle: Access Policy\n---\n\n# Authorization\n\nAuthorization controls protected access.\n")

	results, err := SearchKnowledge(root, SearchOptions{Query: "authorizatiom", Limit: 5, Fuzzy: false, NoExpand: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Results) != 1 || results.Results[0].Path != "policy.md" {
		t.Fatalf("expected vector candidate, got %#v", results.Results)
	}
	if results.Results[0].VectorScore < 0.25 || results.Results[0].RerankScore == 0 {
		t.Fatalf("expected vector and rerank diagnostics, got %#v", results.Results[0])
	}
}

func TestSearchKnowledgeAppliesMetadataFiltersBeforeRanking(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Release\ntags: [operations]\n---\n\n# Release\n\nRelease checklist.\n")
	writeFile(t, root, "note.md", "---\ntype: Note\ntitle: Release\ntags: [internal]\n---\n\n# Release\n\nRelease checklist.\n")

	results, err := SearchKnowledge(root, SearchOptions{Query: "release checklist", Limit: 1, Filters: SearchFilters{Types: []string{"Guide"}, Tags: []string{"operations"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Results) != 1 || results.Results[0].Path != "guide.md" {
		t.Fatalf("expected filtered guide, got %#v", results.Results)
	}
}

func TestSearchKnowledgeUsesWholeDocumentEvidenceForOverviewPages(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "guides/projects.md", strings.Join([]string{
		"---",
		"type: Guide",
		"title: Working on projects",
		"---",
		"",
		"# Projects",
		"",
		"Project workflow overview.",
		"",
		"## Managing packages",
		"",
		"Use add to include a dependency.",
		"",
		"## Lock data",
		"",
		"The lockfile records the selected versions.",
		"",
		"## Environment",
		"",
		"The project environment is updated automatically.",
	}, "\n"))
	for index, body := range []string{
		"Dependency project reference.",
		"Lockfile project reference.",
		"Project environment reference.",
		"Add dependency reference.",
		"Lockfile environment reference.",
		"Dependency environment reference.",
	} {
		writeFile(t, root, fmt.Sprintf("reference/%d.md", index), fmt.Sprintf("---\ntype: Reference\ntitle: Project %s\n---\n\n# Project %s\n\n%s\n", body, body, body))
	}

	results, err := SearchKnowledge(root, SearchOptions{
		Query: "add dependency lockfile project environment",
		Limit: 10,
		Fuzzy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var overview SearchResult
	for _, result := range results.Results {
		if result.Path == "guides/projects.md" {
			overview = result
			break
		}
	}
	if overview.Path == "" || overview.Heading != "Managing packages" {
		t.Fatalf("expected the overview's best-covered section in the top ten, got %#v", results.Results)
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

	results, err := SearchKnowledge(root, SearchOptions{Query: "deploy checklist", Limit: 5, Fuzzy: true})
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

	directOnly, err := SearchKnowledge(root, SearchOptions{Query: "deploy checklist", Limit: 5, Fuzzy: true, NoExpand: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range directOnly.Results {
		if result.Neighbor {
			t.Fatalf("expected NoExpand to omit graph neighbors, got %#v", directOnly.Results)
		}
	}
}

func TestSearchKnowledgeExpansionCanReplaceWeakDirectMatchAtLimit(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "runbook.md", "---\ntype: Runbook\ntitle: Deploy Checklist\n---\n\n# Deploy Checklist\n\nUse the production deploy checklist and read [Rollback](rollback.md).\n")
	writeFile(t, root, "rollback.md", "---\ntype: Runbook\ntitle: Rollback Plan\n---\n\n# Rollback\n\nRestore the previous release.\n")
	writeFile(t, root, "notes.md", "---\ntype: Note\ntitle: Miscellaneous\n---\n\n# Notes\n\nThe deploy word appears in a broad note.\n")

	results, err := SearchKnowledge(root, SearchOptions{Query: "deploy checklist", Limit: 2, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Results) != 2 {
		t.Fatalf("expected two limited results, got %#v", results.Results)
	}
	if results.Results[0].Path != "runbook.md" || results.Results[0].Relation != "direct" {
		t.Fatalf("expected strongest direct result first, got %#v", results.Results)
	}
	if results.Results[1].Path != "rollback.md" || results.Results[1].Relation != "outgoing-link" {
		t.Fatalf("expected strong authored neighbor to replace weak direct match, got %#v", results.Results)
	}
}

func TestSearchKnowledgeExpansionTargetsAnchorsAndBoostsWeakDirectMatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "runbook.md", "---\ntype: Runbook\ntitle: Deploy Checklist\n---\n\n# Deploy\n\nUse the production deploy checklist and read [Rollback](guide.md#rollback).\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Operations Guide\n---\n\n# Overview\n\nGeneral notes.\n\n## Recovery\n\nDeploy recovery overview.\n\n#### Rollback\n\nRestore the previous release.\n")
	writeFile(t, root, "owner.md", "---\ntype: Team\ntitle: Recovery Owner\n---\n\n# Owner\n\nPlatform owns [recovery](guide.md#recovery).\n")
	writeFile(t, root, "wrong-owner.md", "---\ntype: Team\ntitle: Overview Owner\n---\n\n# Owner\n\nDocs owns [the overview](guide.md#overview).\n")

	directOnly, err := SearchKnowledge(root, SearchOptions{Query: "deploy checklist", Limit: 10, Fuzzy: true, NoExpand: true})
	if err != nil {
		t.Fatal(err)
	}
	baseline, ok := searchResultByID(directOnly.Results, "guide#recovery")
	if !ok {
		t.Fatalf("expected a weak direct guide match, got %#v", directOnly.Results)
	}

	expanded, err := SearchKnowledge(root, SearchOptions{Query: "deploy checklist", Limit: 10, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	boosted, ok := searchResultByID(expanded.Results, "guide#recovery")
	if !ok || boosted.Neighbor || boosted.Relation != "direct" || boosted.Score <= baseline.Score || !testContainsString(boosted.Matches, "graph") {
		t.Fatalf("expected weak lexical match to retain direct evidence and receive the graph score, baseline=%#v expanded=%#v", baseline, boosted)
	}
	owner, ok := searchResultByID(expanded.Results, "owner#owner")
	if !ok || !owner.Neighbor || owner.Relation != "backlink" {
		t.Fatalf("expected fragment-specific backlink to the recovery chunk, got %#v", expanded.Results)
	}
	if _, ok := searchResultByID(expanded.Results, "wrong-owner#owner"); ok {
		t.Fatalf("did not expect a backlink addressed to another chunk, got %#v", expanded.Results)
	}
}

func searchResultByID(results []SearchResult, id string) (SearchResult, bool) {
	for _, result := range results {
		if result.ID == id {
			return result, true
		}
	}
	return SearchResult{}, false
}
