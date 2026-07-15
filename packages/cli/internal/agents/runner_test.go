package agents

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	if got := dockerCommandArgs(plan, command); !reflect.DeepEqual(got, want) {
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
	args := dockerCommandArgs(plan, command)
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

	cmd := commandForPlan(context.Background(), plan, Command{Command: "agent"})
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

func TestRunJobSkipsAndRecordsHeldConcurrencyKey(t *testing.T) {
	root := t.TempDir()
	runTestGit(t, root, "init")
	jobPath := filepath.Join(root, "job.md")
	content := `---
id: concurrency-test
agent: {command: agent}
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
	t.Setenv(AgentsStateDirEnv, filepath.Join(t.TempDir(), "agent-state"))

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
