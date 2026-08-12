package okf

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type searchRelevanceFixture struct {
	Version  int                    `json:"version"`
	Sections []ContextSection       `json:"sections"`
	Queries  []searchRelevanceQuery `json:"queries"`
}

type searchRelevanceQuery struct {
	Name     string                `json:"name"`
	Query    string                `json:"query"`
	Fuzzy    bool                  `json:"fuzzy"`
	NoExpand bool                  `json:"noExpand"`
	Expected []searchResultFixture `json:"expected"`
}

type searchResultFixture struct {
	Path          string   `json:"path"`
	ID            string   `json:"id"`
	Locator       string   `json:"locator"`
	Score         float64  `json:"score"`
	Matches       []string `json:"matches"`
	Snippet       string   `json:"snippet"`
	HighlightText string   `json:"highlightText"`
	Relation      string   `json:"relation"`
	Neighbor      bool     `json:"neighbor"`
}

func TestInvertedKnowledgeSearchMatchesReferenceScorer(t *testing.T) {
	fixture := readSearchRelevanceFixture(t)
	index := ContextIndex{
		Root:                 "/fixture",
		Sections:             fixture.Sections,
		searchCorpus:         newKnowledgeSearchCorpus(fixture.Sections),
		documentSearchCorpus: newKnowledgeSearchDocumentCorpus(aggregateKnowledgeSearchSections(fixture.Sections)),
		sectionLookup:        newContextSectionLookup(fixture.Sections),
	}

	for _, query := range fixture.Queries {
		t.Run(query.Name, func(t *testing.T) {
			options := SearchOptions{Query: query.Query, Limit: 12, Fuzzy: query.Fuzzy, NoExpand: query.NoExpand}
			got := index.Search(options)
			wantReference := searchKnowledgeWithCandidateResolver(index, options, allKnowledgeSearchDocumentIDs)
			if !reflect.DeepEqual(got.Results, wantReference.Results) {
				t.Fatalf("inverted search differs from reference scorer:\ngot:  %#v\nwant: %#v", got.Results, wantReference.Results)
			}

			summary := searchResultFixtures(got.Results)
			if !reflect.DeepEqual(summary, query.Expected) {
				t.Fatalf("relevance fixture changed:\ngot:  %#v\nwant: %#v", summary, query.Expected)
			}
		})
	}
}

func TestKnowledgeSearchExactCandidatesDoNotScanEverySection(t *testing.T) {
	sections := benchmarkContextSections(10_000)
	corpus := newKnowledgeSearchCorpus(sections)
	candidates := knowledgeSearchCandidateIDs(corpus, []string{"deployment"}, false)
	if len(candidates) != 1_000 {
		t.Fatalf("expected postings for 1,000 matching sections, got %d", len(candidates))
	}
	if len(candidates) == len(corpus.documents) {
		t.Fatal("exact lookup selected every section")
	}
}

func TestKnowledgeSearchIndexIsDeterministicForShuffledSections(t *testing.T) {
	sections := benchmarkContextSections(100)
	shuffled := append([]ContextSection(nil), sections...)
	for left, right := 0, len(shuffled)-1; left < right; left, right = left+1, right-1 {
		shuffled[left], shuffled[right] = shuffled[right], shuffled[left]
	}
	first := newKnowledgeSearchCorpus(sections)
	second := newKnowledgeSearchCorpus(shuffled)
	if !reflect.DeepEqual(first.vocabulary, second.vocabulary) || !reflect.DeepEqual(first.postings, second.postings) {
		t.Fatal("expected stable vocabulary and postings for shuffled input")
	}
	for documentID := range first.documents {
		if first.documents[documentID].section.ID != second.documents[documentID].section.ID {
			t.Fatal("expected stable numeric section identities for shuffled input")
		}
	}
}

func TestEditDistanceWithinPreservesASCIIAndUnicodeBehavior(t *testing.T) {
	tests := []struct {
		name    string
		left    string
		right   string
		maximum int
		want    bool
	}{
		{name: "exact ASCII", left: "validation", right: "validation", maximum: 0, want: true},
		{name: "ASCII substitution", left: "validation", right: "validatoin", maximum: 2, want: true},
		{name: "ASCII rejected", left: "validation", right: "release", maximum: 2, want: false},
		{name: "Unicode substitution", left: "příkaz", right: "prikaz", maximum: 2, want: true},
		{name: "Unicode rejected", left: "příkaz", right: "release", maximum: 2, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := editDistanceWithin(test.left, test.right, test.maximum); got != test.want {
				t.Fatalf("editDistanceWithin(%q, %q, %d) = %t, want %t", test.left, test.right, test.maximum, got, test.want)
			}
		})
	}
}

func readSearchRelevanceFixture(t *testing.T) searchRelevanceFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/search_relevance_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture searchRelevanceFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 {
		t.Fatalf("unsupported search relevance fixture version %d", fixture.Version)
	}
	return fixture
}

func allKnowledgeSearchDocumentIDs(corpus knowledgeSearchCorpus, _ []string, _ bool) []int {
	documentIDs := make([]int, len(corpus.documents))
	for documentID := range corpus.documents {
		documentIDs[documentID] = documentID
	}
	return documentIDs
}

func searchKnowledgeWithCandidateResolver(index ContextIndex, options SearchOptions, resolver knowledgeSearchCandidateResolver) SearchResultSet {
	result := SearchResultSet{Root: index.Root, Query: options.Query, Limit: options.Limit}
	result.Results = index.rankKnowledgeSearchWithCandidateResolver(options, resolver)
	if !options.NoExpand && len(result.Results) > 0 {
		seedCount := minInt(options.Limit, len(result.Results))
		direct, neighbors := index.knowledgeSearchGraphExpansion(result.Results[:seedCount], result.Results)
		result.Results = mergeKnowledgeSearchResults(direct, neighbors)
	}
	if len(result.Results) > options.Limit {
		result.Results = result.Results[:options.Limit]
	}
	return result
}

func searchResultFixtures(results []SearchResult) []searchResultFixture {
	fixtures := make([]searchResultFixture, 0, len(results))
	for _, result := range results {
		fixtures = append(fixtures, searchResultFixture{
			Path: result.Path, ID: result.ID, Locator: result.Locator, Score: result.Score,
			Matches: result.Matches, Snippet: result.Snippet, HighlightText: result.HighlightText,
			Relation: result.Relation, Neighbor: result.Neighbor,
		})
	}
	return fixtures
}
