package okf

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const recursiveDependencyRules = `
depends(X, Y) :- claim(_, X, "auth:dependsOn", Y).
reachable(X, Y) :- depends(X, Y).
reachable(X, Z) :- reachable(X, Y), depends(Y, Z).
`

func TestDatalogSnapshotReturnsRecursiveDerivedFactsWithProofPaths(t *testing.T) {
	facts := sparqlFixtureFacts()
	snapshot, err := DatalogSnapshotFromFacts(facts, DatalogQueryOptions{AllowedAccess: []string{"team:identity"}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.Query(context.Background(), DatalogQuery{
		Query: `reachable("auth:service", Dependency)`, Rules: recursiveDependencyRules,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RuleProfile != DatalogProfileSafe || len(result.Results) != 2 || result.Results[0].Kind != DatalogResultDerived || len(result.Results[1].Sources) != 2 {
		t.Fatalf("unexpected recursive Datalog result: %#v", result)
	}
	if result.Results[1].Values[1].Value != "auth:secrets" || result.Results[1].Proof.Kind != "rule" || !proofContainsKind(result.Results[1].Proof, "asserted") {
		t.Fatalf("derived result lost recursive proof: %#v", result.Results[1])
	}
	validateMachineInstance(t, compileMachineSchemas(t), "datalog-query", machineJSONValue(t, result))
}

func TestDatalogSnapshotAppliesAccessBeforeReasoning(t *testing.T) {
	snapshot, err := DatalogSnapshotFromFacts(sparqlFixtureFacts(), DatalogQueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.Query(context.Background(), DatalogQuery{
		Query: `reachable("auth:service", Dependency)`, Rules: recursiveDependencyRules,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Policy.RemovedClaims != 1 || len(result.Results) != 1 || result.Results[0].Values[1].Value != "auth:database" {
		t.Fatalf("restricted facts affected Datalog reasoning: %#v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secrets") || strings.Contains(string(encoded), "restricted.md") {
		t.Fatalf("restricted data leaked through proof: %s", encoded)
	}
}

func TestDatalogClosedWorldNegationRequiresExplicitProfile(t *testing.T) {
	facts := sparqlFixtureFacts()
	facts.Claims[1].Status = "supported"
	snapshot, err := DatalogSnapshotFromFacts(facts, DatalogQueryOptions{AllowedAccess: []string{"team:identity"}})
	if err != nil {
		t.Fatal(err)
	}
	rules := `needs_review(ID) :- claim(ID, _, _, _), !status(ID, "verified").`
	if _, err := snapshot.Query(context.Background(), DatalogQuery{Query: `needs_review(ID)`, Rules: rules}); err == nil || !strings.Contains(err.Error(), "closed-world") {
		t.Fatalf("expected explicit closed-world profile gate, got %v", err)
	}
	result, err := snapshot.Query(context.Background(), DatalogQuery{
		Query: `needs_review(ID)`, Rules: rules, RuleProfile: DatalogProfileClosedWorld,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].Values[0].Value != "auth:claim/db-secret" || !proofContainsKind(result.Results[0].Proof, "closed-world-absence") {
		t.Fatalf("unexpected closed-world result or proof: %#v", result)
	}
}

func TestDatalogSnapshotEnforcesSafeSubsetAndResourceLimits(t *testing.T) {
	facts := sparqlFixtureFacts()
	if _, err := DatalogSnapshotFromFacts(facts, DatalogQueryOptions{Limits: DatalogLimits{MaxBaseFacts: 1}}); err == nil || !strings.Contains(err.Error(), "base facts") {
		t.Fatalf("expected base fact limit, got %v", err)
	}
	snapshot, err := DatalogSnapshotFromFacts(facts, DatalogQueryOptions{
		AllowedAccess: []string{"team:identity"}, Limits: DatalogLimits{MaxRuleBytes: 256, MaxResults: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := snapshot.Query(context.Background(), DatalogQuery{Query: `claim(ID, S, P, O)`})
	if err != nil || len(result.Results) != 1 || !result.Truncated || result.Results[0].Kind != DatalogResultAsserted {
		t.Fatalf("expected bounded asserted results: %#v err=%v", result, err)
	}
	if _, err := snapshot.Query(context.Background(), DatalogQuery{Query: `injected(X)`, Rules: `injected("x").`}); err == nil || !strings.Contains(err.Error(), "base facts") {
		t.Fatalf("expected rule fact injection gate, got %v", err)
	}
	if _, err := snapshot.Query(context.Background(), DatalogQuery{Query: `bad(X)`, Rules: `bad(X) :- X = "x".`}); err == nil || !strings.Contains(err.Error(), "only positive atoms") {
		t.Fatalf("expected safe premise subset gate, got %v", err)
	}
	queryLimited, err := DatalogSnapshotFromFacts(facts, DatalogQueryOptions{Limits: DatalogLimits{MaxQueryBytes: 8}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queryLimited.Query(context.Background(), DatalogQuery{Query: `claim(ID, S, P, O)`}); err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Fatalf("expected query byte limit, got %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := snapshot.Query(canceled, DatalogQuery{Query: `claim(ID, S, P, O)`}); err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("expected canceled context, got %v", err)
	}
}

func proofContainsKind(proof DatalogProof, kind string) bool {
	if proof.Kind == kind {
		return true
	}
	for _, input := range proof.Inputs {
		if proofContainsKind(input, kind) {
			return true
		}
	}
	return false
}
