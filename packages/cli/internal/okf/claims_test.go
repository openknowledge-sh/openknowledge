package okf

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaimEvidenceVersionDerivesStaleFromSeparateLocalLiveSource(t *testing.T) {
	root := t.TempDir()
	live := "The current token format is JWT.\n"
	artifact := "Pinned JWT evidence.\n"
	liveDigest := sha256.Sum256([]byte(live))
	artifactDigest := sha256.Sum256([]byte(artifact))
	liveResource := filepath.Join(root, "live.txt")
	writeFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeFile(t, root, "live.txt", live)
	writeFile(t, root, "artifact.txt", artifact)
	writeFile(t, root, "claim.md", `---
type: Authentication
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces: {auth: https://example.test/auth/}
  entities: [{id: auth:service}]
  predicates: [{id: auth:format, object_kind: literal, datatype: xsd:string, maximum_count: 1}]
sources:
  - id: contract
    resource: artifact.txt
    live_resource: `+liveResource+`
    observe: pinned
    sha256: `+hex.EncodeToString(artifactDigest[:])+`
claims:
  - id: auth:claim/format
    slot: auth:slot/format
    subject: auth:service
    predicate: auth:format
    object: {value: JWT, datatype: xsd:string}
    evidence: [{id: auth:evidence/format, source_ref: contract}]
    status: verified
    verification:
      method: claim-review
      by: human:alice
      at: 2026-08-22T00:00:00Z
      evidence_refs: [auth:evidence/format]
      evidence_versions:
        - evidence_ref: auth:evidence/format
          source_ref: contract
          resource: `+liveResource+`
          sha256: `+hex.EncodeToString(liveDigest[:])+`
          by: human:alice
          at: 2026-08-22T00:00:00Z
---

# Claim
`)
	bundle, err := ParseASTWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	profile := AnalyzeClaimProfile(bundle, time.Date(2026, 8, 22, 1, 0, 0, 0, time.UTC))
	if len(profile.Issues) != 0 || len(profile.Claims) != 1 || profile.Claims[0].Stale {
		t.Fatalf("fresh observed claim failed: claims=%#v issues=%#v", profile.Claims, profile.Issues)
	}
	writeFile(t, root, "live.txt", "The current token format is opaque.\n")
	bundle, err = ParseASTWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	profile = AnalyzeClaimProfile(bundle, time.Date(2026, 8, 22, 1, 0, 0, 0, time.UTC))
	if len(profile.Claims) != 1 || !profile.Claims[0].Stale || len(profile.Claims[0].StaleEvidence) != 1 || profile.Claims[0].StaleEvidence[0] != "auth:evidence/format" {
		t.Fatalf("live evidence drift did not mark the claim stale: %#v", profile.Claims)
	}
}

func TestClaimProfileValidatesTypedClaimAndSectionBinding(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeFile(t, root, "token-evidence.txt", "Production tokens use the declared format.")
	writeFile(t, root, "auth.md", validTypedClaimDocument("okn:claim/token-format/2026-08-22", "JWT", "verified"))
	bundle, err := ParseASTWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	profile := AnalyzeClaimProfile(bundle, time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	if len(profile.Issues) != 0 {
		t.Fatalf("unexpected issues: %#v", profile.Issues)
	}
	if len(profile.Claims) != 1 || profile.Claims[0].Slot != "okn:slot/token-format" || profile.Claims[0].Object.Datatype != "xsd:string" {
		t.Fatalf("unexpected claim: %#v", profile.Claims)
	}
	validation, _ := ValidateASTWithOptions(bundle, ValidationOptions{})
	index := ContextIndexFromAST(validation, bundle)
	found := false
	for _, section := range index.Sections {
		projected := ClaimProfileForSection(section)
		if projected != nil && len(projected.Claims) == 1 {
			found = section.Heading == "Token format"
		}
	}
	if !found {
		t.Fatal("expected section-bound typed claim")
	}
}

func TestClaimProfileRejectsLegacyShapeAndDuplicateOccurrenceID(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Index\n")
	writeFile(t, root, "token-evidence.txt", "Production tokens use the declared format.")
	writeFile(t, root, "legacy.md", "---\ntype: Guide\nopenknowledge_claim_profile: \"1\"\nclaims:\n  - id: auth.token-format\n    value: JWT\n---\n# Legacy\n")
	writeFile(t, root, "one.md", validTypedClaimDocument("okn:claim/duplicate", "JWT", "proposed"))
	writeFile(t, root, "two.md", validTypedClaimDocument("okn:claim/duplicate", "opaque", "proposed"))
	result, err := ValidateWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, issue := range issuesWithRule(result.Errors, ClaimValidationRule) {
		joined += issue.Message + "\n"
	}
	if !strings.Contains(joined, `unknown field "value"`) || !strings.Contains(joined, "not globally unique") {
		t.Fatalf("missing strict v1 errors: %s", joined)
	}
}

func TestValidationProfilesSeparateOKFCoreFromBundleExtensions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeFile(t, root, "legacy.md", "---\ntype: Guide\nopenknowledge_claim_profile: \"1\"\nclaims:\n  - id: legacy.claim\n    value: JWT\n---\n# Legacy\n")
	bundle, err := ParseASTWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}

	bundleResult, err := ValidateASTWithOptions(bundle, ValidationOptions{Profile: ValidationProfileBundle})
	if err != nil {
		t.Fatal(err)
	}
	if bundleResult.Profile != ValidationProfileBundle || !hasIssueRule(bundleResult.Errors, ClaimValidationRule) {
		t.Fatalf("bundle profile must report the invalid claim extension: %#v", bundleResult)
	}
	if !hasCheckGroup(bundleResult.Checks, "Open Knowledge extensions") {
		t.Fatalf("bundle profile must include extension checks: %#v", bundleResult.Checks)
	}

	coreResult, err := ValidateASTWithOptions(bundle, ValidationOptions{Profile: ValidationProfileOKF})
	if err != nil {
		t.Fatal(err)
	}
	if coreResult.Profile != ValidationProfileOKF || hasIssueRule(coreResult.Errors, ClaimValidationRule) || hasCheckGroup(coreResult.Checks, "Open Knowledge extensions") {
		t.Fatalf("okf profile must contain only OKF core results: %#v", coreResult)
	}
	if !hasCheckGroup(coreResult.Checks, "OKF core") {
		t.Fatalf("okf profile must label core checks: %#v", coreResult.Checks)
	}
}

func hasCheckGroup(checks []Check, group string) bool {
	for _, check := range checks {
		if check.Group == group {
			return true
		}
	}
	return false
}

func TestClaimComparisonUsesSlotSPOAndTypedObject(t *testing.T) {
	number, _ := NormalizeClaimObject(ClaimObject{Value: 60, Datatype: "xsd:integer"})
	text, _ := NormalizeClaimObject(ClaimObject{Value: "60", Datatype: "xsd:string"})
	if number == text {
		t.Fatal("typed values must remain distinct")
	}
	left := Claim{ID: "okn:claim/1", Slot: "okn:slot/timeout", Subject: "okn:service/api", Predicate: "okn:timeout", Scope: map[string]ClaimObject{"okn:environment": {Value: "production", Datatype: "xsd:string"}}}
	right := left
	right.ID = "okn:claim/2"
	if ClaimComparisonKey(left) != ClaimComparisonKey(right) {
		t.Fatal("occurrence ID must not change comparison slot")
	}
	right.Scope = map[string]ClaimObject{"okn:environment": {Value: "staging", Datatype: "xsd:string"}}
	if ClaimComparisonKey(left) == ClaimComparisonKey(right) {
		t.Fatal("scope must change comparison slot")
	}
	a := Claim{Status: "verified", ValidTime: ClaimTimeInterval{From: "2026-01-01", Until: "2026-02-01"}}
	b := Claim{Status: "verified", ValidTime: ClaimTimeInterval{From: "2026-02-01", Until: "2026-03-01"}}
	if ClaimValidityOverlaps(a, b) {
		t.Fatal("half-open adjacent intervals must not overlap")
	}
}

func validTypedClaimDocument(id, value, status string) string {
	return `---
type: Authentication
title: Authentication
owner: team:identity
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces:
    auth: https://example.test/auth/
  entities:
    - id: okn:service/auth
      types: [okn:Service]
  predicates:
    - id: auth:tokenFormat
      object_kind: literal
      datatype: xsd:string
      maximum_count: 1
sources:
  - id: identity-openapi
    resource: token-evidence.txt
    observe: pinned
    sha256: bb5a64e1c45b93136f128d1a3cf3d791d138709763ee26c2653ad4065f36c384
    role: authoritative
claims:
  - id: ` + id + `
    slot: okn:slot/token-format
    subject: okn:service/auth
    predicate: auth:tokenFormat
    object:
      value: ` + value + `
      datatype: xsd:string
    evidence:
      - id: okn:evidence/token-format
        source_ref: identity-openapi
        stance: supports
        role: primary
        selector:
          type: text_quote
          exact: Production tokens use the declared format.
    status: ` + status + `
    section_ref: "#claim-token-format"
` + func() string {
		if status == "verified" {
			return "    verification:\n      method: human-review\n      by: human:alice\n      at: 2026-08-22T00:00:00Z\n      evidence_refs: [okn:evidence/token-format]\n"
		}
		return ""
	}() + `---

# Authentication

<a id="claim-token-format"></a>

## Token format

Production tokens use the declared format.
`
}

func issuesWithRule(issues []Issue, rule string) []Issue {
	var result []Issue
	for _, issue := range issues {
		if issue.Rule == rule {
			result = append(result, issue)
		}
	}
	return result
}
