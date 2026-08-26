package okf

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

type hybridRelevanceFixture struct {
	Version   int `json:"version"`
	Documents []struct {
		Path  string `json:"path"`
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"documents"`
	Queries []struct {
		Text         string `json:"text"`
		RelevantPath string `json:"relevantPath"`
	} `json:"queries"`
}

type hybridRelevanceEmbeddingProvider struct {
	mu    sync.Mutex
	calls int
}

func (provider *hybridRelevanceEmbeddingProvider) Model() EmbeddingModel {
	return EmbeddingModel{Provider: "fixture", ID: "hybrid-relevance-v1", Revision: "1", Dimensions: 3, Metric: EmbeddingMetricCosine}
}

func (provider *hybridRelevanceEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	provider.mu.Lock()
	provider.calls++
	provider.mu.Unlock()
	result := make([][]float32, len(texts))
	for index, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lower := strings.ToLower(text)
		switch {
		case containsAny(lower, "authentication", "credential", "identity", "login", "account"):
			result[index] = []float32{1, 0, 0}
		case containsAny(lower, "deployment", "rollout", "release", "shipping"):
			result[index] = []float32{0, 1, 0}
		case containsAny(lower, "storage", "persistence", "backup", "database", "records"):
			result[index] = []float32{0, 0, 1}
		default:
			result[index] = []float32{0, 0, 0}
		}
	}
	return result, nil
}

func (provider *hybridRelevanceEmbeddingProvider) callCount() int {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.calls
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func TestHybridQueryRoutesExplicitIntentsAndFusesSourceEvidence(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeFile(t, root, "token-evidence.txt", "Production tokens use the declared format.")
	writeFile(t, root, "auth.md", validTypedClaimDocument("okn:claim/token-format/2026-08-22", "JWT", "verified"))
	snapshot, err := BuildHybridSnapshotWithVersion(root, "0.2", HybridQueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.Query(context.Background(), HybridQuery{
		Text: "declared token format",
		SPARQL: `PREFIX okn: <https://openknowledge.dev/ns/>
SELECT ?claim WHERE { ?claim a okn:ClaimOccurrence }`,
		Datalog: &DatalogQuery{
			Query: `uses_jwt(ID)`, Rules: `uses_jwt(ID) :- claim(ID, _, _, "JWT").`,
		},
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Routes) != 5 || result.Routes[0].Name != "bm25" || result.Routes[2].Name != "section-focus" || result.Routes[4].Name != "datalog" || result.Fusion.Method != HybridFusionRRF {
		t.Fatalf("unexpected hybrid routing: %#v", result.Routes)
	}
	kinds := map[string]int{}
	textStructuredBoost := false
	for _, item := range result.Results {
		kinds[item.Kind]++
		if item.Text != nil {
			for _, component := range item.Components {
				if component.Route == "sparql-source" || component.Route == "datalog-source" {
					textStructuredBoost = true
				}
			}
		}
	}
	if kinds[HybridKindRetrievedText] == 0 || kinds[HybridKindAssertedFact] == 0 || kinds[HybridKindDerivedFact] == 0 || !textStructuredBoost {
		t.Fatalf("hybrid result did not preserve text, asserted, derived, or source joins: %#v", result.Results)
	}
	validateMachineInstance(t, compileMachineSchemas(t), "hybrid-query", machineJSONValue(t, result))
}

func TestHybridQueryDoesNotRunUnrequestedStructuredEngines(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Index\n\nLocal vector retrieval.\n")
	snapshot, err := BuildHybridSnapshotWithVersion(root, "0.2", HybridQueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.Query(context.Background(), HybridQuery{Text: "vector retrieval"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Routes) != 3 || result.Routes[0].Name != "bm25" || result.Routes[1].Name != "vector" || result.Routes[2].Name != "section-focus" {
		t.Fatalf("text-only query ran unnecessary engines: %#v", result.Routes)
	}
	for _, item := range result.Results {
		if item.Kind != HybridKindRetrievedText {
			t.Fatalf("text-only query returned structured result: %#v", item)
		}
	}
}

func TestHybridRRFPreservesRankResolution(t *testing.T) {
	first := HybridResult{}
	second := HybridResult{}
	addHybridComponent(&first, "bm25", 1, 10)
	addHybridComponent(&second, "bm25", 2, 9)
	if first.Components[0].RRFScore <= second.Components[0].RRFScore {
		t.Fatalf("RRF rounding collapsed adjacent ranks: first=%v second=%v", first.Components, second.Components)
	}
}

func TestHybridSectionFocusPrefersSpecificAnswerSection(t *testing.T) {
	query := "How do deterministic maintenance jobs run verification in isolated worktrees?"
	generic := ContextSection{Heading: "Jobs", HeadingPath: []string{"Jobs"}, Text: "Run maintenance jobs in isolated worktrees."}
	specific := ContextSection{Heading: "Runtime behavior", HeadingPath: []string{"Jobs", "Runtime behavior"}, Text: "A deterministic job runs verification in an isolated Git worktree after the agent completes."}
	genericScore := hybridSectionFocusScore(query, generic)
	specificScore := hybridSectionFocusScore(query, specific)
	if specificScore <= genericScore {
		t.Fatalf("specific answer section did not beat generic document introduction: generic=%f specific=%f", genericScore, specificScore)
	}
}

func TestHybridVersionedRelevanceCorpusImprovesOverBM25(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("testdata", "hybrid-relevance-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture hybridRelevanceFixture
	if err := json.Unmarshal(content, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Documents) == 0 || len(fixture.Queries) == 0 {
		t.Fatalf("invalid relevance fixture: %#v", fixture)
	}
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Relevance fixture\n")
	for _, document := range fixture.Documents {
		writeFile(t, root, document.Path, "---\ntype: Guide\ntitle: "+document.Title+"\n---\n\n# "+document.Title+"\n\n"+document.Body+"\n")
	}
	provider := &hybridRelevanceEmbeddingProvider{}
	snapshot, err := BuildHybridSnapshotWithVersion(root, "0.2", HybridQueryOptions{Embedding: provider})
	if err != nil {
		t.Fatal(err)
	}
	baselineMRR := 0.0
	hybridMRR := 0.0
	for _, query := range fixture.Queries {
		ranked := snapshot.contextIndex.rankKnowledgeSearch(SearchOptions{Query: query.Text, Limit: 12, Fuzzy: true, NoExpand: true})
		lexicalRank := 0
		for _, item := range ranked {
			if item.LexicalScore <= 0 {
				continue
			}
			lexicalRank++
			if item.Path == query.RelevantPath {
				baselineMRR += 1 / float64(lexicalRank)
				break
			}
		}
		result, err := snapshot.Query(context.Background(), HybridQuery{Text: query.Text, Limit: 12})
		if err != nil {
			t.Fatal(err)
		}
		for rank, item := range result.Results {
			if item.Text != nil && item.Text.Path == query.RelevantPath {
				hybridMRR += 1 / float64(rank+1)
				break
			}
		}
	}
	baselineMRR /= float64(len(fixture.Queries))
	hybridMRR /= float64(len(fixture.Queries))
	if hybridMRR <= baselineMRR || hybridMRR < 0.99 {
		t.Fatalf("hybrid relevance did not improve: baseline MRR=%.3f hybrid MRR=%.3f", baselineMRR, hybridMRR)
	}
	t.Logf("hybrid relevance v1: baseline BM25 MRR=%.3f, hybrid MRR=%.3f, queries=%d", baselineMRR, hybridMRR, len(fixture.Queries))
}

func TestHybridSnapshotEmbedsOnceAndDoesNotRereadSources(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n\nAuthentication guidance.\n")
	provider := &hybridRelevanceEmbeddingProvider{}
	snapshot, err := BuildHybridSnapshotWithVersion(root, "0.1", HybridQueryOptions{Embedding: provider})
	if err != nil {
		t.Fatal(err)
	}
	buildCalls := provider.callCount()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n\nReplacement content.\n")
	result, err := snapshot.Query(context.Background(), HybridQuery{Text: "login security", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if provider.callCount() != buildCalls+1 {
		t.Fatalf("snapshot rebuilt corpus embeddings during query: build=%d current=%d", buildCalls, provider.callCount())
	}
	if len(result.Results) == 0 || result.Results[0].Text == nil || len(result.Results[0].Components) == 0 || result.Results[0].Components[0].RawScore != 1 {
		t.Fatalf("snapshot reread replacement source content: %#v", result.Results)
	}
}

func TestHybridQueryRejectsSPARQLAskBecauseItCannotRankBindings(t *testing.T) {
	facts := sparqlFixtureFacts()
	snapshot := &HybridSnapshot{
		root: "/knowledge", version: "0.2", facts: facts,
		contextIndex: ContextIndex{Root: "/knowledge", Revision: facts.Revision},
	}
	_, err := snapshot.Query(context.Background(), HybridQuery{SPARQL: `ASK { ?s ?p ?o }`})
	if err == nil || !strings.Contains(err.Error(), "requires a SELECT") {
		t.Fatalf("expected explicit hybrid ASK rejection, got %v", err)
	}
}

func TestHybridQueryAppliesAccessAndLifecycleBeforeDatalog(t *testing.T) {
	facts := sparqlFixtureFacts()
	facts.Claims[0].Status = "archived"
	snapshot := &HybridSnapshot{
		root: "/knowledge", version: "0.2", facts: facts,
		contextIndex: ContextIndex{Root: "/knowledge", Revision: facts.Revision},
	}
	result, err := snapshot.Query(context.Background(), HybridQuery{
		Datalog: &DatalogQuery{Query: `claim(ID, Subject, Predicate, Object)`}, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 0 || len(result.Rejections) != 3 {
		t.Fatalf("policy filters were not applied before reasoning: %#v", result)
	}
	encoded := string(mustJSON(t, result))
	if strings.Contains(encoded, "auth:secrets") || strings.Contains(encoded, "restricted.md") {
		t.Fatalf("policy-rejected values leaked: %s", encoded)
	}
}

func TestSPARQLAndDatalogAssertedClaimQueriesHaveParity(t *testing.T) {
	facts := sparqlFixtureFacts()
	access := []string{"team:identity"}
	sparqlSnapshot, err := SPARQLSnapshotFromFacts(facts, SPARQLQueryOptions{AllowedAccess: access})
	if err != nil {
		t.Fatal(err)
	}
	graph, err := sparqlSnapshot.Query(context.Background(), `
PREFIX rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#>
PREFIX okn: <https://openknowledge.dev/ns/>
SELECT ?claim ?subject ?predicate ?object WHERE {
  ?claim a okn:ClaimOccurrence ;
         rdf:subject ?subject ;
         rdf:predicate ?predicate ;
         rdf:object ?object .
}`)
	if err != nil {
		t.Fatal(err)
	}
	datalogSnapshot, err := DatalogSnapshotFromFacts(facts, DatalogQueryOptions{AllowedAccess: access})
	if err != nil {
		t.Fatal(err)
	}
	logic, err := datalogSnapshot.Query(context.Background(), DatalogQuery{Query: `claim(ID, Subject, Predicate, Object)`})
	if err != nil {
		t.Fatal(err)
	}
	graphTuples := make([]string, 0, len(graph.Bindings))
	for _, binding := range graph.Bindings {
		graphTuples = append(graphTuples, strings.Join([]string{
			binding.Values["claim"].Value, binding.Values["subject"].Value,
			binding.Values["predicate"].Value, binding.Values["object"].Value,
		}, "\x00"))
	}
	logicTuples := make([]string, 0, len(logic.Results))
	namespaces := map[string]string{"auth": "https://example.test/auth/"}
	for _, item := range logic.Results {
		values := make([]string, len(item.Values))
		for index, value := range item.Values {
			values[index] = expandFixtureCURIE(value.Value, namespaces)
		}
		logicTuples = append(logicTuples, strings.Join(values, "\x00"))
	}
	slices.Sort(graphTuples)
	slices.Sort(logicTuples)
	if !slices.Equal(graphTuples, logicTuples) {
		t.Fatalf("asserted query parity failed:\nSPARQL=%q\nDatalog=%q", graphTuples, logicTuples)
	}
}

func expandFixtureCURIE(value string, namespaces map[string]string) string {
	prefix, local, ok := strings.Cut(value, ":")
	if ok && namespaces[prefix] != "" {
		return namespaces[prefix] + local
	}
	return value
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
