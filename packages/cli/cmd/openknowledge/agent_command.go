package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
)

type agentCLIOptions struct {
	exec    bool
	isolate bool
	path    string
	model   string
	prompt  string
}

var runAgentProcess = func(ctx context.Context, arguments []string, directory string) error {
	command := exec.CommandContext(ctx, "codex", arguments...)
	command.Dir = directory
	command.Env = os.Environ()
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func runAgent(args []string) int {
	if hasHelpFlag(args) {
		if len(args) > 0 && args[0] == "exec" {
			fmt.Fprint(os.Stdout, agentExecHelpText())
		} else {
			fmt.Fprint(os.Stdout, agentHelpText())
		}
		return 0
	}
	options, err := parseAgentArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	directory, err := filepath.Abs(options.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if info, err := os.Stat(directory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	} else if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "agent path is not a directory: %s\n", directory)
		return 1
	}

	var isolated agents.IsolatedWorkspace
	if options.isolate {
		isolated, err = agents.PrepareIsolatedWorkspace(directory)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		directory = isolated.WorkDir
		fmt.Fprintf(os.Stderr, "isolated agent workspace: %s\n", isolated.Worktree)
		fmt.Fprintf(os.Stderr, "branch: %s\n", isolated.Branch)
	}

	arguments := make([]string, 0, 8)
	if options.exec {
		arguments = append(arguments, "exec")
	}
	arguments = append(arguments, "--sandbox", "workspace-write")
	if options.model != "" {
		arguments = append(arguments, "--model", options.model)
	}
	if options.prompt != "" {
		arguments = append(arguments, options.prompt)
	}

	ctx := context.Background()
	if options.exec {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, os.Interrupt)
		defer stop()
	} else {
		// The interactive Codex TUI receives terminal interrupts directly. Keep
		// this wrapper alive so it does not terminate the session first.
		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)
		defer signal.Stop(interrupts)
	}
	if err := runAgentProcess(ctx, arguments, directory); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return exitError.ExitCode()
		}
		if errors.Is(err, exec.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "codex executable was not found in PATH")
		} else {
			fmt.Fprintf(os.Stderr, "run Codex agent: %v\n", err)
		}
		return 1
	}
	return 0
}

func parseAgentArgs(args []string) (agentCLIOptions, error) {
	options := agentCLIOptions{path: "."}
	if len(args) > 0 && args[0] == "exec" {
		options.exec = true
		args = args[1:]
	}
	positionals := make([]string, 0, 1)
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--":
			positionals = append(positionals, args[index+1:]...)
			index = len(args)
		case argument == "--isolate":
			options.isolate = true
		case argument == "--path" || argument == "--model":
			value, next, err := nextFlagValue(args, index, argument)
			if err != nil {
				return options, err
			}
			if argument == "--path" {
				options.path = value
			} else {
				options.model = value
			}
			index = next
		case strings.HasPrefix(argument, "--path="):
			options.path = strings.TrimPrefix(argument, "--path=")
		case strings.HasPrefix(argument, "--model="):
			options.model = strings.TrimPrefix(argument, "--model=")
		case strings.HasPrefix(argument, "-"):
			return options, fmt.Errorf("unknown agent option: %s", argument)
		default:
			positionals = append(positionals, argument)
		}
	}
	if strings.TrimSpace(options.path) == "" {
		return options, fmt.Errorf("--path requires a value")
	}
	if len(positionals) > 0 {
		options.prompt = strings.Join(positionals, " ")
	}
	if options.exec && strings.TrimSpace(options.prompt) == "" {
		return options, fmt.Errorf("agent exec requires a prompt")
	}
	return options, nil
}

func agentHelpText() string {
	return `openknowledge agent

Start a human-driven Codex agent in a local knowledge workspace. By default the
agent edits the selected filesystem directly; no branch, worktree, commit, or
pull request is created.

Usage:
  openknowledge agent
  openknowledge agent "<initial prompt>"
  openknowledge agent --path <directory>
  openknowledge agent --isolate ["<initial prompt>"]
  openknowledge agent exec "<prompt>"
  openknowledge agent exec --isolate "<prompt>"

Flags:
  --path       Directory the agent should edit. Defaults to the current directory.
  --model      Codex model override.
  --isolate    Create and retain a dedicated Git branch and worktree at HEAD.

Run openknowledge agent exec --help for non-interactive usage.
`
}

func agentExecHelpText() string {
	return `openknowledge agent exec

Run one non-interactive Codex task. The task edits the selected filesystem
directly unless --isolate is set.

Usage:
  openknowledge agent exec "<prompt>"
  openknowledge agent exec --path <directory> "<prompt>"
  openknowledge agent exec --model <model> "<prompt>"
  openknowledge agent exec --isolate "<prompt>"

Flags:
  --path       Directory the agent should edit. Defaults to the current directory.
  --model      Codex model override.
  --isolate    Create and retain a dedicated Git branch and worktree at HEAD.
`
}
