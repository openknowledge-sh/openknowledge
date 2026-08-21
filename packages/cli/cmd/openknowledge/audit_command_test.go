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
	writeMainTestFile(t, root, "guide.md", "---\ntype: Guide\nowner: team:docs\nsources:\n  - resource: https://example.test/guide\n    last_modified: 2026-08-01\n---\n\n# Guide\n")
	out := filepath.Join(t.TempDir(), "audit", "report.json")
	baseline := filepath.Join(t.TempDir(), "audit", "sources.json")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(baseline), 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runAudit([]string{root, "--format=json", "--out=" + out, "--baseline=" + baseline, "--update-baseline"})
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
	loaded, err := knowledgeaudit.ReadBaseline(baseline)
	if err != nil || len(loaded.Sources) != 1 {
		t.Fatalf("unexpected baseline: %#v err=%v", loaded, err)
	}
}

func TestParseAuditOptionsRejectsUnsafeCombinations(t *testing.T) {
	invalid := [][]string{
		{"a", "b"},
		{"--format", "markdown"},
		{"--out", "report.json"},
		{"--update-baseline"},
		{"--fail-on", "critical"},
		{"--high-use-threshold", "0"},
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
