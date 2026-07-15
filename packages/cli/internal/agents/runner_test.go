package agents

import (
	"reflect"
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
}
