package okf

import (
	"strings"
	"testing"
)

func TestFederatedSearchUsesDeterministicRRFAcrossKnowledgeBases(t *testing.T) {
	alpha := t.TempDir()
	beta := t.TempDir()
	writeFile(t, alpha, "index.md", "# Alpha\n")
	writeFile(t, alpha, "guide.md", "---\ntype: Guide\ntitle: Alpha Guide\n---\n\n# Release\n\nUse the release checklist.\n")
	writeFile(t, alpha, "notes.md", "---\ntype: Note\ntitle: Alpha Notes\n---\n\n# Checklist\n\nArchive the release checklist.\n")
	writeFile(t, beta, "index.md", "# Beta\n")
	writeFile(t, beta, "runbook.md", "---\ntype: Runbook\ntitle: Beta Runbook\n---\n\n# Release\n\nRelease release release checklist and rollback.\n")
	writeFile(t, beta, "notes.md", "---\ntype: Note\ntitle: Beta Notes\n---\n\n# Checklist\n\nReview the release checklist.\n")
	targets := []FederatedTarget{{Name: "beta", Root: beta}, {Name: "alpha", Root: alpha}}

	matches, err := SearchFederatedKnowledgeWithVersion(targets, "0.1", SearchOptions{Query: "release checklist", Limit: 5, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	if matches.Fusion.Method != "rrf" || matches.Fusion.RankConstant != 60 || len(matches.KnowledgeBases) != 2 {
		t.Fatalf("unexpected federation metadata: %#v", matches)
	}
	if matches.KnowledgeBases[0].Name != "alpha" || matches.KnowledgeBases[0].Status != "ok" || matches.KnowledgeBases[0].Revision == nil {
		t.Fatalf("expected sorted healthy knowledge bases: %#v", matches.KnowledgeBases)
	}
	if len(matches.Results) < 2 || matches.Results[0].KnowledgeBase != "alpha" || matches.Results[1].KnowledgeBase != "beta" {
		t.Fatalf("equal local ranks must use deterministic namespace tie-breaks, got %#v", matches.Results)
	}
	healthy := map[string]bool{}
	for _, base := range matches.KnowledgeBases {
		healthy[base.Name] = base.Status == "ok"
	}
	for _, candidate := range matches.Results {
		if !healthy[candidate.KnowledgeBase] || candidate.Result.Locator == "" {
			t.Fatalf("result must reference one healthy knowledge base: %#v", candidate)
		}
	}
	if matches.Results[0].FusionScore != matches.Results[1].FusionScore || matches.Results[1].Result.Score <= matches.Results[0].Result.Score {
		t.Fatalf("RRF must not compare raw cross-corpus BM25 scores: %#v", matches.Results)
	}
	if len(matches.Results) < 4 {
		t.Fatalf("expected two ranks from each corpus, got %#v", matches.Results)
	}
	wantOrder := []struct {
		name string
		rank int
	}{{"alpha", 1}, {"beta", 1}, {"alpha", 2}, {"beta", 2}}
	for index, want := range wantOrder {
		candidate := matches.Results[index]
		if candidate.KnowledgeBase != want.name || candidate.Rank != want.rank || candidate.FusionScore != federatedFusionScore(want.rank) {
			t.Fatalf("unexpected RRF interleave at %d: want=%#v got=%#v", index, want, candidate)
		}
	}

	context, err := ResolveFederatedContextWithVersion(targets, "0.1", ContextOptions{Query: "release checklist", Budget: 60, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(context.Sources) == 0 || context.EstimatedTokens > 60 || context.Sources[0].KnowledgeBase != "alpha" {
		t.Fatalf("expected one global deterministic context pack: %#v", context)
	}
	if !strings.Contains(context.Sources[0].Source.Locator, context.KnowledgeBases[0].Revision.IndexSHA256) {
		t.Fatalf("expected namespaced source to retain revision locator: %#v", context.Sources[0])
	}
}

func TestFederatedContextPacksOnceAndDefersOversizedCandidates(t *testing.T) {
	alpha := t.TempDir()
	beta := t.TempDir()
	writeFile(t, alpha, "index.md", "# Alpha\n")
	writeFile(t, alpha, "large.md", "---\ntype: Guide\n---\n\n# Topic\n\n"+strings.Repeat("federation evidence ", 80)+"\n")
	writeFile(t, beta, "index.md", "# Beta\n")
	writeFile(t, beta, "small.md", "---\ntype: Note\n---\n\n# Topic\n\nSmall federation evidence.\n")

	result, err := ResolveFederatedContextWithVersion([]FederatedTarget{{Name: "alpha", Root: alpha}, {Name: "beta", Root: beta}}, "0.1", ContextOptions{Query: "federation evidence", Budget: 30, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 2 || result.Sources[0].KnowledgeBase != "beta" || result.Sources[1].KnowledgeBase != "alpha" {
		t.Fatalf("expected small candidate then deferred truncation, got %#v", result.Sources)
	}
	if result.EstimatedTokens > result.Budget || result.Sources[1].Source.EstimatedTokens <= 0 {
		t.Fatalf("global context pack exceeded or failed to fill budget: %#v", result)
	}
}

func TestFederatedSearchIsolatesPerKnowledgeBaseFailures(t *testing.T) {
	healthy := t.TempDir()
	writeFile(t, healthy, "index.md", "# Healthy\n")
	writeFile(t, healthy, "guide.md", "---\ntype: Guide\n---\n\n# Recovery\n\nUse the recovery checklist.\n")
	targets := []FederatedTarget{{Name: "broken", Root: healthy + "/missing"}, {Name: "healthy", Root: healthy}}

	result, err := SearchFederatedKnowledgeWithVersion(targets, "0.1", SearchOptions{Query: "recovery", Limit: 5, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) == 0 || result.Results[0].KnowledgeBase != "healthy" {
		t.Fatalf("healthy results must survive peer failure: %#v", result)
	}
	if result.KnowledgeBases[0].Status != "error" || result.KnowledgeBases[0].Error == "" || result.KnowledgeBases[0].Revision != nil {
		t.Fatalf("expected explicit failed knowledge-base status: %#v", result.KnowledgeBases[0])
	}
	if _, err := SearchFederatedKnowledgeWithVersion([]FederatedTarget{{Name: "same", Root: healthy}, {Name: "same", Root: healthy}}, "0.1", SearchOptions{Query: "recovery"}); err == nil {
		t.Fatal("expected duplicate namespace rejection")
	}
	if _, err := SearchFederatedKnowledgeWithVersion(nil, "9.9", SearchOptions{Query: "recovery"}); err == nil {
		t.Fatal("expected unsupported spec rejection even for an empty target set")
	}
}
