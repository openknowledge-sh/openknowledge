package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
)

func TestRunEvalProducesVersionedJSONAndSemanticExitStatus(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n\nRead [Rollback](rollback.md).\n")
	writeMainTestFile(t, root, "rollback.md", "---\ntype: Runbook\ntitle: Rollback\n---\n\n# Rollback\n\nRestore the previous release.\n")
	dataset := filepath.Join(t.TempDir(), "deploy.yaml")
	writeMainTestFile(t, filepath.Dir(dataset), filepath.Base(dataset), `type: openknowledge.eval
version: 1
id: deploy
cases:
  - id: rollback
    question: How do we restore a release?
    expect:
      sources: [rollback.md]
      evidence_contains: [restore the previous release]
`)
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runMain([]string{"--no-telemetry", "eval", "run", dataset, root, "--format", "json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("expected passing eval, code=%d stderr=%s", code, stderr)
	}
	var report knowledgeeval.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode eval report: %v\n%s", err, stdout)
	}
	if report.SchemaVersion != "1" || report.Dataset.ID != "deploy" || report.Summary.Status != "pass" || report.Summary.Passed != 1 {
		t.Fatalf("unexpected eval report: %#v", report)
	}

	writeMainTestFile(t, filepath.Dir(dataset), filepath.Base(dataset), strings.ReplaceAll(
		`type: openknowledge.eval
version: 1
id: deploy
cases:
  - id: rollback
    question: How do we restore a release?
    expect: {sources: [missing.md]}
`, "\r", ""))
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runMain([]string{"--no-telemetry", "eval", "run", dataset, root, "--json"})
	})
	if code != 1 || stderr != "" {
		t.Fatalf("expected complete failing eval result, code=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || report.Summary.Status != "fail" {
		t.Fatalf("expected failing semantic report, err=%v report=%#v", err, report)
	}
}

func TestRunEvalWritesReportAtomically(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n\nKnowledge evaluation.\n")
	dataset := filepath.Join(t.TempDir(), "eval.yaml")
	writeMainTestFile(t, filepath.Dir(dataset), filepath.Base(dataset), "type: openknowledge.eval\nversion: 1\nid: smoke\ncases: [{id: home, question: Knowledge evaluation, expect: {min_sources: 1}}]\n")
	out := filepath.Join(t.TempDir(), "reports", "eval.json")
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runEval([]string{"run", dataset, root, "--json", "--out", out})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Wrote eval report") {
		t.Fatalf("unexpected file output: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var report knowledgeeval.Report
	if err := json.Unmarshal(content, &report); err != nil || report.Summary.Status != "pass" {
		t.Fatalf("unexpected persisted report: err=%v report=%#v", err, report)
	}
}

func TestRunEvalWritesMarkdownPRReport(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n\nKnowledge evaluation.\n")
	dataset := filepath.Join(t.TempDir(), "eval.yaml")
	writeMainTestFile(t, filepath.Dir(dataset), filepath.Base(dataset), "type: openknowledge.eval\nversion: 1\nid: smoke\ncases: [{id: home, question: Knowledge evaluation, expect: {min_sources: 1}}]\n")
	out := filepath.Join(t.TempDir(), "reports", "eval.md")
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runEval([]string{"run", dataset, root, "--format", "markdown", "--out", out})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Wrote eval report") {
		t.Fatalf("unexpected Markdown output: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	markdown := string(content)
	if !strings.Contains(markdown, "# Open Knowledge eval: smoke") || !strings.Contains(markdown, "All checks passed") {
		t.Fatalf("unexpected Markdown report:\n%s", markdown)
	}
}

func TestComparisonMarkdownShowsAnswerAndGroundednessChanges(t *testing.T) {
	baseGroundedness := 0.5
	proposedGroundedness := 1.0
	report := knowledgeeval.ComparisonReport{
		Dataset: knowledgeeval.DatasetIdentity{ID: "deploy"},
		Summary: knowledgeeval.ComparisonSummary{Status: "pass", Gate: knowledgeeval.GateRegressions, Total: 1, Improved: 1},
		Cases: []knowledgeeval.CaseComparison{{
			ID: "rollback", Question: "How do we roll back?", Classification: "improved",
			Base:     knowledgeeval.CaseResult{Status: "fail", Answer: &knowledgeeval.AnswerResult{Text: "Call support.", ClaimCount: 2, GroundedClaims: 1, Groundedness: baseGroundedness}},
			Proposed: knowledgeeval.CaseResult{Status: "pass", Answer: &knowledgeeval.AnswerResult{Text: "Restore the release.", ClaimCount: 1, GroundedClaims: 1, Groundedness: proposedGroundedness, CitationCount: 1, ValidCitations: 1, CitedSources: []string{"rollback.md"}}},
		}},
	}
	markdown := knowledgeeval.RenderComparisonMarkdown(report)
	for _, expected := range []string{"Answer change", "Call support.", "Restore the release.", "Groundedness: **50.0%**", "rollback.md"} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("comparison Markdown missing %q:\n%s", expected, markdown)
		}
	}
}

func TestRunEvalComparesGitBaseAndWorkingTree(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "eval@example.test")
	runGit(t, repo, "config", "user.name", "Eval Test")
	root := filepath.Join(repo, "Wiki")
	writeMainTestFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n\nDeployment notes.\n")
	writeMainTestFile(t, root, "rollback.md", "---\ntype: Runbook\n---\n\n# Rollback\n\nContact support.\n")
	runGit(t, repo, "add", "Wiki")
	runGit(t, repo, "commit", "-m", "base")
	writeMainTestFile(t, root, "rollback.md", "---\ntype: Runbook\n---\n\n# Rollback\n\nRestore the previous release.\n")
	dataset := filepath.Join(repo, "eval.yaml")
	writeMainTestFile(t, repo, "eval.yaml", "type: openknowledge.eval\nversion: 1\nid: deploy\ncases: [{id: rollback, question: restore deployment, expect: {evidence_contains: [restore the previous release]}}]\n")
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runEval([]string{"run", dataset, root, "--base", "HEAD", "--gate", "regressions", "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("expected improvement comparison, code=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	var report knowledgeeval.ComparisonReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode comparison: %v\n%s", err, stdout)
	}
	if report.Summary.Improved != 1 || report.Summary.Status != "pass" || report.Base.Ref != "HEAD" {
		t.Fatalf("unexpected comparison report: %#v", report)
	}
}

func TestEvalRejectsInvalidUsageAndDatasets(t *testing.T) {
	if _, err := parseEvalRunOptions(nil); err == nil {
		t.Fatal("expected missing dataset to fail")
	}
	if _, err := parseEvalRunOptions([]string{"eval.yaml", "Wiki", "extra"}); err == nil {
		t.Fatal("expected extra operand to fail")
	}
	if _, err := parseEvalRunOptions([]string{"eval.yaml", "--out", "report.json"}); err == nil {
		t.Fatal("expected text --out to fail")
	}
	if _, err := parseEvalRunOptions([]string{"eval.yaml", "--gate", "regressions"}); err == nil {
		t.Fatal("expected regression gate without base to fail")
	}
	if _, err := parseEvalRunOptions([]string{"eval.yaml", "--answer-arg", "model"}); err == nil {
		t.Fatal("expected answer argument without command to fail")
	}
	options, err := parseEvalRunOptions([]string{"eval.yaml", "--format", "markdown", "--answer-command", "runner", "--answer-arg", "model", "--answer-timeout", "30s"})
	if err != nil || options.answerCommand != "runner" || len(options.answerArgs) != 1 || options.answerTimeout.Seconds() != 30 {
		t.Fatalf("unexpected answer runner options: %#v err=%v", options, err)
	}
	invalid := filepath.Join(t.TempDir(), "invalid.yaml")
	writeMainTestFile(t, filepath.Dir(invalid), filepath.Base(invalid), "type: openknowledge.eval\nversion: 2\nid: invalid\ncases: []\n")
	_, stderr, code := captureMainOutput(t, func() int { return runEval([]string{"run", invalid}) })
	if code != 2 || !strings.Contains(stderr, "eval dataset is invalid") {
		t.Fatalf("expected dataset validation failure, code=%d stderr=%q", code, stderr)
	}
}

func TestEvalHelpDocumentsDatasetAndExitContract(t *testing.T) {
	for _, expected := range []string{"openknowledge eval run <dataset>", "type openknowledge.eval", "version 1"} {
		if !strings.Contains(evalHelpText(), expected) {
			t.Fatalf("eval help missing %q:\n%s", expected, evalHelpText())
		}
	}
	for _, expected := range []string{"--format", "--out", "--spec", "--answer-command", "Exit codes:"} {
		if !strings.Contains(evalRunHelpText(), expected) {
			t.Fatalf("eval run help missing %q:\n%s", expected, evalRunHelpText())
		}
	}
}
