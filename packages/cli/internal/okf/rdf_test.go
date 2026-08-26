package okf

import (
	"bytes"
	"strings"
	"testing"
)

func TestRDFDatasetProjectsSemanticFactsAndStableNQuads(t *testing.T) {
	facts := SemanticFactSet{
		SchemaVersion: SemanticFactsSchemaVersion,
		Root:          "/knowledge",
		Revision:      RetrievalRevision{SpecVersion: "0.2", IndexSHA256: strings.Repeat("a", 64)},
		Valid:         true,
		Namespaces: []SemanticNamespace{
			{Prefix: "auth", IRI: "https://example.test/auth/"},
			{Prefix: "okn", IRI: oknVocabularyBase},
			{Prefix: "xsd", IRI: "http://www.w3.org/2001/XMLSchema#"},
			{Prefix: "unit", IRI: "http://qudt.org/vocab/unit/"},
			{Prefix: "quantitykind", IRI: "http://qudt.org/vocab/quantitykind/"},
		},
		Entities: []ClaimEntity{
			{ID: "auth:service", Types: []string{"auth:Service"}, PrefLabel: "Authentication service", AltLabels: []string{"Identity API"}},
		},
		Predicates: []ClaimPredicate{
			{ID: "auth:tokenFormat", SubjectTypes: []string{"auth:Service"}, ObjectKind: "literal", Datatype: "xsd:string", RequiredScope: []string{"auth:environment"}, MaximumCount: 1, PrefLabel: "token format"},
		},
		EvidenceRoles: []ClaimEvidenceRole{{ID: "auth:contract", PrefLabel: "API contract"}},
		Sources: []SemanticSource{{
			Key: "auth.md#contract", Document: "auth.md", ID: "contract", Resource: "openapi.yaml",
			Observe: "pinned", SHA256: strings.Repeat("b", 64), Role: "authoritative", Access: []string{"team:identity"},
			AuthorityApprovedBy: "human:alice",
		}},
		Claims: []SemanticClaim{{
			ID: "auth:claim/token-format", Slot: "auth:slot/token-format", Subject: "auth:service", Predicate: "auth:tokenFormat",
			Object: ClaimObject{Value: "JWT", Datatype: "xsd:string", Language: "en"},
			Status: "verified", TrustTier: "human-reviewed", Owners: []string{"team:identity"},
			Scope:     []SemanticScope{{Key: "auth:environment", Value: ClaimObject{Value: 60, Datatype: "xsd:integer", Unit: "unit:SEC", QuantityKind: "quantitykind:Time"}}},
			ValidTime: ClaimTimeInterval{From: "2026-08-25", Until: "2027-01-01"}, Stale: false, StaleAfter: "2027-01-01",
			Verification: &ClaimVerification{Method: "human-review", By: "human:alice", At: "2026-08-25T08:00:00Z", EvidenceRefs: []string{"evidence-1"}},
			Decisions:    []ClaimDecision{{Action: "archived", By: "human:alice", At: "2027-01-02", Reason: "fixture history"}},
			SectionRef:   "#token-format", EvidenceKeys: []string{"auth:claim/token-format#evidence-1"},
			Provenance: SemanticProvenance{Document: "auth.md", DocumentID: "auth", Locator: "okf+sha256://" + strings.Repeat("a", 64) + "/auth.md#" + strings.Repeat("c", 64), ContentSHA256: strings.Repeat("c", 64), LineStart: 10, LineEnd: 20},
		}},
		Evidence: []SemanticEvidence{{
			Key: "auth:claim/token-format#evidence-1", ClaimID: "auth:claim/token-format", ID: "evidence-1",
			SourceKey: "auth.md#contract", SourceRef: "contract", Stance: "supports", Role: "auth:contract",
			ObservedAt: "2026-08-25T07:00:00Z", Selector: &ClaimSelector{Type: "text_quote", Exact: "Tokens use JWT.", Prefix: "Production ", Suffix: " Always."},
		}},
		Relations:  []SemanticRelation{{SourceID: "auth:claim/token-format", Kind: "derived_from", TargetID: "auth:claim/source"}},
		References: []SemanticReference{{Document: "runbook.md", ClaimID: "auth:claim/token-format"}},
	}

	dataset, err := RDFDatasetFromFacts(facts)
	if err != nil {
		t.Fatal(err)
	}
	if dataset.SchemaVersion != RDFDatasetSchemaVersion || dataset.GraphIRI != "urn:openknowledge:revision:"+strings.Repeat("a", 64) || len(dataset.Quads) < 50 {
		t.Fatalf("unexpected RDF dataset identity or coverage: %#v", dataset)
	}
	claimIRI := "https://example.test/auth/claim/token-format"
	if !hasRDFQuad(dataset, claimIRI, rdfSubjectIRI, RDFTerm{Type: RDFTermIRI, Value: "https://example.test/auth/service"}) {
		t.Fatal("RDF projection lost the claim subject")
	}
	if !hasRDFPredicate(dataset, qudtUnitIRI) || !hasRDFPredicate(dataset, oaHasSelectorIRI) || !hasRDFPredicate(dataset, provWasDerivedFromIRI) || !hasRDFPredicate(dataset, okn("access")) {
		t.Fatal("RDF projection lost quantity, selector, provenance, or access data")
	}
	first, err := dataset.NQuads()
	if err != nil {
		t.Fatal(err)
	}
	secondDataset, err := RDFDatasetFromFacts(facts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := secondDataset.NQuads()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("N-Quads serialization is not deterministic")
	}
	for _, expected := range []string{"Tokens use JWT.", "team:identity", "fixture history", "auth.md", "wasDerivedFrom"} {
		if !bytes.Contains(first, []byte(expected)) {
			t.Fatalf("N-Quads output lost %q\n%s", expected, first)
		}
	}
	validateMachineInstance(t, compileMachineSchemas(t), "rdf-dataset", machineJSONValue(t, dataset))
}

func TestRDFDatasetRefusesInvalidSemanticFacts(t *testing.T) {
	facts := SemanticFactSet{Valid: false, Revision: RetrievalRevision{IndexSHA256: strings.Repeat("a", 64)}}
	if _, err := RDFDatasetFromFacts(facts); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected invalid fact gate, got %v", err)
	}
}

func hasRDFQuad(dataset RDFDataset, subject, predicate string, object RDFTerm) bool {
	for _, quad := range dataset.Quads {
		if quad.Subject == subject && quad.Predicate == predicate && quad.Object == object {
			return true
		}
	}
	return false
}

func hasRDFPredicate(dataset RDFDataset, predicate string) bool {
	for _, quad := range dataset.Quads {
		if quad.Predicate == predicate {
			return true
		}
	}
	return false
}
