package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
)

func TestEvalClaimReplayUsesImmutableGitCheckpointsAndExplicitGates(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "Wiki")
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "claims@example.test")
	runGit(t, repo, "config", "user.name", "Claim Replay")
	writeMainTestFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeMainTestFile(t, root, "auth.md", mainTypedClaimDocument("okn:claim/token-format/1", "verified", "JWT"))
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "supported claim")
	writeMainTestFile(t, root, "auth.md", mainTypedClaimDocument("okn:claim/token-format/1", "verified", "opaque"))
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "stale claim")

	dataset := filepath.Join(repo, "claim-replay.yaml")
	writeMainTestFile(t, repo, "claim-replay.yaml", `type: openknowledge.claim-replay-eval
version: 1
id: auth-history
checkpoints:
  - id: supported
    revision: HEAD~1
    expectations:
      - claim_id: okn:claim/token-format/1
        state: supported
  - id: stale
    revision: HEAD
    expectations:
      - claim_id: okn:claim/token-format/1
        state: refuted
`)
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runEval([]string{"claims", dataset, root, "--json", "--max-stale", "1"})
	})
	var report knowledgeeval.ClaimReplayReport
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &report) != nil {
		t.Fatalf("claim replay failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if report.Summary.Supported != 1 || report.Summary.Stale != 1 || report.Summary.Hallucinated != 0 {
		t.Fatalf("unexpected claim replay summary: %#v", report.Summary)
	}
	if len(report.Checkpoints) != 2 || report.Checkpoints[1].Claims[0].LastSupportedCheckpoint != "supported" {
		t.Fatalf("claim history was not preserved: %#v", report.Checkpoints)
	}

	_, _, code = captureMainOutput(t, func() int {
		return runEval([]string{"claims", dataset, root, "--format", "markdown"})
	})
	if code != 1 {
		t.Fatalf("default stale gate must fail, code=%d", code)
	}
}

func TestParseEvalClaimReplayOptionsRejectsUnsafeValues(t *testing.T) {
	for _, args := range [][]string{{}, {"dataset.yaml", "--max-stale", "-1"}, {"dataset.yaml", "--max-unverified", "-1"}, {"dataset.yaml", "--format", "xml"}, {"dataset.yaml", "--out", "report.json"}} {
		if _, err := parseEvalClaimReplayOptions(args); err == nil {
			t.Fatalf("expected invalid options: %s", strings.Join(args, " "))
		}
	}
}
