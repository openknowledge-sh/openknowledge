package okf

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSemanticFactsPreserveClaimIdentityAndSourceBindings(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeFile(t, root, "token-evidence.txt", "Production tokens use the declared format.")
	document := validTypedClaimDocument("okn:claim/token-format/2026-08-22", "JWT", "verified")
	document = strings.Replace(document, "claims:\n", `claims:
  - id: okn:claim/token-format/source
    slot: okn:slot/token-format
    subject: okn:service/auth
    predicate: auth:tokenFormat
    object:
      value: source-extraction
      datatype: xsd:string
    evidence: []
    status: proposed
`, 1)
	document = strings.Replace(document, "    evidence:\n", `    scope:
      okn:environment:
        value: production
        datatype: xsd:string
    valid_time:
      from: 2026-08-22
      until: 2027-01-01
    evidence:
`, 1)
	document = strings.Replace(document, "    status: verified\n", `    status: verified
    relations:
      derived_from: [okn:claim/token-format/source]
`, 1)
	document = strings.Replace(document, "    role: authoritative\n", "    role: authoritative\n    access: [team:identity]\n    authority_approved_by: human:alice\n", 1)
	writeFile(t, root, "auth.md", document)
	writeFile(t, root, "runbook.md", `---
type: Runbook
openknowledge_claim_profile: "1"
claim_refs: [okn:claim/token-format/2026-08-22]
---

# Runbook

Use the verified token format.
`)

	ast, err := ParseASTWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	validation, err := ValidateASTWithOptions(ast, ValidationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	facts := SemanticFactsFromAST(validation, ast, time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC))
	if len(facts.Issues) != 0 {
		t.Fatalf("unexpected semantic fact issues: %#v", facts.Issues)
	}
	if facts.SchemaVersion != SemanticFactsSchemaVersion || !facts.Valid || facts.Revision.SpecVersion != "0.2" || len(facts.Revision.IndexSHA256) != 64 {
		t.Fatalf("unexpected semantic fact identity: %#v", facts)
	}
	if len(facts.Claims) != 2 || facts.Claims[0].ID != "okn:claim/token-format/2026-08-22" {
		t.Fatalf("claims are not stable and sorted: %#v", facts.Claims)
	}
	claim := facts.Claims[0]
	if len(claim.Scope) != 1 || claim.Scope[0].Key != "okn:environment" || claim.ValidTime.Until != "2027-01-01" {
		t.Fatalf("typed scope or validity was lost: %#v", claim)
	}
	if claim.Provenance.Document != "auth.md" || claim.Provenance.ContentSHA256 == "" || !strings.HasPrefix(claim.Provenance.Locator, "okf+sha256://") || claim.Provenance.LineStart == 0 {
		t.Fatalf("claim provenance is not source-bound: %#v", claim.Provenance)
	}
	if len(facts.Sources) != 1 || facts.Sources[0].Key != "auth.md#identity-openapi" || !reflect.DeepEqual(facts.Sources[0].Access, []string{"team:identity"}) || facts.Sources[0].AuthorityApprovedBy != "human:alice" {
		t.Fatalf("source identity or access was lost: %#v", facts.Sources)
	}
	if len(facts.Evidence) != 1 || facts.Evidence[0].SourceKey != facts.Sources[0].Key || claim.EvidenceKeys[0] != facts.Evidence[0].Key {
		t.Fatalf("evidence bindings are not lossless: claim=%#v evidence=%#v", claim, facts.Evidence)
	}
	if len(facts.Relations) != 1 || facts.Relations[0].Kind != "derived_from" || facts.Relations[0].TargetID != "okn:claim/token-format/source" {
		t.Fatalf("claim relation was lost: %#v", facts.Relations)
	}
	if len(facts.References) != 1 || facts.References[0].Document != "runbook.md" || facts.References[0].ClaimID != claim.ID {
		t.Fatalf("claim reference was lost: %#v", facts.References)
	}

	first, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	second := SemanticFactsFromAST(validation, ast, time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC))
	encoded, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, encoded) {
		t.Fatal("semantic fact serialization is not deterministic")
	}
	validateMachineInstance(t, compileMachineSchemas(t), "semantic-facts", machineJSONValue(t, facts))
}

func TestSemanticFactsKeepAssertedOccurrencesSeparate(t *testing.T) {
	validation := Result{Root: "/knowledge", SpecVersion: "0.2"}
	ast := ASTBundle{Root: "/knowledge", SpecVersion: "0.2", Documents: []ASTDocument{}}
	facts := SemanticFactsFromAST(validation, ast, time.Unix(0, 0).UTC())
	if facts.Claims == nil || facts.Evidence == nil || facts.Relations == nil || facts.References == nil {
		t.Fatalf("empty semantic collections must serialize as arrays: %#v", facts)
	}
}
