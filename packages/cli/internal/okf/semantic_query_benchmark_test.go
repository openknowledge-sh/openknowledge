package okf

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func BenchmarkSPARQLSnapshotSelect(b *testing.B) {
	snapshot, err := SPARQLSnapshotFromFacts(sparqlFixtureFacts(), SPARQLQueryOptions{AllowedAccess: []string{"team:identity"}})
	if err != nil {
		b.Fatal(err)
	}
	query := `PREFIX okn: <https://openknowledge.dev/ns/>
SELECT ?claim WHERE { ?claim a okn:ClaimOccurrence } ORDER BY ?claim`
	b.ReportAllocs()
	b.ResetTimer()
	resultCount := 0
	for range b.N {
		result, err := snapshot.Query(context.Background(), query)
		if err != nil || len(result.Bindings) == 0 {
			b.Fatalf("SPARQL benchmark failed: bindings=%d err=%v", len(result.Bindings), err)
		}
		resultCount = len(result.Bindings)
	}
	b.ReportMetric(float64(resultCount), "results/op")
}

func BenchmarkDatalogSnapshotRecursiveProof(b *testing.B) {
	facts := benchmarkDatalogFacts(50)
	snapshot, err := DatalogSnapshotFromFacts(facts, DatalogQueryOptions{})
	if err != nil {
		b.Fatal(err)
	}
	request := DatalogQuery{
		Query: `depends_on("claim:049", Target)`,
		Rules: `depends_on(X, Y) :- relation(X, "derived_from", Y).
depends_on(X, Z) :- relation(X, "derived_from", Y), depends_on(Y, Z).`,
	}
	b.ReportAllocs()
	b.ResetTimer()
	resultCount := 0
	for range b.N {
		result, err := snapshot.Query(context.Background(), request)
		if err != nil || len(result.Results) != 49 {
			b.Fatalf("Datalog benchmark failed: results=%d err=%v", len(result.Results), err)
		}
		resultCount = len(result.Results)
	}
	b.ReportMetric(float64(resultCount), "results/op")
}

func BenchmarkHybridTextSnapshot(b *testing.B) {
	index := benchmarkContextIndex(1_000)
	index.Revision = RetrievalRevision{SpecVersion: "0.2", IndexSHA256: strings.Repeat("a", 64)}
	snapshot := &HybridSnapshot{
		root: "/benchmark", version: "0.2", contextIndex: index,
		facts: SemanticFactSet{Valid: true, Revision: index.Revision},
	}
	query := HybridQuery{Text: "deployment validation workflow", Limit: 12}
	b.ReportAllocs()
	b.ResetTimer()
	resultCount := 0
	for range b.N {
		result, err := snapshot.Query(context.Background(), query)
		if err != nil || len(result.Results) == 0 {
			b.Fatalf("hybrid benchmark failed: results=%d err=%v", len(result.Results), err)
		}
		resultCount = len(result.Results)
	}
	b.ReportMetric(float64(resultCount), "results/op")
}

func benchmarkDatalogFacts(count int) SemanticFactSet {
	revision := RetrievalRevision{SpecVersion: "0.2", IndexSHA256: strings.Repeat("b", 64)}
	facts := SemanticFactSet{SchemaVersion: SemanticFactsSchemaVersion, Root: "/benchmark", Revision: revision, Valid: true}
	for index := range count {
		id := fmt.Sprintf("claim:%03d", index)
		provenance := SemanticProvenance{
			Document: fmt.Sprintf("claims/%03d.md", index), Locator: fmt.Sprintf("okf+sha256://%s/claims/%03d.md", revision.IndexSHA256, index),
		}
		facts.Claims = append(facts.Claims, SemanticClaim{
			ID: id, Subject: id, Predicate: "benchmark:value", Object: ClaimObject{Value: index}, Status: "verified", Provenance: provenance,
		})
		if index > 0 {
			facts.Relations = append(facts.Relations, SemanticRelation{
				SourceID: id, Kind: "derived_from", TargetID: fmt.Sprintf("claim:%03d", index-1),
			})
		}
	}
	return facts
}
