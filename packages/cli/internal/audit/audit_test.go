package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/claimops"
	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

func TestRunReportsConcreteKnowledgeRisks(t *testing.T) {
	root := t.TempDir()
	writeAuditFile(t, root, "index.md", "---\ntype: Index\n---\n\n# Index\n")
	writeAuditFile(t, root, "evidence.txt", "revision one")
	writeAuditFile(t, root, "first.md", `---
type: Runbook
title: Rollback policy
owner: team:platform
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces: {deploy: https://example.test/deploy/}
  entities: [{id: deploy:service}]
  predicates: [{id: deploy:region, object_kind: literal, datatype: xsd:string, maximum_count: 1}]
stale_after: 2026-01-01T00:00:00Z
sources:
  - id: runbook
    resource: evidence.txt
claims:
  - id: deploy:claim/region/eu
    slot: deploy:slot/region
    subject: deploy:service
    predicate: deploy:region
    object: {value: eu-west-1, datatype: xsd:string}
    evidence: [{id: deploy:evidence/region/eu, source_ref: runbook, stance: supports, role: primary}]
    status: proposed
---

# Rollback policy

Restore the prior release. See [missing](missing.md).
`)
	writeAuditFile(t, root, "second.md", `---
type: Runbook
title: Rollback policy
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces: {deploy: https://example.test/deploy/}
  entities: [{id: deploy:service}]
  predicates: [{id: deploy:region, object_kind: literal, datatype: xsd:string, maximum_count: 1}]
sources:
  - id: missing
    resource: absent.txt
claims:
  - id: deploy:claim/region/us
    slot: deploy:slot/region
    subject: deploy:service
    predicate: deploy:region
    object: {value: us-east-1, datatype: xsd:string}
    evidence: [{id: deploy:evidence/region/us, source_ref: missing, stance: supports, role: primary}]
    status: proposed
---

# Rollback policy

Restore the prior release. See [missing](missing.md).
`)
	writeAuditFile(t, root, "orphan.md", "---\ntype: Guide\ntitle: Orphan\n---\n\n# Orphan\n\nNo source or owner.\n")

	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	events := []knowledgeusage.Event{
		usageEvent("1", "gap", nil, now),
		usageEvent("2", "gap", nil, now.Add(time.Minute)),
	}
	for index := 0; index < 5; index++ {
		events = append(events, usageEvent(string(rune('a'+index)), "used", []knowledgeusage.Evidence{{ID: "orphan#orphan", Locator: "okf+sha256://x", Path: "orphan.md"}}, now.Add(time.Duration(index+2)*time.Minute)))
	}

	report, baseline, err := Run(Options{Root: root, Spec: "0.2", Now: now, Usage: events})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"broken-dependency", "claim-conflict", "duplicate-title", "high-use-unverified", "identical-body", "missing-owner", "missing-source", "missing-source-resource", "stale", "unanswered-question"}
	for _, category := range want {
		if !hasCategory(report, category) {
			t.Fatalf("missing %s finding: %#v", category, report.Findings)
		}
	}
	if report.Summary.Total != len(report.Findings) || report.Summary.High == 0 || report.Summary.Medium == 0 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if report.Sources.Current != 2 || report.Sources.Missing != 1 || len(baseline.Sources) != 2 {
		t.Fatalf("unexpected source inventory: report=%#v baseline=%#v", report.Sources, baseline)
	}
	for index := 1; index < len(report.Findings); index++ {
		previous, current := report.Findings[index-1], report.Findings[index]
		if severityRank(previous.Severity) < severityRank(current.Severity) {
			t.Fatalf("findings are not severity sorted: %#v", report.Findings)
		}
	}
}

func TestRemoteObservationIsExplicitAndDetectsMetadataDrift(t *testing.T) {
	etag := `"one"`
	requests := 0
	client := &http.Client{Transport: auditRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		header := http.Header{}
		header.Set("ETag", etag)
		return &http.Response{
			StatusCode: http.StatusOK, Status: "200 OK", Header: header,
			Body: io.NopCloser(strings.NewReader("")), Request: request,
		}, nil
	})}
	root := t.TempDir()
	writeAuditFile(t, root, "index.md", "---\ntype: Index\n---\n\n# Index\n")
	writeAuditFile(t, root, "guide.md", "---\ntype: Guide\nowner: team:docs\nsources:\n  - id: remote\n    resource: https://example.test/guide\n    observe: metadata\n---\n\n# Guide\n")
	if _, _, err := Run(Options{Root: root, Spec: "latest"}); err != nil {
		t.Fatal(err)
	}
	if requests != 0 {
		t.Fatalf("remote source was contacted without opt-in: %d", requests)
	}
	_, baseline, err := Run(Options{Root: root, Spec: "latest", ObserveRemote: true, HTTPClient: client})
	if err != nil || requests != 1 {
		t.Fatalf("metadata observation failed: requests=%d err=%v", requests, err)
	}
	etag = `"two"`
	report, _, err := Run(Options{Root: root, Spec: "latest", ObserveRemote: true, HTTPClient: client, Baseline: &baseline})
	if err != nil || !hasCategory(report, "source-changed") {
		t.Fatalf("remote metadata drift was not detected: %#v err=%v", report, err)
	}
}

type auditRoundTripFunc func(*http.Request) (*http.Response, error)

func (function auditRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestRunDetectsTypedClaimConflictAndIncludesDependents(t *testing.T) {
	root := t.TempDir()
	writeAuditFile(t, root, "index.md", "---\ntype: Index\n---\n\n# Index\n")
	claimDocument := func(value string) string {
		return `---
type: API Reference
owner: team:identity
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces: {api: https://example.test/api/}
  entities: [{id: api:users}]
  predicates: [{id: api:path, object_kind: literal, datatype: xsd:string, maximum_count: 1}]
sources:
  - id: schema
    resource: https://example.test/openapi.yaml
    role: authoritative
claims:
  - id: api:claim/users/` + strings.Trim(strings.ReplaceAll(value, "/", "-"), "-") + `
    slot: api:slot/users-path
    subject: api:users
    predicate: api:path
    object: {value: ` + value + `, datatype: xsd:string}
    evidence: [{id: api:evidence/users/` + strings.Trim(strings.ReplaceAll(value, "/", "-"), "-") + `, source_ref: schema, stance: supports, role: primary}]
    status: supported
---

# Users

The endpoint is ` + value + `.
`
	}
	writeAuditFile(t, root, "first.md", claimDocument("/v1/users"))
	writeAuditFile(t, root, "second.md", claimDocument("/v2/users"))
	writeAuditFile(t, root, "runbook.md", `---
type: Runbook
owner: team:identity
sources:
  - id: runbook
    resource: https://example.test/runbook
claim_refs:
  - api:claim/users/v1-users
  - api:claim/users/v2-users
openknowledge_claim_profile: "1"
---

# Runbook

Call the users endpoint.
`)
	report, _, err := Run(Options{Root: root, Spec: "0.2", Now: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range report.Findings {
		if finding.Category != "claim-conflict" {
			continue
		}
		if !containsAuditString(finding.Targets, "runbook.md") {
			t.Fatalf("claim conflict did not include dependent runbook: %#v", finding)
		}
		return
	}
	t.Fatalf("missing typed claim conflict: %#v", report.Findings)
}

func TestRunDetectsSourceDriftAndKeepsFindingIDsStable(t *testing.T) {
	root := t.TempDir()
	writeAuditFile(t, root, "index.md", "---\ntype: Index\n---\n\n# Index\n")
	writeAuditFile(t, root, "source.txt", "one")
	writeAuditFile(t, root, "guide.md", "---\ntype: Guide\nowner: team:docs\nopenknowledge_claim_profile: \"1\"\nclaim_ontology:\n  namespaces: {docs: https://example.test/docs/}\n  entities: [{id: docs:source}]\n  predicates: [{id: docs:version, object_kind: literal, datatype: xsd:string, maximum_count: 1}]\nsources:\n  - id: source-file\n    resource: source.txt\nclaims:\n  - id: docs:claim/source-version/1\n    slot: docs:slot/source-version\n    subject: docs:source\n    predicate: docs:version\n    object: {value: one, datatype: xsd:string}\n    evidence: [{id: docs:evidence/source-version/1, source_ref: source-file, stance: supports, role: primary}]\n    status: supported\n---\n\n# Guide\n")
	writeAuditFile(t, root, "dependent.md", "---\ntype: Runbook\nowner: team:docs\nopenknowledge_claim_profile: \"1\"\nsources:\n  - resource: guide.md\nclaim_refs: [docs:claim/source-version/1]\n---\n\n# Dependent\n")
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	first, baseline, err := Run(Options{Root: root, Spec: "latest", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	writeAuditFile(t, root, "source.txt", "two")
	second, _, err := Run(Options{Root: root, Spec: "latest", Now: now.Add(time.Hour), Baseline: &baseline})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCategory(second, "source-changed") || second.Sources.Changed != 1 {
		t.Fatalf("expected source drift finding: %#v", second)
	}
	for _, finding := range second.Findings {
		if finding.Category == "source-changed" && !containsAuditString(finding.Targets, "dependent.md") {
			t.Fatalf("source drift did not include claim dependent: %#v", finding)
		}
	}
	withoutDrift := filterCategory(second.Findings, "source-changed")
	if !reflect.DeepEqual(findingIDs(first.Findings), findingIDs(withoutDrift)) {
		t.Fatalf("finding identities changed with evaluation time: first=%#v second=%#v", first.Findings, withoutDrift)
	}

	encoded, err := EncodeBaseline(baseline)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "baseline.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadBaseline(path)
	if err != nil || !reflect.DeepEqual(loaded, baseline) {
		t.Fatalf("baseline round trip failed: loaded=%#v err=%v", loaded, err)
	}
}

func TestRunClearsTypedSourceDriftOnlyAfterClaimReconciliation(t *testing.T) {
	root := t.TempDir()
	writeAuditFile(t, root, "index.md", "---\ntype: Index\n---\n\n# Index\n")
	writeAuditFile(t, root, "source.txt", "one\n")
	writeAuditFile(t, root, "claim.md", `---
type: Guide
owner: team:docs
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces: {docs: https://example.test/docs/}
  entities: [{id: docs:source}]
  predicates: [{id: docs:value, object_kind: literal, datatype: xsd:string, maximum_count: 1}]
sources: [{id: source-file, resource: source.txt, observe: manual, role: authoritative}]
claims:
  - id: docs:claim/source-value
    slot: docs:slot/source-value
    subject: docs:source
    predicate: docs:value
    object: {value: one, datatype: xsd:string}
    evidence: [{id: docs:evidence/source-value, source_ref: source-file, stance: supports, role: primary}]
    status: proposed
---

# Claim
`)
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	if changed, err := claimops.Verify(root, "0.2", "docs:claim/source-value", "claim.md", "human:alice", now); err != nil || !changed {
		t.Fatalf("verify claim: changed=%t err=%v", changed, err)
	}
	_, baseline, err := Run(Options{Root: root, Spec: "0.2", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	writeAuditFile(t, root, "source.txt", "two\n")
	drifted, _, err := Run(Options{Root: root, Spec: "0.2", Now: now.Add(time.Hour), Baseline: &baseline})
	if err != nil || !hasCategory(drifted, "source-changed") || !hasCategory(drifted, "claim-evidence-stale") {
		t.Fatalf("expected source and claim drift: %#v err=%v", drifted.Findings, err)
	}
	if _, err := claimops.RefreshClaimEvidenceVersions(root, "0.2", "docs:claim/source-value", "claim.md", "human:bob", now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	reconciled, _, err := Run(Options{Root: root, Spec: "0.2", Now: now.Add(3 * time.Hour), Baseline: &baseline})
	if err != nil || hasCategory(reconciled, "source-changed") || hasCategory(reconciled, "claim-evidence-stale") {
		t.Fatalf("reconciled claim kept drift findings: %#v err=%v", reconciled.Findings, err)
	}
}

func TestReadReportRejectsTamperedFindingIdentity(t *testing.T) {
	root := t.TempDir()
	writeAuditFile(t, root, "index.md", "---\ntype: Index\n---\n\n# Index\n")
	writeAuditFile(t, root, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n")
	report, _, err := Run(Options{Root: root, Spec: "0.2", Now: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)})
	if err != nil || len(report.Findings) == 0 {
		t.Fatalf("build report: %#v err=%v", report, err)
	}
	path := filepath.Join(t.TempDir(), "audit.json")
	write := func(value Report) {
		content, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(report)
	if loaded, err := ReadReport(path); err != nil || !reflect.DeepEqual(loaded, report) {
		t.Fatalf("read report: loaded=%#v err=%v", loaded, err)
	}
	report.Findings[0].Evidence[0].Value = "tampered"
	write(report)
	if _, err := ReadReport(path); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("expected tampered identity rejection, got %v", err)
	}
}

func usageEvent(id string, fingerprint string, selected []knowledgeusage.Evidence, at time.Time) knowledgeusage.Event {
	outcome := "no-evidence"
	if len(selected) > 0 {
		outcome = "evidence-selected"
	}
	return knowledgeusage.Event{
		Type: knowledgeusage.EventType, Version: knowledgeusage.EventVersion,
		ID: strings.Repeat(id, 32)[:32], At: at.Format(time.RFC3339Nano), KnowledgeBase: "wiki",
		Generation: knowledgeusage.Generation{Name: "generation", Commit: "abc", Spec: "0.2", ContentDigest: strings.Repeat("a", 64), Checks: []string{}},
		Channel:    "http-search", QueryFingerprint: strings.Repeat(fingerprint, 64)[:64], QueryLength: "1-32", Outcome: outcome,
		Selected: selected, Rejected: []knowledgeusage.Rejection{},
	}
}

func writeAuditFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasCategory(report Report, category string) bool {
	for _, finding := range report.Findings {
		if finding.Category == category {
			return true
		}
	}
	return false
}

func containsAuditString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func filterCategory(findings []Finding, excluded string) []Finding {
	var result []Finding
	for _, finding := range findings {
		if finding.Category != excluded {
			result = append(result, finding)
		}
	}
	return result
}

func findingIDs(findings []Finding) []string {
	result := make([]string, len(findings))
	for index, finding := range findings {
		result[index] = finding.ID
	}
	return result
}
