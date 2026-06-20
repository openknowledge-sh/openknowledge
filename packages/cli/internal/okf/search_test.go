package okf

import (
	"reflect"
	"testing"
)

func TestSearchBundleRanksTitleAndBodyMatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nRead the incident playbook.\n")
	writeFile(t, root, "guides/incident.md", "---\ntype: Guide\ntitle: Incident Playbook\ndescription: Triage production alerts.\n---\n\n# Incident Response\n\nRun `openknowledge validate` before sharing updates.\n")
	writeFile(t, root, "notes/release.md", "---\ntype: Note\ntitle: Release Notes\n---\n\n# Release\n\nIncident details belong in the guide.\n")

	bundle, err := ParseBundle(root)
	if err != nil {
		t.Fatal(err)
	}

	results := SearchBundle(bundle, SearchOptions{Query: "incident playbook", Limit: 5, Fuzzy: true})
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0].Path != "guides/incident.md" {
		t.Fatalf("expected guide title match first, got %#v", results)
	}
	if results[0].Snippet == "" {
		t.Fatalf("expected snippet in result: %#v", results[0])
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
	_, ast, err := parseAndValidateASTBundle(root, LatestSpecVersion)
	if err != nil {
		t.Fatal(err)
	}

	options := SearchOptions{Query: "incident playbook", Limit: 5, Fuzzy: true}
	bundleResults := SearchBundle(bundle, options)
	astResults := newSearchIndexFromAST(ast).Search(options)
	if len(bundleResults) == 0 || len(astResults) == 0 {
		t.Fatalf("expected search results from both paths, bundle=%#v ast=%#v", bundleResults, astResults)
	}
	if !reflect.DeepEqual(bundleResults[0], astResults[0]) {
		t.Fatalf("expected AST search to match bundle search, bundle=%#v ast=%#v", bundleResults[0], astResults[0])
	}
}

func TestSearchBundleSupportsFuzzyAndDiacriticInsensitiveMatches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n")
	writeFile(t, root, "guides/commands.md", "---\ntype: Guide\ntitle: Prikazy\n---\n\n# Prikazova Radka\n\nPříkazová řádka spouští validaci wiki.\n")

	bundle, err := ParseBundle(root)
	if err != nil {
		t.Fatal(err)
	}

	diacriticResults := SearchBundle(bundle, SearchOptions{Query: "prikazova radka", Limit: 5, Fuzzy: true})
	if len(diacriticResults) == 0 || diacriticResults[0].Path != "guides/commands.md" {
		t.Fatalf("expected diacritic-insensitive match, got %#v", diacriticResults)
	}

	fuzzyResults := SearchBundle(bundle, SearchOptions{Query: "validaci", Limit: 5, Fuzzy: false})
	if len(fuzzyResults) == 0 {
		t.Fatal("expected exact normalized match before fuzzy check")
	}

	fuzzyResults = SearchBundle(bundle, SearchOptions{Query: "validace", Limit: 5, Fuzzy: true})
	if len(fuzzyResults) == 0 || fuzzyResults[0].Path != "guides/commands.md" {
		t.Fatalf("expected fuzzy match, got %#v", fuzzyResults)
	}
}

func TestSearchBundleRanksIndexMarkdownBelowRegularPages(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/index.md", "# Index\n\nShared ranking topic.\n")
	writeFile(t, root, "docs/topic.md", "---\ntype: Note\ntitle: Index\n---\n\n# Index\n\nShared ranking topic.\n")

	bundle, err := ParseBundle(root)
	if err != nil {
		t.Fatal(err)
	}

	results := SearchBundle(bundle, SearchOptions{Query: "index", Limit: 5, Fuzzy: true})
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
