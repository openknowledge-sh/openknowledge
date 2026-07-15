package agents

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
