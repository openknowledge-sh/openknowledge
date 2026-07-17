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
	if !reflect.DeepEqual(gotArguments[:3], []string{"exec", "--sandbox", "workspace-write"}) ||
		!strings.Contains(gotArguments[3], agents.AgentProtocolVersion) || !strings.Contains(gotArguments[3], "Task:\nUpdate the wiki") {
		t.Fatalf("agent arguments = %#v", gotArguments)
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
	if !reflect.DeepEqual(got[:4], []string{"--sandbox", "workspace-write", "--model", "gpt-test"}) ||
		!strings.Contains(got[4], "Task:\nStart here") {
		t.Fatalf("agent arguments = %#v", got)
	}
}

func TestAgentSupportsClaudeAndOpenCodeAdapters(t *testing.T) {
	directory := t.TempDir()
	originalRun := runAgentProcess
	originalProbe := probeCodexExecutable
	t.Cleanup(func() {
		runAgentProcess = originalRun
		probeCodexExecutable = originalProbe
	})
	probeCodexExecutable = func(_ context.Context, _ string) error { return nil }
	for _, test := range []struct {
		runtime string
		env     string
		path    string
		prefix  []string
	}{
		{runtime: "claude", env: "OPENKNOWLEDGE_CLAUDE", path: "/test/claude", prefix: []string{"--print", "--no-session-persistence", "--permission-mode", "acceptEdits"}},
		{runtime: "grok", env: "OPENKNOWLEDGE_GROK", path: "/test/grok", prefix: []string{"--no-auto-update", "--always-approve", "--single"}},
		{runtime: "opencode", env: "OPENKNOWLEDGE_OPENCODE", path: "/test/opencode", prefix: []string{"run", "--auto"}},
	} {
		t.Run(test.runtime, func(t *testing.T) {
			t.Setenv(test.env, test.path)
			var executable string
			var arguments []string
			runAgentProcess = func(_ context.Context, gotExecutable string, gotArguments []string, _ string) error {
				executable = gotExecutable
				arguments = append([]string(nil), gotArguments...)
				return nil
			}
			if code := runAgent([]string{"exec", "--runtime", test.runtime, "--path", directory, "--no-steer", "Update docs"}); code != 0 {
				t.Fatalf("agent exited %d", code)
			}
			if executable != test.path || len(arguments) < len(test.prefix)+1 || !reflect.DeepEqual(arguments[:len(test.prefix)], test.prefix) || arguments[len(arguments)-1] != "Update docs" {
				t.Fatalf("unexpected invocation executable=%q args=%#v", executable, arguments)
			}
		})
	}
}

func TestAgentInitAndFromExecuteExistingPromptBuilders(t *testing.T) {
	directory := t.TempDir()
	stubCodexResolver(t, "/test/codex")
	original := runAgentProcess
	t.Cleanup(func() { runAgentProcess = original })
	var prompts []string
	runAgentProcess = func(_ context.Context, _ string, arguments []string, _ string) error {
		prompts = append(prompts, arguments[len(arguments)-1])
		return nil
	}
	if code := runAgent([]string{"init", "--path", directory, "--rules", "docs"}); code != 0 {
		t.Fatalf("agent init exited %d", code)
	}
	if code := runAgent([]string{"from", "https://example.test/docs", "--out", "Wiki", "--path", directory}); code != 0 {
		t.Fatalf("agent from exited %d", code)
	}
	if len(prompts) != 2 || !strings.Contains(prompts[0], "This setup guide is meant to be executed") || !strings.Contains(prompts[0], "Open Knowledge agent contract") {
		t.Fatalf("init did not reuse setup prompt: %#v", prompts)
	}
	if !strings.Contains(prompts[1], "https://example.test/docs") || !strings.Contains(prompts[1], "Wiki") || !strings.Contains(prompts[1], "Execution mode") && !strings.Contains(prompts[1], "Generate the requested") {
		t.Fatalf("from did not reuse source prompt: %s", prompts[1])
	}
}

func TestAgentDoctorReportsExplicitRuntime(t *testing.T) {
	t.Setenv("OPENKNOWLEDGE_CLAUDE", "/test/claude")
	originalProbe := probeCodexExecutable
	probeCodexExecutable = func(_ context.Context, executable string) error {
		if executable != "/test/claude" {
			return fmt.Errorf("unexpected executable %s", executable)
		}
		return nil
	}
	t.Cleanup(func() { probeCodexExecutable = originalProbe })
	output, stderr, code := captureMainOutput(t, func() int {
		return runAgent([]string{"doctor", "--runtime", "claude", "--json"})
	})
	if code != 0 || stderr != "" || !strings.Contains(output, `"runtime": "claude"`) || !strings.Contains(output, `"available": true`) {
		t.Fatalf("unexpected doctor result code=%d stdout=%s stderr=%s", code, output, stderr)
	}
}

func TestAgentRejectsOperationSpecificFlagsOutsideTheirOperation(t *testing.T) {
	for _, args := range [][]string{
		{"exec", "--rules", "docs", "Update docs"},
		{"doctor", "--isolate"},
		{"doctor", "--path", t.TempDir()},
	} {
		if _, err := parseAgentArgs(args); err == nil {
			t.Fatalf("expected strict option refusal for %#v", args)
		}
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
