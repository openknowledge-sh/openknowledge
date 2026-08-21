package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
stale_after: 2026-01-01
sources:
  - id: runbook
    resource: evidence.txt
claims:
  - id: deploy.region
    value: eu-west-1
---

# Rollback policy

Restore the prior release. See [missing](missing.md).
`)
	writeAuditFile(t, root, "second.md", `---
type: Runbook
title: Rollback policy
sources:
  - id: missing
    resource: absent.txt
claims:
  - id: deploy.region
    value: us-east-1
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

func TestRunDetectsSourceDriftAndKeepsFindingIDsStable(t *testing.T) {
	root := t.TempDir()
	writeAuditFile(t, root, "index.md", "---\ntype: Index\n---\n\n# Index\n")
	writeAuditFile(t, root, "source.txt", "one")
	writeAuditFile(t, root, "guide.md", "---\ntype: Guide\nowner: team:docs\nsources:\n  - resource: source.txt\n---\n\n# Guide\n")
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
