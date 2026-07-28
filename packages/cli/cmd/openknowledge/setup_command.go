package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	"golang.org/x/term"
)

var setupRuntimeInput io.Reader = os.Stdin
var setupRuntimeInputIsTerminal = func() bool {
	file, ok := setupRuntimeInput.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

type setupCLIOptions struct {
	wiki            string
	source          string
	runtime         string
	model           string
	rules           string
	wikiType        string
	about           string
	depth           int
	agent           bool
	runtimeExplicit bool
	targetExplicit  bool
}

func runSetup(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, setupHelpText())
		return 0
	}
	options, err := parseSetupArgs(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if !options.agent {
		task, _, _, err := agentTask(setupAgentOptions(options, options.wiki))
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 2
		}
		fmt.Print(task)
		return 0
	}
	wikiAbs, err := filepath.Abs(options.wiki)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	repository, err := integration.RepositoryRoot(wikiAbs)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	relWiki, err := filepath.Rel(repository, wikiAbs)
	if err != nil || relWiki == "." || relWiki == ".." || strings.HasPrefix(relWiki, ".."+string(filepath.Separator)) {
		fmt.Fprintln(stderrOutput(), "setup target must be a directory inside its Git repository")
		return 2
	}
	relWiki = filepath.ToSlash(relWiki)

	var executable string
	if options.runtime == "" {
		options.runtime, executable, err = selectSetupRuntime(context.Background())
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
	}
	agentOptions := setupAgentOptions(options, relWiki)
	agentOptions.path = repository
	if executable == "" {
		executable, err = resolveAgentExecutable(context.Background(), options.runtime)
		if err != nil {
			fmt.Fprintf(stderrOutput(), "setup cannot start the %s runtime: %v\n", options.runtime, err)
			fmt.Fprintf(stderrOutput(), "Run \"openknowledge agent doctor --runtime %s\" to diagnose the installation, then install or repair the runtime and rerun setup.\n", options.runtime)
			return 1
		}
	}
	agentOptions.executable = executable
	if code := runAgentWithOptions(agentOptions); code != 0 {
		fmt.Fprintf(stderrOutput(), "setup agent runtime %s exited with status %d; verify its authentication and rerun the same setup command.\n", options.runtime, code)
		return code
	}
	if info, err := os.Stat(wikiAbs); err != nil || !info.IsDir() {
		fmt.Fprintf(stderrOutput(), "setup agent did not create the knowledge base at %s\n", relWiki)
		return 1
	}
	if code := runValidate([]string{wikiAbs}); code != 0 {
		return code
	}
	if code := runIntegrate([]string{wikiAbs}); code != 0 {
		return code
	}
	fmt.Printf("\nReady: %s\n", relWiki)
	return 0
}

type setupRuntimeOption struct {
	name       string
	executable string
}

func selectSetupRuntime(ctx context.Context) (string, string, error) {
	var available []setupRuntimeOption
	for _, runtimeName := range agents.SupportedAgentRuntimes() {
		executable, err := resolveAgentExecutable(ctx, runtimeName)
		if err == nil {
			available = append(available, setupRuntimeOption{name: runtimeName, executable: executable})
		}
	}
	if len(available) == 0 {
		return "", "", fmt.Errorf("setup found no installed agent runtime; install codex, claude, or opencode")
	}
	names := make([]string, 0, len(available))
	for _, option := range available {
		names = append(names, option.name)
	}
	if !setupRuntimeInputIsTerminal() {
		return "", "", fmt.Errorf("setup --agent requires --runtime when input is not interactive; available runtimes: %s", strings.Join(names, ", "))
	}

	fmt.Fprintln(os.Stdout, "Available agent runtimes:")
	for index, option := range available {
		fmt.Fprintf(os.Stdout, "  %d. %s\n", index+1, option.name)
	}
	reader := bufio.NewReader(setupRuntimeInput)
	for {
		fmt.Fprint(os.Stdout, "Select a runtime: ")
		answer, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(answer) == "" {
			return "", "", fmt.Errorf("read setup runtime selection: %w", err)
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if index, err := strconv.Atoi(answer); err == nil && index >= 1 && index <= len(available) {
			selected := available[index-1]
			return selected.name, selected.executable, nil
		}
		for _, option := range available {
			if answer == option.name {
				return option.name, option.executable, nil
			}
		}
		fmt.Fprintf(os.Stdout, "Enter a number from 1 to %d or a runtime name.\n", len(available))
	}
}

func setupAgentOptions(options setupCLIOptions, target string) agentCLIOptions {
	agentOptions := agentCLIOptions{
		runtime: options.runtime,
		model:   options.model,
	}
	if options.source == "" {
		agentOptions.operation = "init"
		agentOptions.rules = options.rules
		agentOptions.setupTarget = target
		return agentOptions
	}
	agentOptions.operation = "from"
	agentOptions.from = fromOptions{
		source:   options.source,
		out:      target,
		wikiType: options.wikiType,
		about:    options.about,
		depth:    options.depth,
	}
	return agentOptions
}

func parseSetupArgs(args []string) (setupCLIOptions, error) {
	options := setupCLIOptions{wiki: "Wiki", wikiType: okf.DefaultFromType}
	var positionals []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--agent":
			options.agent = true
		case argument == "--from" || argument == "--runtime" || argument == "--model" || argument == "--rules" || argument == "--type" || argument == "--about" || argument == "--depth":
			value, next, err := nextFlagValue(args, index, argument)
			if err != nil {
				return options, err
			}
			if err := setSetupOption(&options, argument, value); err != nil {
				return options, err
			}
			index = next
		case strings.HasPrefix(argument, "--from="):
			if err := setSetupOption(&options, "--from", strings.TrimPrefix(argument, "--from=")); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "--runtime="):
			if err := setSetupOption(&options, "--runtime", strings.TrimPrefix(argument, "--runtime=")); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "--model="):
			if err := setSetupOption(&options, "--model", strings.TrimPrefix(argument, "--model=")); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "--rules="):
			if err := setSetupOption(&options, "--rules", strings.TrimPrefix(argument, "--rules=")); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "--type="):
			if err := setSetupOption(&options, "--type", strings.TrimPrefix(argument, "--type=")); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "--about="):
			if err := setSetupOption(&options, "--about", strings.TrimPrefix(argument, "--about=")); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "--depth="):
			if err := setSetupOption(&options, "--depth", strings.TrimPrefix(argument, "--depth=")); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "-"):
			return options, fmt.Errorf("unknown setup option: %s", argument)
		default:
			positionals = append(positionals, argument)
		}
	}
	if len(positionals) > 1 {
		return options, fmt.Errorf("setup accepts at most one knowledge base path")
	}
	if len(positionals) == 1 {
		options.wiki = positionals[0]
		options.targetExplicit = true
	}
	if strings.TrimSpace(options.wiki) == "" {
		return options, fmt.Errorf("setup knowledge base path must not be empty")
	}
	if options.runtime != "" {
		if _, err := agents.HarnessForRuntime(options.runtime); err != nil {
			return options, err
		}
	}
	if !options.agent && options.runtimeExplicit {
		return options, fmt.Errorf("--runtime requires --agent")
	}
	if !options.agent && strings.TrimSpace(options.model) != "" {
		return options, fmt.Errorf("--model requires --agent")
	}
	// The zero-argument path prints the primary project onboarding task for the
	// current directory and Wiki. An explicit target without --from keeps the
	// guided, open-ended setup workflow.
	if options.source == "" &&
		!options.targetExplicit &&
		strings.TrimSpace(options.rules) == "" &&
		options.wikiType == okf.DefaultFromType &&
		options.about == "" &&
		options.depth == 0 {
		options.source = "."
	}
	if options.source == "" {
		if options.wikiType != okf.DefaultFromType || options.about != "" || options.depth != 0 {
			return options, fmt.Errorf("--type, --about, and --depth require --from")
		}
	} else if strings.TrimSpace(options.rules) != "" {
		return options, fmt.Errorf("--rules cannot be combined with --from")
	}
	if _, err := parseRuleIDs(options.rules); err != nil {
		return options, err
	}
	return options, nil
}

func setSetupOption(options *setupCLIOptions, flagName, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s requires a value", flagName)
	}
	switch flagName {
	case "--from":
		options.source = value
	case "--runtime":
		options.runtime = strings.ToLower(value)
		options.runtimeExplicit = true
	case "--model":
		options.model = value
	case "--rules":
		options.rules = value
	case "--type":
		options.wikiType = value
	case "--about":
		options.about = value
	case "--depth":
		depth, err := parseNonNegativeIntFlag("--depth", value)
		if err != nil {
			return err
		}
		options.depth = depth
	}
	return nil
}

func setupHelpText() string {
	return `openknowledge setup

Print portable instructions to create or update an OKF knowledge base.
Use --agent to run the instructions, validate the result, and integrate it.

Usage:
  openknowledge setup
  openknowledge setup --agent
  openknowledge setup [wiki]
  openknowledge setup [wiki] --rules <rules>
  openknowledge setup [wiki] --from <source>
  openknowledge setup --agent --runtime <codex|claude|opencode>

Arguments:
  wiki        Target knowledge-base directory. Defaults to Wiki. Agent mode
              requires the target to be inside the current Git repository.

Core flags:
  --from      Repository, local folder, or website source.
  --agent     Run the setup instructions with an agent. By default, print them.
  --rules     Comma-separated maintenance rules for guided setup. Cannot be
              combined with --from.

Advanced flags:
  --runtime   Agent runtime: codex, claude, or opencode. Requires --agent.
  --model     Harness-specific model override. Requires --agent.
  --type      Source workflow: understanding or custom. Requires --from.
  --about     Custom source-to-wiki goal. Requires --from.
  --depth     Non-negative source traversal hint. Requires --from; 0 lets the
              agent choose the minimum depth.

With no arguments, setup prints instructions that use the current directory as
the source and Wiki as the target. An explicit wiki path without --from prints
the guided workflow for a new or open-ended knowledge base. Use --from only for
another repository, local folder, or website.

Use --agent in the Git repository that should own the knowledge base. If you do
not specify --runtime, setup detects installed runtimes and asks you to select
one. Non-interactive use requires --runtime. Setup then launches the selected
runtime, validates the result, and installs project discovery skills and
observation hooks. Before launch, setup verifies that the runtime executable is
available. Run openknowledge agent doctor --runtime <runtime> to diagnose the
installation. Runtime authentication remains owned by the selected agent CLI.

After setup, the knowledge base is ready. Use search or validate directly when
you need retrieval or an independent check. The viewer, publishing, registry,
runtime, deterministic scaffold, and portable prompt commands are optional
workflows; discover them from the grouped root help.
`
}
