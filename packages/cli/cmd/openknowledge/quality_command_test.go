package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledgequality "github.com/openknowledge-sh/openknowledge/packages/cli/internal/quality"
)

func TestQualityReportCommandReturnsVersionedEvidenceStatus(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\ntitle: Quality\nokf_version: \"0.2\"\n---\n\n# Quality\n")
	writeMainTestFile(t, root, "runbook.md", "---\ntype: Runbook\ntitle: Recovery\nverified:\n  - by: human:owner\n    at: 2026-08-01T00:00:00Z\nstale_after: 2027-01-01\nsources:\n  - resource: https://example.test/recovery\n---\n\n# Recovery\n")
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runMain([]string{"--no-telemetry", "quality", "report", root, "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("quality report failed: code=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	var report knowledgequality.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode quality report: %v\n%s", err, stdout)
	}
	if report.Type != knowledgequality.ReportType || report.Version != 1 || len(report.Concepts) != 1 || report.Concepts[0].Path != "runbook.md" {
		t.Fatalf("unexpected quality report: %#v", report)
	}
	if report.Concepts[0].EvalCoverageStatus != "unavailable" || report.Concepts[0].Priority != "none" {
		t.Fatalf("missing eval inputs must stay unknown rather than uncovered: %#v", report.Concepts[0])
	}
	for _, metric := range report.Metrics {
		if metric.ID == "agent-used-current-trusted-eval-covered" && metric.Status != "unavailable" {
			t.Fatalf("empty observation window must not fabricate north-star value: %#v", metric)
		}
	}
	if !strings.Contains(helpText(), "quality") || cliErrorCommand([]string{"quality", "report"}) != "quality report" {
		t.Fatal("quality command is missing from the command catalog")
	}
}

func TestQualityReportConsumesCurrentEvalContract(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\ntitle: Quality\nokf_version: \"0.2\"\n---\n\n# Quality\n")
	writeMainTestFile(t, root, "runbook.md", "---\ntype: Runbook\ntitle: Recovery\nverified:\n  - by: process:eval\n    at: 2026-08-01T00:00:00Z\n---\n\n# Recovery\n\nRestore the previous release.\n")
	dataset := filepath.Join(t.TempDir(), "quality.yaml")
	writeMainTestFile(t, filepath.Dir(dataset), filepath.Base(dataset), "type: openknowledge.eval\nversion: 1\nid: quality\ncases: [{id: recovery, question: restore release, expect: {sources: [runbook.md]}}]\n")
	evalPath := filepath.Join(t.TempDir(), "eval.json")
	_, stderr, code := captureMainOutput(t, func() int {
		return runEval([]string{"run", dataset, root, "--json", "--out", evalPath})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("eval setup failed: code=%d stderr=%q", code, stderr)
	}
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runQuality([]string{"report", root, "--eval", evalPath, "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("quality report failed: code=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	var report knowledgequality.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Inputs.EvalReports != 1 || len(report.Concepts) != 1 || !report.Concepts[0].EvalCovered {
		t.Fatalf("eval coverage was not linked to the current concept: %#v", report)
	}
	if _, err := os.Stat(evalPath); err != nil {
		t.Fatal(err)
	}
}

func TestParseQualityReportOptionsAcceptsRepeatableInputs(t *testing.T) {
	options, err := parseQualityReportOptions([]string{"Wiki", "--usage", "one", "--usage=two", "--feedback", "feedback", "--eval=eval.json", "--audit", "audit.json", "--format", "markdown", "--out", "quality.md"})
	if err != nil || options.target != "Wiki" || len(options.usage) != 2 || len(options.feedback) != 1 || len(options.evals) != 1 || len(options.audits) != 1 || options.format != "markdown" {
		t.Fatalf("unexpected quality options: %#v err=%v", options, err)
	}
	for _, invalid := range [][]string{{"one", "two"}, {"--format", "html"}, {"--out", "quality.json"}, {"--unknown"}} {
		if _, err := parseQualityReportOptions(invalid); err == nil {
			t.Fatalf("expected invalid options: %v", invalid)
		}
	}
}
