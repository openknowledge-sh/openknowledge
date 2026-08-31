package eval

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCompareClassifiesImprovementsAndRegressions(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, "init")
	runGitTest(t, repo, "config", "core.autocrlf", "false")
	runGitTest(t, repo, "config", "user.email", "eval@example.test")
	runGitTest(t, repo, "config", "user.name", "Eval Test")
	wiki := filepath.Join(repo, "Wiki")
	writeTestFile(t, wiki, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n\nDeployment notes.\n")
	writeTestFile(t, wiki, "rollback.md", "---\ntype: Runbook\n---\n\n# Rollback\n\nContact support.\n")
	runGitTest(t, repo, "add", "Wiki")
	runGitTest(t, repo, "commit", "-m", "base")
	baseCommit := stringGitTest(t, repo, "rev-parse", "HEAD")

	loaded := LoadedDataset{
		Path: "/evals/deploy.yaml", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Dataset: Dataset{Type: DatasetType, Version: DatasetVersion, ID: "deploy", Cases: []Case{{
			ID: "rollback", Question: "How do we restore a deployment?", Agents: []string{"support-agent"},
			Expect: Expectations{Sources: []string{"rollback.md"}, EvidenceContains: []string{"restore the previous release"}},
		}}},
	}
	writeTestFile(t, wiki, "rollback.md", "---\ntype: Runbook\n---\n\n# Rollback\n\nRestore the previous release.\n")
	writeTestFile(t, wiki, "assets/uncovered.txt", "unrelated asset change\n")
	improved, err := Compare(wiki, "0.2", loaded, "HEAD", GateAll)
	if err != nil {
		t.Fatal(err)
	}
	if improved.Base.Commit != baseCommit || improved.Summary.Improved != 1 || improved.Summary.Status != "pass" || improved.Cases[0].Classification != "improved" {
		t.Fatalf("unexpected improvement report: %#v", improved)
	}
	if len(improved.Impact.AffectedQuestions) != 1 || len(improved.Impact.AffectedAgents) != 1 || improved.Impact.AffectedAgents[0] != "support-agent" ||
		len(improved.Impact.AffectedQuestions[0].Paths) != 1 || improved.Impact.AffectedQuestions[0].Paths[0] != "rollback.md" ||
		len(improved.Impact.UncoveredPaths) != 1 || improved.Impact.UncoveredPaths[0] != "assets/uncovered.txt" {
		t.Fatalf("unexpected improvement impact: %#v", improved.Impact)
	}

	runGitTest(t, repo, "add", "Wiki")
	runGitTest(t, repo, "commit", "-m", "good")
	writeTestFile(t, wiki, "rollback.md", "---\ntype: Runbook\n---\n\n# Rollback\n\nContact support.\n")
	regressed, err := Compare(wiki, "0.2", loaded, "HEAD", GateRegressions)
	if err != nil {
		t.Fatal(err)
	}
	if regressed.Summary.Regressed != 1 || regressed.Summary.Status != "fail" || regressed.Cases[0].Classification != "regressed" {
		t.Fatalf("unexpected regression report: %#v", regressed)
	}
}

func TestComparisonGateCanPermitUnchangedFailures(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, "init")
	runGitTest(t, repo, "config", "core.autocrlf", "false")
	runGitTest(t, repo, "config", "user.email", "eval@example.test")
	runGitTest(t, repo, "config", "user.name", "Eval Test")
	wiki := filepath.Join(repo, "Wiki")
	writeTestFile(t, wiki, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n\nNo rollback.\n")
	runGitTest(t, repo, "add", "Wiki")
	runGitTest(t, repo, "commit", "-m", "base")
	loaded := LoadedDataset{Path: "/eval.yaml", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Dataset: Dataset{
		Type: DatasetType, Version: DatasetVersion, ID: "test", Cases: []Case{{ID: "missing", Question: "rollback", Expect: Expectations{Sources: []string{"missing.md"}}}},
	}}
	regressionsOnly, err := Compare(wiki, "0.2", loaded, "HEAD", GateRegressions)
	if err != nil {
		t.Fatal(err)
	}
	if regressionsOnly.Summary.Status != "pass" || regressionsOnly.Summary.UnchangedFailed != 1 {
		t.Fatalf("unexpected regressions-only gate: %#v", regressionsOnly.Summary)
	}
	all, err := Compare(wiki, "0.2", loaded, "HEAD", GateAll)
	if err != nil {
		t.Fatal(err)
	}
	if all.Summary.Status != "fail" {
		t.Fatalf("all gate must reject proposed failures: %#v", all.Summary)
	}
}

func runGitTest(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func stringGitTest(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(bytesTrimSpace(output))
}

func bytesTrimSpace(value []byte) []byte {
	start := 0
	for start < len(value) && (value[start] == ' ' || value[start] == '\n' || value[start] == '\r' || value[start] == '\t') {
		start++
	}
	end := len(value)
	for end > start && (value[end-1] == ' ' || value[end-1] == '\n' || value[end-1] == '\r' || value[end-1] == '\t') {
		end--
	}
	return value[start:end]
}
