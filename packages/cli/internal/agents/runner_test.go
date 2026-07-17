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
