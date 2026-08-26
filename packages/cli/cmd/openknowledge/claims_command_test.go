package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestClaimsCommandProposalApplyFindLinkImpactAndValidate(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeMainTestFile(t, root, "openapi.yaml", "Production token format.\n")
	writeMainTestFile(t, root, "auth.md", `---
type: Authentication
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces: {auth: https://example.test/auth/}
  entities: [{id: okn:service/auth}]
  predicates: [{id: auth:tokenFormat, object_kind: literal, datatype: xsd:string, maximum_count: 1}]
sources:
  - id: identity-openapi
    resource: openapi.yaml
    observe: pinned
    sha256: b87733eadcf3da6b39f1407939f630dd48ec885a5a97bd4c0e94df9fb13dd344
    role: authoritative
---

<a id="token-format"></a>

## Token format

Production token format.
`)
	writeMainTestFile(t, root, "runbook.md", "---\ntype: Runbook\n---\n\n# Runbook\n")
	writeMainTestFile(t, root, ".openknowledge/evals/auth.yaml", "type: openknowledge.eval\nversion: 1\nid: auth\ncases:\n  - id: token\n    question: What token format is used?\n    expect:\n      sources: [auth.md]\n")
	proposalPath := filepath.Join(root, "proposal.json")
	claimJSON := mainAuthoredClaimJSON("okn:claim/token-format/1", "JWT")

	stdout, stderr, code := captureMainOutput(t, func() int {
		return runClaims([]string{
			"propose", "--path", root, "--from", "auth.md", "--claim-json", claimJSON,
			"--reason", "OpenAPI evidence", "--confidence", "0.95", "--out", proposalPath,
		})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Wrote claim proposal") {
		t.Fatalf("propose failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"apply", proposalPath, "--path", root, "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("apply failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var mutation claimsMutationReport
	if err := json.Unmarshal([]byte(stdout), &mutation); err != nil || !mutation.Changed || mutation.ClaimID != "okn:claim/token-format/1" {
		t.Fatalf("unexpected apply report: %#v err=%v", mutation, err)
	}

	stdout, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"find", "token format", "--path", root, "--json"})
	})
	var found claimsFindReport
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &found) != nil || len(found.Matches) != 1 {
		t.Fatalf("find failed: code=%d stdout=%q stderr=%q report=%#v", code, stdout, stderr, found)
	}

	_, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"link", "okn:claim/token-format/1", "runbook.md", "--path", root, "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("link failed: code=%d stderr=%q", code, stderr)
	}

	stdout, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"impact", "okn:claim/token-format/1", "--path", root, "--json"})
	})
	var impact claimsImpactReport
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &impact) != nil || len(impact.Impact.Dependents) != 1 || len(impact.Impact.Evals) != 1 {
		t.Fatalf("impact failed: code=%d stdout=%q stderr=%q report=%#v", code, stdout, stderr, impact)
	}

	stdout, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"validate", "--path", root, "--json"})
	})
	var validation claimsValidationReport
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &validation) != nil || !validation.Valid {
		t.Fatalf("validate failed: code=%d stdout=%q stderr=%q report=%#v", code, stdout, stderr, validation)
	}

	_, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"dispute", "okn:claim/token-format/1", "--document", "auth.md", "--path", root, "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("dispute failed: code=%d stderr=%q", code, stderr)
	}
	disputedContent, _ := os.ReadFile(filepath.Join(root, "auth.md"))
	if !strings.Contains(string(disputedContent), "status: disputed") {
		t.Fatalf("claim was not marked disputed:\n%s", disputedContent)
	}

	if _, err := os.Stat(proposalPath); err != nil {
		t.Fatal(err)
	}
}

func TestClaimsEntitiesFindAndProposeCommands(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\ntype: Index\n---\n# Index\n")
	writeMainTestFile(t, root, "ontology.md", `---
type: Ontology
claim_ontology:
  entities:
    - id: okn:service/auth
      types: [okn:Service]
      pref_label: Authentication Service
      alt_labels: [Identity API]
---
# Ontology
`)
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runClaims([]string{"entities", "find", "Identity API", "--path", root, "--json"})
	})
	var report claimEntitiesReport
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &report) != nil || len(report.Matches) != 1 || report.Matches[0].Entity.ID != "okn:service/auth" {
		t.Fatalf("entity find failed: code=%d stdout=%q stderr=%q report=%#v", code, stdout, stderr, report)
	}
	proposalPath := filepath.Join(root, "entity-proposal.json")
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"entities", "propose", "--path", root, "--document", "ontology.md", "--entity", "okn:service/auth", "--alias", "Login API", "--reason", "Equivalent product name", "--confidence", "0.9", "--out", proposalPath})
	})
	content, err := os.ReadFile(proposalPath)
	if code != 0 || stderr != "" || err != nil || !strings.Contains(stdout, "Wrote entity proposal") || !strings.Contains(string(content), `"action": "add_alias"`) {
		t.Fatalf("entity proposal failed: code=%d stdout=%q stderr=%q content=%s err=%v", code, stdout, stderr, content, err)
	}
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"entities", "impact", proposalPath, "--path", root, "--json"})
	})
	var impact claimEntityImpactReport
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &impact) != nil || impact.Impact.Action != "add_alias" || !reflect.DeepEqual(impact.Impact.Documents, []string{"ontology.md"}) {
		t.Fatalf("entity impact failed: code=%d stdout=%q stderr=%q report=%#v", code, stdout, stderr, impact)
	}
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"entities", "apply", proposalPath, "--approved-by", "human:alice", "--path", root, "--json"})
	})
	var mutation claimEntityMutationReport
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &mutation) != nil || !mutation.Mutation.Changed || mutation.Mutation.ApprovedBy != "human:alice" {
		t.Fatalf("entity apply failed: code=%d stdout=%q stderr=%q report=%#v", code, stdout, stderr, mutation)
	}
	ontology, _ := os.ReadFile(filepath.Join(root, "ontology.md"))
	if !strings.Contains(string(ontology), "Login API") {
		t.Fatalf("entity alias was not applied:\n%s", ontology)
	}
}

func TestClaimsValidateRejectsProtectedHistoryRemoval(t *testing.T) {
	base := t.TempDir()
	candidate := t.TempDir()
	writeMainTestFile(t, base, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeMainTestFile(t, candidate, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeMainTestFile(t, base, "auth.md", mainTypedClaimDocument("okn:claim/token-format/1", "verified", "JWT"))
	writeMainTestFile(t, candidate, "auth.md", "---\ntype: Authentication\n---\n\n# Auth\n")
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runClaims([]string{"validate", "--path", candidate, "--against", base, "--json"})
	})
	var report claimsValidationReport
	if code != 1 || stderr != "" || json.Unmarshal([]byte(stdout), &report) != nil || report.Valid || len(report.Lifecycle) != 1 {
		t.Fatalf("expected lifecycle failure: code=%d stdout=%q stderr=%q report=%#v", code, stdout, stderr, report)
	}
}

func mainAuthoredClaimJSON(id, value string) string {
	claim := map[string]any{"id": id, "slot": "okn:slot/token-format", "subject": "okn:service/auth", "predicate": "auth:tokenFormat", "object": map[string]any{"value": value, "datatype": "xsd:string"}, "evidence": []any{map[string]any{"id": "okn:evidence/token-format", "sourceRef": "identity-openapi", "stance": "supports", "role": "primary", "selector": map[string]any{"type": "text_quote", "exact": "Production token format."}}}, "status": "proposed", "sectionRef": "#token-format"}
	content, _ := json.Marshal(claim)
	return string(content)
}

func mainTypedClaimDocument(id, status, value string) string {
	verification := ""
	if status == "verified" {
		verification = "    verification:\n      method: human-review\n      by: human:alice\n      at: 2026-08-22T00:00:00Z\n      evidence_refs: [okn:evidence/token-format]\n"
	}
	return `---
type: Authentication
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces: {auth: https://example.test/auth/}
  entities: [{id: okn:service/auth}]
  predicates: [{id: auth:tokenFormat, object_kind: literal, datatype: xsd:string, maximum_count: 1}]
sources: [{id: identity-openapi, resource: openapi.yaml, observe: pinned, sha256: b87733eadcf3da6b39f1407939f630dd48ec885a5a97bd4c0e94df9fb13dd344, role: authoritative}]
claims:
  - id: ` + id + `
    slot: okn:slot/token-format
    subject: okn:service/auth
    predicate: auth:tokenFormat
    object: {value: ` + value + `, datatype: xsd:string}
    evidence:
      - {id: okn:evidence/token-format, source_ref: identity-openapi, stance: supports, role: primary}
    status: ` + status + `
` + verification + `---

# Authentication

<a id="token-format"></a>

## Token format

Production token format.
`
}

func TestClaimsStaleAndReconcileEvidenceChange(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeMainTestFile(t, root, "source.txt", "one\n")
	document := strings.Replace(mainTypedClaimDocument("okn:claim/token-format/1", "proposed", "JWT"), "resource: openapi.yaml, observe: pinned, sha256: b87733eadcf3da6b39f1407939f630dd48ec885a5a97bd4c0e94df9fb13dd344", "resource: source.txt, observe: manual", 1)
	writeMainTestFile(t, root, "auth.md", document)

	_, stderr, code := captureMainOutput(t, func() int {
		return runClaims([]string{"verify", "okn:claim/token-format/1", "--document", "auth.md", "--approved-by", "human:alice", "--path", root})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("verify failed: code=%d stderr=%q", code, stderr)
	}
	writeMainTestFile(t, root, "source.txt", "two\n")
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runClaims([]string{"stale", "--path", root, "--json"})
	})
	var stale claimsStaleReport
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &stale) != nil || len(stale.Claims) != 1 || len(stale.Claims[0].Claim.StaleEvidence) != 1 {
		t.Fatalf("stale listing failed: code=%d stdout=%q stderr=%q report=%#v", code, stdout, stderr, stale)
	}
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"reconcile", "okn:claim/token-format/1", "--document", "auth.md", "--approved-by", "human:bob", "--path", root, "--json"})
	})
	var reconciled claimsReconcileReport
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &reconciled) != nil || !reconciled.Changed || len(reconciled.Versions) != 1 {
		t.Fatalf("reconcile failed: code=%d stdout=%q stderr=%q report=%#v", code, stdout, stderr, reconciled)
	}
	content, err := os.ReadFile(filepath.Join(root, "auth.md"))
	if err != nil || !strings.Contains(string(content), "verified:\n    - at:") || !strings.Contains(string(content), "by: human:bob") {
		t.Fatalf("reconcile did not project page verification: %v\n%s", err, content)
	}
}

func TestClaimsSuggestReturnsUncoveredSectionsWithoutMutatingKnowledge(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeMainTestFile(t, root, "auth.md", `---
type: Authentication
sources:
  - id: identity-policy
    resource: policy.md
---

# Authentication

## Token expiration

Access tokens expire after sixty minutes.
`)
	before, err := os.ReadFile(filepath.Join(root, "auth.md"))
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runClaims([]string{"suggest", "auth.md", "--path", root})
	})
	var report claimsSuggestionReport
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &report) != nil || len(report.Suggestions) != 1 {
		t.Fatalf("suggest failed: code=%d stdout=%q stderr=%q report=%#v", code, stdout, stderr, report)
	}
	suggestion := report.Suggestions[0]
	if suggestion.SectionRef != "#token-expiration" || suggestion.Status != "candidate" || !reflect.DeepEqual(suggestion.SourceIDs, []string{"identity-policy"}) {
		t.Fatalf("unexpected suggestion: %#v", suggestion)
	}
	after, err := os.ReadFile(filepath.Join(root, "auth.md"))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("suggest must not mutate knowledge: err=%v", err)
	}
}
