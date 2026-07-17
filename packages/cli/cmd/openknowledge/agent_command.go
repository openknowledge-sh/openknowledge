package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type agentCLIOptions struct {
	operation    string
	isolate      bool
	path         string
	model        string
	prompt       string
	runtime      string
	runtimeSet   bool
	noSteer      bool
	rules        string
	json         bool
	from         fromOptions
	setupTarget  string
	modeOverride string
}

type agentDoctorEntry struct {
	Runtime    string `json:"runtime"`
	Available  bool   `json:"available"`
	Executable string `json:"executable,omitempty"`
	Error      string `json:"error,omitempty"`
}

var runAgentProcess = func(ctx context.Context, executable string, arguments []string, directory string) error {
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = directory
	command.Env = os.Environ()
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func runAgent(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "integrate":
			return runIntegrate(args[1:])
		}
	}
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
	return runAgentWithOptions(options)
}

func runAgentWithOptions(options agentCLIOptions) int {
	if options.operation == "doctor" {
		return runAgentDoctor(options)
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

	task, mode, interactive, err := agentTask(options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.isolate {
		mode = "isolated"
	}
	if options.modeOverride != "" {
		mode = options.modeOverride
	}
	if !options.noSteer {
		task = agents.SteeredAgentPrompt(task, mode)
	}

	command, err := agents.BuildHarnessCommand(agents.AgentSpec{Runtime: options.runtime, Model: options.model}, interactive)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	executable, err := resolveAgentExecutable(context.Background(), options.runtime)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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

	arguments := append([]string(nil), command.Args...)
	if task != "" {
		if command.PromptMode == agents.PromptStdin && len(arguments) > 0 && arguments[len(arguments)-1] == "-" {
			arguments = arguments[:len(arguments)-1]
		}
		arguments = append(arguments, task)
	}

	ctx := context.Background()
	if !interactive {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, os.Interrupt)
		defer stop()
	} else {
		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)
		defer signal.Stop(interrupts)
	}
	if err := runAgentProcess(ctx, executable, arguments, directory); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return exitError.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "run %s agent: %v\n", options.runtime, err)
		return 1
	}
	return 0
}

func agentTask(options agentCLIOptions) (task string, mode string, interactive bool, err error) {
	switch options.operation {
	case "exec":
		return options.prompt, "local", false, nil
	case "init":
		ruleIDs, err := parseRuleIDs(options.rules)
		if err != nil {
			return "", "", false, err
		}
		prompt, err := okf.SetupPromptWithOptions(okf.SetupPromptOptions{Rules: ruleIDs})
		if err == nil && options.setupTarget != "" {
			prompt += fmt.Sprintf("\nFor this setup, create or update the knowledge base at %s.\n", options.setupTarget)
		}
		return prompt, "init", true, err
	case "from":
		prompt, err := okf.FromPrompt(okf.FromPromptOptions{
			Source: options.from.source,
			Out:    options.from.out,
			Type:   options.from.wikiType,
			About:  options.from.about,
			Depth:  options.from.depth,
		})
		return prompt, "from", true, err
	default:
		return options.prompt, "local", true, nil
	}
}

func parseAgentArgs(args []string) (agentCLIOptions, error) {
	options := agentCLIOptions{path: ".", runtime: agents.RuntimeCodex}
	if len(args) > 0 {
		switch args[0] {
		case "exec", "doctor":
			options.operation = args[0]
			args = args[1:]
		case "init", "from":
			return options, fmt.Errorf("agent %s was removed; use openknowledge setup", args[0])
		}
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
		case argument == "--no-steer":
			options.noSteer = true
		case argument == "--json" && options.operation == "doctor":
			options.json = true
		case argument == "--path" || argument == "--model" || argument == "--runtime":
			value, next, err := nextFlagValue(args, index, argument)
			if err != nil {
				return options, err
			}
			switch argument {
			case "--path":
				options.path = value
			case "--model":
				options.model = value
			case "--runtime":
				options.runtime = strings.ToLower(value)
				options.runtimeSet = true
			}
			index = next
		case strings.HasPrefix(argument, "--path="):
			options.path = strings.TrimPrefix(argument, "--path=")
		case strings.HasPrefix(argument, "--model="):
			options.model = strings.TrimPrefix(argument, "--model=")
		case strings.HasPrefix(argument, "--runtime="):
			options.runtime = strings.ToLower(strings.TrimPrefix(argument, "--runtime="))
			options.runtimeSet = true
		case strings.HasPrefix(argument, "-"):
			return options, fmt.Errorf("unknown agent option: %s", argument)
		default:
			positionals = append(positionals, argument)
		}
	}
	if strings.TrimSpace(options.path) == "" {
		return options, fmt.Errorf("--path requires a value")
	}
	if _, err := agents.HarnessForRuntime(options.runtime); err != nil {
		return options, err
	}
	if len(positionals) > 0 {
		options.prompt = strings.Join(positionals, " ")
	}
	if options.operation == "exec" && strings.TrimSpace(options.prompt) == "" {
		return options, fmt.Errorf("agent exec requires a prompt")
	}
	if options.operation == "doctor" && len(positionals) > 0 {
		return options, fmt.Errorf("agent doctor accepts no positional arguments")
	}
	if options.operation == "doctor" && (options.isolate || options.noSteer || options.model != "" || options.path != ".") {
		return options, fmt.Errorf("agent doctor accepts only --runtime and --json")
	}
	return options, nil
}

func runAgentDoctor(options agentCLIOptions) int {
	runtimes := agents.SupportedAgentRuntimes()
	if options.runtimeSet {
		runtimes = []string{options.runtime}
	}
	entries := make([]agentDoctorEntry, 0, len(runtimes))
	available := 0
	for _, runtime := range runtimes {
		executable, err := resolveAgentExecutable(context.Background(), runtime)
		entry := agentDoctorEntry{Runtime: runtime, Available: err == nil, Executable: executable}
		if err != nil {
			entry.Error = err.Error()
		} else {
			available++
		}
		entries = append(entries, entry)
	}
	if options.json {
		encoded, _ := json.MarshalIndent(map[string]any{"schemaVersion": okf.MachineSchemaVersion, "runtimes": entries}, "", "  ")
		fmt.Fprintln(os.Stdout, string(encoded))
	} else {
		for _, entry := range entries {
			if entry.Available {
				fmt.Fprintf(os.Stdout, "%s\tavailable\t%s\n", entry.Runtime, entry.Executable)
			} else {
				fmt.Fprintf(os.Stdout, "%s\tunavailable\t%s\n", entry.Runtime, entry.Error)
			}
		}
	}
	if options.runtimeSet && available == 0 {
		return 1
	}
	if available == 0 {
		return 1
	}
	return 0
}

func agentHelpText() string {
	return `openknowledge agent

Start a steered Open Knowledge agent using Codex, Claude Code, or OpenCode.
Local sessions edit the selected filesystem directly unless --isolate is set.

Usage:
  openknowledge agent ["<initial prompt>"]
  openknowledge agent --runtime <codex|claude|opencode>
  openknowledge agent exec "<prompt>"
  openknowledge agent integrate <wiki>
  openknowledge agent integrate --global
  openknowledge agent doctor [--runtime <runtime>] [--json]

Flags:
  --runtime    Agent harness. Defaults to codex.
  --path       Directory the agent should edit. Defaults to the current directory.
  --model      Harness-specific model override.
  --isolate    Create and retain a dedicated Git branch and worktree at HEAD.
  --no-steer   Pass only the user or generated workflow prompt.

Executable overrides:
  OPENKNOWLEDGE_CODEX, OPENKNOWLEDGE_CLAUDE, OPENKNOWLEDGE_OPENCODE

Integration installs native harness discovery and project observation. Use
openknowledge insights to capture and resolve knowledge-maintenance insights.

Run openknowledge agent exec --help for non-interactive usage.
`
}

func agentExecHelpText() string {
	return `openknowledge agent exec

Run one non-interactive Open Knowledge agent task. The task edits the selected
filesystem directly unless --isolate is set.

Usage:
  openknowledge agent exec "<prompt>"
  openknowledge agent exec --runtime claude "<prompt>"
  openknowledge agent exec --runtime opencode --model <provider/model> "<prompt>"
  openknowledge agent exec --isolate "<prompt>"

Flags:
  --runtime    codex, claude, or opencode. Defaults to codex.
  --path       Directory the agent should edit. Defaults to the current directory.
  --model      Harness-specific model override.
  --isolate    Create and retain a dedicated Git branch and worktree at HEAD.
  --no-steer   Do not prepend the Open Knowledge agent contract.
`
}
