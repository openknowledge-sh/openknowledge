package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type setupCLIOptions struct {
	wiki     string
	source   string
	runtime  string
	model    string
	rules    string
	wikiType string
	about    string
	depth    int
}

func runSetup(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, setupHelpText())
		return 0
	}
	options, err := parseSetupArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	wikiAbs, err := filepath.Abs(options.wiki)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	repository, err := integration.RepositoryRoot(wikiAbs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	relWiki, err := filepath.Rel(repository, wikiAbs)
	if err != nil || relWiki == "." || relWiki == ".." || strings.HasPrefix(relWiki, ".."+string(filepath.Separator)) {
		fmt.Fprintln(os.Stderr, "setup target must be a directory inside its Git repository")
		return 2
	}
	relWiki = filepath.ToSlash(relWiki)

	agentOptions := agentCLIOptions{
		path:    repository,
		runtime: options.runtime,
		model:   options.model,
	}
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
		return code
	}
	if info, err := os.Stat(wikiAbs); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "setup agent did not create the knowledge base at %s\n", relWiki)
		return 1
	}
	if code := runValidate([]string{wikiAbs}); code != 0 {
		return code
	}
	return runIntegrate([]string{wikiAbs})
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
	}
	if strings.TrimSpace(options.wiki) == "" {
		return options, fmt.Errorf("setup knowledge base path must not be empty")
	}
	if _, err := agents.HarnessForRuntime(options.runtime); err != nil {
		return options, err
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

Create or update, validate, and integrate an OKF knowledge base through a
supported agent runtime.

Usage:
  openknowledge setup [wiki]
  openknowledge setup [wiki] --rules <rules>
  openknowledge setup [wiki] --from <source>
  openknowledge setup [wiki] --from <source> --type understanding|custom
  openknowledge setup [wiki] --runtime <codex|claude|opencode>

The default target is Wiki and the default runtime is Codex. Without --from,
the agent runs the guided setup workflow. With --from, it runs the
source-to-wiki workflow. A successful run must leave a valid knowledge base;
setup then installs project discovery skills and observation hooks.

Use openknowledge scaffold for a deterministic scaffold without an agent or Git
integration. Use openknowledge prompt for print-only portable instructions.
`
}
