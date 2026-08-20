package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunEvaluatesExpectedSourcesAndEvidence(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n\nRead [Rollback](operations/rollback.md).\n")
	writeTestFile(t, root, "operations/rollback.md", "---\ntype: Runbook\ntitle: Rollback Guide\n---\n\n# Rollback\n\nRestore the previous release after a failed deployment.\n")
	loaded := LoadedDataset{
		Path: "/evals/deploy.yaml", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Dataset: Dataset{Type: DatasetType, Version: DatasetVersion, ID: "deploy", Cases: []Case{{
			ID: "rollback", Question: "How do we restore a failed deployment?",
			Expect: Expectations{Sources: []string{"operations/rollback.md"}, EvidenceContains: []string{"restore the previous release"}, EvidenceExcludes: []string{"delete production"}, MinSources: 1},
		}}},
	}
	report, err := Run(root, "0.2", loaded)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Status != "pass" || report.Summary.Passed != 1 || report.Summary.Checks != 4 {
		t.Fatalf("unexpected passing report: %#v", report)
	}
	if report.Cases[0].Metrics.SourceRecall != 1 || len(report.Cases[0].Context.Sources) == 0 {
		t.Fatalf("unexpected case metrics: %#v", report.Cases[0])
	}
}

func TestRunReturnsCompleteFailingReport(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n\nDeployment overview.\n")
	loaded := LoadedDataset{
		Path: "/evals/deploy.yaml", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Dataset: Dataset{Type: DatasetType, Version: DatasetVersion, ID: "deploy", Cases: []Case{{
			ID: "rollback", Question: "How do we restore a failed deployment?",
			Expect: Expectations{Sources: []string{"missing.md"}, EvidenceContains: []string{"restore the previous release"}},
		}}},
	}
	report, err := Run(root, "0.2", loaded)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Status != "fail" || report.Summary.Failed != 1 || report.Cases[0].Status != "fail" {
		t.Fatalf("unexpected failing report: %#v", report)
	}
	if report.Cases[0].Metrics.SourceRecall != 0 || report.Cases[0].Metrics.PassedChecks != 0 {
		t.Fatalf("unexpected failing metrics: %#v", report.Cases[0].Metrics)
	}
}

func writeTestFile(t *testing.T, root string, relative string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
