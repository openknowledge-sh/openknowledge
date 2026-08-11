package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	"golang.org/x/term"
)

var setupInput io.Reader = os.Stdin
var setupInputIsTerminal = func() bool {
	file, ok := setupInput.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

type setupCLIOptions struct {
	wiki        string
	source      string
	agent       string
	model       string
	rules       string
	about       string
	depth       int
	prompt      bool
	interactive bool
}

type setupActivationPlan struct {
	skill     string
	harnesses []string
	observe   bool
}

type setupWizardPlan struct {
	options    setupCLIOptions
	action     string
	agent      string
	activation setupActivationPlan
}

func runSetup(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "skill":
			return runSetupSkill(args[1:])
		case "complete":
			return runSetupComplete(args[1:])
		case "status":
			return runSetupStatus(args[1:])
		case "repair":
			return runSetupRepair(args[1:])
		case "observe":
			return runSetupObserve(args[1:])
		}
	}
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, setupHelpText())
		return 0
	}
	options, err := parseSetupArgs(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}

	if options.interactive || (!options.prompt && options.agent == "" && setupInputIsTerminal()) {
		plan, err := runSetupWizard(options)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		options = plan.options
		task, err := buildSetupTask(options, &plan.activation)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 2
		}
		if plan.action == "print" {
			fmt.Print(task)
			return 0
		}
		return runSetupAgent(options, plan.agent, task)
	}

	task, err := buildSetupTask(options, nil)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if options.agent == "" {
		fmt.Print(task)
		return 0
	}
	return runSetupAgent(options, options.agent, task)
}

func buildSetupTask(options setupCLIOptions, activation *setupActivationPlan) (string, error) {
	ruleIDs, err := parseRuleIDs(options.rules)
	if err != nil {
		return "", err
	}
	var task string
	if options.source == "" {
		task, err = okf.SetupPromptWithOptions(okf.SetupPromptOptions{Rules: ruleIDs})
	} else {
		task, err = okf.FromPrompt(okf.FromPromptOptions{
			Source: options.source,
			Out:    options.wiki,
			About:  options.about,
			Depth:  options.depth,
			Rules:  ruleIDs,
		})
	}
	if err != nil {
		return "", err
	}
	if options.source == "" {
		task += fmt.Sprintf("\nFor this setup, create or update the knowledge base at %s.\n", options.wiki)
	}
	if activation != nil {
		task += renderSetupActivationInstructions(options.wiki, *activation)
	}
	return task, nil
}

func renderSetupActivationInstructions(wiki string, plan setupActivationPlan) string {
	var command strings.Builder
	fmt.Fprintf(&command, "okn setup complete %q --skill %s", wiki, plan.skill)
	for _, harness := range plan.harnesses {
		fmt.Fprintf(&command, " --harness %s", harness)
	}
	if plan.observe {
		command.WriteString(" --observe on")
	} else {
		command.WriteString(" --observe off")
	}
	return fmt.Sprintf(`

The user already selected this activation plan:
- Skill scope: %s
- Harnesses: %s
- Observation: %s

Do not ask for these choices again. After the bundle is complete and
validation passes, run this exact finalizer:

  %s
`, plan.skill, setupHarnessLabel(plan.harnesses), enabledLabel(plan.observe), command.String())
}

func setupHarnessLabel(harnesses []string) string {
	if len(harnesses) == 0 {
		return "none"
	}
	return strings.Join(harnesses, ", ")
}

func runSetupAgent(options setupCLIOptions, runtime string, task string) int {
	wikiAbs, err := filepath.Abs(options.wiki)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	projectRoot, err := integration.ProjectRoot(wikiAbs)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	executable, err := resolveAgentExecutable(context.Background(), runtime)
	if err != nil {
		fmt.Fprintf(stderrOutput(), "setup cannot start the %s runtime: %v\n", runtime, err)
		fmt.Fprintf(stderrOutput(), "Run \"openknowledge agent doctor --runtime %s\" to diagnose the installation, then install or repair the runtime and rerun setup.\n", runtime)
		return 1
	}
	code := runAgentWithOptions(agentCLIOptions{
		path:         projectRoot,
		executable:   executable,
		model:        options.model,
		prompt:       task,
		runtime:      runtime,
		modeOverride: "init",
	})
	if code != 0 {
		fmt.Fprintf(stderrOutput(), "setup agent runtime %s exited with status %d; verify its authentication and rerun the same setup command.\n", runtime, code)
	}
	return code
}

func parseSetupArgs(args []string) (setupCLIOptions, error) {
	options := setupCLIOptions{wiki: "Wiki"}
	var positionals []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--prompt":
			options.prompt = true
		case argument == "--interactive":
			options.interactive = true
		case argument == "--from" || argument == "--agent" || argument == "--model" || argument == "--rules" || argument == "--about" || argument == "--depth":
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
		case strings.HasPrefix(argument, "--agent="):
			if err := setSetupOption(&options, "--agent", strings.TrimPrefix(argument, "--agent=")); err != nil {
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
	if options.prompt && options.interactive {
		return options, fmt.Errorf("--prompt and --interactive cannot be combined")
	}
	if options.agent != "" && (options.prompt || options.interactive) {
		return options, fmt.Errorf("--agent cannot be combined with --prompt or --interactive")
	}
	if options.agent != "" {
		if _, err := agents.HarnessForRuntime(options.agent); err != nil {
			return options, err
		}
	} else if options.model != "" {
		return options, fmt.Errorf("--model requires --agent")
	}
	if options.source == "" {
		if options.about != "" || options.depth != 0 {
			return options, fmt.Errorf("--about and --depth require --from")
		}
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
	case "--agent":
		options.agent = strings.ToLower(value)
	case "--model":
		options.model = value
	case "--rules":
		options.rules = value
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

func runSetupWizard(options setupCLIOptions) (setupWizardPlan, error) {
	reader := bufio.NewReader(setupInput)
	plan := setupWizardPlan{options: options, action: "print"}

	if options.source == "" {
		choice, err := setupChoice(reader, "What do you want to set up?", []string{
			"A knowledge base for this project",
			"A knowledge base generated from another source",
			"An existing Open Knowledge bundle",
		}, 0)
		if err != nil {
			return plan, err
		}
		switch choice {
		case 1:
			source, err := setupLine(reader, "Source path or URL", "")
			if err != nil {
				return plan, err
			}
			plan.options.source = source
			about, err := setupLine(reader, "What should this knowledge base help with?", "let the agent infer it")
			if err != nil {
				return plan, err
			}
			if about != "let the agent infer it" {
				plan.options.about = about
			}
		case 2:
			wiki, err := setupLine(reader, "Knowledge base path", options.wiki)
			if err != nil {
				return plan, err
			}
			plan.options.wiki = wiki
		}
	}
	if strings.TrimSpace(plan.options.rules) == "" {
		rules, err := setupMaintenanceRules(reader)
		if err != nil {
			return plan, err
		}
		plan.options.rules = strings.Join(rules, ",")
	}

	available := detectSetupRuntimes(context.Background())
	actionLabels := make([]string, 0, len(available)+1)
	for _, runtime := range available {
		actionLabels = append(actionLabels, "Launch "+displayRuntime(runtime))
	}
	actionLabels = append(actionLabels, "Print a task for an existing agent")
	action, err := setupChoice(reader, "How should setup run?", actionLabels, 0)
	if err != nil {
		return plan, err
	}
	if action < len(available) {
		plan.action = "agent"
		plan.agent = available[action]
	}

	skillChoice, err := setupChoice(reader, "Install Open Knowledge instructions for agents?", []string{
		"Personal — available across all projects",
		"Project — shared in this repository",
		"Both — personal and project-specific guidance",
		"None (not recommended) — CLI only",
	}, 0)
	if err != nil {
		return plan, err
	}
	plan.activation.skill = []string{"global", "project", "both", "none"}[skillChoice]
	observeChoice, err := setupChoice(reader, "Capture possible knowledge gaps after agent sessions?", []string{"Not now", "Enable"}, 0)
	if err != nil {
		return plan, err
	}
	plan.activation.observe = observeChoice == 1
	if plan.activation.skill != "none" || plan.activation.observe {
		defaults := available
		if plan.agent != "" {
			defaults = []string{plan.agent}
		}
		harnesses, err := setupHarnesses(reader, available, defaults)
		if err != nil {
			return plan, err
		}
		plan.activation.harnesses = harnesses
	}

	fmt.Fprintln(os.Stdout, "\nOpen Knowledge setup plan")
	fmt.Fprintf(os.Stdout, "  Knowledge base: %s\n", plan.options.wiki)
	if plan.options.source != "" {
		fmt.Fprintf(os.Stdout, "  Source:         %s\n", plan.options.source)
	}
	if plan.action == "agent" {
		fmt.Fprintf(os.Stdout, "  Setup agent:    %s\n", plan.agent)
	} else {
		fmt.Fprintln(os.Stdout, "  Setup agent:    existing agent")
	}
	fmt.Fprintf(os.Stdout, "  Skills:         %s\n", plan.activation.skill)
	fmt.Fprintf(os.Stdout, "  Harnesses:      %s\n", setupHarnessLabel(plan.activation.harnesses))
	fmt.Fprintf(os.Stdout, "  Observation:    %s\n", enabledLabel(plan.activation.observe))
	confirmed, err := setupConfirm(reader, "Continue?", true)
	if err != nil {
		return plan, err
	}
	if !confirmed {
		return plan, fmt.Errorf("setup cancelled")
	}
	return plan, nil
}

func detectSetupRuntimes(ctx context.Context) []string {
	var available []string
	for _, runtime := range agents.SupportedAgentRuntimes() {
		if _, err := resolveAgentExecutable(ctx, runtime); err == nil {
			available = append(available, runtime)
		}
	}
	sort.Strings(available)
	return available
}

func displayRuntime(runtime string) string {
	switch runtime {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	case "opencode":
		return "OpenCode"
	default:
		return runtime
	}
}

func setupChoice(reader *bufio.Reader, question string, choices []string, defaultIndex int) (int, error) {
	if len(choices) == 0 {
		return 0, fmt.Errorf("%s has no available choices", question)
	}
	fmt.Fprintf(os.Stdout, "\n◆ %s\n", question)
	for index, choice := range choices {
		marker := "○"
		if index == defaultIndex {
			marker = "●"
		}
		fmt.Fprintf(os.Stdout, "  %s %d. %s\n", marker, index+1, choice)
	}
	for {
		fmt.Fprintf(os.Stdout, "Select [%d]: ", defaultIndex+1)
		answer, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(answer) == "" {
			return 0, fmt.Errorf("read setup choice: %w", err)
		}
		answer = strings.TrimSpace(answer)
		if answer == "" {
			return defaultIndex, nil
		}
		selected, err := strconv.Atoi(answer)
		if err == nil && selected >= 1 && selected <= len(choices) {
			return selected - 1, nil
		}
		fmt.Fprintf(os.Stdout, "Enter a number from 1 to %d.\n", len(choices))
	}
}

func setupLine(reader *bufio.Reader, question string, defaultValue string) (string, error) {
	if defaultValue == "" {
		fmt.Fprintf(os.Stdout, "%s: ", question)
	} else {
		fmt.Fprintf(os.Stdout, "%s [%s]: ", question, defaultValue)
	}
	answer, err := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if err != nil && answer == "" {
		return "", fmt.Errorf("read setup answer: %w", err)
	}
	if answer == "" {
		answer = defaultValue
	}
	if strings.TrimSpace(answer) == "" {
		return "", fmt.Errorf("%s requires a value", question)
	}
	return answer, nil
}

func setupHarnesses(reader *bufio.Reader, available []string, defaults []string) ([]string, error) {
	if len(available) == 0 {
		return nil, fmt.Errorf("setup found no installed agent harness; choose CLI only or install codex, claude, or opencode")
	}
	fmt.Fprintln(os.Stdout, "\n◆ Install for which agent environments?")
	for index, runtime := range available {
		fmt.Fprintf(os.Stdout, "  %d. %s\n", index+1, displayRuntime(runtime))
	}
	fmt.Fprintf(os.Stdout, "Select comma-separated numbers [%s]: ", setupDefaultHarnessIndexes(available, defaults))
	answer, err := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if err != nil && answer == "" {
		return nil, fmt.Errorf("read setup harnesses: %w", err)
	}
	if answer == "" {
		return append([]string(nil), defaults...), nil
	}
	seen := map[string]bool{}
	var selected []string
	for _, raw := range strings.Split(answer, ",") {
		index, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || index < 1 || index > len(available) {
			return nil, fmt.Errorf("invalid harness selection %q", raw)
		}
		runtime := available[index-1]
		if !seen[runtime] {
			seen[runtime] = true
			selected = append(selected, runtime)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("select at least one harness")
	}
	return selected, nil
}

func setupMaintenanceRules(reader *bufio.Reader) ([]string, error) {
	rules := []struct {
		id    string
		label string
	}{
		{id: "project", label: "Project changes"},
		{id: "writing", label: "Clear, concise writing"},
		{id: "iso-plain-language", label: "ISO 24495-1 plain-language principles"},
		{id: "docs", label: "Documentation"},
		{id: "decisions", label: "Decisions"},
		{id: "changelog", label: "Changelog"},
		{id: "research", label: "Research"},
		{id: "bugs", label: "Bugs"},
		{id: "schemas", label: "Schemas"},
		{id: "summary", label: "Summaries"},
		{id: "agents", label: "Agent guidance"},
	}
	fmt.Fprintln(os.Stdout, "\n◆ Which maintenance behaviors should future agents follow?")
	for index, rule := range rules {
		fmt.Fprintf(os.Stdout, "  %d. %s\n", index+1, rule.label)
	}
	fmt.Fprint(os.Stdout, "Select comma-separated numbers [1,2]: ")
	answer, err := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if err != nil && answer == "" {
		return nil, fmt.Errorf("read setup maintenance rules: %w", err)
	}
	if answer == "" {
		return []string{"project", "writing"}, nil
	}
	seen := map[string]bool{}
	var selected []string
	for _, raw := range strings.Split(answer, ",") {
		index, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || index < 1 || index > len(rules) {
			return nil, fmt.Errorf("invalid maintenance rule selection %q", raw)
		}
		id := rules[index-1].id
		if !seen[id] {
			seen[id] = true
			selected = append(selected, id)
		}
	}
	return selected, nil
}

func setupDefaultHarnessIndexes(available []string, defaults []string) string {
	selected := map[string]bool{}
	for _, runtime := range defaults {
		selected[runtime] = true
	}
	var indexes []string
	for index, runtime := range available {
		if selected[runtime] {
			indexes = append(indexes, strconv.Itoa(index+1))
		}
	}
	if len(indexes) == 0 {
		return "1"
	}
	return strings.Join(indexes, ",")
}

func setupConfirm(reader *bufio.Reader, question string, defaultValue bool) (bool, error) {
	label := "y/N"
	if defaultValue {
		label = "Y/n"
	}
	fmt.Fprintf(os.Stdout, "%s (%s) ", question, label)
	answer, err := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if err != nil && answer == "" {
		return false, fmt.Errorf("read setup confirmation: %w", err)
	}
	if answer == "" {
		return defaultValue, nil
	}
	return answer == "y" || answer == "yes", nil
}

func setupHelpText() string {
	return `openknowledge setup

Set up an Open Knowledge knowledge base and its agent instructions.

Usage:
  openknowledge setup [wiki]
  openknowledge setup [wiki] --prompt
  openknowledge setup [wiki] --interactive
  openknowledge setup [wiki] --agent <codex|claude|opencode>
  openknowledge setup [wiki] --from <source> [--about <goal>] [--depth <n>]
  openknowledge setup skill [--scope <global|project|both>] [--project <target>] [--harness <name>]
  openknowledge setup complete <wiki> --skill <scope> [--harness <name>] [--observe on|off]
  openknowledge setup status [wiki]
  openknowledge setup repair [wiki]
  openknowledge setup observe <on|off> [repository]

With terminal input, setup starts an interactive wizard. Without terminal
input, setup prints a complete task for an agent. Use --prompt or --interactive
to select the mode explicitly. Use --agent to start one installed agent.

The source workflow accepts --from, optional --about intent, and optional
--depth. The agent inspects the source and asks for missing intent. Setup does
not use predefined knowledge-base types.

Flags:
  --prompt       Print the complete agent task without changing files.
  --interactive  Run the terminal wizard.
  --agent        Start codex, claude, or opencode with the setup task.
  --model        Harness-specific model override. Requires --agent.
  --from         Repository, folder, or website source.
  --about        Optional source-to-wiki goal. Requires --from.
  --depth        Non-negative traversal hint. Requires --from.
  --rules        Comma-separated maintenance rules. Works with ordinary and
                 --from setup. Defaults to project,writing.
`
}
