package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
)

const codexExecutableEnv = "OPENKNOWLEDGE_CODEX"

var discoverCodexExecutableCandidates = defaultCodexExecutableCandidates
var probeCodexExecutable = defaultProbeCodexExecutable

func resolveCodexExecutable(ctx context.Context) (string, error) {
	if configured := strings.TrimSpace(os.Getenv(codexExecutableEnv)); configured != "" {
		candidate, err := resolveConfiguredExecutable(configured)
		if err != nil {
			return "", fmt.Errorf("%s: %w", codexExecutableEnv, err)
		}
		if err := probeCodexExecutable(ctx, candidate); err != nil {
			return "", fmt.Errorf("%s points to an unusable Codex executable %s: %w", codexExecutableEnv, candidate, err)
		}
		return candidate, nil
	}

	candidates := discoverCodexExecutableCandidates()
	failures := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if err := probeCodexExecutable(ctx, candidate); err == nil {
			return candidate, nil
		} else {
			failures = append(failures, candidate+": "+compactCodexProbeError(err))
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no Codex CLI executable found; install Codex or set %s to a working executable", codexExecutableEnv)
	}
	return "", fmt.Errorf("no working Codex CLI found; reinstall Codex or set %s to a working executable (checked %s)", codexExecutableEnv, strings.Join(failures, "; "))
}

func resolveAgentExecutable(ctx context.Context, runtimeName string) (string, error) {
	if runtimeName == "codex" {
		return resolveCodexExecutable(ctx)
	}
	definition, err := agents.HarnessForRuntime(runtimeName)
	if err != nil {
		return "", err
	}
	configured := strings.TrimSpace(os.Getenv(definition.ExecutableEnv))
	if configured == "" {
		configured = definition.Executable
	}
	candidate, err := resolveConfiguredExecutable(configured)
	if err != nil {
		return "", fmt.Errorf("%s runtime: %w; install %s or set %s", runtimeName, err, definition.Executable, definition.ExecutableEnv)
	}
	if err := probeCodexExecutable(ctx, candidate); err != nil {
		return "", fmt.Errorf("%s runtime executable %s is unusable: %w", runtimeName, candidate, err)
	}
	return candidate, nil
}

func resolveConfiguredExecutable(configured string) (string, error) {
	if filepath.IsAbs(configured) || strings.ContainsAny(configured, `/\`) {
		absolute, err := filepath.Abs(configured)
		if err != nil {
			return "", err
		}
		return absolute, nil
	}
	resolved, err := exec.LookPath(configured)
	if err != nil {
		return "", fmt.Errorf("cannot find %q in PATH", configured)
	}
	return resolved, nil
}

func defaultCodexExecutableCandidates() []string {
	seen := make(map[string]bool)
	candidates := make([]string, 0, 4)
	add := func(candidate string) {
		if strings.TrimSpace(candidate) == "" {
			return
		}
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			return
		}
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			return
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}

	if resolved, err := exec.LookPath("codex"); err == nil {
		add(resolved)
	}
	binaryName := "codex"
	if runtime.GOOS == "windows" {
		binaryName = "codex.exe"
	}
	for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
		if directory == "" {
			directory = "."
		}
		add(filepath.Join(directory, binaryName))
	}
	if runtime.GOOS == "darwin" {
		for _, application := range []string{"Codex.app", "ChatGPT.app"} {
			add(filepath.Join("/Applications", application, "Contents", "Resources", "codex"))
			if home, err := os.UserHomeDir(); err == nil {
				add(filepath.Join(home, "Applications", application, "Contents", "Resources", "codex"))
			}
		}
	}
	return candidates
}

func defaultProbeCodexExecutable(ctx context.Context, executable string) error {
	probeContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(probeContext, executable, "--version")
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if probeContext.Err() != nil {
		return fmt.Errorf("version probe timed out")
	}
	if message == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, message)
}

func compactCodexProbeError(err error) string {
	message := strings.Join(strings.Fields(err.Error()), " ")
	const limit = 512
	if len(message) > limit {
		message = message[:limit] + "..."
	}
	return message
}
