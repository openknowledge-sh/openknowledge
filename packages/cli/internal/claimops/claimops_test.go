package claimops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestProposalApplyFindImpactAndImmutableID(t *testing.T) {
	root := t.TempDir()
	writeClaimopsFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n# Index\n")
	writeClaimopsFile(t, root, "openapi.yaml", "Tokens use JWT.\n")
	writeClaimopsFile(t, root, "auth.md", typedClaimHeader()+"---\n\n<a id=\"token-format\"></a>\n\n## Token format\n\nTokens use JWT.\n")
	writeClaimopsFile(t, root, "runbook.md", "---\ntype: Runbook\nopenknowledge_claim_profile: \"1\"\n---\n# Runbook\n")
	claim := authoredTypedClaim("okn:claim/token-format/2026-08-22", "JWT")
	proposal, err := NewProposal(root, "auth.md", claim, "The OpenAPI defines the token representation.", 0.94)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := ApplyProposal(root, "0.2", proposal)
	if err != nil || !changed {
		t.Fatalf("apply: %t %v", changed, err)
	}
	if _, err := ApplyProposal(root, "0.2", proposal); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("expected digest guard: %v", err)
	}
	index, err := BuildIndex(root, "0.2", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Issues) != 0 || len(Find(index, "auth:tokenFormat")) != 1 {
		t.Fatalf("unexpected index: %#v", index)
	}
	if _, err := NewProposal(root, "auth.md", claim, "Duplicate occurrence.", .9); err != nil {
		t.Fatal(err)
	}
	linked, err := Link(root, "0.2", claim.ID, "runbook.md")
	if err != nil || !linked {
		t.Fatalf("link: %t %v", linked, err)
	}
	impact, err := BuildImpact(indexAfter(t, root), claim.ID, nil)
	if err != nil || len(impact.Dependents) != 1 {
		t.Fatalf("impact: %#v %v", impact, err)
	}
}

func TestPinnedSelectorProposalRoundTrip(t *testing.T) {
	root := t.TempDir()
	writeClaimopsFile(t, root, "index.md", "# Index\n")
	writeClaimopsFile(t, root, "openapi.yaml", "Tokens use JWT.\n")
	writeClaimopsFile(t, root, "auth.md", typedClaimHeader()+"---\n\n<a id=\"token-format\"></a>\n\nTokens use JWT.\n")
	proposal, err := NewProposal(root, "auth.md", authoredTypedClaim("okn:claim/token-format/round-trip", "JWT"), "Pinned evidence round trip.", 0.9)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeProposal(proposal)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeProposal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, proposal) {
		t.Fatalf("proposal lost pinned selector data:\nwant %#v\ngot  %#v", proposal, decoded)
	}
}

func TestPinEvidenceMaterializesImmutableArtifactAndRepairsSelectorBinding(t *testing.T) {
	root := t.TempDir()
	writeClaimopsFile(t, root, "index.md", "# Index\n")
	writeClaimopsFile(t, root, "capture.txt", "Tokens use JWT.\n")
	writeClaimopsFile(t, root, "auth.md", `---
type: Authentication
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces: {auth: https://example.test/auth/}
  entities: [{id: okn:service/auth}]
  predicates: [{id: auth:tokenFormat, object_kind: literal, datatype: xsd:string, maximum_count: 1}]
sources:
  - id: identity-openapi
    resource: https://example.test/openapi.yaml
    role: authoritative
    access: [profile:support]
claims:
  - id: okn:claim/token-format/pinned
    slot: okn:slot/token-format
    subject: okn:service/auth
    predicate: auth:tokenFormat
    object: {value: JWT, datatype: xsd:string}
    evidence:
      - id: okn:evidence/token-format/pinned
        source_ref: identity-openapi
        stance: supports
        role: primary
        selector: {type: text_quote, exact: Tokens use JWT.}
    status: supported
---

# Authentication
`)
	when := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	result, err := PinEvidence(context.Background(), EvidencePinOptions{
		Root: root, Spec: "0.2", Document: "auth.md", SourceID: "identity-openapi", Input: filepath.Join(root, "capture.txt"), CapturedAt: when,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.SHA256 != "8077d0c482419f60f265d9a5539c4b854601a7d9112b1d9105b7b1ad7648e49b" || result.Capture.CapturedAt != when.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected pin result: %#v", result)
	}
	artifact, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(result.Artifact)))
	if err != nil || string(artifact) != "Tokens use JWT.\n" {
		t.Fatalf("unexpected immutable artifact: %q %v", artifact, err)
	}
	receiptContent, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(result.Receipt)))
	var receipt EvidenceReceipt
	if err != nil || json.Unmarshal(receiptContent, &receipt) != nil || receipt.SHA256 != result.SHA256 || receipt.OriginalResource != filepath.Join(root, "capture.txt") || !reflect.DeepEqual(receipt.Access, []string{"profile:support"}) {
		t.Fatalf("unexpected receipt: %#v %v", receipt, err)
	}
	index := indexAfter(t, root)
	if len(index.Issues) != 0 {
		t.Fatalf("pinned selector did not validate: %#v", index.Issues)
	}
	document, _ := os.ReadFile(filepath.Join(root, "auth.md"))
	for _, expected := range []string{"observe: pinned", "sha256: " + result.SHA256, ".openknowledge/evidence/sha256/" + result.SHA256} {
		if !strings.Contains(string(document), expected) {
			t.Fatalf("document did not preserve pin %q:\n%s", expected, document)
		}
	}
	second, err := PinEvidence(context.Background(), EvidencePinOptions{
		Root: root, Spec: "0.2", Document: "auth.md", SourceID: "identity-openapi", CapturedAt: when.Add(time.Hour),
	})
	if err != nil || second.Changed || second.Receipt != result.Receipt || second.Capture.CapturedAt != result.Capture.CapturedAt {
		t.Fatalf("pin must be idempotent: %#v %v", second, err)
	}
	generation := filepath.Join(t.TempDir(), "generation")
	evidenceRoot := filepath.Join(generation, "evidence")
	materialized, err := MaterializeEvidenceStore(root, evidenceRoot)
	if err != nil || materialized.Files != 2 || materialized.Bytes == 0 {
		t.Fatalf("materialize evidence: %#v %v", materialized, err)
	}
	projection := filepath.Join(generation, "mcp")
	writeClaimopsFile(t, projection, "index.md", "# Index\n")
	writeClaimopsFile(t, projection, "auth.md", string(document))
	runtimeIndex, err := okf.BuildContextIndexWithVersionAndEvidence(projection, "0.2", evidenceRoot)
	if err != nil || len(runtimeIndex.Issues) != 0 {
		t.Fatalf("generation-private evidence did not verify projection: %#v %v", runtimeIndex.Issues, err)
	}
	receiptPath := filepath.Join(root, filepath.FromSlash(result.Receipt))
	if err := os.Chmod(receiptPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, []byte("{}\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := PinEvidence(context.Background(), EvidencePinOptions{Root: root, Spec: "0.2", Document: "auth.md", SourceID: "identity-openapi"}); err == nil || !strings.Contains(err.Error(), "receipt") {
		t.Fatalf("tampered receipt must fail closed: %v", err)
	}
	if _, err := MaterializeEvidenceStore(root, filepath.Join(t.TempDir(), "evidence")); err == nil || !strings.Contains(err.Error(), "receipt") {
		t.Fatalf("tampered evidence store must fail closed: %v", err)
	}
}

func TestVerificationAndLifecyclePreserveOccurrenceSemantics(t *testing.T) {
	root := t.TempDir()
	writeClaimopsFile(t, root, "index.md", "# Index\n")
	writeClaimopsFile(t, root, "openapi.yaml", "Tokens use JWT.\n")
	claim := authoredTypedClaim("okn:claim/token-format/2026-08-22", "JWT")
	writeClaimopsFile(t, root, "auth.md", typedClaimDocument(claim))
	base := indexAfter(t, root)
	when := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	changed, err := Verify(root, "0.2", claim.ID, "auth.md", "human:alice", when)
	if err != nil || !changed {
		t.Fatalf("verify: %t %v", changed, err)
	}
	verified := indexAfter(t, root)
	if verified.Occurrences[0].Claim.Verification == nil {
		t.Fatal("verification was not parsed")
	}
	if report := CompareLifecycle(base, verified); !report.Valid {
		t.Fatalf("valid transition rejected: %#v", report)
	}
	changed, err = Archive(root, "0.2", claim.ID, "auth.md", "human:alice")
	if err != nil || !changed {
		t.Fatalf("archive: %t %v", changed, err)
	}
	archived := indexAfter(t, root)
	if archived.Occurrences[0].Claim.Status != "archived" {
		t.Fatalf("unexpected status: %#v", archived.Occurrences[0].Claim)
	}
	if archived.Occurrences[0].Claim.Verification == nil || len(archived.Occurrences[0].Claim.Decisions) != 1 || archived.Occurrences[0].Claim.Decisions[0].Action != "archived" {
		t.Fatalf("archive must preserve verification and append a decision: %#v", archived.Occurrences[0].Claim)
	}
}

func TestEntityFindAndReadOnlyProposalsUseStableOntologyIDs(t *testing.T) {
	root := t.TempDir()
	writeClaimopsFile(t, root, "index.md", "# Index\n")
	writeClaimopsFile(t, root, "ontology.md", `---
type: Ontology
claim_ontology:
  entities:
    - id: okn:service/auth
      types: [okn:Service]
      pref_label: Authentication Service
      alt_labels: [Identity API]
    - id: okn:service/legacy-auth
      types: [okn:Service]
      pref_label: Legacy Authentication
  predicates:
    - id: okn:usesService
      object_kind: entity
---
# Ontology
`)
	writeClaimopsFile(t, root, "openapi.yaml", "Tokens use JWT.\n")
	writeClaimopsFile(t, root, "legacy.md", `---
type: Service
openknowledge_claim_profile: "1"
sources:
  - id: identity-openapi
    resource: openapi.yaml
    observe: pinned
    sha256: 8077d0c482419f60f265d9a5539c4b854601a7d9112b1d9105b7b1ad7648e49b
claims:
  - id: okn:claim/legacy-service/1
    slot: okn:slot/legacy-service
    subject: okn:service/auth
    predicate: okn:usesService
    object: {ref: okn:service/legacy-auth}
    evidence:
      - id: okn:evidence/legacy-service/1
        source_ref: identity-openapi
        stance: supports
        role: primary
        selector: {type: text_quote, exact: Tokens use JWT.}
    status: proposed
---
# Legacy service
`)
	index, err := BuildIndex(root, "0.2", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	matches := FindEntities(index, "Identity API")
	if len(matches) != 1 || matches[0].Entity.ID != "okn:service/auth" || matches[0].Score < 70 {
		t.Fatalf("unexpected entity matches: %#v", matches)
	}
	alias, err := NewEntityProposal(root, "0.2", "ontology.md", "okn:service/auth", "Login API", "", "Observed equivalent product name", 0.9)
	if err != nil || alias.Action != "add_alias" || alias.Alias != "Login API" || len(alias.DocumentSHA256) != 64 {
		t.Fatalf("unexpected alias proposal: %#v %v", alias, err)
	}
	aliasMutation, err := ApplyEntityProposal(root, "0.2", alias, "human:alice")
	if err != nil || !aliasMutation.Changed {
		t.Fatalf("apply alias: %#v %v", aliasMutation, err)
	}
	merge, err := NewEntityProposal(root, "0.2", "ontology.md", "okn:service/auth", "", "okn:service/legacy-auth", "Same deployed service", 0.8)
	if err != nil || merge.Action != "merge" || merge.MergeFrom != "okn:service/legacy-auth" || merge.MergeDocument != "ontology.md" || len(merge.MergeDocumentSHA256) != 64 {
		t.Fatalf("unexpected merge proposal: %#v %v", merge, err)
	}
	impact, err := BuildEntityImpact(indexAfter(t, root), merge)
	if err != nil || len(impact.References) != 1 || impact.References[0].Document != "legacy.md" || impact.References[0].Field != "object.ref" {
		t.Fatalf("unexpected merge impact: %#v %v", impact, err)
	}
	if _, err := ApplyEntityProposal(root, "0.2", merge, "agent:unsafe"); err == nil || !strings.Contains(err.Error(), "approved-by") {
		t.Fatalf("merge without accountable approval must fail: %v", err)
	}
	mutation, err := ApplyEntityProposal(root, "0.2", merge, "github:alice")
	if err != nil || !mutation.Changed || len(mutation.Impact.References) != 1 {
		t.Fatalf("apply merge: %#v %v", mutation, err)
	}
	mergedIndex := indexAfter(t, root)
	if len(mergedIndex.Issues) != 0 || mergedIndex.Ontology.Entities["okn:service/legacy-auth"].ReplacedBy != "okn:service/auth" || !mergedIndex.Ontology.Entities["okn:service/legacy-auth"].Deprecated || mergedIndex.Occurrences[0].Claim.Object.Ref != "okn:service/auth" {
		t.Fatalf("merge did not preserve history and rewrite references: %#v issues=%#v", mergedIndex.Ontology.Entities, mergedIndex.Issues)
	}
	encoded, err := EncodeEntityProposal(merge)
	if err != nil || !strings.Contains(string(encoded), `"type": "openknowledge.entity-proposal"`) {
		t.Fatalf("proposal did not encode: %s %v", encoded, err)
	}
}

func authoredTypedClaim(id, value string) AuthoredClaim {
	return AuthoredClaim{
		ID: id, Slot: "okn:slot/token-format", Subject: "okn:service/auth", Predicate: "auth:tokenFormat",
		Object: okf.ClaimObject{Value: value, Datatype: "xsd:string"}, Status: "proposed", SectionRef: "#token-format",
		Evidence: []okf.ClaimEvidence{{ID: "okn:evidence/token-format", SourceRef: "identity-openapi", Stance: "supports", Role: "primary", Selector: &okf.ClaimSelector{Type: "text_quote", Exact: "Tokens use JWT."}}},
	}
}

func typedClaimHeader() string {
	return `---
type: Authentication
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces:
    auth: https://example.test/auth/
  entities:
    - id: okn:service/auth
  predicates:
    - id: auth:tokenFormat
      object_kind: literal
      datatype: xsd:string
      maximum_count: 1
sources:
  - id: identity-openapi
    resource: openapi.yaml
    observe: pinned
    sha256: 8077d0c482419f60f265d9a5539c4b854601a7d9112b1d9105b7b1ad7648e49b
    role: authoritative
`
}

func typedClaimDocument(claim AuthoredClaim) string {
	return typedClaimHeader() + `claims:
  - id: ` + claim.ID + `
    slot: ` + claim.Slot + `
    subject: ` + claim.Subject + `
    predicate: ` + claim.Predicate + `
    object: {value: JWT, datatype: xsd:string}
    evidence:
      - id: okn:evidence/token-format
        source_ref: identity-openapi
        stance: supports
        role: primary
        selector: {type: text_quote, exact: Tokens use JWT.}
    status: proposed
    section_ref: "#token-format"
---

<a id="token-format"></a>

## Token format

Tokens use JWT.
`
}

func indexAfter(t *testing.T, root string) Index {
	t.Helper()
	index, err := BuildIndex(root, "0.2", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return index
}
func writeClaimopsFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
