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
	mode        string
	includes    []string
	excludes    []string
	useCase     string
	agent       string
	model       string
	rules       string
	about       string
	depth       int
	prompt      bool
	interactive bool
	plan        bool
}

const (
	setupUseCaseBase    = "base"
	setupUseCaseTrusted = "trusted"
	setupUseCaseCustom  = "custom"
)

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
		case "ci":
			return runSetupCI(args[1:])
		case "github":
			return runSetupGitHub(args[1:])
		case "runtime":
			return runSetupRuntime(args[1:])
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
	if setupExistingBundle(options.wiki) {
		return printExistingSetup(options.wiki)
	}
	if options.mode != "" {
		return runSetupImport(options)
	}

	if options.interactive || (!options.prompt && options.agent == "" && setupInputIsTerminal()) {
		return runSetupInteractive(options)
	}

	task, err := buildSetupTask(options)
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

func buildSetupTask(options setupCLIOptions) (string, error) {
	useCase, err := normalizeSetupUseCase(options.useCase)
	if err != nil {
		return "", err
	}
	ruleIDs, err := parseRuleIDs(options.rules)
	if err != nil {
		return "", err
	}
	if len(ruleIDs) == 0 {
		ruleIDs = defaultSetupRules(useCase)
	}
	var task string
	if options.source == "" {
		task, err = okf.SetupPromptWithOptions(okf.SetupPromptOptions{Rules: ruleIDs, UseCase: useCase})
	} else {
		task, err = okf.FromPrompt(okf.FromPromptOptions{
			Source:  options.source,
			Out:     options.wiki,
			About:   options.about,
			Depth:   options.depth,
			Rules:   ruleIDs,
			UseCase: useCase,
		})
	}
	if err != nil {
		return "", err
	}
	if options.source == "" {
		task += fmt.Sprintf("\nFor this setup, create or update the knowledge base at %s.\n", options.wiki)
	}
	return task, nil
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
		case argument == "--plan":
			options.plan = true
		case argument == "--include" || argument == "--exclude":
			value, next, err := nextFlagValue(args, index, argument)
			if err != nil {
				return options, err
			}
			if argument == "--include" {
				options.includes = append(options.includes, value)
			} else {
				options.excludes = append(options.excludes, value)
			}
			index = next
		case argument == "--from" || argument == "--mode" || argument == "--use-case" || argument == "--agent" || argument == "--model" || argument == "--rules" || argument == "--about" || argument == "--depth":
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
		case strings.HasPrefix(argument, "--mode="):
			if err := setSetupOption(&options, "--mode", strings.TrimPrefix(argument, "--mode=")); err != nil {
				return options, err
			}
		case strings.HasPrefix(argument, "--include="):
			options.includes = append(options.includes, strings.TrimPrefix(argument, "--include="))
		case strings.HasPrefix(argument, "--exclude="):
			options.excludes = append(options.excludes, strings.TrimPrefix(argument, "--exclude="))
		case strings.HasPrefix(argument, "--use-case="):
			if err := setSetupOption(&options, "--use-case", strings.TrimPrefix(argument, "--use-case=")); err != nil {
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
	if options.mode != "" && (options.prompt || options.interactive) {
		return options, fmt.Errorf("--mode cannot be combined with --prompt or --interactive")
	}
	if options.mode == "" && (len(options.includes) > 0 || len(options.excludes) > 0 || options.plan) {
		return options, fmt.Errorf("--include, --exclude, and --plan require --mode")
	}
	if options.mode != "" && options.source == "" {
		options.source = "."
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
	if _, err := normalizeSetupUseCase(options.useCase); err != nil {
		return options, err
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
	case "--mode":
		mode := strings.ToLower(strings.TrimSpace(value))
		if mode != string(okf.SetupImportCopy) && mode != string(okf.SetupImportInPlace) {
			return fmt.Errorf("--mode requires copy or in-place")
		}
		options.mode = mode
	case "--use-case":
		useCase, err := normalizeSetupUseCase(value)
		if err != nil {
			return err
		}
		options.useCase = useCase
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

func runSetupImport(options setupCLIOptions) int {
	plan, err := setupImportPlanForOptions(options)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	printSetupImportPlan(plan)
	if options.plan {
		return 0
	}
	return applySetupImportPlan(plan)
}

func setupImportPlanForOptions(options setupCLIOptions) (okf.SetupImportPlan, error) {
	rules, err := parseRuleIDs(options.rules)
	if err != nil {
		return okf.SetupImportPlan{}, err
	}
	if len(rules) == 0 {
		rules = defaultSetupRules(options.useCase)
	}
	return okf.BuildSetupImportPlan(okf.SetupImportOptions{
		Mode: okf.SetupImportMode(options.mode), Source: options.source, Target: options.wiki,
		SpecVersion: "latest", Rules: rules, Include: options.includes, Exclude: options.excludes,
	})
}

func applySetupImportPlan(plan okf.SetupImportPlan) int {
	result, err := okf.ApplySetupImportPlan(plan)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "\nKnowledge base created at %s.\n\n", result.Root)
	fmt.Fprintf(os.Stdout, "%-16s%d\n", "Documents", result.Documents)
	fmt.Fprintf(os.Stdout, "%-16s%s\n", "Validation", "READY")
	fmt.Fprintf(os.Stdout, "%-16s%s (%d hits)\n", "Search index", "READY", result.SearchHits)
	fmt.Fprintf(os.Stdout, "%-16s%s\n", "Publication", "DISABLED")
	fmt.Fprintf(os.Stdout, "\nOptional enrichment: okn review %q --scope full\n", result.Root)
	fmt.Fprintln(os.Stdout, "Use it to classify documents, find duplicates or conflicts, improve navigation, and identify missing documentation.")
	return 0
}

func printSetupImportPlan(plan okf.SetupImportPlan) {
	fmt.Fprintln(os.Stdout, "Setup plan")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%-12s%s\n", "Mode:", plan.Mode)
	fmt.Fprintf(os.Stdout, "%-12s%s\n", "Source:", plan.Source)
	fmt.Fprintf(os.Stdout, "%-12s%s\n", "Target:", plan.Target)
	fmt.Fprintf(os.Stdout, "%-12s%d\n", "Documents:", plan.Summary.Documents)
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Changes:")
	fmt.Fprintf(os.Stdout, "  %3d create\n", plan.Summary.Create)
	fmt.Fprintf(os.Stdout, "  %3d update\n", plan.Summary.Update)
	fmt.Fprintf(os.Stdout, "  %3d preserve existing frontmatter\n", plan.Summary.Preserve)
	fmt.Fprintf(os.Stdout, "  %3d add minimal frontmatter\n", plan.Summary.AddFrontmatter)
	fmt.Fprintf(os.Stdout, "  %3d complete frontmatter\n", plan.Summary.CompleteFrontmatter)
	fmt.Fprintf(os.Stdout, "  %3d move\n", plan.Summary.Move)
	fmt.Fprintf(os.Stdout, "  %3d delete\n", plan.Summary.Delete)
}

func runSetupInteractive(options setupCLIOptions) int {
	reader := bufio.NewReader(setupInput)
	if setupExistingBundle(options.wiki) {
		return printExistingSetup(options.wiki)
	}

	source := options.source
	if strings.TrimSpace(source) == "" {
		source = "."
	}
	excludes := append([]string(nil), options.excludes...)
	if rel, err := filepath.Rel(source, options.wiki); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		excludes = append(excludes, filepath.ToSlash(rel)+"/**")
	}
	discovery, err := okf.DiscoverMarkdown(source, okf.MarkdownDiscoveryOptions{Include: options.includes, Exclude: excludes})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if len(discovery.Documents) == 0 {
		return runSetupWithoutMarkdown(reader, options, source)
	}

	selected, err := setupSelectKnowledgeSources(reader, discovery)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	choice, err := setupChoice(reader, "How should Open Knowledge manage these files?", []string{
		"Create a managed copy",
		"Adopt a directory in place",
	}, 0)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	options.source = source
	options.includes = selected
	options.excludes = excludes
	if choice == 0 {
		options.mode = string(okf.SetupImportCopy)
		target, err := setupLine(reader, "Knowledge base path", options.wiki)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		options.wiki = target
	} else {
		defaultDirectory := setupDefaultAdoptDirectory(discovery.Root, selected)
		directory, err := setupLine(reader, "Directory to adopt", defaultDirectory)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		absolute, err := filepath.Abs(filepath.Join(discovery.Root, filepath.FromSlash(directory)))
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		info, err := os.Stat(absolute)
		if err != nil || !info.IsDir() {
			fmt.Fprintf(stderrOutput(), "in-place setup requires one existing directory: %s\n", directory)
			return 1
		}
		options.mode = string(okf.SetupImportInPlace)
		options.source = absolute
		options.wiki = absolute
		options.includes = nil
		options.excludes = nil
	}

	plan, err := setupImportPlanForOptions(options)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	printSetupImportPlan(plan)
	confirmed, err := setupConfirm(reader, "Apply this setup plan?", true)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if !confirmed {
		fmt.Fprintln(os.Stdout, "Setup cancelled. No files changed.")
		return 0
	}
	return applySetupImportPlan(plan)
}

func printExistingSetup(path string) int {
	fmt.Fprintf(os.Stdout, "Open Knowledge is already set up at %s.\n\n", path)
	fmt.Fprintln(os.Stdout, "Check it:")
	fmt.Fprintf(os.Stdout, "  okn check %q\n\n", path)
	fmt.Fprintln(os.Stdout, "Review its content:")
	fmt.Fprintf(os.Stdout, "  okn review %q\n\n", path)
	fmt.Fprintln(os.Stdout, "Upgrade its format:")
	fmt.Fprintf(os.Stdout, "  okn upgrade %q\n", path)
	return 0
}

func setupExistingBundle(path string) bool {
	if _, err := os.Stat(filepath.Join(path, okf.ValidationConfigFile)); err == nil {
		return true
	}
	content, err := os.ReadFile(filepath.Join(path, "index.md"))
	if err != nil {
		return false
	}
	document, err := okf.ParseFrontmatterDocument(content)
	return err == nil && strings.TrimSpace(document.Values["okf_version"]) != ""
}

type setupSourceCandidate struct {
	path  string
	count int
}

func setupSelectKnowledgeSources(reader *bufio.Reader, discovery okf.MarkdownDiscovery) ([]string, error) {
	candidates := setupSourceCandidates(discovery.Documents)
	fmt.Fprintln(os.Stdout, "\n◆ Select knowledge sources")
	for index, candidate := range candidates {
		suffix := ""
		if candidate.count > 1 {
			suffix = fmt.Sprintf("  (%d Markdown files)", candidate.count)
		}
		fmt.Fprintf(os.Stdout, "  %d. %s%s\n", index+1, candidate.path, suffix)
	}
	defaults := make([]string, len(candidates))
	for index := range candidates {
		defaults[index] = strconv.Itoa(index + 1)
	}
	fmt.Fprintf(os.Stdout, "Select comma-separated numbers [%s]: ", strings.Join(defaults, ","))
	answer, err := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if err != nil && answer == "" {
		return nil, fmt.Errorf("read knowledge source selection: %w", err)
	}
	if answer == "" {
		answer = strings.Join(defaults, ",")
	}
	seen := map[string]bool{}
	var selected []string
	for _, raw := range strings.Split(answer, ",") {
		index, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || index < 1 || index > len(candidates) {
			return nil, fmt.Errorf("invalid knowledge source selection %q", raw)
		}
		path := candidates[index-1].path
		if !seen[path] {
			seen[path] = true
			selected = append(selected, path)
		}
	}
	for {
		fmt.Fprint(os.Stdout, "Add another path (leave empty to continue): ")
		additional, readErr := reader.ReadString('\n')
		additional = strings.TrimSpace(additional)
		if readErr != nil && additional != "" {
			return nil, fmt.Errorf("read additional knowledge path: %w", readErr)
		}
		if additional == "" {
			break
		}
		if !seen[additional] {
			seen[additional] = true
			selected = append(selected, additional)
		}
	}
	return selected, nil
}

func setupSourceCandidates(documents []okf.DiscoveredMarkdown) []setupSourceCandidate {
	counts := map[string]int{}
	for _, document := range documents {
		parts := strings.Split(filepath.ToSlash(document.Path), "/")
		candidate := document.Path
		if len(parts) > 1 {
			candidate = parts[0]
			if (parts[0] == "packages" || parts[0] == "apps" || parts[0] == "services") && len(parts) >= 3 {
				if len(parts) == 3 {
					candidate = document.Path
				} else {
					candidate = strings.Join(parts[:3], "/")
				}
			}
		}
		counts[candidate]++
	}
	result := make([]setupSourceCandidate, 0, len(counts))
	for path, count := range counts {
		result = append(result, setupSourceCandidate{path: path, count: count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].path < result[j].path })
	return result
}

func setupDefaultAdoptDirectory(root string, selected []string) string {
	if len(selected) == 1 {
		candidate := filepath.Join(root, filepath.FromSlash(selected[0]))
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return selected[0]
		}
	}
	return "."
}

func runSetupWithoutMarkdown(reader *bufio.Reader, options setupCLIOptions, source string) int {
	fmt.Fprintln(os.Stdout, "No existing Markdown knowledge was found.")
	goal := strings.TrimSpace(options.about)
	if goal == "" {
		var err error
		goal, err = setupLine(reader, "What should this knowledge base help you or your agents do?", "Understand and maintain this project")
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
	}
	wiki, err := setupLine(reader, "Knowledge base location", options.wiki)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	useSource, err := setupConfirm(reader, "Use the current repository as source context?", true)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	options.wiki = wiki
	options.about = goal
	if useSource {
		options.source = source
	}
	if strings.TrimSpace(options.rules) == "" {
		options.rules = strings.Join(defaultSetupRules(options.useCase), ",")
	}
	task, err := buildSetupTask(options)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	return continueSetupTask(reader, options, task)
}

func continueSetupTask(reader *bufio.Reader, options setupCLIOptions, task string) int {
	available := detectSetupRuntimes(context.Background())
	labels := make([]string, 0, len(available)+2)
	for _, runtime := range available {
		labels = append(labels, "Run "+displayRuntime(runtime))
	}
	labels = append(labels, "Copy the task for an agent", "Save the task to a file")
	choice, err := setupChoice(reader, "How would you like to continue?", labels, 0)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if choice < len(available) {
		return runSetupAgent(options, available[choice], task)
	}
	if choice == len(available) {
		fmt.Print(task)
		return 0
	}
	path, err := setupLine(reader, "Task file", filepath.Join(".openknowledge", "setup-task.md"))
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if err := writeOutputFileAtomically(path, []byte(task)); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Saved setup task to %s.\n", path)
	return 0
}

func normalizeSetupUseCase(value string) (string, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(value)); normalized {
	case "", setupUseCaseBase:
		return setupUseCaseBase, nil
	case setupUseCaseTrusted, setupUseCaseCustom:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported setup use case %q; use base, trusted, or custom", value)
	}
}

func defaultSetupRules(useCase string) []string {
	return []string{"project", "writing"}
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

Create or adopt a managed knowledge base from ordinary Markdown.

Usage:
	openknowledge setup [wiki]
	openknowledge setup [wiki] --from <directory> --mode copy [--include <path>]
	openknowledge setup <directory> --from <directory> --mode in-place
	openknowledge setup [wiki] --from <directory> --mode copy --plan

Interactive setup discovers generic Markdown, lets you select source files or
directories, and offers only two management modes:

	Create a managed copy     Preserve sources and import them into a new bundle.
	Adopt a directory in place Add minimal metadata without moving or deleting files.

When no Markdown exists, setup creates a tailored task and lets you run an
installed agent, print the task to copy, or save it. An existing OKF bundle is
redirected to check, review, and upgrade instead of being changed by setup.

After the first result:
	openknowledge setup skill [--scope <global|project|both>] [--project <target>] [--harness <name>]
	openknowledge setup complete <wiki> --skill <scope> [--harness <name>] [--observe on|off]
	openknowledge setup status [wiki]
	openknowledge setup repair [wiki]
	openknowledge setup observe <on|off> [repository]

Optional production upgrades:
	openknowledge setup github [wiki] [--plan] [--force]
	openknowledge setup ci [wiki] [--plan] [--force]
	openknowledge setup runtime [wiki] [--maintenance auto|github-actions|runtime] [--runtimes <list>] [--plan] [--force]

The older prompt/agent workflow and --use-case remain available for compatibility.

Flags:
  --from         Directory to discover or adopt. Defaults to the current directory.
  --mode         Deterministic import mode: copy or in-place.
  --include      Import-only source path or pattern. Repeat as needed.
  --exclude      Import-only excluded path or pattern. Repeat as needed.
  --plan         Print the deterministic change plan without writing.
  --interactive  Run source selection even when input is not a terminal.
  --prompt       Print the complete agent task without changing files.
  --agent        Start codex, claude, or opencode with the setup task.
  --model        Harness-specific model override. Requires --agent.
  --about        Optional source-to-wiki goal. Requires --from.
  --depth        Non-negative traversal hint. Requires --from.
  --rules        Comma-separated maintenance rules. Works with ordinary and
                 --from setup. Defaults follow the selected use case.
  --use-case     Compatibility preset: base, trusted, or custom.
`
}
