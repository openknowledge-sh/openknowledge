package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentsCommandValidatesListsAndDryRuns(t *testing.T) {
	root := newAgentTestRepo(t)
	jobPath := writeAgentJob(t, root, `---
id: docs-audit
enabled: true
schedule:
  cron: "0 9 * * MON"
  timezone: UTC
agent:
  command: go
  args: ["version"]
workspace:
  repo: "."
  base: HEAD
verify:
  commands:
    - go version
---

Audit docs.
`)

	output, stderr, code := captureMainOutput(t, func() int {
		return runAgents([]string{"validate", jobPath})
	})
	if code != 0 {
		t.Fatalf("expected agents validate to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	if !strings.Contains(output, "valid agent job: docs-audit") {
		t.Fatalf("expected validate output to include job id:\n%s", output)
	}

	output, stderr, code = captureMainOutput(t, func() int {
		return runAgents([]string{"list", filepath.Dir(jobPath)})
	})
	if code != 0 {
		t.Fatalf("expected agents list to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	if !strings.Contains(output, "docs-audit") || !strings.Contains(output, "cron=0 9 * * MON") {
		t.Fatalf("expected list output to include schedule:\n%s", output)
	}

	output, stderr, code = captureMainOutput(t, func() int {
		return runAgents([]string{"run", jobPath, "--dry-run", "--at", "2026-07-07T09:00:00Z"})
	})
	if code != 0 {
		t.Fatalf("expected agents run --dry-run to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	for _, expected := range []string{`"job_id": "docs-audit"`, `"branch": "agents/docs-audit/20260707-090000-`, `"command": "go"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected dry-run output to include %q:\n%s", expected, output)
		}
	}
}

func TestAgentsNewPrintsCatalogReferenceAndWritesTemplate(t *testing.T) {
	output, stderr, code := captureMainOutput(t, func() int {
		return runAgents([]string{"new"})
	})
	if code != 0 {
		t.Fatalf("expected agents new catalog to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	for _, expected := range []string{
		"Open Knowledge Agent Job Templates",
		"docs-audit",
		"wiki-health",
		"release-check",
		"custom",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected catalog to include %q:\n%s", expected, output)
		}
	}

	output, stderr, code = captureMainOutput(t, func() int {
		return runAgents([]string{"new", "--reference"})
	})
	if code != 0 {
		t.Fatalf("expected agents new --reference to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	for _, expected := range []string{
		"Open Knowledge Agent Job Frontmatter",
		"schedule.cron",
		"workspace.branch",
		"sandbox.type",
		"verify.commands",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected reference to include %q:\n%s", expected, output)
		}
	}

	root := t.TempDir()
	out := filepath.Join(root, ".openknowledge", "agents", "jobs", "docs-audit.md")
	output, stderr, code = captureMainOutput(t, func() int {
		return runAgents([]string{"new", "docs-audit", "--out", out})
	})
	if code != 0 {
		t.Fatalf("expected agents new --out to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "id: docs-audit") || !strings.Contains(output, "created agent job: "+out) {
		t.Fatalf("unexpected created template\noutput=%s\ncontent=%s", output, string(content))
	}

	_, stderr, code = captureMainOutput(t, func() int {
		return runAgents([]string{"new", "docs-audit", "--out", out})
	})
	if code != 1 || !strings.Contains(stderr, "use --force to overwrite") {
		t.Fatalf("expected overwrite guard, got code=%d stderr=%s", code, stderr)
	}
}

func TestAgentsSubcommandHelpDispatchesToSpecificCommand(t *testing.T) {
	tests := []struct {
		subcommand string
		expected   string
	}{
		{subcommand: "new", expected: "openknowledge agents new --reference"},
		{subcommand: "list", expected: "openknowledge agents list [path]"},
		{subcommand: "validate", expected: "openknowledge agents validate <job-or-dir>"},
		{subcommand: "run", expected: "openknowledge agents run <job.md> --at <time>"},
		{subcommand: "daemon", expected: "openknowledge agents daemon [jobs-dir] --tick <duration>"},
	}

	for _, test := range tests {
		t.Run(test.subcommand, func(t *testing.T) {
			output, stderr, code := captureMainOutput(t, func() int {
				return runAgents([]string{test.subcommand, "--help"})
			})
			if code != 0 {
				t.Fatalf("expected agents %s --help to succeed, got %d\nstdout=%s\nstderr=%s", test.subcommand, code, output, stderr)
			}
			if !strings.Contains(output, test.expected) {
				t.Fatalf("expected agents %s subcommand help to include %q:\n%s", test.subcommand, test.expected, output)
			}
			if strings.Contains(output, "Experimental command group for deterministic local agent jobs") {
				t.Fatalf("expected specific subcommand help, got group help:\n%s", output)
			}
		})
	}
}

func TestAgentsRunCreatesRunRecord(t *testing.T) {
	root := newAgentTestRepo(t)
	jobPath := writeAgentJob(t, root, `---
id: go-version
agent:
  command: go
  args:
    - version
workspace:
  repo: "."
  base: HEAD
---

Print the Go version.
`)
	runGit(t, root, "add", ".")
	runGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add agent job")

	output, stderr, code := captureMainOutput(t, func() int {
		return runAgents([]string{"run", jobPath, "--at", "2026-07-07T09:00:00Z"})
	})
	if code != 0 {
		t.Fatalf("expected agents run to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	if !strings.Contains(output, "agent run ") || !strings.Contains(output, "worktree: ") {
		t.Fatalf("unexpected run output:\n%s", output)
	}
	runLine := lineWithPrefix(output, "run: ")
	if runLine == "" {
		t.Fatalf("expected run record path in output:\n%s", output)
	}
	runRecordPath := strings.TrimSpace(strings.TrimPrefix(runLine, "run: "))
	content, err := os.ReadFile(runRecordPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"status": "succeeded"`) || !strings.Contains(string(content), `"job_id": "go-version"`) {
		t.Fatalf("unexpected run record:\n%s", string(content))
	}
}

func newAgentTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	writeMainTestFile(t, root, "README.md", "# Test\n")
	runGit(t, root, "add", "README.md")
	runGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	return root
}

func writeAgentJob(t *testing.T, root string, content string) string {
	t.Helper()
	rel := ".openknowledge/agents/jobs/docs.md"
	writeMainTestFile(t, root, rel, content)
	return filepath.Join(root, rel)
}

func lineWithPrefix(output string, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}
