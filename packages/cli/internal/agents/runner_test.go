package agents

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDockerCommandArgsEnforceIsolationBeforeImage(t *testing.T) {
	plan := RunPlan{
		Worktree: "/repo/worktree",
		Sandbox: SandboxSpec{
			Type:  "docker",
			Image: "example.test/agent:latest",
		},
	}
	command := Command{Command: "agent", Args: []string{"exec", "--write"}}
	want := []string{
		"run", "--rm", "-i", "--init",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--pids-limit", "512",
		"--network", "none",
		"-v", "/repo/worktree:/workspace",
		"-w", "/workspace",
		"--", "example.test/agent:latest",
		"agent", "exec", "--write",
	}
	if got := dockerCommandArgs(plan, command, ""); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected hardened Docker arguments:\ngot  %#v\nwant %#v", got, want)
	}
}

func TestPersistPublicEvalArtifactCreatesBrowsableBundle(t *testing.T) {
	worktree := t.TempDir()
	reports := t.TempDir()
	jsonPath := filepath.Join(reports, "eval.json")
	markdownPath := filepath.Join(reports, "eval.md")
	if err := os.WriteFile(jsonPath, []byte("{\"schemaVersion\":\"1\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(markdownPath, []byte("# Eval\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := RunPlan{RunID: "run-123", Worktree: worktree}
	result := EvalResult{Base: "main", JSONPath: jsonPath, MarkdownPath: markdownPath}
	if err := persistPublicEvalArtifact(plan, result); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(worktree, ".openknowledge", "reports", plan.RunID)
	for _, name := range []string{"artifact.json", "index.md", "eval.md", "eval.json"} {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Fatalf("expected durable artifact %s: %v", name, err)
		}
	}
	index, err := os.ReadFile(filepath.Join(target, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "[eval.md](eval.md)") || !strings.Contains(string(index), "Run: run-123") {
		t.Fatalf("unexpected browsable artifact index:\n%s", index)
	}
	var manifest struct {
		Type  string   `json:"type"`
		Files []string `json:"files"`
	}
	content, err := os.ReadFile(filepath.Join(target, "artifact.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Type != "openknowledge.artifact" || !reflect.DeepEqual(manifest.Files, []string{"index.md", "eval.md", "eval.json"}) {
		t.Fatalf("unexpected artifact manifest: %#v", manifest)
	}
}

func TestDockerCommandArgsRequireExplicitBridgeNetwork(t *testing.T) {
	plan := RunPlan{
		Worktree: "/repo/worktree",
		Sandbox: SandboxSpec{
			Type:    "docker",
			Image:   "agent:latest",
			Network: "bridge",
			Env:     []string{"OPENAI_API_KEY"},
		},
	}
	command := Command{Command: "go test ./...", Shell: true}
	args := dockerCommandArgs(plan, command, "")
	if !reflect.DeepEqual(args[len(args)-5:], []string{"--", "agent:latest", "sh", "-lc", "go test ./..."}) {
		t.Fatalf("expected image boundary and shell command, got %#v", args)
	}
	foundBridge := false
	for index := range args {
		if index+1 < len(args) && args[index] == "--network" && args[index+1] == "bridge" {
			foundBridge = true
		}
	}
	if !foundBridge {
		t.Fatalf("expected explicit bridge network in %#v", args)
	}
	foundExplicitEnvironment := false
	for index := range args {
		if index+1 < len(args) && args[index] == "--env" && args[index+1] == "OPENAI_API_KEY" {
			foundExplicitEnvironment = true
		}
	}
	if !foundExplicitEnvironment {
		t.Fatalf("expected only the named environment capability in %#v", args)
	}
}

func TestHostCommandEnvironmentDoesNotInheritSecretsByDefault(t *testing.T) {
	t.Setenv("PATH", "/safe/bin")
	t.Setenv("LANG", "C.UTF-8")
	t.Setenv("OPENKNOWLEDGE_TEST_SECRET", "secret-value")
	t.Setenv("OPENKNOWLEDGE_ALLOWED_TOKEN", "allowed-value")
	plan := RunPlan{
		RunDir:   filepath.Join(t.TempDir(), "run"),
		Worktree: t.TempDir(),
		Sandbox: SandboxSpec{
			Type: "host",
			Env:  []string{"OPENKNOWLEDGE_ALLOWED_TOKEN"},
		},
	}

	cmd := commandForPlan(context.Background(), plan, Command{Command: "agent"}, "")
	environment := make(map[string]string)
	for _, entry := range cmd.Env {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			environment[name] = value
		}
	}
	if _, leaked := environment["OPENKNOWLEDGE_TEST_SECRET"]; leaked {
		t.Fatalf("unexpected ambient secret in host command environment: %#v", environment)
	}
	if environment["OPENKNOWLEDGE_ALLOWED_TOKEN"] != "allowed-value" {
		t.Fatalf("expected explicitly allowed environment value, got %#v", environment)
	}
	if environment["PATH"] != "/safe/bin" || environment["LANG"] != "C.UTF-8" {
		t.Fatalf("expected minimal runtime environment, got %#v", environment)
	}
	if environment["HOME"] != filepath.Join(plan.RunDir, "home") || environment["TMPDIR"] != filepath.Join(plan.RunDir, "tmp") {
		t.Fatalf("expected isolated home and temp paths, got %#v", environment)
	}
	if _, err := os.Stat(environment["HOME"]); !os.IsNotExist(err) {
		t.Fatalf("command construction must not create runtime directories, got %v", err)
	}
}

func TestAgentCredentialEnvironmentIsNotPassedToVerification(t *testing.T) {
	t.Setenv("CODEX_API_KEY", "agent-only-secret")
	plan := RunPlan{
		RunDir:   filepath.Join(t.TempDir(), "run"),
		Worktree: t.TempDir(),
		Sandbox:  SandboxSpec{Type: "host"},
		Agent:    Command{Runtime: RuntimeCodex, Command: "codex", Env: []string{"CODEX_API_KEY"}},
	}
	agentEnvironment := hostCommandEnvironment(plan, plan.Agent)
	verifyEnvironment := hostCommandEnvironment(plan, Command{Command: "openknowledge validate Wiki", Shell: true})
	if !environmentContains(agentEnvironment, "CODEX_API_KEY=agent-only-secret") {
		t.Fatalf("agent credential missing from agent command: %#v", agentEnvironment)
	}
	if environmentContains(verifyEnvironment, "CODEX_API_KEY=agent-only-secret") {
		t.Fatalf("agent credential leaked into verification: %#v", verifyEnvironment)
	}
}

func environmentContains(environment []string, expected string) bool {
	for _, value := range environment {
		if value == expected {
			return true
		}
	}
	return false
}

func TestRunJobSkipsAndRecordsHeldConcurrencyKey(t *testing.T) {
	root := t.TempDir()
	runTestGit(t, root, "init")
	jobPath := filepath.Join(root, "job.md")
	content := `---
id: concurrency-test
agent: {runtime: codex}
workspace: {repo: ".", base: HEAD}
concurrency: {key: wiki-maintenance, policy: skip}
---
Maintain the wiki.
`
	if err := os.WriteFile(jobPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, root, "add", "job.md")
	runTestGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "job")
	t.Setenv(JobsStateDirEnv, filepath.Join(t.TempDir(), "jobs-state"))

	job, err := ParseJobFile(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	scheduledAt := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
	plan, err := BuildRunPlan(job, scheduledAt, "")
	if err != nil {
		t.Fatal(err)
	}
	release, acquired, err := acquireConcurrency(plan)
	if err != nil || !acquired {
		t.Fatalf("hold concurrency key: acquired=%t err=%v", acquired, err)
	}
	defer func() { _ = release() }()

	record, err := RunJob(job, RunOptions{ScheduledAt: scheduledAt})
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != "skipped" || !strings.Contains(record.StatusText, `"wiki-maintenance" is already running`) {
		t.Fatalf("unexpected skipped record: %#v", record)
	}
	if _, err := os.Stat(plan.Worktree); !os.IsNotExist(err) {
		t.Fatalf("skipped run must not create a worktree: %v", err)
	}
	contentJSON, err := os.ReadFile(filepath.Join(plan.RunDir, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted RunRecord
	if err := json.Unmarshal(contentJSON, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Status != "skipped" || persisted.Plan.Concurrency.Key != "wiki-maintenance" || persisted.Plan.Concurrency.Policy != "skip" {
		t.Fatalf("unexpected persisted concurrency record: %#v", persisted)
	}
}

func TestRunJobRunsPreflightBeforeAgentAndStopsOnFailure(t *testing.T) {
	repo := t.TempDir()
	runTestGit(t, repo, "init")
	marker := filepath.Join(t.TempDir(), "agent-ran")
	script := "#!/bin/sh\nprintf ran > '" + marker + "'\ncat >/dev/null\n"
	installTestCodex(t, script)
	jobPath := filepath.Join(repo, "job.md")
	content := `---
id: preflight-order
agent: {runtime: codex}
workspace: {repo: ".", base: HEAD}
preflight:
  commands:
    - exit 9
verify:
  commands:
    - "true"
---
Agent should not run.
`
	if err := os.WriteFile(jobPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repo, "add", "job.md")
	runTestGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "job")
	t.Setenv(JobsStateDirEnv, filepath.Join(t.TempDir(), "jobs-state"))

	job, err := ParseJobFile(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	record, err := RunJob(job, RunOptions{ScheduledAt: time.Date(2026, 8, 11, 8, 0, 0, 0, time.UTC)})
	if err == nil || record.Status != "preflight_failed" {
		t.Fatalf("expected preflight failure, record=%#v err=%v", record, err)
	}
	if len(record.Preflight) != 1 || record.Preflight[0].ExitCode != 9 || record.Agent.Command != "" || len(record.Verify) != 0 {
		t.Fatalf("expected preflight to short-circuit agent and verification: %#v", record)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("agent must not start after preflight failure: %v", err)
	}
}

func TestRunJobSupportsAgentlessDeterministicValidation(t *testing.T) {
	repo := t.TempDir()
	runTestGit(t, repo, "init")
	if err := os.Mkdir(filepath.Join(repo, "Wiki"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Wiki", "index.md"), []byte("---\nokf_version: \"0.2\"\n---\n\n# Wiki\n\nKnowledge evaluation.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".openknowledge", "evals"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".openknowledge", "evals", "docs.yaml"), []byte("type: openknowledge.eval\nversion: 1\nid: docs\ncases: [{id: home, question: Knowledge evaluation, expect: {min_sources: 1}}]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	jobPath := filepath.Join(repo, "job.md")
	content := `---
id: deterministic-validation
workspace: {repo: ".", base: HEAD}
verify:
  commands:
    - test -f Wiki/index.md
  eval:
    dataset: .openknowledge/evals/docs.yaml
    target: Wiki
    gate: all
---
`
	if err := os.WriteFile(jobPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repo, "add", "Wiki", ".openknowledge/evals/docs.yaml", "job.md")
	runTestGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "job")
	t.Setenv(JobsStateDirEnv, filepath.Join(t.TempDir(), "jobs-state"))

	job, err := ParseJobFile(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	record, err := RunJob(job, RunOptions{ScheduledAt: time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)})
	if err != nil || record.Status != "succeeded" {
		t.Fatalf("expected deterministic validation to succeed, record=%#v err=%v", record, err)
	}
	if record.Agent.Command != "" || len(record.Verify) != 1 || record.Verify[0].ExitCode != 0 {
		t.Fatalf("expected verification without an agent: %#v", record)
	}
	if record.Eval == nil || record.Eval.Status != "pass" || record.Eval.JSONPath == "" || record.Eval.MarkdownPath == "" {
		t.Fatalf("expected native eval verification artifacts: %#v", record.Eval)
	}
	for _, path := range []string{record.Eval.JSONPath, record.Eval.MarkdownPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Fatalf("expected private eval artifact %s, got %04o", path, info.Mode().Perm())
		}
	}
}

func TestRunJobFailsClosedWhenEvalGateFails(t *testing.T) {
	repo := t.TempDir()
	runTestGit(t, repo, "init")
	if err := os.MkdirAll(filepath.Join(repo, "Wiki"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Wiki", "index.md"), []byte("---\nokf_version: \"0.2\"\n---\n\n# Wiki\n"), 0644); err != nil {
		t.Fatal(err)
	}
	dataset := filepath.Join(repo, "eval.yaml")
	if err := os.WriteFile(dataset, []byte("type: openknowledge.eval\nversion: 1\nid: failing\ncases: [{id: missing, question: absent evidence, expect: {sources: [missing.md]}}]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	jobPath := filepath.Join(repo, "job.md")
	if err := os.WriteFile(jobPath, []byte("---\nid: eval-failure\nworkspace: {repo: \".\", base: HEAD}\nverify: {eval: {dataset: eval.yaml, target: Wiki, gate: all}}\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repo, "add", "Wiki", "eval.yaml", "job.md")
	runTestGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "eval job")
	t.Setenv(JobsStateDirEnv, filepath.Join(t.TempDir(), "jobs-state"))
	job, err := ParseJobFile(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	record, err := RunJob(job, RunOptions{ScheduledAt: time.Date(2026, 8, 11, 9, 30, 0, 0, time.UTC)})
	if err == nil || record.Status != "verification_failed" || record.Eval == nil || record.Eval.Status != "fail" {
		t.Fatalf("expected closed eval gate, record=%#v err=%v", record, err)
	}
	if _, statErr := os.Stat(record.Eval.JSONPath); statErr != nil {
		t.Fatalf("failing gate must retain its report: %v", statErr)
	}
}

func TestBuildRunPlanKeepsScheduledRunIDAcrossRepositoryChanges(t *testing.T) {
	repo := t.TempDir()
	runTestGit(t, repo, "init")
	jobPath := filepath.Join(repo, "job.md")
	writeJob := func(prompt string) {
		t.Helper()
		content := "---\nid: stable-schedule\nagent: {runtime: codex}\nworkspace: {repo: \".\", base: HEAD}\n---\n" + prompt + "\n"
		if err := os.WriteFile(jobPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		runTestGit(t, repo, "add", "job.md")
		runTestGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", prompt)
	}
	writeJob("First prompt.")
	t.Setenv(JobsStateDirEnv, filepath.Join(t.TempDir(), "jobs-state"))
	scheduledAt := time.Date(2026, 7, 15, 5, 0, 0, 0, time.UTC)

	firstJob, err := ParseJobFile(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	first, err := BuildRunPlan(firstJob, scheduledAt, "")
	if err != nil {
		t.Fatal(err)
	}
	writeJob("Updated prompt.")
	secondJob, err := ParseJobFile(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildRunPlan(secondJob, scheduledAt, "")
	if err != nil {
		t.Fatal(err)
	}

	if first.RunID != second.RunID {
		t.Fatalf("the same scheduled slot must keep its run ID: %s != %s", first.RunID, second.RunID)
	}
	if first.BaseSHA == second.BaseSHA || first.Prompt == second.Prompt {
		t.Fatalf("expected plans to reflect repository changes: first=%#v second=%#v", first, second)
	}
	later, err := BuildRunPlan(secondJob, scheduledAt.Add(24*time.Hour), "")
	if err != nil {
		t.Fatal(err)
	}
	if later.RunID == second.RunID {
		t.Fatalf("different scheduled slots must have different run IDs: %s", later.RunID)
	}
}

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func installTestCodex(t *testing.T, script string) string {
	t.Helper()
	name := "codex"
	if runtime.GOOS == "windows" {
		name = "codex.cmd"
		if script == "" {
			script = "@echo off\r\nexit /b 0\r\n"
		}
	} else if script == "" {
		script = "#!/bin/sh\ncat >/dev/null\nexit 0\n"
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENKNOWLEDGE_CODEX", path)
	return path
}
