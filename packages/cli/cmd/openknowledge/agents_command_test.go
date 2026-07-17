package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestJobsCommandValidatesListsAndDryRuns(t *testing.T) {
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
	writeMainTestFile(t, root, ".openknowledge/jobs/alpha.md", `---
id: alpha-check
enabled: false
agent: {command: codex}
concurrency: {key: wiki-maintenance}
---
Check first.
`)

	output, stderr, code := captureMainOutput(t, func() int {
		return runJobs([]string{"validate", jobPath})
	})
	if code != 0 {
		t.Fatalf("expected jobs validate to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	if !strings.Contains(output, "valid job: docs-audit") {
		t.Fatalf("expected validate output to include job id:\n%s", output)
	}
	output, stderr, code = captureMainOutput(t, func() int {
		return runJobs([]string{"validate", jobPath, "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("expected JSON agent validation, code=%d stderr=%s", code, stderr)
	}
	var validation agentValidationOutput
	if err := json.Unmarshal([]byte(output), &validation); err != nil || !validation.Valid || len(validation.Jobs) != 1 || validation.Jobs[0].ID != "docs-audit" || validation.Issues == nil {
		t.Fatalf("unexpected valid agent report: %#v err=%v", validation, err)
	}
	invalidPath := filepath.Join(root, ".openknowledge", "invalid.md")
	writeMainTestFile(t, root, ".openknowledge/invalid.md", "---\nid: invalid\nagent: {command: codex, argz: []}\n---\nPrompt.\n")
	output, stderr, code = captureMainOutput(t, func() int {
		return runJobs([]string{"validate", "--json", invalidPath})
	})
	if code != 1 || stderr != "" {
		t.Fatalf("expected structured invalid report, code=%d stderr=%s", code, stderr)
	}
	if err := json.Unmarshal([]byte(output), &validation); err != nil || validation.Valid || validation.Jobs == nil || len(validation.Issues) != 1 || validation.Issues[0].Field != "agent.argz" {
		t.Fatalf("unexpected invalid agent report: %#v err=%v", validation, err)
	}

	output, stderr, code = captureMainOutput(t, func() int {
		return runJobs([]string{"list", filepath.Dir(jobPath)})
	})
	if code != 0 {
		t.Fatalf("expected jobs list to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	if !strings.Contains(output, "docs-audit") || !strings.Contains(output, "cron=0 9 * * MON") {
		t.Fatalf("expected list output to include schedule:\n%s", output)
	}

	output, stderr, code = captureMainOutput(t, func() int {
		return runJobs([]string{"list", "--json", filepath.Dir(jobPath)})
	})
	if code != 0 {
		t.Fatalf("expected jobs list --json to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	var inventory agentListOutput
	if err := json.Unmarshal([]byte(output), &inventory); err != nil {
		t.Fatal(err)
	}
	if inventory.SchemaVersion != okf.MachineSchemaVersion || len(inventory.Jobs) != 2 || inventory.Jobs[0].ID != "alpha-check" || inventory.Jobs[1].ID != "docs-audit" {
		t.Fatalf("unexpected sorted agent inventory: %#v", inventory)
	}
	if inventory.Jobs[0].Concurrency.Policy != "skip" || inventory.Jobs[1].Schedule.Cron != "0 9 * * MON" || inventory.Jobs[1].Agent != "go" {
		t.Fatalf("agent inventory omitted structured discovery fields: %#v", inventory.Jobs)
	}

	output, stderr, code = captureMainOutput(t, func() int {
		return runJobs([]string{"list", filepath.Join(root, "missing"), "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("expected missing JSON inventory to succeed, code=%d stderr=%s", code, stderr)
	}
	if err := json.Unmarshal([]byte(output), &inventory); err != nil || inventory.Jobs == nil || len(inventory.Jobs) != 0 {
		t.Fatalf("expected explicit empty jobs array, inventory=%#v err=%v", inventory, err)
	}

	output, stderr, code = captureMainOutput(t, func() int {
		return runJobs([]string{"run", jobPath, "--dry-run", "--at", "2026-07-07T09:00:00Z"})
	})
	if code != 0 {
		t.Fatalf("expected jobs run --dry-run to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	for _, expected := range []string{`"schemaVersion": "1"`, `"job_id": "docs-audit"`, `"branch": "jobs/docs-audit/20260707-090000-`, `"command": "go"`, `"key": "wiki-maintenance"`, `"policy": "skip"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected dry-run output to include %q:\n%s", expected, output)
		}
	}
}

func TestJobsNewPrintsCatalogReferenceAndWritesTemplate(t *testing.T) {
	output, stderr, code := captureMainOutput(t, func() int {
		return runJobs([]string{"new"})
	})
	if code != 0 {
		t.Fatalf("expected jobs new catalog to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	for _, expected := range []string{
		"Open Knowledge Job Templates",
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
		return runJobs([]string{"new", "--reference"})
	})
	if code != 0 {
		t.Fatalf("expected jobs new --reference to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	for _, expected := range []string{
		"Open Knowledge Job Frontmatter",
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
	out := filepath.Join(root, ".openknowledge", "jobs", "jobs", "docs-audit.md")
	output, stderr, code = captureMainOutput(t, func() int {
		return runJobs([]string{"new", "docs-audit", "--out", out})
	})
	if code != 0 {
		t.Fatalf("expected jobs new --out to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "id: docs-audit") || !strings.Contains(output, "created job: "+out) {
		t.Fatalf("unexpected created template\noutput=%s\ncontent=%s", output, string(content))
	}

	_, stderr, code = captureMainOutput(t, func() int {
		return runJobs([]string{"new", "docs-audit", "--out", out})
	})
	if code != 1 || !strings.Contains(stderr, "use --force to overwrite") {
		t.Fatalf("expected overwrite guard, got code=%d stderr=%s", code, stderr)
	}
}

func TestJobsSubcommandHelpDispatchesToSpecificCommand(t *testing.T) {
	tests := []struct {
		subcommand string
		expected   string
	}{
		{subcommand: "new", expected: "openknowledge jobs new --reference"},
		{subcommand: "list", expected: "openknowledge jobs list [path]"},
		{subcommand: "status", expected: "openknowledge jobs status [jobs-dir]"},
		{subcommand: "runs", expected: "openknowledge jobs runs [repo]"},
		{subcommand: "start", expected: "openknowledge jobs start <job.md>"},
		{subcommand: "stop", expected: "openknowledge jobs stop <run-id>"},
		{subcommand: "kill", expected: "openknowledge jobs kill <run-id>"},
		{subcommand: "validate", expected: "openknowledge jobs validate <job-or-dir>"},
		{subcommand: "run", expected: "openknowledge jobs run <job.md> --at <time>"},
		{subcommand: "daemon", expected: "openknowledge jobs daemon [jobs-dir] --tick <duration>"},
	}

	for _, test := range tests {
		t.Run(test.subcommand, func(t *testing.T) {
			output, stderr, code := captureMainOutput(t, func() int {
				return runJobs([]string{test.subcommand, "--help"})
			})
			if code != 0 {
				t.Fatalf("expected agents %s --help to succeed, got %d\nstdout=%s\nstderr=%s", test.subcommand, code, output, stderr)
			}
			if !strings.Contains(output, test.expected) {
				t.Fatalf("expected agents %s subcommand help to include %q:\n%s", test.subcommand, test.expected, output)
			}
			if strings.Contains(output, "Experimental command group for deterministic local jobs") {
				t.Fatalf("expected specific subcommand help, got group help:\n%s", output)
			}
		})
	}
}

func TestJobsStartStatusRunsAndTerminalControl(t *testing.T) {
	root := newAgentTestRepo(t)
	jobPath := writeAgentJob(t, root, `---
id: managed-docs
enabled: true
schedule: {every: 1h, timezone: UTC}
agent: {command: git, args: [--version]}
workspace: {repo: ".", base: HEAD}
concurrency: {key: managed-docs}
---
Inspect docs.
`)
	runGit(t, root, "add", ".openknowledge/jobs/docs.md")
	runGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "job")
	stateDir := filepath.Join(t.TempDir(), "state")
	t.Setenv(agents.JobsStateDirEnv, stateDir)
	originalStarter := startDetachedJobProcess
	startDetachedJobProcess = func(_ string, args []string, _ []string) (int, error) {
		if len(args) < 5 || args[0] != "jobs" || args[1] != "run" || args[3] != "--at" {
			t.Fatalf("unexpected detached arguments: %#v", args)
		}
		job, err := agents.ParseJobFile(args[2])
		if err != nil {
			return 0, err
		}
		scheduledAt, err := time.Parse(time.RFC3339Nano, args[4])
		if err != nil {
			return 0, err
		}
		go func() {
			_, _ = agents.RunJob(job, agents.RunOptions{ScheduledAt: scheduledAt, Stdout: io.Discard, Stderr: io.Discard})
		}()
		return 4242, nil
	}
	t.Cleanup(func() { startDetachedJobProcess = originalStarter })

	output, stderr, code := captureMainOutput(t, func() int {
		return runJobs([]string{"start", jobPath, "--at", "2026-07-15T10:00:00Z", "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("expected detached start to succeed, code=%d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	var started agentStartOutput
	if err := json.Unmarshal([]byte(output), &started); err != nil || started.SupervisorPID != 4242 || started.Run.JobID != "managed-docs" {
		t.Fatalf("unexpected start output: %#v err=%v", started, err)
	}
	// RunJob performs several real Git filesystem operations after the agent
	// command exits. Leave headroom for loaded CI and encrypted macOS volumes;
	// the test still polls the persisted terminal state rather than sleeping.
	deadline := time.Now().Add(20 * time.Second)
	for !agents.IsTerminalRunStatus(started.Run.Status) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		started.Run, _ = agents.GetRunSummary(root, started.Run.RunID)
	}
	if started.Run.Status != "succeeded" {
		t.Fatalf("expected started run to succeed, got %#v", started.Run)
	}

	output, stderr, code = captureMainOutput(t, func() int {
		return runJobs([]string{"runs", root, "--json"})
	})
	var history agentRunsOutput
	if code != 0 || stderr != "" || json.Unmarshal([]byte(output), &history) != nil || len(history.Runs) != 1 || history.Runs[0].Status != "succeeded" {
		t.Fatalf("unexpected run history: code=%d output=%s stderr=%s parsed=%#v", code, output, stderr, history)
	}

	output, stderr, code = captureMainOutput(t, func() int {
		return runJobs([]string{"status", filepath.Dir(jobPath), "--json"})
	})
	var status agentStatusOutput
	if code != 0 || stderr != "" || json.Unmarshal([]byte(output), &status) != nil || len(status.Jobs) != 1 || status.Jobs[0].LastRun == nil || status.Jobs[0].NextEligibleAt == nil {
		t.Fatalf("unexpected agent status: code=%d output=%s stderr=%s parsed=%#v", code, output, stderr, status)
	}

	output, stderr, code = captureMainOutput(t, func() int {
		return runJobs([]string{"stop", started.Run.RunID, "--repo", root, "--json"})
	})
	var controlled agentControlOutput
	if code != 0 || stderr != "" || json.Unmarshal([]byte(output), &controlled) != nil || controlled.Run.Status != "succeeded" {
		t.Fatalf("terminal control should be idempotent: code=%d output=%s stderr=%s parsed=%#v", code, output, stderr, controlled)
	}
}

func TestJobsExecutorOverrideRejectsUnknownValuesBeforeExecution(t *testing.T) {
	tests := [][]string{
		{"run", filepath.Join(t.TempDir(), "missing-job.md"), "--executor", "doker"},
		{"run", filepath.Join(t.TempDir(), "missing-job.md"), "--executor=doker"},
		{"daemon", filepath.Join(t.TempDir(), "missing-jobs"), "--once", "--executor", "doker"},
		{"daemon", filepath.Join(t.TempDir(), "missing-jobs"), "--once", "--executor=doker"},
	}
	for _, args := range tests {
		_, stderr, code := captureMainOutput(t, func() int {
			return runJobs(args)
		})
		if code != 2 || !strings.Contains(stderr, "--executor must be host or docker") {
			t.Fatalf("expected fail-closed executor usage error for %v, code=%d stderr=%s", args, code, stderr)
		}
		if strings.Contains(stderr, "no such file") {
			t.Fatalf("executor validation must happen before job discovery for %v: %s", args, stderr)
		}
	}
}

func TestJobsDaemonPassIsolatesLoadAndPlanningFailures(t *testing.T) {
	root := newAgentTestRepo(t)
	jobsDir := filepath.Join(root, ".openknowledge", "jobs")
	writeMainTestFile(t, root, ".openknowledge/jobs/00-invalid.md", `---
id: invalid
agent: {command: agent, argz: []}
---
Invalid.
`)
	writeMainTestFile(t, root, ".openknowledge/jobs/10-broken-plan.md", `---
id: broken-plan
schedule: {every: 1h}
agent: {command: agent}
workspace: {repo: ../../missing}
---
Cannot resolve a repository.
`)
	writeMainTestFile(t, root, ".openknowledge/jobs/20-valid.md", `---
id: valid-due
schedule: {every: 1h}
agent: {command: agent}
workspace: {repo: ../..}
---
Plan this job.
`)

	output, stderr, code := captureMainOutput(t, func() int {
		return runDueAgentJobs(jobsDir, "", true)
	})
	if code != 1 {
		t.Fatalf("expected aggregate daemon failure after the complete pass, code=%d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	if !strings.Contains(output, `"job_id": "valid-due"`) {
		t.Fatalf("expected the later valid job to be planned despite earlier failures:\n%s", output)
	}
	for _, expected := range []string{"00-invalid.md failed to load", "broken-plan failed to plan", "completed with 2 failure(s)"} {
		if !strings.Contains(stderr, expected) {
			t.Fatalf("expected daemon diagnostics to include %q:\n%s", expected, stderr)
		}
	}
	if strings.Contains(output, "no due jobs") {
		t.Fatalf("a pass with a planned due job must not report no work:\n%s", output)
	}
}

func TestJobsRunCreatesRunRecord(t *testing.T) {
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
	runGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add job")

	output, stderr, code := captureMainOutput(t, func() int {
		return runJobs([]string{"run", jobPath, "--at", "2026-07-07T09:00:00Z"})
	})
	if code != 0 {
		t.Fatalf("expected jobs run to succeed, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
	}
	if !strings.Contains(output, "job run ") || !strings.Contains(output, "worktree: ") {
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

func TestJobsSequentialRunsKeepSourceRepositoryClean(t *testing.T) {
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
			return runJobs([]string{"run", jobPath, "--at", scheduledAt})
		})
		if code != 0 {
			t.Fatalf("expected sequential run at %s to succeed, code=%d stdout=%s stderr=%s", scheduledAt, code, output, stderr)
		}
		if status := agentGitOutput(t, root, "status", "--porcelain"); strings.TrimSpace(status) != "" {
			t.Fatalf("job runtime must not dirty the source repository after %s: %s", scheduledAt, status)
		}
		runPath := strings.TrimSpace(strings.TrimPrefix(lineWithPrefix(output, "run: "), "run: "))
		if runPath == "" || strings.HasPrefix(runPath, root+string(filepath.Separator)) {
			t.Fatalf("expected external run path, got %q", runPath)
		}
	}
}

func TestJobsRejectStateDirectoryInsideSourceRepository(t *testing.T) {
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
	t.Setenv(agents.JobsStateDirEnv, filepath.Join(root, ".agent-runtime"))
	_, stderr, code := captureMainOutput(t, func() int {
		return runJobs([]string{"run", jobPath, "--dry-run", "--at", "2026-07-07T09:00:00Z"})
	})
	if code != 1 || !strings.Contains(stderr, "jobs state directory must be outside the Git repository") {
		t.Fatalf("expected in-repository state refusal, code=%d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".agent-runtime")); !os.IsNotExist(err) {
		t.Fatalf("refused state directory must not be created, got %v", err)
	}
}

func TestJobsVerificationTimeoutFailsRunPromptly(t *testing.T) {
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
		return runJobs([]string{"run", jobPath, "--at", "2026-07-07T09:00:00Z"})
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
	t.Setenv(agents.JobsStateDirEnv, t.TempDir())
	root := t.TempDir()
	runGit(t, root, "init")
	writeMainTestFile(t, root, "README.md", "# Test\n")
	runGit(t, root, "add", "README.md")
	runGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	return root
}

func writeAgentJob(t *testing.T, root string, content string) string {
	t.Helper()
	rel := ".openknowledge/jobs/docs.md"
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
