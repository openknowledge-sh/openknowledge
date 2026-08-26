package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledgeaudit "github.com/openknowledge-sh/openknowledge/packages/cli/internal/audit"
)

func TestRunAuditPrintsVersionedFindingsAndUsesSemanticExitStatus(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\ntype: Index\n---\n\n# Home\n")
	writeMainTestFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Guide\n---\n\n# Guide\n\nSee [missing](missing.md).\n")
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runMain([]string{"--no-telemetry", "audit", root, "--json", "--fail-on", "high"})
	})
	if code != 1 || stderr != "" {
		t.Fatalf("expected high finding failure, code=%d stderr=%q stdout=%s", code, stderr, stdout)
	}
	var report knowledgeaudit.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode audit report: %v\n%s", err, stdout)
	}
	if report.Type != knowledgeaudit.ReportType || report.Version != 1 || report.Summary.High < 1 || len(report.Findings) < 3 {
		t.Fatalf("unexpected audit report: %#v", report)
	}
	if report.Findings[0].Impact == "" || len(report.Findings[0].Evidence) == 0 || len(report.Findings[0].Targets) == 0 {
		t.Fatalf("finding is not evidence backed: %#v", report.Findings[0])
	}
}

func TestRunAuditWritesReportAndSourceBaselineAtomically(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\ntype: Index\n---\n\n# Home\n")
	writeMainTestFile(t, root, "guide.md", "---\ntype: Guide\nowner: team:docs\nsources:\n  - resource: https://example.test/guide\n    last_modified: 2026-08-01T00:00:00Z\n---\n\n# Guide\n")
	out := filepath.Join(t.TempDir(), "audit", "report.json")
	markdownOut := filepath.Join(filepath.Dir(out), "report.md")
	baseline := filepath.Join(t.TempDir(), "audit", "sources.json")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(baseline), 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runAudit([]string{root, "--format=json", "--out=" + out, "--markdown-out=" + markdownOut, "--baseline=" + baseline, "--update-baseline"})
	})
	if code != 0 || stderr != "" || stdout != "" {
		t.Fatalf("unexpected audit output, code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, path := range []string{out, baseline} {
		content, err := os.ReadFile(path)
		if err != nil || len(content) == 0 || content[len(content)-1] != '\n' {
			t.Fatalf("expected JSON file %s, err=%v content=%q", path, err, content)
		}
	}
	markdown, err := os.ReadFile(markdownOut)
	if err != nil || !strings.Contains(string(markdown), "# Open Knowledge audit") {
		t.Fatalf("expected Markdown from the same audit: err=%v content=%q", err, markdown)
	}
	loaded, err := knowledgeaudit.ReadBaseline(baseline)
	if err != nil || len(loaded.Sources) != 1 {
		t.Fatalf("unexpected baseline: %#v err=%v", loaded, err)
	}
}

func TestRunAuditRefusesToAdvanceBaselineAcrossUnreconciledSourceChange(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\ntype: Index\n---\n\n# Home\n")
	writeMainTestFile(t, root, "source.txt", "version one\n")
	writeMainTestFile(t, root, "guide.md", "---\ntype: Guide\nowner: team:docs\nsources:\n  - id: source\n    resource: source.txt\n---\n\n# Guide\n")
	baseline := filepath.Join(t.TempDir(), "sources.json")
	_, stderr, code := captureMainOutput(t, func() int {
		return runAudit([]string{root, "--baseline", baseline, "--update-baseline", "--format", "json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("create baseline: code=%d stderr=%q", code, stderr)
	}
	before, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	writeMainTestFile(t, root, "source.txt", "version two\n")
	_, stderr, code = captureMainOutput(t, func() int {
		return runAudit([]string{root, "--baseline", baseline, "--update-baseline", "--format", "json"})
	})
	if code != 1 || !strings.Contains(stderr, "cannot advance") {
		t.Fatalf("expected baseline guard, code=%d stderr=%q", code, stderr)
	}
	after, err := os.ReadFile(baseline)
	if err != nil || string(after) != string(before) {
		t.Fatalf("baseline changed across unresolved source drift: err=%v", err)
	}
}

func TestRunAuditWritesMarkdownAndProposesExactFinding(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "Wiki")
	writeMainTestFile(t, root, "index.md", "---\ntype: Index\n---\n\n# Home\n")
	writeMainTestFile(t, root, "guide.md", "---\ntype: Guide\nowner: team:docs\n---\n\n# Guide\n\nSee [missing](missing.md).\n")
	writeMainTestFile(t, repo, ".openknowledge/integration.toml", "version = 1\nknowledge_base = \"Wiki\"\ninsights = \"Wiki/insights\"\nruntime = \"codex\"\n")
	reportPath := filepath.Join(repo, ".openknowledge", "reports", "audit.json")
	markdownPath := filepath.Join(repo, ".openknowledge", "reports", "audit.md")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := captureMainOutput(t, func() int {
		return runAudit([]string{root, "--format", "json", "--out", reportPath})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("write audit report: code=%d stderr=%q", code, stderr)
	}
	_, stderr, code = captureMainOutput(t, func() int {
		return runAudit([]string{root, "--format", "markdown", "--out", markdownPath})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("write Markdown report: code=%d stderr=%q", code, stderr)
	}
	markdown, err := os.ReadFile(markdownPath)
	if err != nil || !strings.Contains(string(markdown), "# Open Knowledge audit") || !strings.Contains(string(markdown), "Evidence:") {
		t.Fatalf("unexpected Markdown report: err=%v\n%s", err, markdown)
	}
	report, err := knowledgeaudit.ReadReport(reportPath)
	if err != nil || len(report.Findings) == 0 {
		t.Fatalf("read audit report: %#v err=%v", report, err)
	}
	t.Chdir(repo)
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runAudit([]string{"propose", report.Findings[0].ID, "--report", reportPath, "--path", root})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"state": "created"`) {
		t.Fatalf("propose exact finding: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "insights")); err != nil || len(entries) != 1 {
		t.Fatalf("expected one durable insight proposal: entries=%v err=%v", entries, err)
	}
}

func TestParseAuditOptionsRejectsUnsafeCombinations(t *testing.T) {
	invalid := [][]string{
		{"a", "b"},
		{"--out", "report.json"},
		{"--update-baseline"},
		{"--fail-on", "critical"},
		{"--high-use-threshold", "0"},
		{"--format=json", "--out", "report.json", "--markdown-out", "report.json"},
		{"--unknown"},
	}
	for _, args := range invalid {
		if _, err := parseAuditOptions(args); err == nil {
			t.Fatalf("expected options to fail: %s", strings.Join(args, " "))
		}
	}
	options, err := parseAuditOptions([]string{"Wiki", "--usage", "one", "--usage=two", "--baseline", "sources.json", "--update-baseline", "--fail-on=medium"})
	if err != nil || options.target != "Wiki" || len(options.usage) != 2 || !options.updateBaseline || options.failOn != "medium" {
		t.Fatalf("unexpected options: %#v err=%v", options, err)
	}
}
