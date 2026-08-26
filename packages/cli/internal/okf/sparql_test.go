package okf

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSPARQLSnapshotSupportsSelectAskAggregationAndPropertyPaths(t *testing.T) {
	facts := sparqlFixtureFacts()
	snapshot, err := SPARQLSnapshotFromFacts(facts, SPARQLQueryOptions{AllowedAccess: []string{"team:identity"}})
	if err != nil {
		t.Fatal(err)
	}
	selectResult, err := snapshot.Query(context.Background(), `
PREFIX okn: <https://openknowledge.dev/ns/>
SELECT ?claim WHERE { ?claim a okn:ClaimOccurrence }
ORDER BY ?claim`)
	if err != nil {
		t.Fatal(err)
	}
	if selectResult.QueryType != "select" || len(selectResult.Bindings) != 2 || len(selectResult.Bindings[0].Sources) != 1 || selectResult.Revision != facts.Revision {
		t.Fatalf("unexpected source-bound SELECT result: %#v", selectResult)
	}

	askResult, err := snapshot.Query(context.Background(), `
PREFIX auth: <https://example.test/auth/>
ASK { auth:service auth:dependsOn auth:database }`)
	if err != nil || askResult.Boolean == nil || !*askResult.Boolean || len(askResult.Bindings) != 0 {
		t.Fatalf("unexpected ASK result: %#v err=%v", askResult, err)
	}

	aggregate, err := snapshot.Query(context.Background(), `
PREFIX okn: <https://openknowledge.dev/ns/>
SELECT (COUNT(?claim) AS ?count) WHERE { ?claim a okn:ClaimOccurrence }`)
	if err != nil || len(aggregate.Bindings) != 1 || aggregate.Bindings[0].Values["count"].Value != "2" {
		t.Fatalf("unexpected aggregate result: %#v err=%v", aggregate, err)
	}

	pathResult, err := snapshot.Query(context.Background(), `
PREFIX auth: <https://example.test/auth/>
SELECT ?dependency WHERE { auth:service auth:dependsOn+ ?dependency }`)
	if err != nil || len(pathResult.Bindings) != 2 || pathResult.Bindings[1].Values["dependency"].Value != "https://example.test/auth/secrets" {
		t.Fatalf("unexpected property-path result: %#v err=%v", pathResult, err)
	}

	graphResult, err := snapshot.Query(context.Background(), `SELECT ?claim WHERE {
GRAPH <urn:openknowledge:revision:`+strings.Repeat("a", 64)+`> {
  ?claim a <https://openknowledge.dev/ns/ClaimOccurrence>
}}`)
	if err != nil || len(graphResult.Bindings) != 2 {
		t.Fatalf("unexpected named graph result: %#v err=%v", graphResult, err)
	}
	validateMachineInstance(t, compileMachineSchemas(t), "sparql-query", machineJSONValue(t, selectResult))
}

func TestSPARQLSnapshotAppliesAccessBeforeEvaluation(t *testing.T) {
	snapshot, err := SPARQLSnapshotFromFacts(sparqlFixtureFacts(), SPARQLQueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.Query(context.Background(), `
PREFIX auth: <https://example.test/auth/>
SELECT ?dependency WHERE { auth:service auth:dependsOn+ ?dependency }`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Policy.RemovedSources != 1 || result.Policy.RemovedClaims != 1 || len(result.Bindings) != 1 || result.Bindings[0].Values["dependency"].Value != "https://example.test/auth/database" {
		t.Fatalf("restricted facts leaked into SPARQL result: %#v", result)
	}
	encodedBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(encodedBytes)
	if strings.Contains(encoded, "secrets") || strings.Contains(encoded, "restricted.md") {
		t.Fatalf("restricted identifiers or provenance leaked: %s", encoded)
	}
}

func TestSPARQLSnapshotEnforcesLimitsAndReadOnlyQueryForms(t *testing.T) {
	facts := sparqlFixtureFacts()
	if _, err := SPARQLSnapshotFromFacts(facts, SPARQLQueryOptions{Limits: SPARQLLimits{MaxDatasetQuads: 1}}); err == nil || !strings.Contains(err.Error(), "quads") {
		t.Fatalf("expected dataset limit error, got %v", err)
	}
	snapshot, err := SPARQLSnapshotFromFacts(facts, SPARQLQueryOptions{
		AllowedAccess: []string{"team:identity"}, Limits: SPARQLLimits{MaxQueryBytes: 128, MaxResults: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.Query(context.Background(), `SELECT ?s WHERE { ?s ?p ?o }`)
	if err != nil || len(result.Bindings) != 1 || !result.Truncated {
		t.Fatalf("expected bounded truncated result: %#v err=%v", result, err)
	}
	if _, err := snapshot.Query(context.Background(), strings.Repeat("x", 129)); err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Fatalf("expected query byte limit, got %v", err)
	}
	if _, err := snapshot.Query(context.Background(), `CONSTRUCT { ?s ?p ?o } WHERE { ?s ?p ?o }`); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected read-only query form gate, got %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := snapshot.Query(canceled, `ASK { ?s ?p ?o }`); err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("expected canceled query context, got %v", err)
	}
}

func sparqlFixtureFacts() SemanticFactSet {
	provenance := func(document string, marker string) SemanticProvenance {
		return SemanticProvenance{
			Document: document, DocumentID: strings.TrimSuffix(document, ".md"),
			Locator:       "okf+sha256://" + strings.Repeat("a", 64) + "/" + document + "#" + strings.Repeat(marker, 64),
			ContentSHA256: strings.Repeat(marker, 64), LineStart: 10, LineEnd: 20,
		}
	}
	return SemanticFactSet{
		SchemaVersion: SemanticFactsSchemaVersion, Root: "/knowledge", Valid: true,
		Revision: RetrievalRevision{SpecVersion: "0.2", IndexSHA256: strings.Repeat("a", 64)},
		Namespaces: []SemanticNamespace{
			{Prefix: "auth", IRI: "https://example.test/auth/"},
			{Prefix: "okn", IRI: oknVocabularyBase},
			{Prefix: "xsd", IRI: "http://www.w3.org/2001/XMLSchema#"},
		},
		Predicates: []ClaimPredicate{{ID: "auth:dependsOn", ObjectKind: "entity", MaximumCount: 10}},
		Sources: []SemanticSource{
			{Key: "public.md#public", Document: "public.md", ID: "public", Resource: "public.txt"},
			{Key: "restricted.md#private", Document: "restricted.md", ID: "private", Resource: "private.txt", Access: []string{"team:identity"}},
		},
		Claims: []SemanticClaim{
			{ID: "auth:claim/service-db", Slot: "auth:slot/service-db", Subject: "auth:service", Predicate: "auth:dependsOn", Object: ClaimObject{Ref: "auth:database"}, Status: "verified", TrustTier: "human-reviewed", EvidenceKeys: []string{"auth:claim/service-db#e1"}, Owners: []string{}, Scope: []SemanticScope{}, Provenance: provenance("public.md", "b")},
			{ID: "auth:claim/db-secret", Slot: "auth:slot/db-secret", Subject: "auth:database", Predicate: "auth:dependsOn", Object: ClaimObject{Ref: "auth:secrets"}, Status: "verified", TrustTier: "human-reviewed", EvidenceKeys: []string{"auth:claim/db-secret#e2"}, Owners: []string{}, Scope: []SemanticScope{}, Provenance: provenance("restricted.md", "c")},
		},
		Evidence: []SemanticEvidence{
			{Key: "auth:claim/service-db#e1", ClaimID: "auth:claim/service-db", ID: "e1", SourceKey: "public.md#public", SourceRef: "public", Stance: "supports", Role: "primary"},
			{Key: "auth:claim/db-secret#e2", ClaimID: "auth:claim/db-secret", ID: "e2", SourceKey: "restricted.md#private", SourceRef: "private", Stance: "supports", Role: "primary"},
		},
		Entities: []ClaimEntity{}, EvidenceRoles: []ClaimEvidenceRole{}, Relations: []SemanticRelation{}, References: []SemanticReference{},
	}
}
