package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type setupCLIOptions struct {
	wiki           string
	source         string
	runtime        string
	model          string
	rules          string
	wikiType       string
	about          string
	depth          int
	targetExplicit bool
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

	agentOptions := agentCLIOptions{
		path:    repository,
		runtime: options.runtime,
		model:   options.model,
	}
	executable, err := resolveAgentExecutable(context.Background(), options.runtime)
	if err != nil {
		fmt.Fprintf(stderrOutput(), "setup cannot start the %s runtime: %v\n", options.runtime, err)
		fmt.Fprintf(stderrOutput(), "Run \"openknowledge agent doctor --runtime %s\" to diagnose the installation, then install or repair the runtime and rerun setup.\n", options.runtime)
		return 1
	}
	agentOptions.executable = executable
	if options.source == "" {
		agentOptions.operation = "init"
		agentOptions.rules = options.rules
		agentOptions.setupTarget = relWiki
	} else {
		agentOptions.operation = "from"
		agentOptions.from = fromOptions{
			source:   options.source,
			out:      relWiki,
			wikiType: options.wikiType,
			about:    options.about,
			depth:    options.depth,
		}
	}
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

func parseSetupArgs(args []string) (setupCLIOptions, error) {
	options := setupCLIOptions{wiki: "Wiki", runtime: agents.RuntimeCodex, wikiType: okf.DefaultFromType}
	var positionals []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
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
	if _, err := agents.HarnessForRuntime(options.runtime); err != nil {
		return options, err
	}
	// The zero-argument path is the primary project onboarding flow: inspect the
	// current repository and write its knowledge base to Wiki. An explicit
	// target without --from keeps the guided, open-ended setup workflow.
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

Launch a supported agent runtime to create or update, validate, and integrate
an OKF knowledge base.

Usage:
  openknowledge setup
  openknowledge setup --runtime <codex|claude|opencode>
  openknowledge setup [wiki]
  openknowledge setup [wiki] --rules <rules>
  openknowledge setup [wiki] --from <source>

Arguments:
  wiki        Target knowledge-base directory. Defaults to Wiki and must be
              inside the current Git repository.

Core flags:
  --from      Repository, local folder, or website source.
  --runtime   Agent runtime: codex, claude, or opencode. Defaults to codex.
  --rules     Comma-separated maintenance rules for guided setup. Cannot be
              combined with --from.

Advanced flags:
  --model     Harness-specific model override.
  --type      Source workflow: understanding or custom. Requires --from.
  --about     Custom source-to-wiki goal. Requires --from.
  --depth     Non-negative source traversal hint. Requires --from; 0 lets the
              agent choose the minimum depth.

Run setup directly from a terminal in the Git repository that should own the
knowledge base. With no arguments, setup inspects the current repository,
writes its knowledge base to Wiki, and uses Codex. An explicit wiki path
without --from starts the guided workflow for a new or open-ended knowledge
base. Use --from only for another repository, local folder, or website. A
successful run must leave a valid knowledge base; setup then installs project
discovery skills and observation hooks.

Before launching the agent, setup verifies that the selected runtime executable
is available. Run openknowledge agent doctor --runtime <runtime> to diagnose the
installation. Runtime authentication remains owned by the selected agent CLI.

After setup, the knowledge base is ready. Use search or validate directly when
you need retrieval or an independent check. The viewer, publishing, registry,
runtime, deterministic scaffold, and portable prompt commands are optional
workflows; discover them from the grouped root help.
`
}
