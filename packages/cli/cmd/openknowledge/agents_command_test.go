package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
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
concurrency:
  key: wiki-maintenance
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
	for _, expected := range []string{`"schemaVersion": "1"`, `"job_id": "docs-audit"`, `"branch": "agents/docs-audit/20260707-090000-`, `"command": "go"`, `"key": "wiki-maintenance"`, `"policy": "skip"`} {
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
		"concurrency.policy",
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

func TestAgentsExecutorOverrideRejectsUnknownValuesBeforeExecution(t *testing.T) {
	tests := [][]string{
		{"run", filepath.Join(t.TempDir(), "missing-job.md"), "--executor", "doker"},
		{"run", filepath.Join(t.TempDir(), "missing-job.md"), "--executor=doker"},
		{"daemon", filepath.Join(t.TempDir(), "missing-jobs"), "--once", "--executor", "doker"},
		{"daemon", filepath.Join(t.TempDir(), "missing-jobs"), "--once", "--executor=doker"},
	}
	for _, args := range tests {
		_, stderr, code := captureMainOutput(t, func() int {
			return runAgents(args)
		})
		if code != 2 || !strings.Contains(stderr, "--executor must be host or docker") {
			t.Fatalf("expected fail-closed executor usage error for %v, code=%d stderr=%s", args, code, stderr)
		}
		if strings.Contains(stderr, "no such file") {
			t.Fatalf("executor validation must happen before job discovery for %v: %s", args, stderr)
		}
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
	if !strings.Contains(string(content), `"schemaVersion": "1"`) || !strings.Contains(string(content), `"status": "succeeded"`) || !strings.Contains(string(content), `"job_id": "go-version"`) {
		t.Fatalf("unexpected run record:\n%s", string(content))
	}
	if runtime.GOOS != "windows" {
		runDir := filepath.Dir(runRecordPath)
		for _, dir := range []string{filepath.Dir(runDir), runDir} {
			info, err := os.Stat(dir)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0700 {
				t.Fatalf("expected private run directory %s mode 0700, got %04o", dir, info.Mode().Perm())
			}
		}
		for _, name := range []string{"home", "tmp"} {
			dir := filepath.Join(runDir, name)
			info, err := os.Stat(dir)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0700 {
				t.Fatalf("expected isolated runtime directory %s mode 0700, got %04o", dir, info.Mode().Perm())
			}
		}
		for _, name := range []string{
			"job.md", "prompt.md", "plan.json", "run.json",
			"agent.stdout.log", "agent.stderr.log", "diff.patch",
		} {
			path := filepath.Join(runDir, name)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0600 {
				t.Fatalf("expected private artifact %s mode 0600, got %04o", path, info.Mode().Perm())
			}
		}
	}
}

func TestAgentsSequentialRunsKeepSourceRepositoryClean(t *testing.T) {
	root := newAgentTestRepo(t)
	jobPath := writeAgentJob(t, root, `---
id: clean-runs
agent:
  command: go
  args: [version]
workspace:
  repo: "."
  base: HEAD
---

Check the Go version.
`)
	runGit(t, root, "add", ".")
	runGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add clean job")

	for _, scheduledAt := range []string{"2026-07-07T09:00:00Z", "2026-07-08T09:00:00Z"} {
		output, stderr, code := captureMainOutput(t, func() int {
			return runAgents([]string{"run", jobPath, "--at", scheduledAt})
		})
		if code != 0 {
			t.Fatalf("expected sequential run at %s to succeed, code=%d stdout=%s stderr=%s", scheduledAt, code, output, stderr)
		}
		if status := agentGitOutput(t, root, "status", "--porcelain"); strings.TrimSpace(status) != "" {
			t.Fatalf("agent runtime must not dirty the source repository after %s: %s", scheduledAt, status)
		}
		runPath := strings.TrimSpace(strings.TrimPrefix(lineWithPrefix(output, "run: "), "run: "))
		if runPath == "" || strings.HasPrefix(runPath, root+string(filepath.Separator)) {
			t.Fatalf("expected external run path, got %q", runPath)
		}
	}
}

func TestAgentsRejectStateDirectoryInsideSourceRepository(t *testing.T) {
	root := newAgentTestRepo(t)
	jobPath := writeAgentJob(t, root, `---
id: unsafe-state
agent:
  command: go
  args: [version]
workspace:
  repo: "."
---

Plan safely.
`)
	t.Setenv(agents.AgentsStateDirEnv, filepath.Join(root, ".agent-runtime"))
	_, stderr, code := captureMainOutput(t, func() int {
		return runAgents([]string{"run", jobPath, "--dry-run", "--at", "2026-07-07T09:00:00Z"})
	})
	if code != 1 || !strings.Contains(stderr, "agent state directory must be outside the Git repository") {
		t.Fatalf("expected in-repository state refusal, code=%d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".agent-runtime")); !os.IsNotExist(err) {
		t.Fatalf("refused state directory must not be created, got %v", err)
	}
}

func TestAgentsVerificationTimeoutFailsRunPromptly(t *testing.T) {
	root := newAgentTestRepo(t)
	jobPath := writeAgentJob(t, root, `---
id: verify-timeout
agent:
  command: go
  args: [version]
verify:
  commands: ["sleep 5"]
  timeout: 50ms
workspace:
  repo: "."
---

Run bounded verification.
`)
	runGit(t, root, "add", ".")
	runGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add timeout job")

	started := time.Now()
	_, stderr, code := captureMainOutput(t, func() int {
		return runAgents([]string{"run", jobPath, "--at", "2026-07-07T09:00:00Z"})
	})
	if code != 1 || !strings.Contains(stderr, `verification command "sleep 5" timed out after 50ms`) {
		t.Fatalf("expected verification timeout, code=%d stderr=%s", code, stderr)
	}
	if elapsed := time.Since(started); elapsed >= 3*time.Second {
		t.Fatalf("verification timeout did not stop promptly: %s", elapsed)
	}
}

func newAgentTestRepo(t *testing.T) string {
	t.Helper()
	t.Setenv(agents.AgentsStateDirEnv, t.TempDir())
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

func agentGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
