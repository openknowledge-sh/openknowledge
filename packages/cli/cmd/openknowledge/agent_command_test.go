package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
)

func TestAgentExecUsesCurrentFilesystemByDefault(t *testing.T) {
	directory := t.TempDir()
	stubCodexResolver(t, "/test/codex")
	original := runAgentProcess
	defer func() { runAgentProcess = original }()
	var gotArguments []string
	var gotDirectory string
	runAgentProcess = func(_ context.Context, executable string, arguments []string, workingDirectory string) error {
		if executable != "/test/codex" {
			t.Fatalf("resolved executable = %q", executable)
		}
		gotArguments = append([]string(nil), arguments...)
		gotDirectory = workingDirectory
		return nil
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runAgent([]string{"exec", "--path", directory, "Update", "the", "wiki"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("agent exec failed: code=%d stderr=%s", code, stderr)
	}
	absolute, _ := filepath.Abs(directory)
	if gotDirectory != absolute {
		t.Fatalf("agent ran in %q, want %q", gotDirectory, absolute)
	}
	want := []string{"exec", "--sandbox", "workspace-write", "Update the wiki"}
	if !reflect.DeepEqual(gotArguments, want) {
		t.Fatalf("agent arguments = %#v, want %#v", gotArguments, want)
	}
}

func TestAgentInteractiveAcceptsInitialPromptAndModel(t *testing.T) {
	directory := t.TempDir()
	stubCodexResolver(t, "/test/codex")
	original := runAgentProcess
	defer func() { runAgentProcess = original }()
	var got []string
	runAgentProcess = func(_ context.Context, _ string, arguments []string, _ string) error {
		got = append([]string(nil), arguments...)
		return nil
	}
	if code := runAgent([]string{"--path", directory, "--model", "gpt-test", "Start here"}); code != 0 {
		t.Fatalf("interactive agent exited %d", code)
	}
	want := []string{"--sandbox", "workspace-write", "--model", "gpt-test", "Start here"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("agent arguments = %#v, want %#v", got, want)
	}
}

func TestAgentIsolateCreatesRetainedWorktree(t *testing.T) {
	repo := newAgentTestRepo(t)
	stubCodexResolver(t, "/test/codex")
	nested := filepath.Join(repo, "Wiki")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "index.md"), []byte("# Wiki\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "Wiki/index.md")
	runGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add wiki")
	t.Setenv(agents.JobsStateDirEnv, t.TempDir())
	original := runAgentProcess
	defer func() { runAgentProcess = original }()
	var gotDirectory string
	runAgentProcess = func(_ context.Context, _ string, _ []string, workingDirectory string) error {
		gotDirectory = workingDirectory
		return os.WriteFile(filepath.Join(workingDirectory, "agent-created.md"), []byte("created\n"), 0644)
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runAgent([]string{"exec", "--isolate", "--path", nested, "Create a page"})
	})
	if code != 0 {
		t.Fatalf("isolated agent failed: code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "isolated agent workspace:") || !strings.Contains(stderr, "branch: agent/") {
		t.Fatalf("missing isolation handoff:\n%s", stderr)
	}
	if !strings.Contains(gotDirectory, "interactive-worktrees") || filepath.Base(gotDirectory) != "Wiki" {
		t.Fatalf("unexpected isolated working directory: %s", gotDirectory)
	}
	if _, err := os.Stat(filepath.Join(gotDirectory, "agent-created.md")); err != nil {
		t.Fatalf("isolated changes were not retained: %v", err)
	}
}

func TestResolveCodexExecutableSkipsBrokenPATHCandidate(t *testing.T) {
	t.Setenv(codexExecutableEnv, "")
	originalDiscover := discoverCodexExecutableCandidates
	originalProbe := probeCodexExecutable
	t.Cleanup(func() {
		discoverCodexExecutableCandidates = originalDiscover
		probeCodexExecutable = originalProbe
	})
	discoverCodexExecutableCandidates = func() []string { return []string{"/broken/codex", "/working/codex"} }
	probeCodexExecutable = func(_ context.Context, executable string) error {
		if executable == "/broken/codex" {
			return fmt.Errorf("native binary is missing")
		}
		return nil
	}
	resolved, err := resolveCodexExecutable(context.Background())
	if err != nil || resolved != "/working/codex" {
		t.Fatalf("resolved=%q err=%v", resolved, err)
	}
}

func TestResolveCodexExecutableFailsClosedForBrokenExplicitOverride(t *testing.T) {
	t.Setenv(codexExecutableEnv, "/configured/codex")
	originalProbe := probeCodexExecutable
	t.Cleanup(func() { probeCodexExecutable = originalProbe })
	probeCodexExecutable = func(_ context.Context, executable string) error {
		return fmt.Errorf("%s is broken", executable)
	}
	_, err := resolveCodexExecutable(context.Background())
	if err == nil || !strings.Contains(err.Error(), codexExecutableEnv) || !strings.Contains(err.Error(), "/configured/codex") {
		t.Fatalf("unexpected explicit override error: %v", err)
	}
}

func TestRemovedAgentAutomationCommandsHaveNoAliases(t *testing.T) {
	for _, arguments := range [][]string{{"agents"}, {"jobs", "spawn", "job.md"}, {"runtime", "worker", "--role", "agents"}} {
		_, _, code := captureMainOutput(t, func() int { return dispatchCLI(arguments) })
		if code != 2 {
			t.Fatalf("removed command %v exited %d, want usage error", arguments, code)
		}
	}
}

func stubCodexResolver(t *testing.T, executable string) {
	t.Helper()
	t.Setenv(codexExecutableEnv, executable)
	original := probeCodexExecutable
	probeCodexExecutable = func(_ context.Context, candidate string) error {
		if candidate != executable {
			return fmt.Errorf("unexpected candidate %s", candidate)
		}
		return nil
	}
	t.Cleanup(func() { probeCodexExecutable = original })
}
