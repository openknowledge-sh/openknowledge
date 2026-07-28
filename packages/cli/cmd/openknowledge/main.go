package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

var version = "0.8.4"

var terminal = newTerminal(os.Stdout)

func main() {
	os.Exit(runMain(os.Args[1:]))
}

const maxCLIErrorMessageBytes = 256 * 1024

type cliGlobalOptions struct {
	errorFormat string
}

type cliErrorEnvelope struct {
	SchemaVersion string         `json:"schemaVersion"`
	Error         cliErrorDetail `json:"error"`
}

type cliErrorDetail struct {
	Kind      string `json:"kind"`
	Command   string `json:"command"`
	ExitCode  int    `json:"exitCode"`
	Message   string `json:"message"`
	Truncated bool   `json:"truncated"`
}

type boundedCLIErrorBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (buffer *boundedCLIErrorBuffer) Write(content []byte) (int, error) {
	written := len(content)
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining > 0 {
		if remaining > len(content) {
			remaining = len(content)
		}
		_, _ = buffer.buffer.Write(content[:remaining])
	}
	if remaining < len(content) {
		buffer.truncated = true
	}
	return written, nil
}

func runMain(args []string) int {
	cliRunMutex.Lock()
	defer cliRunMutex.Unlock()

	options, commandArgs, err := parseCLIGlobalOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		usage()
		return 2
	}
	if options.errorFormat == "json" {
		return runWithJSONErrorEnvelope(commandArgs)
	}
	return dispatchCLI(commandArgs)
}

func parseCLIGlobalOptions(args []string) (cliGlobalOptions, []string, error) {
	options := cliGlobalOptions{errorFormat: "text"}
	for len(args) > 0 {
		arg := args[0]
		switch {
		case arg == "--error-format":
			if len(args) < 2 || strings.HasPrefix(args[1], "-") {
				return options, nil, fmt.Errorf("--error-format requires text or json")
			}
			options.errorFormat = strings.ToLower(strings.TrimSpace(args[1]))
			args = args[2:]
		case strings.HasPrefix(arg, "--error-format="):
			options.errorFormat = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(arg, "--error-format=")))
			args = args[1:]
		default:
			if options.errorFormat != "text" && options.errorFormat != "json" {
				return options, nil, fmt.Errorf("unsupported error format: %s", options.errorFormat)
			}
			return options, args, nil
		}
	}
	if options.errorFormat != "text" && options.errorFormat != "json" {
		return options, nil, fmt.Errorf("unsupported error format: %s", options.errorFormat)
	}
	return options, args, nil
}

func runWithJSONErrorEnvelope(args []string) int {
	originalStderr := stderrOutput()
	captured := boundedCLIErrorBuffer{limit: maxCLIErrorMessageBytes}
	exitCode := withStderrOutput(&captured, func() int {
		return dispatchCLI(args)
	})

	message := captured.buffer.String()
	if exitCode == 0 || strings.TrimSpace(message) == "" {
		if message != "" {
			_, _ = io.WriteString(originalStderr, message)
		}
		if captured.truncated {
			fmt.Fprintln(originalStderr, "\n[stderr truncated by --error-format json]")
		}
		return exitCode
	}

	_ = writeCLIErrorEnvelope(originalStderr, args, exitCode, strings.TrimSpace(message), captured.truncated)
	return exitCode
}

func writeCLIErrorEnvelope(output io.Writer, args []string, exitCode int, message string, truncated bool) error {
	kind := "runtime"
	if exitCode == 2 {
		kind = "usage"
	}
	envelope := cliErrorEnvelope{
		SchemaVersion: "1",
		Error: cliErrorDetail{
			Kind:      kind,
			Command:   cliErrorCommand(args),
			ExitCode:  exitCode,
			Message:   message,
			Truncated: truncated,
		},
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(envelope)
}

func runPromptSetup(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, promptSetupHelpText())
		return 0
	}
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	var rules string
	fs.StringVar(&rules, "rules", "", "suggest comma-separated maintenance rules for setup")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderrOutput(), "prompt setup accepts no positional arguments")
		return 2
	}

	ruleIDs, err := parseRuleIDs(rules)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	prompt, err := okf.SetupPromptWithOptions(okf.SetupPromptOptions{Rules: ruleIDs})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	fmt.Print(prompt)
	return 0
}

type fromOptions struct {
	source   string
	out      string
	wikiType string
	about    string
	depth    int
}

func runPromptFrom(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, promptFromHelpText())
		return 0
	}
	options, err := parseFromOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	prompt, err := okf.FromPrompt(okf.FromPromptOptions{
		Source: options.source,
		Out:    options.out,
		Type:   options.wikiType,
		About:  options.about,
		Depth:  options.depth,
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	fmt.Print(prompt)
	return 0
}

func parseFromOptions(args []string) (fromOptions, error) {
	options := fromOptions{wikiType: okf.DefaultFromType}
	var positionals []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--out":
			value, next, err := nextFlagValue(args, index, "--out")
			if err != nil {
				return fromOptions{}, err
			}
			options.out = value
			index = next
		case strings.HasPrefix(arg, "--out="):
			options.out = strings.TrimPrefix(arg, "--out=")
			if strings.TrimSpace(options.out) == "" {
				return fromOptions{}, fmt.Errorf("--out requires a value")
			}
		case arg == "--type":
			value, next, err := nextFlagValue(args, index, "--type")
			if err != nil {
				return fromOptions{}, err
			}
			options.wikiType = value
			index = next
		case strings.HasPrefix(arg, "--type="):
			options.wikiType = strings.TrimPrefix(arg, "--type=")
			if strings.TrimSpace(options.wikiType) == "" {
				return fromOptions{}, fmt.Errorf("--type requires a value")
			}
		case arg == "--about":
			value, next, err := nextFlagValue(args, index, "--about")
			if err != nil {
				return fromOptions{}, err
			}
			options.about = value
			index = next
		case strings.HasPrefix(arg, "--about="):
			options.about = strings.TrimPrefix(arg, "--about=")
			if strings.TrimSpace(options.about) == "" {
				return fromOptions{}, fmt.Errorf("--about requires a value")
			}
		case arg == "--depth":
			value, next, err := nextFlagValue(args, index, "--depth")
			if err != nil {
				return fromOptions{}, err
			}
			depth, err := parseNonNegativeIntFlag("--depth", value)
			if err != nil {
				return fromOptions{}, err
			}
			options.depth = depth
			index = next
		case strings.HasPrefix(arg, "--depth="):
			depth, err := parseNonNegativeIntFlag("--depth", strings.TrimPrefix(arg, "--depth="))
			if err != nil {
				return fromOptions{}, err
			}
			options.depth = depth
		case strings.HasPrefix(arg, "-"):
			return fromOptions{}, fmt.Errorf("unknown prompt from option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		return fromOptions{}, fmt.Errorf("usage: openknowledge prompt from <source> --out <path>")
	}
	options.source = positionals[0]
	if strings.TrimSpace(options.out) == "" {
		return fromOptions{}, fmt.Errorf("prompt from requires --out <path>")
	}
	return options, nil
}

func runRules(args []string) int {
	if len(args) > 0 && args[0] == "apply" {
		return runRulesApply(args[1:])
	}
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, rulesHelpText())
		return 0
	}

	options, err := parseRulesArgs(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if options.list {
		output, err := okf.RenderRulesListForWiki(options.wiki)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 2
		}
		fmt.Print(output)
		if options.pathSet {
			printRulesWikiWarnings(options.wiki)
		}
		return 0
	}
	output, err := okf.RenderAgentRules(okf.AgentRulesOptions{
		Wiki:   options.wiki,
		Target: options.target,
		Rules:  options.rules,
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	fmt.Print(output)
	printRulesWikiWarnings(options.wiki)
	return 0
}

func runRulesApply(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, rulesApplyHelpText())
		return 0
	}
	options, err := parseRulesApplyArgs(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	printRulesWikiWarnings(options.wiki)

	targetFile, err := resolveRulesApplyFile(options)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	target := options.target
	if target == "" {
		target = rulesTargetForFile(targetFile)
	}
	rules, err := okf.RenderAgentRules(okf.AgentRulesOptions{
		Wiki:    options.wiki,
		Target:  target,
		Rules:   options.rules,
		Managed: true,
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	block := okf.RenderManagedRulesBlock(rules)
	if options.dryRun {
		fmt.Printf("Would update %s with:\n\n%s", targetFile, block)
		return 0
	}
	if err := okf.RequireRegistryWriteAccess(targetFile); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}

	existingBytes, err := os.ReadFile(targetFile)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if err == nil && !options.yes && isTerminalFile(os.Stdin) {
		confirmed, err := confirmRulesApplyWrite(targetFile, string(existingBytes), block)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		if !confirmed {
			fmt.Fprintln(os.Stdout, "Cancelled.")
			return 0
		}
	}
	updated := okf.UpsertManagedRulesBlock(string(existingBytes), block)
	if err := os.MkdirAll(filepath.Dir(targetFile), 0755); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if err := os.WriteFile(targetFile, []byte(updated), 0644); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	fmt.Printf("Updated %s\n", targetFile)
	return 0
}

func runReview(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, reviewHelpText())
		return 0
	}
	switch args[0] {
	case "rules":
		return runReviewRules(args[1:])
	default:
		fmt.Fprintf(stderrOutput(), "unknown review subcommand: %s\n", args[0])
		return 2
	}
}

func runReviewRules(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, reviewRulesHelpText())
		return 0
	}
	options, err := parseReviewRulesArgs(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	output, err := okf.RenderRuleReviewPrompt(okf.RuleReviewOptions{
		Wiki:  options.wiki,
		Rules: options.rules,
		All:   options.all,
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	fmt.Print(output)
	printRulesWikiWarnings(options.wiki)
	return 0
}

type rulesArgs struct {
	wiki    string
	target  string
	rules   []string
	list    bool
	pathSet bool
}

type rulesApplyArgs struct {
	wiki   string
	target string
	rules  []string
	file   string
	yes    bool
	dryRun bool
}

type reviewRulesArgs struct {
	wiki  string
	rules []string
	all   bool
}

func parseRulesArgs(args []string) (rulesArgs, error) {
	options := rulesArgs{
		wiki:   okf.DefaultRulesWiki,
		target: "generic",
	}
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--list":
			options.list = true
		case arg == "--path":
			i++
			if i >= len(args) {
				return options, fmt.Errorf("--path requires a value")
			}
			if strings.TrimSpace(args[i]) == "" {
				return options, fmt.Errorf("--path requires a non-empty value")
			}
			options.wiki = args[i]
			options.pathSet = true
		case strings.HasPrefix(arg, "--path="):
			value := strings.TrimPrefix(arg, "--path=")
			if strings.TrimSpace(value) == "" {
				return options, fmt.Errorf("--path requires a non-empty value")
			}
			options.wiki = value
			options.pathSet = true
		case arg == "--target":
			i++
			if i >= len(args) {
				return options, fmt.Errorf("--target requires a value")
			}
			options.target = args[i]
		case strings.HasPrefix(arg, "--target="):
			options.target = strings.TrimPrefix(arg, "--target=")
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("unknown rules option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 1 {
		return options, fmt.Errorf("rules accepts at most one comma-separated rules argument; pass the wiki path with --path")
	}
	if len(positionals) == 1 {
		rules, err := parseRuleIDs(positionals[0])
		if err != nil {
			return options, err
		}
		options.rules = rules
	}
	return options, nil
}

func parseRulesApplyArgs(args []string) (rulesApplyArgs, error) {
	options := rulesApplyArgs{wiki: okf.DefaultRulesWiki}
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--path":
			i++
			if i >= len(args) {
				return options, fmt.Errorf("--path requires a value")
			}
			if strings.TrimSpace(args[i]) == "" {
				return options, fmt.Errorf("--path requires a non-empty value")
			}
			options.wiki = args[i]
		case strings.HasPrefix(arg, "--path="):
			value := strings.TrimPrefix(arg, "--path=")
			if strings.TrimSpace(value) == "" {
				return options, fmt.Errorf("--path requires a non-empty value")
			}
			options.wiki = value
		case arg == "--target":
			i++
			if i >= len(args) {
				return options, fmt.Errorf("--target requires a value")
			}
			options.target = args[i]
		case strings.HasPrefix(arg, "--target="):
			options.target = strings.TrimPrefix(arg, "--target=")
		case arg == "--file":
			i++
			if i >= len(args) {
				return options, fmt.Errorf("--file requires a value")
			}
			options.file = args[i]
		case strings.HasPrefix(arg, "--file="):
			options.file = strings.TrimPrefix(arg, "--file=")
		case arg == "--yes" || arg == "-y":
			options.yes = true
		case arg == "--dry-run":
			options.dryRun = true
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("unknown rules apply option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 1 {
		return options, fmt.Errorf("rules apply accepts at most one comma-separated rules argument; pass the wiki path with --path")
	}
	if len(positionals) == 1 {
		rules, err := parseRuleIDs(positionals[0])
		if err != nil {
			return options, err
		}
		options.rules = rules
	}
	return options, nil
}

func parseReviewRulesArgs(args []string) (reviewRulesArgs, error) {
	options := reviewRulesArgs{wiki: okf.DefaultRulesWiki}
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--path":
			i++
			if i >= len(args) {
				return options, fmt.Errorf("--path requires a value")
			}
			if strings.TrimSpace(args[i]) == "" {
				return options, fmt.Errorf("--path requires a non-empty value")
			}
			options.wiki = args[i]
		case strings.HasPrefix(arg, "--path="):
			value := strings.TrimPrefix(arg, "--path=")
			if strings.TrimSpace(value) == "" {
				return options, fmt.Errorf("--path requires a non-empty value")
			}
			options.wiki = value
		case arg == "--rules":
			i++
			if i >= len(args) {
				return options, fmt.Errorf("--rules requires a value")
			}
			if strings.TrimSpace(args[i]) == "" {
				return options, fmt.Errorf("--rules requires a non-empty value")
			}
			rules, err := parseRuleIDs(args[i])
			if err != nil {
				return options, err
			}
			options.rules = rules
		case strings.HasPrefix(arg, "--rules="):
			value := strings.TrimPrefix(arg, "--rules=")
			if strings.TrimSpace(value) == "" {
				return options, fmt.Errorf("--rules requires a non-empty value")
			}
			rules, err := parseRuleIDs(value)
			if err != nil {
				return options, err
			}
			options.rules = rules
		case arg == "--all":
			options.all = true
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("unknown review rules option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 1 {
		return options, fmt.Errorf("review rules accepts at most one wiki path positional argument")
	}
	if len(positionals) == 1 {
		options.wiki = positionals[0]
	}
	if options.all && len(options.rules) > 0 {
		return options, fmt.Errorf("--all cannot be combined with --rules")
	}
	return options, nil
}

func parseRuleIDs(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	rules := make([]string, 0, len(parts))
	for _, part := range parts {
		rule := strings.TrimSpace(part)
		if rule == "" {
			return nil, fmt.Errorf("rules list must not contain empty entries")
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func printRulesWikiWarnings(wiki string) {
	output := stderrOutput()
	if isTerminalFile(os.Stdout) {
		output = os.Stdout
	}
	for _, warning := range okf.RulesWikiWarnings(wiki) {
		printWarning(output, warning)
	}
}

func resolveRulesApplyFile(options rulesApplyArgs) (string, error) {
	if strings.TrimSpace(options.file) != "" {
		return options.file, nil
	}
	candidates, err := discoverAgentRuleFiles(".")
	if err != nil {
		return "", err
	}
	if len(candidates) == 1 || (len(candidates) > 1 && options.yes) {
		return candidates[0], nil
	}
	if len(candidates) == 0 && options.yes {
		return "AGENTS.md", nil
	}
	if isTerminalFile(os.Stdin) {
		defaultFile := "AGENTS.md"
		if len(candidates) > 0 {
			fmt.Fprintln(os.Stdout, "Found agent instruction files:")
			for _, candidate := range candidates {
				fmt.Fprintf(os.Stdout, "  %s\n", candidate)
			}
			defaultFile = candidates[0]
		}
		return prompt("Agent rules file", defaultFile)
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("multiple agent instruction files found; pass --file or --yes")
	}
	return "", fmt.Errorf("no agent instruction file found; pass --file or --yes to create AGENTS.md")
}

func discoverAgentRuleFiles(start string) ([]string, error) {
	absolute, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}
	candidateNames := []string{
		"AGENTS.md",
		"CLAUDE.md",
		filepath.Join(".cursor", "rules", "openknowledge.md"),
		filepath.Join(".cursor", "rules", "openknowledge.mdc"),
	}
	var candidates []string
	seen := map[string]struct{}{}
	for {
		for _, name := range candidateNames {
			candidate := filepath.Join(absolute, name)
			if _, err := os.Stat(candidate); err == nil {
				if _, ok := seen[candidate]; !ok {
					seen[candidate] = struct{}{}
					candidates = append(candidates, candidate)
				}
			}
		}
		parent := filepath.Dir(absolute)
		if parent == absolute {
			break
		}
		absolute = parent
	}
	return candidates, nil
}

func rulesTargetForFile(file string) string {
	base := filepath.Base(file)
	switch base {
	case "AGENTS.md":
		return "codex"
	case "CLAUDE.md":
		return "claude"
	}
	slashed := filepath.ToSlash(file)
	if strings.Contains(slashed, "/.cursor/rules/") || strings.HasPrefix(slashed, ".cursor/rules/") {
		return "cursor"
	}
	return "generic"
}

func confirmRulesApplyWrite(file string, existing string, block string) (bool, error) {
	fmt.Fprintf(os.Stdout, "\nGenerated rules block:\n\n%s", block)
	printWarning(os.Stdout, rulesApplyConfirmationMessage(file, existing))
	fmt.Fprint(os.Stdout, "Continue? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil && len(answer) == 0 {
		return false, nil
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func rulesApplyConfirmationMessage(file string, existing string) string {
	if strings.Contains(existing, okf.RulesBlockStart) && strings.Contains(existing, okf.RulesBlockEnd) {
		return fmt.Sprintf("%s already contains an Open Knowledge rules block. This will replace that block.", file)
	}
	if strings.Contains(existing, okf.RulesBlockStart) || strings.Contains(existing, okf.RulesBlockEnd) {
		return fmt.Sprintf("%s contains a partial Open Knowledge rules marker. This will append a new managed block.", file)
	}
	if strings.TrimSpace(existing) == "" {
		return fmt.Sprintf("%s already exists. This will write an Open Knowledge rules block to it.", file)
	}
	return fmt.Sprintf("%s already exists. This will append an Open Knowledge rules block to the file.", file)
}

func printWarning(output io.Writer, message string) {
	fmt.Fprintln(output)
	fmt.Fprintln(output, warningText(output, message))
	fmt.Fprintln(output)
}

func warningText(output io.Writer, message string) string {
	label := "⚠ Warning:"
	text := label + " " + strings.TrimSpace(message)
	file, ok := output.(*os.File)
	if !ok {
		return text
	}
	return newTerminal(file).yellow(text)
}

func isTerminalFile(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

func runSpec(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, specHelpText())
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge spec latest|<version>")
		return 2
	}

	version, ok := okf.ResolveSpecVersion(args[0])
	if !ok {
		fmt.Fprintf(stderrOutput(), "unsupported OKF spec version: %s\n", args[0])
		return 2
	}

	spec := okf.Spec(version)
	fmt.Print(spec)
	if !strings.HasSuffix(spec, "\n") {
		fmt.Println()
	}
	return 0
}

func runScaffold(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, scaffoldHelpText())
		return 0
	}
	fs := flag.NewFlagSet("scaffold", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	nameFlag := fs.String("name", "", "knowledge base name")
	bundleNameFlag := fs.String("bundle-name", "", "stable bundle id for root okf_bundle_name metadata")
	bundleTitleFlag := fs.String("bundle-title", "", "bundle title for root okf_bundle_title metadata")
	bundlePurposeFlag := fs.String("bundle-purpose", "", "bundle purpose for root okf_bundle_purpose metadata")
	noAgentsFlag := fs.Bool("no-agents", false, "skip AGENTS.md starter agent rules")
	noSetupFlag := fs.Bool("no-setup", false, "skip SETUP.MD setup handoff")
	var bundleTags stringListFlag
	var bundleEntries stringListFlag
	fs.Var(&bundleTags, "bundle-tag", "bundle tag for root okf_bundle_tags metadata; repeatable")
	fs.Var(&bundleEntries, "bundle-entry", "bundle entrypoint as name=path; repeatable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderrOutput(), "scaffold accepts at most one folder path")
		return 2
	}

	path := ""
	if fs.NArg() == 1 {
		path = fs.Arg(0)
	}

	defaultName := strings.TrimSpace(*nameFlag)
	if defaultName == "" && path != "" {
		defaultName = titleFromPath(path)
	}

	terminal.banner()
	name := defaultName
	if strings.TrimSpace(*nameFlag) == "" {
		var err error
		name, err = prompt("Knowledge base name", defaultName)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
	}

	entries, err := parseBundleEntryFlags(bundleEntries)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}

	result, err := okf.NewProject(okf.NewProjectOptions{
		Name:           name,
		Path:           path,
		SkipAgentRules: *noAgentsFlag,
		SkipSetup:      *noSetupFlag,
		BundleMetadata: okf.BundleMetadata{
			Name:    *bundleNameFlag,
			Title:   *bundleTitleFlag,
			Purpose: *bundlePurposeFlag,
			Tags:    []string(bundleTags),
			Entries: entries,
		},
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}

	terminal.success("Created knowledge base")
	fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(result.Root))
	fmt.Println()
	terminal.section("Scaffold")
	for _, path := range result.Created {
		fmt.Printf("  %s %s\n", terminal.green("+"), path)
	}

	if result.SetupPath != "" {
		fmt.Println()
		terminal.section("Agent handoff")
		fmt.Println("  Paste this into your agent:")
		fmt.Println()
		fmt.Printf("  Set up an Open Knowledge agentic wiki for this workspace. Read %s,\n", terminal.path(result.SetupPath))
		fmt.Println("  inspect this workspace and any relevant memories, ask only the setup questions still needed,")
		fmt.Println("  run openknowledge validate, and demonstrate one useful openknowledge search query.")
	}
	return 0
}

type getOptions struct {
	target string
	entry  string
	info   bool
}

type searchOptions struct {
	target    string
	query     string
	all       bool
	format    string
	spec      string
	limit     int
	budget    int
	budgetSet bool
	matches   bool
	noExpand  bool
}

type getSelection struct {
	name string
	rel  string
	abs  string
}

// get is the deterministic Markdown reader. It prints an exact local file,
// named bundle entrypoint, bundle-relative file, or root index fallback.
func runGet(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, getHelpText())
		return 0
	}
	options, err := parseGetOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}

	if options.entry == "" {
		if localFile, rel, ok := resolveDirectGetFile(options.target); ok {
			if !isGetMarkdownFile(localFile) {
				fmt.Fprintf(stderrOutput(), "get only prints Markdown files: %s\n", rel)
				return 1
			}
			if options.info {
				document, err := okf.ReadMarkdownDocumentInfo(localFile, rel)
				if err != nil {
					fmt.Fprintln(stderrOutput(), err)
					return 1
				}
				printGetFileInfo(getSelection{name: rel, rel: rel, abs: localFile}, document)
				return 0
			}
			content, err := os.ReadFile(localFile)
			if err != nil {
				fmt.Fprintln(stderrOutput(), err)
				return 1
			}
			fmt.Print(string(content))
			return 0
		}
	}

	root, err := resolveWhereTarget(options.target)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	info, err := okf.ReadBundleInfo(root)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}

	if options.info {
		if err := printGetInfo(root, info, options.entry); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		return 0
	}

	selection, err := selectGetTarget(root, info, options.entry)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	content, err := os.ReadFile(selection.abs)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	fmt.Print(string(content))
	return 0
}

// search is the CLI retrieval surface: resolve a key/path, rank heading
// sections once, then print source-preserving context or diagnostic matches.
func runSearch(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, searchHelpText())
		return 0
	}
	options, err := parseSearchOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if options.all {
		return runFederatedSearch(options)
	}
	root, err := resolveWhereTarget(options.target)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if options.matches {
		result, err := okf.SearchKnowledgeWithVersion(root, options.spec, okf.SearchOptions{
			Query:    options.query,
			Limit:    options.limit,
			Fuzzy:    true,
			NoExpand: options.noExpand,
		})
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		if err := printSearchMatches(result, options.format); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		return 0
	}

	result, err := okf.ResolveContextWithVersion(root, options.spec, okf.ContextOptions{
		Query:    options.query,
		Budget:   options.budget,
		Limit:    options.limit,
		NoExpand: options.noExpand,
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if err := printSearchContext(result, options.format); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	return 0
}

func runFederatedSearch(options searchOptions) int {
	entries, err := okf.RegistryEntries()
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	targets := make([]okf.FederatedTarget, 0, len(entries))
	for _, entry := range entries {
		targets = append(targets, okf.FederatedTarget{Name: entry.Name, Root: entry.Path})
	}
	if options.matches {
		result, err := okf.SearchFederatedKnowledgeWithVersion(targets, options.spec, okf.SearchOptions{
			Query: options.query, Limit: options.limit, Fuzzy: true, NoExpand: options.noExpand,
		})
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		if err := printFederatedSearchMatches(result, options.format); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		return federatedSearchExitCode(result.KnowledgeBases)
	}
	result, err := okf.ResolveFederatedContextWithVersion(targets, options.spec, okf.ContextOptions{
		Query: options.query, Budget: options.budget, Limit: options.limit, NoExpand: options.noExpand,
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if err := printFederatedSearchContext(result, options.format); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	return federatedSearchExitCode(result.KnowledgeBases)
}

func federatedSearchExitCode(bases []okf.FederatedKnowledgeBase) int {
	if len(bases) == 0 {
		return 0
	}
	for _, base := range bases {
		if base.Revision != nil {
			return 0
		}
	}
	return 1
}

func nextFlagValue(args []string, index int, flag string) (string, int, error) {
	if index+1 >= len(args) {
		return "", index, fmt.Errorf("%s requires a value", flag)
	}
	value := args[index+1]
	if strings.HasPrefix(value, "-") {
		return "", index, fmt.Errorf("%s requires a value", flag)
	}
	return value, index + 1, nil
}

func parsePositiveIntFlag(flag string, value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", flag)
	}
	return parsed, nil
}

func parseNonNegativeIntFlag(flag string, value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be zero or a positive integer", flag)
	}
	return parsed, nil
}

func parseGetOptions(args []string) (getOptions, error) {
	options := getOptions{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--info":
			options.info = true
		case strings.HasPrefix(arg, "-"):
			return getOptions{}, fmt.Errorf("unknown flag: %s", arg)
		case options.target == "":
			options.target = arg
		case options.entry == "":
			options.entry = arg
		default:
			return getOptions{}, fmt.Errorf("get accepts at most one entry or file path")
		}
	}
	if options.target == "" {
		return getOptions{}, fmt.Errorf("usage: openknowledge get <name|path> [entry-or-file]")
	}
	return options, nil
}

func parseSearchOptions(args []string) (searchOptions, error) {
	options := searchOptions{
		format: "markdown",
		spec:   "latest",
		limit:  12,
		budget: okf.DefaultContextBudget,
	}
	// The first positional is the bundle target. Remaining positionals are
	// joined into the query so both quoted and unquoted multi-word queries work.
	var positionals []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--format":
			value, next, err := nextFlagValue(args, index, "--format")
			if err != nil {
				return searchOptions{}, err
			}
			options.format = strings.TrimSpace(strings.ToLower(value))
			index = next
		case strings.HasPrefix(arg, "--format="):
			options.format = strings.TrimSpace(strings.ToLower(strings.TrimPrefix(arg, "--format=")))
		case arg == "--budget":
			value, next, err := nextFlagValue(args, index, "--budget")
			if err != nil {
				return searchOptions{}, err
			}
			budget, err := parsePositiveIntFlag("--budget", value)
			if err != nil {
				return searchOptions{}, err
			}
			options.budget = budget
			options.budgetSet = true
			index = next
		case strings.HasPrefix(arg, "--budget="):
			budget, err := parsePositiveIntFlag("--budget", strings.TrimPrefix(arg, "--budget="))
			if err != nil {
				return searchOptions{}, err
			}
			options.budget = budget
			options.budgetSet = true
		case arg == "--limit":
			value, next, err := nextFlagValue(args, index, "--limit")
			if err != nil {
				return searchOptions{}, err
			}
			limit, err := parsePositiveIntFlag("--limit", value)
			if err != nil {
				return searchOptions{}, err
			}
			options.limit = limit
			index = next
		case strings.HasPrefix(arg, "--limit="):
			limit, err := parsePositiveIntFlag("--limit", strings.TrimPrefix(arg, "--limit="))
			if err != nil {
				return searchOptions{}, err
			}
			options.limit = limit
		case arg == "--spec":
			value, next, err := nextFlagValue(args, index, "--spec")
			if err != nil {
				return searchOptions{}, err
			}
			options.spec = value
			index = next
		case strings.HasPrefix(arg, "--spec="):
			options.spec = strings.TrimPrefix(arg, "--spec=")
			if strings.TrimSpace(options.spec) == "" {
				return searchOptions{}, fmt.Errorf("--spec requires a value")
			}
		case arg == "--matches":
			options.matches = true
		case arg == "--all":
			options.all = true
		case arg == "--no-expand":
			options.noExpand = true
		case strings.HasPrefix(arg, "-"):
			return searchOptions{}, fmt.Errorf("unknown search option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	if options.format == "" {
		options.format = "markdown"
	}
	if options.format != "markdown" && options.format != "json" {
		return searchOptions{}, fmt.Errorf("unsupported search format: %s", options.format)
	}
	if options.all {
		if len(positionals) < 1 {
			return searchOptions{}, fmt.Errorf("usage: openknowledge search --all <query>")
		}
		options.query = strings.TrimSpace(strings.Join(positionals, " "))
	} else {
		if len(positionals) < 2 {
			return searchOptions{}, fmt.Errorf("usage: openknowledge search <name|path> <query>")
		}
		options.target = positionals[0]
		options.query = strings.TrimSpace(strings.Join(positionals[1:], " "))
	}
	if options.query == "" {
		return searchOptions{}, fmt.Errorf("openknowledge search requires a non-empty query")
	}
	if options.matches && options.budgetSet {
		return searchOptions{}, fmt.Errorf("--budget cannot be used with --matches")
	}
	return options, nil
}

func printSearchContext(result okf.ContextResult, format string) error {
	switch format {
	case "json":
		return printSearchJSON(result)
	case "markdown":
		printSearchContextMarkdown(result)
	default:
		return fmt.Errorf("unsupported search format: %s", format)
	}
	return nil
}

func printFederatedSearchContext(result okf.FederatedContextResult, format string) error {
	if format == "json" {
		return printSearchJSON(result)
	}
	if format != "markdown" {
		return fmt.Errorf("unsupported search format: %s", format)
	}
	fmt.Println("# Open Knowledge Federated Context")
	fmt.Println()
	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Knowledge bases: %d\n", len(result.KnowledgeBases))
	fmt.Printf("Fusion: `%s` (rank constant %d)\n", result.Fusion.Method, result.Fusion.RankConstant)
	fmt.Printf("Context: %d / %d estimated tokens\n", result.EstimatedTokens, result.Budget)
	fmt.Printf("Sources: %d\n", len(result.Sources))
	printFederatedKnowledgeBaseErrors(result.KnowledgeBases)
	if len(result.Sources) == 0 {
		fmt.Println()
		fmt.Println("No matching source sections found across the registry.")
		return nil
	}
	for index, candidate := range result.Sources {
		source := candidate.Source
		fmt.Println()
		fmt.Printf("## %d. %s / %s\n", index+1, candidate.KnowledgeBase, searchContextSourceTitle(source))
		fmt.Println()
		fmt.Printf("Source: `%s:%s`\n", candidate.KnowledgeBase, searchSourceLocation(source.Path, source.LineStart, source.LineEnd))
		fmt.Printf("Locator: `%s`\n", source.Locator)
		fmt.Printf("Rank: `%d`; fusion score: `%.9f`; source score: `%.2f`\n", candidate.Rank, candidate.FusionScore, source.Score)
		fmt.Printf("Relation: `%s`\n", source.Relation)
		fmt.Println()
		fmt.Println(source.Markdown)
	}
	return nil
}

func printFederatedKnowledgeBaseErrors(bases []okf.FederatedKnowledgeBase) {
	for _, base := range bases {
		if base.Status == "error" {
			fmt.Printf("Knowledge base error: `%s`: %s\n", base.Name, base.Error)
		}
	}
}

func printSearchContextMarkdown(result okf.ContextResult) {
	fmt.Println("# Open Knowledge Context")
	fmt.Println()
	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Root: `%s`\n", result.Root)
	fmt.Printf("Revision: `%s` (OKF %s)\n", result.Revision.IndexSHA256, result.Revision.SpecVersion)
	fmt.Printf("Context: %d / %d estimated tokens\n", result.EstimatedTokens, result.Budget)
	fmt.Printf("Sources: %d\n", len(result.Sources))
	fmt.Printf("Validation issues: %d\n", len(result.Issues))
	if len(result.Sources) == 0 {
		fmt.Println()
		fmt.Println("No matching source sections found.")
		return
	}

	for index, source := range result.Sources {
		fmt.Println()
		fmt.Printf("## %d. %s\n", index+1, searchContextSourceTitle(source))
		fmt.Println()
		fmt.Printf("Source: `%s`\n", searchSourceLocation(source.Path, source.LineStart, source.LineEnd))
		fmt.Printf("Locator: `%s`\n", source.Locator)
		fmt.Printf("Relation: `%s`\n", source.Relation)
		fmt.Printf("Score: `%.2f`\n", source.Score)
		fmt.Println()
		fmt.Println(source.Markdown)
	}
}

func printSearchMatches(result okf.SearchResultSet, format string) error {
	switch format {
	case "json":
		return printSearchJSON(result)
	case "markdown":
		printSearchMatchesMarkdown(result)
	default:
		return fmt.Errorf("unsupported search format: %s", format)
	}
	return nil
}

func printFederatedSearchMatches(result okf.FederatedSearchResultSet, format string) error {
	if format == "json" {
		return printSearchJSON(result)
	}
	if format != "markdown" {
		return fmt.Errorf("unsupported search format: %s", format)
	}
	fmt.Println("# Open Knowledge Federated Search Matches")
	fmt.Println()
	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Knowledge bases: %d\n", len(result.KnowledgeBases))
	fmt.Printf("Fusion: `%s` (rank constant %d)\n", result.Fusion.Method, result.Fusion.RankConstant)
	fmt.Printf("Matches: %d\n", len(result.Results))
	printFederatedKnowledgeBaseErrors(result.KnowledgeBases)
	if len(result.Results) == 0 {
		fmt.Println()
		fmt.Println("No matching source sections found across the registry.")
		return nil
	}
	for index, candidate := range result.Results {
		match := candidate.Result
		fmt.Println()
		fmt.Printf("## %d. %s / %s\n", index+1, candidate.KnowledgeBase, searchMatchTitle(match))
		fmt.Println()
		fmt.Printf("Source: `%s:%s`\n", candidate.KnowledgeBase, searchSourceLocation(match.Path, match.LineStart, match.LineEnd))
		fmt.Printf("Locator: `%s`\n", match.Locator)
		fmt.Printf("Rank: `%d`; fusion score: `%.9f`; source score: `%.2f`\n", candidate.Rank, candidate.FusionScore, match.Score)
		fmt.Printf("Relation: `%s`\n", searchResultRelation(match))
		if strings.TrimSpace(match.Snippet) != "" {
			fmt.Println()
			fmt.Println(match.Snippet)
		}
	}
	return nil
}

func printSearchMatchesMarkdown(result okf.SearchResultSet) {
	fmt.Println("# Open Knowledge Search Matches")
	fmt.Println()
	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Root: `%s`\n", result.Root)
	fmt.Printf("Revision: `%s` (OKF %s)\n", result.Revision.IndexSHA256, result.Revision.SpecVersion)
	fmt.Printf("Matches: %d\n", len(result.Results))
	fmt.Printf("Validation issues: %d\n", len(result.Issues))
	if len(result.Results) == 0 {
		fmt.Println()
		fmt.Println("No matching source sections found.")
		return
	}

	for index, match := range result.Results {
		fmt.Println()
		fmt.Printf("## %d. %s\n", index+1, searchMatchTitle(match))
		fmt.Println()
		fmt.Printf("Source: `%s`\n", searchSourceLocation(match.Path, match.LineStart, match.LineEnd))
		fmt.Printf("Locator: `%s`\n", match.Locator)
		fmt.Printf("Relation: `%s`\n", searchResultRelation(match))
		fmt.Printf("Score: `%.2f`\n", match.Score)
		if len(match.HeadingPath) > 0 {
			fmt.Printf("Heading path: %s\n", strings.Join(match.HeadingPath, " > "))
		}
		if strings.TrimSpace(match.Type) != "" {
			fmt.Printf("Type: `%s`\n", match.Type)
		}
		if strings.TrimSpace(match.Snippet) != "" {
			fmt.Println()
			fmt.Println(match.Snippet)
		}
	}
}

func printSearchJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func searchContextSourceTitle(source okf.ContextSource) string {
	if strings.TrimSpace(source.Heading) != "" && source.Heading != "Top" {
		return source.Heading
	}
	if strings.TrimSpace(source.Title) != "" {
		return source.Title
	}
	return source.Path
}

func searchMatchTitle(result okf.SearchResult) string {
	if strings.TrimSpace(result.Heading) != "" && result.Heading != "Top" {
		return result.Heading
	}
	if strings.TrimSpace(result.Title) != "" {
		return result.Title
	}
	return result.Path
}

func searchSourceLocation(path string, lineStart int, lineEnd int) string {
	if lineStart <= 0 {
		return path
	}
	return fmt.Sprintf("%s:%d-%d", path, lineStart, lineEnd)
}

func searchResultRelation(result okf.SearchResult) string {
	if strings.TrimSpace(result.Relation) != "" {
		return result.Relation
	}
	return "direct"
}

func resolveDirectGetFile(target string) (string, string, bool) {
	expanded, err := okf.ExpandUserPath(strings.TrimSpace(target))
	if err != nil {
		return "", "", false
	}
	info, err := os.Stat(expanded)
	if err != nil || info.IsDir() {
		return "", "", false
	}
	absolute, err := filepath.Abs(expanded)
	if err != nil {
		absolute = expanded
	}
	rel, err := filepath.Rel(".", absolute)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		rel = filepath.Base(absolute)
	}
	return absolute, filepath.ToSlash(rel), true
}

func selectGetTarget(root string, info okf.BundleInfo, entryName string) (getSelection, error) {
	name := strings.TrimSpace(entryName)
	rel := ""
	pathFallback := false
	if name == "" {
		if path, ok := info.EntryPath("default"); ok {
			name = "default"
			rel = path
		} else {
			name = "index"
			rel = "index.md"
		}
	} else {
		path, ok := info.EntryPath(name)
		if !ok {
			rel = name
			pathFallback = true
		} else {
			rel = path
		}
	}

	abs, normalizedRel, err := resolveBundleRelativeFile(root, rel)
	if err != nil {
		if pathFallback && os.IsNotExist(err) {
			available := info.EntryNames()
			if len(available) == 0 {
				return getSelection{}, fmt.Errorf("entrypoint or path %q does not exist; this bundle has no declared entrypoints", name)
			}
			return getSelection{}, fmt.Errorf("entrypoint or path %q does not exist; available entries: %s", name, strings.Join(available, ", "))
		}
		return getSelection{}, err
	}
	if !isGetMarkdownFile(abs) {
		return getSelection{}, fmt.Errorf("get only prints Markdown files: %s", normalizedRel)
	}
	return getSelection{name: name, rel: normalizedRel, abs: abs}, nil
}

func isGetMarkdownFile(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".md" || extension == ".markdown"
}

func resolveBundleRelativeFile(root string, rel string) (string, string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", "", fmt.Errorf("entrypoint path is empty")
	}
	rel = filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("entrypoint path must stay inside the bundle: %s", rel)
	}
	abs, err := okf.ResolveBundlePath(root, rel)
	if err != nil {
		return "", "", err
	}
	relative := rel
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("entrypoint path is a directory: %s", rel)
	}
	return abs, filepath.ToSlash(relative), nil
}

func printGetInfo(root string, info okf.BundleInfo, entryName string) error {
	terminal.title("Open Knowledge Get", "entrypoint and file metadata")
	fmt.Printf("%-9s %s\n", "name", info.DisplayName())
	fmt.Printf("%-9s %s\n", "root", terminal.path(root))
	if info.Metadata.Purpose != "" {
		fmt.Printf("%-9s %s\n", "purpose", info.Metadata.Purpose)
	}
	if len(info.Metadata.Tags) > 0 {
		fmt.Printf("%-9s %s\n", "tags", strings.Join(info.Metadata.Tags, ", "))
	}
	fmt.Println()

	if strings.TrimSpace(entryName) != "" {
		selection, err := selectGetTarget(root, info, entryName)
		if err != nil {
			return err
		}
		document, err := okf.ReadMarkdownDocumentInfo(selection.abs, selection.rel)
		if err != nil {
			return err
		}
		printGetFileInfo(selection, document)
		return nil
	}

	if len(info.Metadata.Entries) == 0 {
		selection, err := selectGetTarget(root, info, "")
		if err != nil {
			return err
		}
		document, err := okf.ReadMarkdownDocumentInfo(selection.abs, selection.rel)
		if err != nil {
			return err
		}
		printGetFileInfo(selection, document)
		return nil
	}

	terminal.section("Entrypoints")
	for _, entry := range info.Metadata.Entries {
		selection, err := selectGetTarget(root, info, entry.Name)
		if err != nil {
			return err
		}
		document, err := okf.ReadMarkdownDocumentInfo(selection.abs, selection.rel)
		if err != nil {
			return err
		}
		summary := document.Title
		if summary == "" {
			summary = document.Description
		}
		if summary == "" {
			fmt.Printf("  %-12s %s\n", selection.name, selection.rel)
		} else {
			fmt.Printf("  %-12s %s  %s\n", selection.name, selection.rel, terminal.muted(summary))
		}
	}
	return nil
}

func printGetFileInfo(selection getSelection, document okf.MarkdownDocumentInfo) {
	terminal.section("File")
	fmt.Printf("%-12s %s\n", "selection", selection.name)
	fmt.Printf("%-12s %s\n", "path", selection.rel)
	if document.Type != "" {
		fmt.Printf("%-12s %s\n", "type", document.Type)
	}
	if document.Title != "" {
		fmt.Printf("%-12s %s\n", "title", document.Title)
	}
	if document.Description != "" {
		fmt.Printf("%-12s %s\n", "description", document.Description)
	}
	if len(document.Tags) > 0 {
		fmt.Printf("%-12s %s\n", "tags", strings.Join(document.Tags, ", "))
	}
	if len(document.UseWhen) > 0 {
		fmt.Printf("%-12s %s\n", "use_when", strings.Join(document.UseWhen, ", "))
	}
}

func printRegistryEntries(entries []okf.RegistryEntry) {
	terminal.title("Open Knowledge Registry", "known knowledge bases")
	path, err := okf.RegistryFile()
	if err == nil {
		fmt.Printf("%s %s\n", terminal.muted("config"), terminal.path(path))
	}
	fmt.Println()
	if len(entries) == 0 {
		fmt.Println(terminal.muted("No registered knowledge bases."))
		return
	}
	for _, entry := range entries {
		fmt.Printf("  %-18s %s\n", entry.Name, terminal.path(entry.Path))
	}
}

type registryListReport struct {
	SchemaVersion string              `json:"schemaVersion"`
	Registry      string              `json:"registry"`
	Entries       []registryListEntry `json:"entries"`
}

type registryListEntry struct {
	Name    string              `json:"name"`
	Path    string              `json:"path"`
	Access  string              `json:"access"`
	Managed bool                `json:"managed"`
	Source  *okf.RegistrySource `json:"source,omitempty"`
}

func runRegistryList(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, registryListHelpText())
		return 0
	}
	fs := flag.NewFlagSet("registry list", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	jsonFlag := fs.Bool("json", false, "print versioned JSON inventory")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge registry list [--json]")
		return 2
	}

	entries, err := okf.RegistryEntries()
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if !*jsonFlag {
		printRegistryEntries(entries)
		return 0
	}
	registryPath, err := okf.RegistryFile()
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	report := registryListReport{
		SchemaVersion: okf.MachineSchemaVersion,
		Registry:      registryPath,
		Entries:       make([]registryListEntry, 0, len(entries)),
	}
	for _, entry := range entries {
		item := registryListEntry{
			Name:    entry.Name,
			Path:    entry.Path,
			Access:  registryEntryAccess(entry),
			Managed: entry.Managed,
		}
		if entry.Source != (okf.RegistrySource{}) {
			source := entry.Source
			item.Source = &source
		}
		report.Entries = append(report.Entries, item)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	fmt.Println(string(encoded))
	return 0
}

type registryStatusReport struct {
	SchemaVersion string                `json:"schemaVersion"`
	Registry      string                `json:"registry"`
	Summary       registryStatusSummary `json:"summary"`
	Entries       []registryEntryStatus `json:"entries"`
}

type registryStatusSummary struct {
	Total      int `json:"total"`
	OK         int `json:"ok"`
	Warnings   int `json:"warnings"`
	Unverified int `json:"unverified"`
	Modified   int `json:"modified"`
	Invalid    int `json:"invalid"`
	Missing    int `json:"missing"`
}

type registryEntryStatus struct {
	Name       string                   `json:"name"`
	Path       string                   `json:"path"`
	Access     string                   `json:"access"`
	Managed    bool                     `json:"managed"`
	State      string                   `json:"state"`
	Healthy    bool                     `json:"healthy"`
	Source     *okf.RegistrySource      `json:"source,omitempty"`
	Validation registryValidationStatus `json:"validation"`
	Identity   *registryIdentityStatus  `json:"identity,omitempty"`
	Problems   []string                 `json:"problems,omitempty"`
}

type registryValidationStatus struct {
	SpecVersion string `json:"specVersion"`
	Status      string `json:"status"`
	Errors      int    `json:"errors"`
	Warnings    int    `json:"warnings"`
}

type registryIdentityStatus struct {
	ExpectedContentSHA256 string `json:"expectedContentSha256,omitempty"`
	ActualContentSHA256   string `json:"actualContentSha256,omitempty"`
	ContentMatches        *bool  `json:"contentMatches,omitempty"`
	ExpectedGitCommit     string `json:"expectedGitCommit,omitempty"`
	ActualGitCommit       string `json:"actualGitCommit,omitempty"`
	GitCommitMatches      *bool  `json:"gitCommitMatches,omitempty"`
	GitDirty              *bool  `json:"gitDirty,omitempty"`
	ProvenanceMatches     *bool  `json:"provenanceMatches,omitempty"`
}

func runRegistryStatus(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, registryStatusHelpText())
		return 0
	}
	fs := flag.NewFlagSet("registry status", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	jsonFlag := fs.Bool("json", false, "print versioned JSON status")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge registry status [key|path] [--json]")
		return 2
	}

	entries, err := okf.RegistryEntries()
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if fs.NArg() == 1 {
		entry, ok, err := okf.ResolveRegistryTarget(fs.Arg(0))
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		if !ok {
			printUnknownConnection(fs.Arg(0))
			return 1
		}
		entries = []okf.RegistryEntry{entry}
	}

	registryPath, err := okf.RegistryFile()
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	report := registryStatusReport{
		SchemaVersion: okf.MachineSchemaVersion,
		Registry:      registryPath,
		Entries:       make([]registryEntryStatus, 0, len(entries)),
	}
	for _, entry := range entries {
		status := inspectRegistryEntry(entry)
		report.Entries = append(report.Entries, status)
		addRegistryStatusSummary(&report.Summary, status.State)
	}

	if *jsonFlag {
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		fmt.Println(string(encoded))
	} else {
		printRegistryStatus(report)
	}
	if report.Summary.Modified > 0 || report.Summary.Invalid > 0 || report.Summary.Missing > 0 || report.Summary.Unverified > 0 {
		return 1
	}
	return 0
}

func inspectRegistryEntry(entry okf.RegistryEntry) registryEntryStatus {
	return inspectRegistryEntryWithCacheLock(entry, false)
}

func inspectRegistryEntryWithCacheLock(entry okf.RegistryEntry, cacheLocked bool) registryEntryStatus {
	status := registryEntryStatus{
		Name:    entry.Name,
		Path:    entry.Path,
		Access:  registryEntryAccess(entry),
		Managed: entry.Managed,
		State:   "ok",
		Healthy: true,
		Validation: registryValidationStatus{
			SpecVersion: okf.LatestSpecVersion,
			Status:      "unknown",
		},
	}
	if entry.Managed {
		source := entry.Source
		status.Source = &source
		status.Identity = &registryIdentityStatus{
			ExpectedContentSHA256: entry.Source.ContentSHA256,
			ExpectedGitCommit:     entry.Source.GitCommit,
		}
		if resolved, ok := okf.ResolveSpecVersion(entry.Source.Spec); ok {
			status.Validation.SpecVersion = resolved
		} else if entry.Source.Spec != "" {
			status.Problems = append(status.Problems, fmt.Sprintf("unsupported recorded spec %q", entry.Source.Spec))
		}
	}

	if info, err := os.Stat(entry.Path); err != nil || !info.IsDir() {
		status.State = "missing"
		status.Healthy = false
		if err != nil {
			status.Problems = append(status.Problems, err.Error())
		} else {
			status.Problems = append(status.Problems, "registered path is not a directory")
		}
		return status
	}

	validation, err := okf.ValidateWithVersion(entry.Path, status.Validation.SpecVersion)
	if err != nil {
		status.Validation.Status = "error"
		status.Problems = append(status.Problems, err.Error())
	} else {
		status.Validation.Errors = len(validation.Errors)
		status.Validation.Warnings = len(validation.Warnings)
		for _, issue := range validation.Errors {
			status.Problems = append(status.Problems, formatRegistryValidationIssue("error", issue))
		}
		for _, issue := range validation.Warnings {
			status.Problems = append(status.Problems, formatRegistryValidationIssue("warning", issue))
		}
		switch {
		case len(validation.Errors) > 0:
			status.Validation.Status = "invalid"
		case len(validation.Warnings) > 0:
			status.Validation.Status = "warnings"
		default:
			status.Validation.Status = "valid"
		}
	}

	modified := false
	unverified := false
	if entry.Managed {
		managedRoot, rootErr := managedCacheRootForEntry(entry)
		if rootErr != nil {
			status.Problems = append(status.Problems, rootErr.Error())
			modified = true
		} else if info, statErr := os.Stat(managedRoot); statErr != nil || !info.IsDir() {
			if statErr != nil {
				status.Problems = append(status.Problems, statErr.Error())
			} else {
				status.Problems = append(status.Problems, "managed root is not a directory")
			}
			status.State = "missing"
			status.Healthy = false
			return status
		} else {
			inspectIdentity := func() {
				actual, hashErr := okf.DirectorySHA256(managedRoot)
				if hashErr != nil {
					status.Problems = append(status.Problems, hashErr.Error())
					modified = true
				} else {
					status.Identity.ActualContentSHA256 = actual
					if entry.Source.ContentSHA256 == "" {
						unverified = true
					} else {
						matches := strings.EqualFold(actual, entry.Source.ContentSHA256)
						status.Identity.ContentMatches = &matches
						modified = modified || !matches
					}
				}
				cachedSource, provenanceErr := loadRemoteCacheSource(managedRoot, entry.Source.URL)
				if provenanceErr != nil {
					status.Problems = append(status.Problems, provenanceErr.Error())
					modified = true
				} else {
					matches := cachedSource == entry.Source
					status.Identity.ProvenanceMatches = &matches
					modified = modified || !matches
				}
			}
			if cacheLocked {
				inspectIdentity()
			} else {
				unlock, lockErr := lockRemoteCache(managedRoot)
				if lockErr != nil {
					status.Problems = append(status.Problems, lockErr.Error())
					modified = true
				} else {
					inspectIdentity()
					if unlockErr := unlock(); unlockErr != nil {
						status.Problems = append(status.Problems, unlockErr.Error())
						modified = true
					}
				}
			}
		}

		if entry.Source.Type == "git" {
			actualCommit, commitErr := gitCommitForDirectory(entry.Path)
			if commitErr != nil {
				status.Problems = append(status.Problems, commitErr.Error())
				modified = true
			} else {
				status.Identity.ActualGitCommit = actualCommit
				if entry.Source.GitCommit == "" {
					unverified = true
				} else {
					matches := actualCommit == entry.Source.GitCommit
					status.Identity.GitCommitMatches = &matches
					modified = modified || !matches
				}
			}
			dirty, dirtyErr := gitWorkingTreeDirty(entry.Path)
			if dirtyErr != nil {
				status.Problems = append(status.Problems, dirtyErr.Error())
				modified = true
			} else {
				status.Identity.GitDirty = &dirty
				modified = modified || dirty
			}
		}
	}

	switch {
	case status.Validation.Status == "invalid" || status.Validation.Status == "error":
		status.State = "invalid"
		status.Healthy = false
	case modified:
		status.State = "modified"
		status.Healthy = false
	case unverified:
		status.State = "unverified"
		status.Healthy = false
	case status.Validation.Status == "warnings":
		status.State = "warnings"
	}
	return status
}

func formatRegistryValidationIssue(severity string, issue okf.Issue) string {
	location := issue.Path
	if issue.Line > 0 {
		location = fmt.Sprintf("%s:%d", location, issue.Line)
	}
	if location == "" {
		location = "bundle"
	}
	return fmt.Sprintf("validation %s at %s [%s]: %s", severity, location, issue.Rule, issue.Message)
}

func gitWorkingTreeDirty(root string) (bool, error) {
	command := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=all")
	output, err := command.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("could not inspect Git working tree: %s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)) != "", nil
}

func addRegistryStatusSummary(summary *registryStatusSummary, state string) {
	summary.Total++
	switch state {
	case "ok":
		summary.OK++
	case "warnings":
		summary.Warnings++
	case "unverified":
		summary.Unverified++
	case "modified":
		summary.Modified++
	case "invalid":
		summary.Invalid++
	case "missing":
		summary.Missing++
	}
}

func printRegistryStatus(report registryStatusReport) {
	terminal.title("Open Knowledge Registry Status", "offline cache and bundle integrity")
	fmt.Printf("%s %s\n\n", terminal.muted("config"), terminal.path(report.Registry))
	if len(report.Entries) == 0 {
		fmt.Println(terminal.muted("No registered knowledge bases."))
		return
	}
	for _, entry := range report.Entries {
		fmt.Printf("  %-10s %-18s %s\n", strings.ToUpper(entry.State), entry.Name, terminal.path(entry.Path))
		for _, problem := range entry.Problems {
			fmt.Printf("    %s %s\n", terminal.muted("-"), problem)
		}
	}
}

func runRegistryWhere(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, registryWhereHelpText())
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge registry where <name|path>")
		return 2
	}

	root, err := resolveWhereTarget(args[0])
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	fmt.Println(root)
	return 0
}

func resolveWhereTarget(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("name or path is required")
	}

	root, err := okf.ResolveKnowledgeRoot(value)
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	if okf.LooksLikePath(value) {
		return absolute, nil
	}
	if info, err := os.Stat(absolute); err == nil && info.IsDir() {
		return absolute, nil
	}
	if _, ok, err := okf.ResolveRegistryEntry(value); err != nil {
		return "", err
	} else if ok {
		return absolute, nil
	}
	return "", fmt.Errorf("unknown knowledge base: %s", value)
}

func runValidate(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, validateHelpText())
		return 0
	}
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	quiet := fs.Bool("quiet", false, "print only errors")
	specVersion := fs.String("spec", "latest", "OKF spec version")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write a machine-readable JSON report to this file")
	asJSON := fs.Bool("json", false, "print the machine-readable JSON report")
	ruleOverrides := stringListFlag{}
	fs.Var(&ruleOverrides, "rule", "override validation rule severity as rule=off|warn|error; may be repeated")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *asJSON {
		*format = "json"
	}
	*format = strings.TrimSpace(strings.ToLower(*format))
	if *format == "" {
		*format = "text"
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderrOutput(), "unsupported validate format: %s\n", *format)
		return 2
	}
	if *quiet && *format == "json" {
		fmt.Fprintln(stderrOutput(), "--quiet cannot be combined with JSON validation output")
		return 2
	}
	if strings.TrimSpace(*out) != "" && *format != "json" {
		fmt.Fprintln(stderrOutput(), "--out requires --format json or --json")
		return 2
	}

	root := "."
	if fs.NArg() > 1 {
		fmt.Fprintln(stderrOutput(), "validate accepts at most one key or path")
		return 2
	}
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}

	root, err := okf.ResolveKnowledgeRoot(root)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}

	validationOptions, err := okf.LoadValidationOptions(root)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	cliOptions := okf.ValidationOptions{}
	for _, override := range ruleOverrides {
		rule, severity, err := okf.ParseValidationRuleOverride(override)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 2
		}
		if err := okf.SetValidationRuleSeverity(&cliOptions, rule, severity); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 2
		}
	}
	validationOptions = okf.MergeValidationOptions(validationOptions, cliOptions)

	result, err := okf.ValidateWithVersionAndOptions(root, *specVersion, validationOptions)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}

	if *format == "json" {
		if err := printValidationJSONResult(result, strings.TrimSpace(*out)); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		if len(result.Errors) > 0 {
			return 1
		}
		return 0
	}

	if *quiet {
		for _, issue := range result.Errors {
			fmt.Fprintln(stderrOutput(), issue)
		}
		if len(result.Errors) > 0 {
			return 1
		}
		return 0
	}

	printValidationResult(result)
	if len(result.Errors) > 0 {
		return 1
	}
	return 0
}

func printValidationJSONResult(result okf.Result, out string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if out == "" {
		fmt.Print(string(data))
		return nil
	}
	if err := writeOutputFileAtomically(out, data); err != nil {
		return err
	}
	terminal.success("Wrote validation report")
	fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(result.Root))
	fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(out))
	return nil
}

func writeOutputFileAtomically(path string, data []byte) error {
	if err := atomic.WriteFile(path, bytes.NewReader(data)); err != nil {
		return err
	}
	return os.Chmod(path, 0644)
}

func printValidationResult(result okf.Result) {
	terminal.title("Open Knowledge Validate", "against Open Knowledge Format v"+result.SpecVersion)

	fmt.Printf("%s %s\n", terminal.muted("target"), terminal.path(result.Root))
	fmt.Printf("%s Open Knowledge Format v%s\n", terminal.muted("spec"), result.SpecVersion)
	fmt.Printf("%s %d markdown files, %d concepts, %d indexes, %d logs\n",
		terminal.muted("scan"), result.Files, result.Concepts, result.Indexes, result.Logs)
	fmt.Println()

	terminal.section("Checks")
	for _, check := range result.Checks {
		fmt.Printf("  %-4s %s\n", terminal.status(check.Status), check.Name)
		fmt.Printf("       %s\n", terminal.muted(check.Message))
	}

	if len(result.Errors) > 0 || len(result.Warnings) > 0 {
		fmt.Println()
		terminal.section("Issues")
		for _, issue := range result.Errors {
			fmt.Printf("  %s %s\n", terminal.red("error"), issue)
		}
		for _, issue := range result.Warnings {
			fmt.Printf("  %s %s\n", terminal.yellow("warning"), issue)
		}
	}

	if len(result.Errors) > 0 {
		fmt.Println()
		terminal.failure("Validation failed")
		return
	}

	fmt.Println()
	if len(result.Warnings) > 0 {
		terminal.success("Validation passed with warnings")
		return
	}
	terminal.success("Validation passed")
}

func runList(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, listHelpText())
		return 0
	}
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	asJSON := fs.Bool("json", false, "print JSON")
	specVersion := fs.String("spec", "latest", "OKF spec version")
	depth := fs.Int("depth", 0, "maximum tree depth; 0 means unlimited")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *depth < 0 {
		fmt.Fprintln(stderrOutput(), "--depth must be zero or a positive integer")
		return 2
	}

	root := "."
	if fs.NArg() > 1 {
		fmt.Fprintln(stderrOutput(), "list accepts at most one key or path")
		return 2
	}
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}

	root, err := okf.ResolveKnowledgeRoot(root)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}

	listing, err := okf.ListWithVersion(root, *specVersion)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}

	if *asJSON {
		listing.Entries = filterListEntriesByDepth(listing.Entries, *depth)
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(listing); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 2
		}
		return 0
	}

	printListTree(listing, *depth)
	return 0
}

func runExport(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, exportHelpText())
		return 0
	}

	switch args[0] {
	case "html":
		return runExportHTML(args[1:])
	case "json":
		return runExportJSON(args[1:])
	case "tar":
		return runExportTar(args[1:])
	case "graph":
		return runExportGraph(args[1:])
	default:
		fmt.Fprintf(stderrOutput(), "unknown export target: %s\n\n", args[0])
		fmt.Fprint(stderrOutput(), exportHelpText())
		return 2
	}
}

type exportOptions struct {
	path       string
	out        string
	spec       string
	graphType  string
	plain      bool
	headHTML   string
	headFile   string
	scriptSrcs []string
}

func runExportHTML(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, exportHTMLHelpText())
		return 0
	}
	options, err := parseExportOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if options.out == "" {
		fmt.Fprintln(stderrOutput(), "openknowledge export html requires --out <folder>")
		return 2
	}
	if options.plain {
		if flag := options.headFlag(); flag != "" {
			fmt.Fprintf(stderrOutput(), "%s requires the default viewer export; remove --plain\n", flag)
			return 2
		}
	}
	if options.graphType != "" {
		fmt.Fprintln(stderrOutput(), "unknown flag: --type")
		return 2
	}

	var result okf.HTMLResult
	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if options.plain {
		result, err = okf.WritePlainHTMLWithVersion(root, options.out, options.spec)
	} else {
		headInjection, loadErr := loadHeadInjection(options.headInjectionOptions())
		if loadErr != nil {
			fmt.Fprintln(stderrOutput(), loadErr)
			return 2
		}
		result, err = writeViewerHTMLWithOptions(root, options.out, options.spec, viewerHTMLExportOptions{HeadHTML: headInjection})
	}
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}

	terminal.success("Exported HTML")
	fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(result.Root))
	fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(result.Out))
	fmt.Printf("%s %d files\n", terminal.muted("wrote"), len(result.Written))
	return 0
}

func runExportJSON(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, exportJSONHelpText())
		return 0
	}
	options, err := parseExportOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if options.plain {
		fmt.Fprintln(stderrOutput(), "unknown flag: --plain")
		return 2
	}
	if options.graphType != "" {
		fmt.Fprintln(stderrOutput(), "unknown flag: --type")
		return 2
	}
	if flag := options.headFlag(); flag != "" {
		fmt.Fprintf(stderrOutput(), "unknown flag: %s\n", flag)
		return 2
	}

	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}

	bundle, err := okf.ParseBundleWithVersion(root, options.spec)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	data = append(data, '\n')

	if options.out != "" {
		if err := writeOutputFileAtomically(options.out, data); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		terminal.success("Exported JSON")
		fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(bundle.Root))
		fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(options.out))
		return 0
	}

	fmt.Print(string(data))
	return 0
}

func runExportTar(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, exportTarHelpText())
		return 0
	}
	options, err := parseExportOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if options.plain {
		fmt.Fprintln(stderrOutput(), "unknown flag: --plain")
		return 2
	}
	if options.graphType != "" {
		fmt.Fprintln(stderrOutput(), "unknown flag: --type")
		return 2
	}
	if flag := options.headFlag(); flag != "" {
		fmt.Fprintf(stderrOutput(), "unknown flag: %s\n", flag)
		return 2
	}
	if options.out == "" {
		fmt.Fprintln(stderrOutput(), "openknowledge export tar requires --out <file>")
		return 2
	}

	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	result, err := okf.WriteBundleTarGzipWithVersion(root, options.out, options.spec, nil)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}

	terminal.success("Exported TAR")
	fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(result.Root))
	fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(result.Out))
	fmt.Printf("%s %s\n", terminal.muted("sha256"), result.SHA256)
	return 0
}

// graph export has two shapes: source preserves the original file/link graph,
// while search adds derivative chunk nodes for retrieval and visualization.
func runExportGraph(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, exportGraphHelpText())
		return 0
	}
	options, err := parseExportOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if options.plain {
		fmt.Fprintln(stderrOutput(), "unknown flag: --plain")
		return 2
	}
	if flag := options.headFlag(); flag != "" {
		fmt.Fprintf(stderrOutput(), "unknown flag: %s\n", flag)
		return 2
	}

	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}

	graph, err := okf.BuildGraphWithType(root, options.spec, options.graphType)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}

	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	data = append(data, '\n')

	if options.out != "" {
		if err := writeOutputFileAtomically(options.out, data); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		terminal.success("Exported graph")
		fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(graph.Root))
		fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(options.out))
		return 0
	}

	fmt.Print(string(data))
	return 0
}

func parseExportOptions(args []string) (exportOptions, error) {
	options := exportOptions{path: ".", spec: "latest"}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--out":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return exportOptions{}, fmt.Errorf("--out requires a value")
			}
			options.out = args[index]
		case strings.HasPrefix(arg, "--out="):
			options.out = strings.TrimPrefix(arg, "--out=")
			if strings.TrimSpace(options.out) == "" {
				return exportOptions{}, fmt.Errorf("--out requires a value")
			}
		case arg == "--spec":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return exportOptions{}, fmt.Errorf("--spec requires a value")
			}
			options.spec = args[index]
		case strings.HasPrefix(arg, "--spec="):
			options.spec = strings.TrimPrefix(arg, "--spec=")
			if strings.TrimSpace(options.spec) == "" {
				return exportOptions{}, fmt.Errorf("--spec requires a value")
			}
		case arg == "--type":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return exportOptions{}, fmt.Errorf("--type requires a value")
			}
			options.graphType = args[index]
		case strings.HasPrefix(arg, "--type="):
			options.graphType = strings.TrimPrefix(arg, "--type=")
			if strings.TrimSpace(options.graphType) == "" {
				return exportOptions{}, fmt.Errorf("--type requires a value")
			}
		case arg == "--plain":
			options.plain = true
		case arg == "--head-file":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return exportOptions{}, fmt.Errorf("--head-file requires a value")
			}
			options.headFile = args[index]
		case strings.HasPrefix(arg, "--head-file="):
			options.headFile = strings.TrimPrefix(arg, "--head-file=")
			if strings.TrimSpace(options.headFile) == "" {
				return exportOptions{}, fmt.Errorf("--head-file requires a value")
			}
		case arg == "--head-html":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return exportOptions{}, fmt.Errorf("--head-html requires a value")
			}
			options.headHTML = args[index]
		case strings.HasPrefix(arg, "--head-html="):
			options.headHTML = strings.TrimPrefix(arg, "--head-html=")
			if strings.TrimSpace(options.headHTML) == "" {
				return exportOptions{}, fmt.Errorf("--head-html requires a value")
			}
		case arg == "--script-src":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return exportOptions{}, fmt.Errorf("--script-src requires a value")
			}
			options.scriptSrcs = append(options.scriptSrcs, args[index])
		case strings.HasPrefix(arg, "--script-src="):
			src := strings.TrimPrefix(arg, "--script-src=")
			if strings.TrimSpace(src) == "" {
				return exportOptions{}, fmt.Errorf("--script-src requires a value")
			}
			options.scriptSrcs = append(options.scriptSrcs, src)
		case strings.HasPrefix(arg, "-"):
			return exportOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			if options.path != "." {
				return exportOptions{}, fmt.Errorf("to accepts at most one path")
			}
			options.path = arg
		}
	}
	return options, nil
}

func (options exportOptions) headFlag() string {
	if options.headFile != "" {
		return "--head-file"
	}
	if options.headHTML != "" {
		return "--head-html"
	}
	if len(options.scriptSrcs) > 0 {
		return "--script-src"
	}
	return ""
}

func (options exportOptions) headInjectionOptions() headInjectionOptions {
	headHTML := options.headHTML
	if strings.TrimSpace(headHTML) == "" {
		headHTML = os.Getenv("OPENKNOWLEDGE_HEAD_HTML")
	}
	headFile := options.headFile
	if strings.TrimSpace(headFile) == "" {
		headFile = os.Getenv("OPENKNOWLEDGE_HEAD_FILE")
	}
	scriptSrcs := append([]string{}, splitHeadList(os.Getenv("OPENKNOWLEDGE_SCRIPT_SRC"))...)
	scriptSrcs = append(scriptSrcs, options.scriptSrcs...)
	return headInjectionOptions{
		HTML:       headHTML,
		File:       headFile,
		ScriptSrcs: scriptSrcs,
	}
}

func runVersion(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, versionHelpText())
		return 0
	}
	if len(args) != 0 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge version")
		return 2
	}
	fmt.Println(version)
	return 0
}

type listTreeNode struct {
	name     string
	entry    *okf.ListEntry
	children map[string]*listTreeNode
}

func printListTree(listing okf.ListResult, depth int) {
	terminal.title("Open Knowledge List", "bundle tree")
	fmt.Printf("%s %s\n", terminal.muted("target"), terminal.path(listing.Root))
	if depth > 0 {
		fmt.Printf("%s %d\n", terminal.muted("depth"), depth)
	}
	fmt.Println()

	root := &listTreeNode{children: make(map[string]*listTreeNode)}
	for _, entry := range listing.Entries {
		addListEntry(root, entry)
	}

	name := filepath.Base(filepath.Clean(listing.Root))
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = listing.Root
	}
	fmt.Println(terminal.path(name) + "/")

	children := sortedListChildren(root)
	if len(children) == 0 {
		fmt.Printf("  %s\n", terminal.muted("(empty)"))
		return
	}
	printListChildren(children, "", depth, 1)
}

func addListEntry(root *listTreeNode, entry okf.ListEntry) {
	current := root
	parts := strings.Split(entry.Path, "/")
	for index, part := range parts {
		child, ok := current.children[part]
		if !ok {
			child = &listTreeNode{name: part, children: make(map[string]*listTreeNode)}
			current.children[part] = child
		}
		if index == len(parts)-1 {
			entryCopy := entry
			child.entry = &entryCopy
		}
		current = child
	}
}

func printListChildren(nodes []*listTreeNode, prefix string, maxDepth int, currentDepth int) {
	for index, node := range nodes {
		last := index == len(nodes)-1
		connector := "|-- "
		nextPrefix := prefix + "|   "
		if last {
			connector = "`-- "
			nextPrefix = prefix + "    "
		}
		fmt.Println(prefix + connector + formatListNode(node))
		if len(node.children) > 0 && (maxDepth == 0 || currentDepth < maxDepth) {
			printListChildren(sortedListChildren(node), nextPrefix, maxDepth, currentDepth+1)
		}
	}
}

func filterListEntriesByDepth(entries []okf.ListEntry, maxDepth int) []okf.ListEntry {
	if maxDepth == 0 {
		return entries
	}
	filtered := make([]okf.ListEntry, 0, len(entries))
	for _, entry := range entries {
		if listPathDepth(entry.Path) <= maxDepth {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func listPathDepth(path string) int {
	path = strings.Trim(strings.TrimSpace(filepath.ToSlash(path)), "/")
	if path == "" {
		return 0
	}
	return len(strings.Split(path, "/"))
}

func sortedListChildren(node *listTreeNode) []*listTreeNode {
	children := make([]*listTreeNode, 0, len(node.children))
	for _, child := range node.children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		leftDir := children[i].entry == nil
		rightDir := children[j].entry == nil
		if leftDir != rightDir {
			return leftDir
		}
		return strings.ToLower(children[i].name) < strings.ToLower(children[j].name)
	})
	return children
}

func formatListNode(node *listTreeNode) string {
	if node.entry == nil {
		return terminal.path(node.name + "/")
	}

	entry := *node.entry
	if len(entry.Issues) > 0 {
		return terminal.red(node.name) + terminal.red("  "+entry.Issues[0].Message)
	}
	if entry.Reserved {
		return terminal.muted(node.name + "  " + entry.Kind)
	}
	if entry.Kind == "asset" {
		return node.name + terminal.muted("  asset")
	}

	meta := entry.Type
	if entry.Title != "" {
		if meta != "" {
			meta += "  "
		}
		meta += entry.Title
	}
	if meta == "" {
		return node.name
	}
	return node.name + terminal.muted("  "+meta)
}

func usage() {
	fmt.Fprint(stderrOutput(), helpText())
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if isHelpFlag(arg) {
			return true
		}
	}
	return false
}

func isHelpFlag(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "-help"
}

func prompt(label, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	} else {
		fmt.Printf("%s: ", label)
	}

	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil && len(value) == 0 {
		if defaultValue != "" {
			return defaultValue, nil
		}
		return "", fmt.Errorf("%s is required", strings.ToLower(label))
	}

	value = strings.TrimSpace(value)
	if value == "" {
		value = defaultValue
	}
	if value == "" {
		return "", fmt.Errorf("%s is required", strings.ToLower(label))
	}
	return value, nil
}

func titleFromPath(path string) string {
	base := filepath.Base(filepath.Clean(path))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.TrimSpace(base)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return ""
	}

	words := strings.Fields(base)
	for index, word := range words {
		if len(word) == 0 {
			continue
		}
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
