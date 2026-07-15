package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/natefinch/atomic"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

var version = "0.6.0"

var terminal = newTerminal(os.Stdout)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "--help", "-h":
		fmt.Fprint(os.Stdout, helpText())
	case "setup":
		os.Exit(runSetup(os.Args[2:]))
	case "from":
		os.Exit(runFrom(os.Args[2:]))
	case "rules":
		os.Exit(runRules(os.Args[2:]))
	case "review":
		os.Exit(runReview(os.Args[2:]))
	case "agents":
		os.Exit(runAgents(os.Args[2:]))
	case "new":
		os.Exit(runNew(os.Args[2:]))
	case "connect":
		os.Exit(runConnect(os.Args[2:], "openknowledge connect"))
	case "disconnect":
		os.Exit(runDisconnect(os.Args[2:], "openknowledge disconnect"))
	case "get":
		os.Exit(runGet(os.Args[2:]))
	case "search":
		os.Exit(runSearch(os.Args[2:]))
	case "mcp":
		os.Exit(runMCP(os.Args[2:]))
	case "ast":
		os.Exit(runAST(os.Args[2:]))
	case "registry":
		os.Exit(runRegistry(os.Args[2:]))
	case "view":
		os.Exit(runView(os.Args[2:]))
	case "to":
		os.Exit(runTo(os.Args[2:]))
	case "spec":
		os.Exit(runSpec(os.Args[2:]))
	case "validate":
		os.Exit(runValidate(os.Args[2:]))
	case "list":
		os.Exit(runList(os.Args[2:]))
	case "version":
		os.Exit(runVersion(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func runSetup(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, setupHelpText())
		return 0
	}
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var rules string
	fs.StringVar(&rules, "rules", "", "suggest comma-separated maintenance rules for setup")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "setup accepts no positional arguments")
		return 2
	}

	ruleIDs, err := parseRuleIDs(rules)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	prompt, err := okf.SetupPromptWithOptions(okf.SetupPromptOptions{Rules: ruleIDs})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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

func runFrom(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, fromHelpText())
		return 0
	}
	options, err := parseFromOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintln(os.Stderr, err)
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
			return fromOptions{}, fmt.Errorf("unknown from option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		return fromOptions{}, fmt.Errorf("usage: openknowledge from <source> --out <path>")
	}
	options.source = positionals[0]
	if strings.TrimSpace(options.out) == "" {
		return fromOptions{}, fmt.Errorf("from requires --out <path>")
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
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.list {
		output, err := okf.RenderRulesListForWiki(options.wiki)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	printRulesWikiWarnings(options.wiki)

	targetFile, err := resolveRulesApplyFile(options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	block := okf.RenderManagedRulesBlock(rules)
	if options.dryRun {
		fmt.Printf("Would update %s with:\n\n%s", targetFile, block)
		return 0
	}
	if err := okf.RequireRegistryWriteAccess(targetFile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	existingBytes, err := os.ReadFile(targetFile)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err == nil && !options.yes && isTerminalFile(os.Stdin) {
		confirmed, err := confirmRulesApplyWrite(targetFile, string(existingBytes), block)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if !confirmed {
			fmt.Fprintln(os.Stdout, "Cancelled.")
			return 0
		}
	}
	updated := okf.UpsertManagedRulesBlock(string(existingBytes), block)
	if err := os.MkdirAll(filepath.Dir(targetFile), 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(targetFile, []byte(updated), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintf(os.Stderr, "unknown review subcommand: %s\n", args[0])
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
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	output, err := okf.RenderRuleReviewPrompt(okf.RuleReviewOptions{
		Wiki:  options.wiki,
		Rules: options.rules,
		All:   options.all,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
	output := os.Stderr
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

func printWarning(output *os.File, message string) {
	fmt.Fprintln(output)
	fmt.Fprintln(output, warningText(output, message))
	fmt.Fprintln(output)
}

func warningText(output *os.File, message string) string {
	label := "⚠ Warning:"
	text := label + " " + strings.TrimSpace(message)
	return newTerminal(output).yellow(text)
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
		fmt.Fprintln(os.Stderr, "usage: openknowledge spec latest|<version>")
		return 2
	}

	version, ok := okf.ResolveSpecVersion(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "unsupported OKF spec version: %s\n", args[0])
		return 2
	}

	spec := okf.Spec(version)
	fmt.Print(spec)
	if !strings.HasSuffix(spec, "\n") {
		fmt.Println()
	}
	return 0
}

func runNew(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, newHelpText())
		return 0
	}
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
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
		fmt.Fprintln(os.Stderr, "new accepts at most one folder path")
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
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	entries, err := parseBundleEntryFlags(bundleEntries)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintln(os.Stderr, err)
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
		fmt.Println("  run openknowledge validate, and show me how to inspect it with openknowledge view.")
	}
	return 0
}

func runRegistry(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, registryHelpText())
		return 0
	}

	switch args[0] {
	case "list":
		return runRegistryList(args[1:])
	case "connect":
		return runConnect(args[1:], "openknowledge registry connect")
	case "disconnect":
		return runDisconnect(args[1:], "openknowledge registry disconnect")
	case "refresh":
		return runRegistryRefresh(args[1:])
	case "status":
		return runRegistryStatus(args[1:])
	case "where":
		return runRegistryWhere(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown registry command: %s\n\n", args[0])
		fmt.Fprint(os.Stderr, registryHelpText())
		return 2
	}
}

type stringListFlag []string

func (flag *stringListFlag) String() string {
	return strings.Join(*flag, ",")
}

func (flag *stringListFlag) Set(value string) error {
	*flag = append(*flag, value)
	return nil
}

func parseBundleEntryFlags(values []string) ([]okf.BundleEntry, error) {
	entries := make([]okf.BundleEntry, 0, len(values))
	for _, value := range values {
		name, path, ok := strings.Cut(value, "=")
		if !ok {
			return nil, fmt.Errorf("bundle entry must use name=path: %s", value)
		}
		entries = append(entries, okf.BundleEntry{Name: name, Path: path})
	}
	return entries, nil
}

func runConnect(args []string, command string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, connectHelpText(command))
		return 0
	}
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	keyFlag := fs.String("as", "", "connection key")
	accessFlag := fs.String("access", "read", "connection access: read or write")
	noValidateFlag := fs.Bool("no-validate", false, "skip validation status")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s <source> [--as <key>]\n", command)
		return 2
	}

	source := fs.Arg(0)
	access := strings.TrimSpace(*accessFlag)
	if access != "read" && access != "write" {
		fmt.Fprintln(os.Stderr, "access must be read or write")
		return 2
	}
	if access == "write" && looksLikeRemoteSource(source) {
		fmt.Fprintln(os.Stderr, "managed remote connections are read-only")
		return 2
	}
	sourceInfo := okf.RegistrySource{}
	if looksLikeRemoteSource(source) {
		var err error
		var materializedRoot string
		materializedRoot, sourceInfo, err = materializeRemoteSource(source)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		source = materializedRoot
	}

	root, err := okf.ResolveKnowledgeRoot(source)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	root, err = filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if info, err := os.Stat(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	} else if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "%s is not a directory\n", root)
		return 1
	}

	bundleInfo, metadataErr := okf.ReadBundleInfo(root)
	key := strings.TrimSpace(*keyFlag)
	explicitKey := key != ""
	if key == "" {
		key = bundleInfo.Metadata.Name
	}
	if key == "" {
		key = filepath.Base(filepath.Clean(root))
	}

	entry, warning, err := okf.ConnectRegistryEntryWithSource(key, root, access, explicitKey, sourceInfo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	status := "unknown"
	if !*noValidateFlag {
		status = bundleValidationStatus(entry.Path)
	}

	printConnectResult(entry, bundleInfo, status)
	if warning != "" {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	if metadataErr != nil {
		fmt.Fprintf(os.Stderr, "warning: bundle metadata could not be read: %v\n", metadataErr)
	}
	return 0
}

func bundleValidationStatus(root string) string {
	result, err := okf.Validate(root)
	if err != nil {
		return "unknown"
	}
	if len(result.Errors) > 0 {
		return "invalid"
	}
	if len(result.Warnings) > 0 {
		return "warnings"
	}
	return "valid"
}

func printConnectResult(entry okf.RegistryEntry, info okf.BundleInfo, status string) {
	terminal.success("Connected knowledge bundle")
	fmt.Printf("%-8s %s\n", "key", entry.Name)
	fmt.Printf("%-8s %s\n", "name", info.DisplayName())
	fmt.Printf("%-8s %s\n", "path", terminal.path(entry.Path))
	fmt.Printf("%-8s %s\n", "access", registryEntryAccess(entry))
	fmt.Printf("%-8s %s\n", "status", status)
	if info.Metadata.Purpose != "" {
		fmt.Printf("%-8s %s\n", "purpose", info.Metadata.Purpose)
	}
	if names := info.EntryNames(); len(names) > 0 {
		fmt.Printf("%-8s %s\n", "entries", strings.Join(names, ", "))
	}
	if !info.HasMetadata {
		fmt.Printf("%-8s %s\n", "metadata", "none")
	}
}

func registryEntryAccess(entry okf.RegistryEntry) string {
	if entry.Access != "" {
		return entry.Access
	}
	return "read"
}

func looksLikeRemoteSource(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(value, "http://") ||
		strings.HasPrefix(value, "https://") ||
		strings.HasPrefix(value, "file://") ||
		strings.HasPrefix(value, "git://") ||
		strings.HasPrefix(value, "ssh://") ||
		strings.HasPrefix(value, "git@")
}

func materializeRemoteSource(source string) (root string, sourceInfo okf.RegistrySource, resultErr error) {
	source = strings.TrimSpace(source)
	cacheRoot, err := remoteBundleCacheRoot()
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	target := filepath.Join(cacheRoot, registryCacheName(source))
	if registeredTarget, ok := registeredRemoteCacheTarget(source); ok {
		target = registeredTarget
	}
	return materializeRemoteSourceAtTarget(source, target, true)
}

func materializeRemoteSourceAtTarget(source string, target string, reuseCache bool) (root string, sourceInfo okf.RegistrySource, resultErr error) {
	source = strings.TrimSpace(source)
	cacheRoot := filepath.Dir(target)
	if err := os.MkdirAll(cacheRoot, 0700); err != nil {
		return "", okf.RegistrySource{}, err
	}
	if err := os.Chmod(cacheRoot, 0700); err != nil {
		return "", okf.RegistrySource{}, err
	}
	unlock, err := lockRemoteCache(target)
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	defer func() {
		if err := unlock(); err != nil && resultErr == nil {
			root = ""
			sourceInfo = okf.RegistrySource{}
			resultErr = err
		}
	}()

	if root, ok := cachedBundleRoot(target); reuseCache && ok {
		cachedSource, err := loadRemoteCacheSource(target, source)
		if err == nil {
			return root, cachedSource, nil
		}
		if !os.IsNotExist(err) {
			return "", okf.RegistrySource{}, err
		}
		legacySource := okf.RegistrySource{
			Type:        legacyRemoteSourceType(source, target),
			URL:         source,
			ManagedRoot: target,
		}
		if err := saveRemoteCacheSource(target, legacySource); err != nil {
			return "", okf.RegistrySource{}, err
		}
		return root, legacySource, nil
	}
	if looksLikeManifestSource(source) {
		archive, manifestURL, spec, err := materializeManifestSource(source, target)
		if err != nil {
			return "", okf.RegistrySource{}, err
		}
		return finishRemoteMaterialization(archive.Root, target, okf.RegistrySource{
			Type:          "manifest",
			URL:           source,
			Ref:           archive.FinalURL,
			ResolvedURL:   manifestURL,
			ManifestURL:   manifestURL,
			ArchiveURL:    archive.FinalURL,
			SHA256:        archive.SHA256,
			ContentSHA256: archive.ContentSHA256,
			Spec:          spec,
			FetchedAt:     remoteFetchTimestamp(),
			ManagedRoot:   target,
		})
	}
	if looksLikeArchiveSource(source) {
		archive, err := materializeArchiveSource(source, target, "", okf.LatestSpecVersion, false)
		if err != nil {
			return "", okf.RegistrySource{}, err
		}
		return finishRemoteMaterialization(archive.Root, target, okf.RegistrySource{
			Type:          "tar",
			URL:           source,
			Ref:           archive.FinalURL,
			ResolvedURL:   archive.FinalURL,
			ArchiveURL:    archive.FinalURL,
			SHA256:        archive.SHA256,
			ContentSHA256: archive.ContentSHA256,
			Spec:          okf.LatestSpecVersion,
			FetchedAt:     remoteFetchTimestamp(),
			ManagedRoot:   target,
		})
	}
	if isHTTPSource(source) {
		for _, candidate := range manifestCandidateURLs(source) {
			manifest, manifestURL, ok, err := fetchBundleManifest(candidate)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			if !ok {
				continue
			}
			archiveURL, err := resolveManifestArchiveURL(manifestURL, manifest.Archive)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			archive, err := materializeArchiveSource(archiveURL, target, manifest.ArchiveSHA256, manifest.Spec, true)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			return finishRemoteMaterialization(archive.Root, target, okf.RegistrySource{
				Type:          "manifest",
				URL:           source,
				Ref:           archive.FinalURL,
				ResolvedURL:   manifestURL,
				ManifestURL:   manifestURL,
				ArchiveURL:    archive.FinalURL,
				SHA256:        archive.SHA256,
				ContentSHA256: archive.ContentSHA256,
				Spec:          manifest.Spec,
				FetchedAt:     remoteFetchTimestamp(),
				ManagedRoot:   target,
			})
		}
		if archive, ok, err := tryMaterializeDirectArchive(source, target); err != nil {
			return "", okf.RegistrySource{}, err
		} else if ok {
			return finishRemoteMaterialization(archive.Root, target, okf.RegistrySource{
				Type:          "tar",
				URL:           source,
				Ref:           archive.FinalURL,
				ResolvedURL:   archive.FinalURL,
				ArchiveURL:    archive.FinalURL,
				SHA256:        archive.SHA256,
				ContentSHA256: archive.ContentSHA256,
				Spec:          okf.LatestSpecVersion,
				FetchedAt:     remoteFetchTimestamp(),
				ManagedRoot:   target,
			})
		}
	}

	stagingParent, err := os.MkdirTemp(cacheRoot, ".openknowledge-git-*")
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	defer os.RemoveAll(stagingParent)
	staging := filepath.Join(stagingParent, "bundle")
	cmd := exec.Command("git", "clone", "--depth", "1", source, staging)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return "", okf.RegistrySource{}, fmt.Errorf("could not clone remote bundle %s: %s", source, detail)
	}
	if _, valid, err := validateExtractedBundleCandidate(staging, okf.LatestSpecVersion, false); err != nil {
		return "", okf.RegistrySource{}, err
	} else if !valid {
		return "", okf.RegistrySource{}, fmt.Errorf("Git source does not contain a valid Open Knowledge bundle: %s", source)
	}
	commit, err := gitCommitForDirectory(staging)
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	contentSHA256, err := okf.DirectorySHA256(staging)
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	if err := publishRemoteCache(staging, target); err != nil {
		return "", okf.RegistrySource{}, err
	}
	return finishRemoteMaterialization(target, target, okf.RegistrySource{
		Type:          "git",
		URL:           source,
		ResolvedURL:   source,
		GitCommit:     commit,
		ContentSHA256: contentSHA256,
		Spec:          okf.LatestSpecVersion,
		FetchedAt:     remoteFetchTimestamp(),
		ManagedRoot:   target,
	})
}

type archiveMaterialization struct {
	Root          string
	FinalURL      string
	SHA256        string
	ContentSHA256 string
}

func materializeManifestSource(source string, target string) (archiveMaterialization, string, string, error) {
	manifest, manifestURL, ok, err := fetchBundleManifest(source)
	if err != nil {
		return archiveMaterialization{}, "", "", err
	}
	if !ok {
		return archiveMaterialization{}, "", "", fmt.Errorf("Open Knowledge manifest not found: %s", source)
	}
	archiveURL, err := resolveManifestArchiveURL(manifestURL, manifest.Archive)
	if err != nil {
		return archiveMaterialization{}, "", "", err
	}
	archive, err := materializeArchiveSource(archiveURL, target, manifest.ArchiveSHA256, manifest.Spec, true)
	if err != nil {
		return archiveMaterialization{}, "", "", err
	}
	return archive, manifestURL, manifest.Spec, nil
}

func materializeArchiveSource(source string, target string, expectedSHA256 string, specVersion string, requireDeclaredSpec bool) (archiveMaterialization, error) {
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-source-*")
	if err != nil {
		return archiveMaterialization{}, err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "bundle.tar.gz")
	download, err := downloadRemoteFile(source, archivePath, okf.MaxBundleArchiveBytes)
	if err != nil {
		return archiveMaterialization{}, err
	}
	if !looksLikeArchiveSource(source) && !downloadedFileLooksLikeArchive(archivePath, download.ContentType) {
		return archiveMaterialization{}, fmt.Errorf("remote source is not a tar archive: %s", source)
	}
	actual, err := okf.SHA256File(archivePath)
	if err != nil {
		return archiveMaterialization{}, err
	}
	if strings.TrimSpace(expectedSHA256) != "" {
		if !strings.EqualFold(actual, strings.TrimSpace(expectedSHA256)) {
			return archiveMaterialization{}, fmt.Errorf("archive checksum mismatch for %s", source)
		}
	}

	extractRoot := filepath.Join(tempDir, "extract")
	if err := okf.ExtractBundleArchive(archivePath, extractRoot); err != nil {
		return archiveMaterialization{}, err
	}
	bundleRoot, err := validatedExtractedBundleRoot(extractRoot, specVersion, requireDeclaredSpec)
	if err != nil {
		return archiveMaterialization{}, err
	}
	contentSHA256, err := okf.DirectorySHA256(extractRoot)
	if err != nil {
		return archiveMaterialization{}, err
	}
	if err := publishRemoteCache(extractRoot, target); err != nil {
		return archiveMaterialization{}, err
	}
	result := archiveMaterialization{Root: target, FinalURL: download.FinalURL, SHA256: actual, ContentSHA256: contentSHA256}
	if bundleRoot == extractRoot {
		return result, nil
	}
	rel, err := filepath.Rel(extractRoot, bundleRoot)
	if err != nil {
		return archiveMaterialization{}, err
	}
	result.Root = filepath.Join(target, rel)
	return result, nil
}

func tryMaterializeDirectArchive(source string, target string) (archiveMaterialization, bool, error) {
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-probe-*")
	if err != nil {
		return archiveMaterialization{}, false, err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "probe")
	download, err := downloadRemoteFile(source, archivePath, okf.MaxBundleArchiveBytes)
	if err != nil {
		return archiveMaterialization{}, false, nil
	}
	if !downloadedFileLooksLikeArchive(archivePath, download.ContentType) {
		return archiveMaterialization{}, false, nil
	}
	archive, err := materializeArchiveFile(archivePath, target, "", okf.LatestSpecVersion, false)
	if err != nil {
		return archiveMaterialization{}, false, err
	}
	archive.FinalURL = download.FinalURL
	return archive, true, nil
}

func materializeArchiveFile(archivePath string, target string, expectedSHA256 string, specVersion string, requireDeclaredSpec bool) (archiveMaterialization, error) {
	actual, err := okf.SHA256File(archivePath)
	if err != nil {
		return archiveMaterialization{}, err
	}
	if strings.TrimSpace(expectedSHA256) != "" {
		if !strings.EqualFold(actual, strings.TrimSpace(expectedSHA256)) {
			return archiveMaterialization{}, fmt.Errorf("archive checksum mismatch")
		}
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-extract-*")
	if err != nil {
		return archiveMaterialization{}, err
	}
	defer os.RemoveAll(tempDir)
	extractRoot := filepath.Join(tempDir, "extract")
	if err := okf.ExtractBundleArchive(archivePath, extractRoot); err != nil {
		return archiveMaterialization{}, err
	}
	bundleRoot, err := validatedExtractedBundleRoot(extractRoot, specVersion, requireDeclaredSpec)
	if err != nil {
		return archiveMaterialization{}, err
	}
	contentSHA256, err := okf.DirectorySHA256(extractRoot)
	if err != nil {
		return archiveMaterialization{}, err
	}
	if err := publishRemoteCache(extractRoot, target); err != nil {
		return archiveMaterialization{}, err
	}
	result := archiveMaterialization{Root: target, SHA256: actual, ContentSHA256: contentSHA256}
	if bundleRoot == extractRoot {
		return result, nil
	}
	rel, err := filepath.Rel(extractRoot, bundleRoot)
	if err != nil {
		return archiveMaterialization{}, err
	}
	result.Root = filepath.Join(target, rel)
	return result, nil
}

type remoteDownload struct {
	ContentType string
	FinalURL    string
}

type remoteHTTPStatusError struct {
	URL        string
	Status     string
	StatusCode int
}

var remoteHTTPClient = &http.Client{Timeout: 30 * time.Second}

func (err *remoteHTTPStatusError) Error() string {
	return fmt.Sprintf("GET %s returned %s", err.URL, err.Status)
}

func fetchBundleManifest(source string) (okf.BundleManifest, string, bool, error) {
	tempDir, err := os.MkdirTemp("", "openknowledge-manifest-*")
	if err != nil {
		return okf.BundleManifest{}, "", false, err
	}
	defer os.RemoveAll(tempDir)
	manifestPath := filepath.Join(tempDir, "openknowledge.json")
	download, err := downloadRemoteFile(source, manifestPath, okf.MaxBundleManifestBytes)
	if err != nil {
		var statusError *remoteHTTPStatusError
		if os.IsNotExist(err) || (errors.As(err, &statusError) && statusError.StatusCode == http.StatusNotFound) {
			return okf.BundleManifest{}, "", false, nil
		}
		return okf.BundleManifest{}, "", false, err
	}
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return okf.BundleManifest{}, "", false, err
	}
	manifest, err := okf.DecodeBundleManifest(content)
	if err != nil {
		return okf.BundleManifest{}, "", false, fmt.Errorf("invalid Open Knowledge manifest at %s: %w", download.FinalURL, err)
	}
	return manifest, download.FinalURL, true, nil
}

func downloadRemoteFile(source string, target string, maxBytes int64) (remoteDownload, error) {
	if maxBytes <= 0 {
		return remoteDownload{}, fmt.Errorf("download byte limit must be positive")
	}
	parsed, err := url.Parse(source)
	if err != nil {
		return remoteDownload{}, err
	}
	if parsed.Scheme == "file" {
		inputPath, err := url.PathUnescape(parsed.Path)
		if err != nil {
			return remoteDownload{}, err
		}
		input, err := os.Open(inputPath)
		if err != nil {
			return remoteDownload{}, err
		}
		defer input.Close()
		if err := writeLimitedDownload(input, target, maxBytes); err != nil {
			return remoteDownload{}, fmt.Errorf("download %s: %w", source, err)
		}
		return remoteDownload{FinalURL: source}, nil
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return remoteDownload{}, fmt.Errorf("unsupported archive URL scheme: %s", parsed.Scheme)
	}
	response, err := remoteHTTPClient.Get(source)
	if err != nil {
		return remoteDownload{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return remoteDownload{}, &remoteHTTPStatusError{URL: source, Status: response.Status, StatusCode: response.StatusCode}
	}
	if response.ContentLength > maxBytes {
		return remoteDownload{}, fmt.Errorf("download %s exceeds maximum size of %d bytes", source, maxBytes)
	}
	if err := writeLimitedDownload(response.Body, target, maxBytes); err != nil {
		return remoteDownload{}, fmt.Errorf("download %s: %w", source, err)
	}
	return remoteDownload{
		ContentType: response.Header.Get("Content-Type"),
		FinalURL:    response.Request.URL.String(),
	}, nil
}

func writeLimitedDownload(input io.Reader, target string, maxBytes int64) (resultErr error) {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	output, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		if resultErr != nil {
			_ = os.Remove(target)
		}
	}()
	written, err := io.Copy(output, io.LimitReader(input, maxBytes+1))
	if err != nil {
		_ = output.Close()
		return err
	}
	if written > maxBytes {
		_ = output.Close()
		return fmt.Errorf("content exceeds maximum size of %d bytes", maxBytes)
	}
	return output.Close()
}

func validatedExtractedBundleRoot(root string, specVersion string, requireDeclaredSpec bool) (string, error) {
	if validatedRoot, valid, err := validateExtractedBundleCandidate(root, specVersion, requireDeclaredSpec); err != nil {
		return "", err
	} else if valid {
		return validatedRoot, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	var directories []string
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, filepath.Join(root, entry.Name()))
		}
	}
	if len(directories) == 1 {
		if validatedRoot, valid, err := validateExtractedBundleCandidate(directories[0], specVersion, requireDeclaredSpec); err != nil {
			return "", err
		} else if valid {
			return validatedRoot, nil
		}
	}
	return "", fmt.Errorf("archive does not contain a valid Open Knowledge bundle")
}

func validateExtractedBundleCandidate(root string, specVersion string, requireDeclaredSpec bool) (string, bool, error) {
	result, err := okf.ValidateWithVersion(root, specVersion)
	if err != nil {
		return "", false, err
	}
	if len(result.Errors) > 0 {
		return "", false, nil
	}
	if requireDeclaredSpec {
		declared, err := okf.DeclaredBundleSpecVersion(result.Root)
		if err != nil {
			return "", false, err
		}
		if declared != "" && declared != result.SpecVersion {
			return "", false, fmt.Errorf("archive bundle declares okf_version %q but manifest requires %q", declared, result.SpecVersion)
		}
	}
	return result.Root, true, nil
}

func cachedBundleRoot(target string) (string, bool) {
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		return "", false
	}
	root, err := validatedExtractedBundleRoot(target, okf.LatestSpecVersion, false)
	if err != nil {
		return "", false
	}
	return root, true
}

func resolveManifestArchiveURL(manifestURL string, archive string) (string, error) {
	base, err := url.Parse(manifestURL)
	if err != nil {
		return "", err
	}
	relative, err := url.Parse(archive)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(relative).String(), nil
}

func manifestCandidateURLs(source string) []string {
	parsed, err := url.Parse(source)
	if err != nil {
		return nil
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil
	}
	var candidates []string
	withPath := *parsed
	withPath.Path = path.Join(withPath.Path, okf.BundleManifestRelPath)
	if !strings.HasPrefix(withPath.Path, "/") {
		withPath.Path = "/" + withPath.Path
	}
	candidates = append(candidates, withPath.String())

	wellKnown := *parsed
	wellKnown.RawQuery = ""
	wellKnown.Fragment = ""
	wellKnown.Path = "/.well-known/openknowledge.json"
	if wellKnown.String() != candidates[0] {
		candidates = append(candidates, wellKnown.String())
	}
	return candidates
}

func downloadedFileLooksLikeArchive(file string, contentType string) bool {
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "gzip") || strings.Contains(contentType, "x-tar") || strings.Contains(contentType, "tar") {
		return true
	}
	input, err := os.Open(file)
	if err != nil {
		return false
	}
	defer input.Close()
	buffer := make([]byte, 265)
	n, _ := io.ReadFull(input, buffer)
	if n >= 2 && buffer[0] == 0x1f && buffer[1] == 0x8b {
		return true
	}
	return n >= 263 && string(buffer[257:262]) == "ustar"
}

func looksLikeManifestSource(source string) bool {
	parsed, err := url.Parse(source)
	if err != nil {
		return false
	}
	return strings.EqualFold(path.Base(parsed.Path), okf.BundleManifestRelPath)
}

func looksLikeArchiveSource(source string) bool {
	parsed, err := url.Parse(source)
	if err != nil {
		return false
	}
	lower := strings.ToLower(parsed.Path)
	return strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")
}

func isHTTPSource(source string) bool {
	lower := strings.ToLower(strings.TrimSpace(source))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func legacyRemoteSourceType(source string, target string) string {
	if looksLikeManifestSource(source) {
		return "manifest"
	}
	if looksLikeArchiveSource(source) {
		return "tar"
	}
	if info, err := os.Stat(filepath.Join(target, ".git")); err == nil && info.IsDir() {
		return "git"
	}
	return "unknown"
}

func remoteFetchTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

var remoteCacheProcessLocks sync.Map

func lockRemoteCache(target string) (func() error, error) {
	processLockValue, _ := remoteCacheProcessLocks.LoadOrStore(target, &sync.Mutex{})
	processLock := processLockValue.(*sync.Mutex)
	processLock.Lock()

	fileLock := flock.New(target+".lock", flock.SetPermissions(0600))
	if err := fileLock.Lock(); err != nil {
		processLock.Unlock()
		return nil, fmt.Errorf("lock remote cache: %w", err)
	}
	return func() error {
		err := fileLock.Close()
		processLock.Unlock()
		if err != nil {
			return fmt.Errorf("unlock remote cache: %w", err)
		}
		return nil
	}, nil
}

func publishRemoteCache(staging string, target string) error {
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		return os.Rename(staging, target)
	} else if err != nil {
		return err
	}

	backup, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-previous-*")
	if err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	if err := os.Rename(target, backup); err != nil {
		return err
	}
	if err := os.Rename(staging, target); err != nil {
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore previous remote cache: %w", restoreErr))
		}
		return err
	}
	if err := os.RemoveAll(backup); err != nil {
		moveNewErr := os.Rename(target, staging)
		restoreErr := os.Rename(backup, target)
		if moveNewErr != nil || restoreErr != nil {
			errorsToJoin := []error{err}
			if moveNewErr != nil {
				errorsToJoin = append(errorsToJoin, fmt.Errorf("move new cache during rollback: %w", moveNewErr))
			}
			if restoreErr != nil {
				errorsToJoin = append(errorsToJoin, fmt.Errorf("restore previous remote cache: %w", restoreErr))
			}
			return errors.Join(errorsToJoin...)
		}
		return err
	}
	return nil
}

func finishRemoteMaterialization(root string, target string, source okf.RegistrySource) (string, okf.RegistrySource, error) {
	if err := saveRemoteCacheSource(target, source); err != nil {
		return "", okf.RegistrySource{}, err
	}
	return root, source, nil
}

func remoteCacheSourcePath(target string) string {
	return target + ".source.json"
}

const remoteCacheSchemaVersion = "1"

type remoteCacheRecord struct {
	SchemaVersion string             `json:"schemaVersion"`
	Source        okf.RegistrySource `json:"source"`
}

func saveRemoteCacheSource(target string, source okf.RegistrySource) error {
	record := remoteCacheRecord{SchemaVersion: remoteCacheSchemaVersion, Source: source}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := remoteCacheSourcePath(target)
	if err := os.Chmod(path, 0600); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := atomic.WriteFile(path, bytes.NewReader(data)); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func loadRemoteCacheSource(target string, requestedSource string) (okf.RegistrySource, error) {
	content, err := os.ReadFile(remoteCacheSourcePath(target))
	if err != nil {
		return okf.RegistrySource{}, err
	}
	var record remoteCacheRecord
	if err := json.Unmarshal(content, &record); err != nil {
		return okf.RegistrySource{}, fmt.Errorf("invalid remote cache provenance for %s: %w", target, err)
	}
	if record.SchemaVersion != remoteCacheSchemaVersion {
		return okf.RegistrySource{}, fmt.Errorf("unsupported remote cache provenance version %q for %s", record.SchemaVersion, target)
	}
	source := record.Source
	if strings.TrimSpace(source.Type) == "" || strings.TrimSpace(source.URL) == "" {
		return okf.RegistrySource{}, fmt.Errorf("invalid remote cache provenance for %s: source type and URL are required", target)
	}
	if normalizeRemoteSource(source.URL) != normalizeRemoteSource(requestedSource) {
		return okf.RegistrySource{}, fmt.Errorf("remote cache provenance for %s belongs to a different source", target)
	}
	source.URL = strings.TrimSpace(requestedSource)
	source.ManagedRoot = target
	return source, nil
}

func gitCommitForDirectory(root string) (string, error) {
	command := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not resolve cloned Git commit: %s", strings.TrimSpace(string(output)))
	}
	commit := strings.TrimSpace(string(output))
	decoded, decodeErr := hex.DecodeString(commit)
	if decodeErr != nil || (len(decoded) != 20 && len(decoded) != 32) {
		return "", fmt.Errorf("could not resolve cloned Git commit: unexpected object ID %q", commit)
	}
	return commit, nil
}

func remoteBundleCacheRoot() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(okf.RegistryFileEnv)); configured != "" {
		registryFile, err := okf.ExpandUserPath(configured)
		if err != nil {
			return "", err
		}
		return filepath.Join(filepath.Dir(registryFile), "bundles"), nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "openknowledge", "bundles"), nil
}

func registryCacheName(source string) string {
	normalized := normalizeRemoteSource(source)
	base := remoteSourceBaseName(normalized)
	if base == "" {
		base = "bundle"
	}
	sum := sha256.Sum256([]byte(normalized))
	return base + "-" + hex.EncodeToString(sum[:])[:12]
}

func registeredRemoteCacheTarget(source string) (string, bool) {
	entries, err := okf.RegistryEntries()
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if !entry.Managed || normalizeRemoteSource(entry.Source.URL) != normalizeRemoteSource(source) {
			continue
		}
		managedRoot, err := managedCacheRootForEntry(entry)
		if err == nil {
			return managedRoot, true
		}
	}
	return "", false
}

func normalizeRemoteSource(source string) string {
	source = strings.TrimSpace(source)
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme == "" {
		return source
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String()
}

func remoteSourceBaseName(source string) string {
	candidate := source
	if parsed, err := url.Parse(source); err == nil && parsed.Path != "" {
		candidate = parsed.Path
	}
	candidate = strings.TrimRight(candidate, "/")
	base := path.Base(candidate)
	if strings.EqualFold(base, okf.BundleManifestRelPath) {
		base = path.Base(path.Dir(candidate))
	}
	lower := strings.ToLower(base)
	for _, suffix := range []string{".tar.gz", ".tgz", ".tar", ".git"} {
		if strings.HasSuffix(lower, suffix) {
			base = base[:len(base)-len(suffix)]
			break
		}
	}
	return okf.RegistryKeyFromNameForCache(base)
}

func runDisconnect(args []string, command string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, disconnectHelpText(command))
		return 0
	}
	fs := flag.NewFlagSet("disconnect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	deleteFilesFlag := fs.Bool("delete-files", false, "delete CLI-managed bundle files")
	keepFilesFlag := fs.Bool("keep-files", false, "keep bundle files")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s <key|path>\n", command)
		return 2
	}
	if *deleteFilesFlag && *keepFilesFlag {
		fmt.Fprintln(os.Stderr, "--delete-files and --keep-files cannot be used together")
		return 2
	}

	target := fs.Arg(0)
	if *deleteFilesFlag {
		entry, ok, deleteErr, err := disconnectManagedRegistryEntry(target)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if !ok {
			printUnknownConnection(target)
			return 1
		}
		files := "deleted"
		if deleteErr != nil {
			fmt.Fprintf(os.Stderr, "warning: disconnected but could not delete managed cache: %v\n", deleteErr)
			files = "delete failed"
		}
		printDisconnectResult(entry, files)
		if deleteErr != nil {
			return 1
		}
		return 0
	}

	entry, ok, err := okf.RemoveRegistryEntry(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !ok {
		printUnknownConnection(target)
		return 1
	}

	printDisconnectResult(entry, "kept")
	return 0
}

func runRegistryRefresh(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, registryRefreshHelpText())
		return 0
	}
	fs := flag.NewFlagSet("registry refresh", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	forceFlag := fs.Bool("force", false, "discard local changes in the managed cache")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: openknowledge registry refresh <key|path> [--force]")
		return 2
	}

	target := fs.Arg(0)
	entry, ok, err := okf.ResolveRegistryTarget(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !ok {
		printUnknownConnection(target)
		return 1
	}
	if !entry.Managed {
		fmt.Fprintf(os.Stderr, "connection %q is local and cannot be refreshed from a remote source\n", entry.Name)
		return 1
	}
	if strings.TrimSpace(entry.Source.URL) == "" {
		fmt.Fprintf(os.Stderr, "connection %q has no recorded remote source\n", entry.Name)
		return 1
	}
	oldManagedRoot, err := managedCacheRootForEntry(entry)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	unlock, err := lockRemoteCache(oldManagedRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer unlock()

	current, ok, err := okf.ResolveRegistryTarget(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !ok || current != entry {
		fmt.Fprintf(os.Stderr, "connection %q changed while it was being refreshed\n", entry.Name)
		return 1
	}
	if status := inspectRegistryEntryWithCacheLock(current, true); status.State == "modified" && !*forceFlag {
		fmt.Fprintf(os.Stderr, "managed cache for %q has local changes; use --force to discard them\n", entry.Name)
		return 1
	}

	newTarget, err := newRefreshCacheTarget(oldManagedRoot, entry.Source.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	newRoot, source, err := materializeRemoteSourceAtTarget(entry.Source.URL, newTarget, false)
	if err != nil {
		cleanupErr := removeRemoteCacheGeneration(newTarget, true)
		fmt.Fprintln(os.Stderr, errors.Join(err, cleanupErr))
		return 1
	}
	if status := inspectRegistryEntryWithCacheLock(current, true); status.State == "modified" && !*forceFlag {
		cleanupErr := removeRemoteCacheGeneration(source.ManagedRoot, true)
		fmt.Fprintln(os.Stderr, errors.Join(fmt.Errorf("managed cache for %q changed during refresh; use --force to discard local changes", entry.Name), cleanupErr))
		return 1
	}

	replacement := current
	replacement.Path = newRoot
	replacement.Managed = true
	replacement.Source = source
	if _, err := okf.ReplaceRegistryEntry(current, replacement); err != nil {
		cleanupErr := removeRemoteCacheGeneration(source.ManagedRoot, true)
		fmt.Fprintln(os.Stderr, errors.Join(err, cleanupErr))
		return 1
	}

	cleanupErr := removeRemoteCacheGeneration(oldManagedRoot, false)
	terminal.success("Refreshed knowledge bundle")
	fmt.Printf("%-10s %s\n", "key", replacement.Name)
	fmt.Printf("%-10s %s\n", "old path", terminal.path(entry.Path))
	fmt.Printf("%-10s %s\n", "path", terminal.path(replacement.Path))
	fmt.Printf("%-10s %s\n", "source", replacement.Source.Type)
	if replacement.Source.GitCommit != "" {
		fmt.Printf("%-10s %s\n", "identity", replacement.Source.GitCommit)
	} else if replacement.Source.SHA256 != "" {
		fmt.Printf("%-10s %s\n", "identity", replacement.Source.SHA256)
	}
	if cleanupErr != nil {
		fmt.Fprintf(os.Stderr, "warning: refreshed but could not delete the previous managed cache: %v\n", cleanupErr)
		return 1
	}
	return 0
}

func newRefreshCacheTarget(oldManagedRoot string, source string) (string, error) {
	parent := filepath.Dir(oldManagedRoot)
	placeholder, err := os.MkdirTemp(parent, registryCacheName(source)+"-refresh-*")
	if err != nil {
		return "", err
	}
	if err := os.Remove(placeholder); err != nil {
		return "", err
	}
	return placeholder, nil
}

func removeRemoteCacheGeneration(managedRoot string, removeLock bool) error {
	var cleanupErrors []error
	if err := os.RemoveAll(managedRoot); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete managed cache: %w", err))
	}
	if err := os.Remove(remoteCacheSourcePath(managedRoot)); err != nil && !os.IsNotExist(err) {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete cache provenance: %w", err))
	}
	if removeLock {
		if err := os.Remove(managedRoot + ".lock"); err != nil && !os.IsNotExist(err) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete cache lock: %w", err))
		}
	}
	return errors.Join(cleanupErrors...)
}

func disconnectManagedRegistryEntry(target string) (okf.RegistryEntry, bool, error, error) {
	entry, ok, err := okf.ResolveRegistryTarget(target)
	if err != nil || !ok {
		return entry, ok, nil, err
	}
	managedRoot, err := managedCacheRootForEntry(entry)
	if err != nil {
		return okf.RegistryEntry{}, false, nil, err
	}

	unlock, err := lockRemoteCache(managedRoot)
	if err != nil {
		return okf.RegistryEntry{}, false, nil, err
	}
	defer unlock()

	current, ok, err := okf.ResolveRegistryTarget(target)
	if err != nil || !ok {
		return current, ok, nil, err
	}
	if current != entry {
		return okf.RegistryEntry{}, false, nil, fmt.Errorf("connection %q changed while it was being disconnected", entry.Name)
	}
	if _, err := os.Lstat(managedRoot); err != nil {
		return okf.RegistryEntry{}, false, nil, fmt.Errorf("managed cache is unavailable: %w", err)
	}

	tombstone, err := newCacheTombstone(managedRoot)
	if err != nil {
		return okf.RegistryEntry{}, false, nil, err
	}
	if err := os.Rename(managedRoot, tombstone); err != nil {
		return okf.RegistryEntry{}, false, nil, err
	}
	sourcePath := remoteCacheSourcePath(managedRoot)
	tombstoneSourcePath := remoteCacheSourcePath(tombstone)
	sourceMoved := false
	if err := os.Rename(sourcePath, tombstoneSourcePath); err == nil {
		sourceMoved = true
	} else if !os.IsNotExist(err) {
		rollbackErr := os.Rename(tombstone, managedRoot)
		return okf.RegistryEntry{}, false, nil, errors.Join(err, rollbackErr)
	}

	removed, ok, err := okf.RemoveRegistryEntryWithOptions(target, okf.RemoveRegistryOptions{
		RequireManaged: true,
		ExpectedEntry:  &entry,
	})
	if err != nil || !ok {
		rollbackErrors := []error{err}
		if sourceMoved {
			if rollbackErr := os.Rename(tombstoneSourcePath, sourcePath); rollbackErr != nil {
				rollbackErrors = append(rollbackErrors, rollbackErr)
			}
		}
		if rollbackErr := os.Rename(tombstone, managedRoot); rollbackErr != nil {
			rollbackErrors = append(rollbackErrors, rollbackErr)
		}
		return okf.RegistryEntry{}, ok, nil, errors.Join(rollbackErrors...)
	}

	var deleteErrors []error
	if err := os.RemoveAll(tombstone); err != nil {
		deleteErrors = append(deleteErrors, fmt.Errorf("delete %s: %w", tombstone, err))
	}
	if sourceMoved {
		if err := os.Remove(tombstoneSourcePath); err != nil && !os.IsNotExist(err) {
			deleteErrors = append(deleteErrors, fmt.Errorf("delete cache provenance: %w", err))
		}
	}
	return removed, true, errors.Join(deleteErrors...), nil
}

func managedCacheRootForEntry(entry okf.RegistryEntry) (string, error) {
	if !entry.Managed {
		return "", fmt.Errorf("refusing to delete non-managed files: %s", entry.Path)
	}
	cacheRoot, err := remoteBundleCacheRoot()
	if err != nil {
		return "", err
	}
	cacheRoot, err = filepath.Abs(cacheRoot)
	if err != nil {
		return "", err
	}
	managedRoot := strings.TrimSpace(entry.Source.ManagedRoot)
	if managedRoot == "" {
		managedRoot = entry.Path
	}
	managedRoot, err = filepath.Abs(managedRoot)
	if err != nil {
		return "", err
	}
	if filepath.Dir(managedRoot) != cacheRoot {
		return "", fmt.Errorf("refusing to delete managed path outside the Open Knowledge cache: %s", managedRoot)
	}
	entryPath, err := filepath.Abs(entry.Path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(managedRoot, entryPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to delete cache root that does not contain the registered bundle: %s", managedRoot)
	}
	return managedRoot, nil
}

func newCacheTombstone(managedRoot string) (string, error) {
	tombstone, err := os.MkdirTemp(filepath.Dir(managedRoot), ".openknowledge-delete-*")
	if err != nil {
		return "", err
	}
	if err := os.Remove(tombstone); err != nil {
		return "", err
	}
	return tombstone, nil
}

// parseInterspersedFlags preserves flag.FlagSet's parsing rules while allowing
// registered flags to appear on either side of positional arguments. The
// standard flag package stops parsing at the first positional argument.
func parseInterspersedFlags(fs *flag.FlagSet, args []string) error {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))

	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			positionals = append(positionals, args[index+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if equals := strings.IndexByte(name, '='); equals >= 0 {
			continue
		}
		registered := fs.Lookup(name)
		if registered == nil {
			continue
		}
		if boolean, ok := registered.Value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
			continue
		}
		if index+1 < len(args) {
			index++
			flags = append(flags, args[index])
		}
	}

	reordered := append(flags, "--")
	reordered = append(reordered, positionals...)
	return fs.Parse(reordered)
}

func printUnknownConnection(target string) {
	fmt.Fprintf(os.Stderr, "unknown knowledge bundle: %s\n", target)
	entries, err := okf.RegistryEntries()
	if err != nil || len(entries) == 0 {
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	fmt.Fprintf(os.Stderr, "available keys: %s\n", strings.Join(names, ", "))
}

func printDisconnectResult(entry okf.RegistryEntry, files string) {
	terminal.success("Disconnected knowledge bundle")
	fmt.Printf("%-6s %s\n", "key", entry.Name)
	fmt.Printf("%-6s %s\n", "path", terminal.path(entry.Path))
	fmt.Printf("%-6s %s\n", "files", files)
}

type getOptions struct {
	target string
	entry  string
	info   bool
}

type searchOptions struct {
	target    string
	query     string
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
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if options.entry == "" {
		if localFile, rel, ok := resolveDirectGetFile(options.target); ok {
			if !isGetMarkdownFile(localFile) {
				fmt.Fprintf(os.Stderr, "get only prints Markdown files: %s\n", rel)
				return 1
			}
			if options.info {
				document, err := okf.ReadMarkdownDocumentInfo(localFile, rel)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return 1
				}
				printGetFileInfo(getSelection{name: rel, rel: rel, abs: localFile}, document)
				return 0
			}
			content, err := os.ReadFile(localFile)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			fmt.Print(string(content))
			return 0
		}
	}

	root, err := resolveWhereTarget(options.target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	info, err := okf.ReadBundleInfo(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if options.info {
		if err := printGetInfo(root, info, options.entry); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	selection, err := selectGetTarget(root, info, options.entry)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	content, err := os.ReadFile(selection.abs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	root, err := resolveWhereTarget(options.target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := printSearchMatches(result, options.format); err != nil {
			fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := printSearchContext(result, options.format); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
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
	if len(positionals) < 2 {
		return searchOptions{}, fmt.Errorf("usage: openknowledge search <name|path> <query>")
	}
	options.target = positionals[0]
	options.query = strings.TrimSpace(strings.Join(positionals[1:], " "))
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

func printSearchContextMarkdown(result okf.ContextResult) {
	fmt.Println("# Open Knowledge Context")
	fmt.Println()
	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Root: `%s`\n", result.Root)
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

func printSearchMatchesMarkdown(result okf.SearchResultSet) {
	fmt.Println("# Open Knowledge Search Matches")
	fmt.Println()
	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Root: `%s`\n", result.Root)
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
	fs.SetOutput(os.Stderr)
	jsonFlag := fs.Bool("json", false, "print versioned JSON inventory")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: openknowledge registry list [--json]")
		return 2
	}

	entries, err := okf.RegistryEntries()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !*jsonFlag {
		printRegistryEntries(entries)
		return 0
	}
	registryPath, err := okf.RegistryFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintln(os.Stderr, err)
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
	fs.SetOutput(os.Stderr)
	jsonFlag := fs.Bool("json", false, "print versioned JSON status")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: openknowledge registry status [key|path] [--json]")
		return 2
	}

	entries, err := okf.RegistryEntries()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if fs.NArg() == 1 {
		entry, ok, err := okf.ResolveRegistryTarget(fs.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintln(os.Stderr, err)
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
			fmt.Fprintln(os.Stderr, err)
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
		fmt.Fprintln(os.Stderr, "usage: openknowledge registry where <name|path>")
		return 2
	}

	root, err := resolveWhereTarget(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
	fs.SetOutput(os.Stderr)
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
		fmt.Fprintf(os.Stderr, "unsupported validate format: %s\n", *format)
		return 2
	}
	if *quiet && *format == "json" {
		fmt.Fprintln(os.Stderr, "--quiet cannot be combined with JSON validation output")
		return 2
	}
	if strings.TrimSpace(*out) != "" && *format != "json" {
		fmt.Fprintln(os.Stderr, "--out requires --format json or --json")
		return 2
	}

	root := "."
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "validate accepts at most one key or path")
		return 2
	}
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}

	root, err := okf.ResolveKnowledgeRoot(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	validationOptions, err := okf.LoadValidationOptions(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	cliOptions := okf.ValidationOptions{}
	for _, override := range ruleOverrides {
		rule, severity, err := okf.ParseValidationRuleOverride(override)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if err := okf.SetValidationRuleSeverity(&cliOptions, rule, severity); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	validationOptions = okf.MergeValidationOptions(validationOptions, cliOptions)

	result, err := okf.ValidateWithVersionAndOptions(root, *specVersion, validationOptions)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if *format == "json" {
		if err := printValidationJSONResult(result, strings.TrimSpace(*out)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(result.Errors) > 0 {
			return 1
		}
		return 0
	}

	if *quiet {
		for _, issue := range result.Errors {
			fmt.Fprintln(os.Stderr, issue)
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
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "print JSON")
	specVersion := fs.String("spec", "latest", "OKF spec version")
	depth := fs.Int("depth", 0, "maximum tree depth; 0 means unlimited")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *depth < 0 {
		fmt.Fprintln(os.Stderr, "--depth must be zero or a positive integer")
		return 2
	}

	root := "."
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "list accepts at most one key or path")
		return 2
	}
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}

	root, err := okf.ResolveKnowledgeRoot(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	listing, err := okf.ListWithVersion(root, *specVersion)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if *asJSON {
		listing.Entries = filterListEntriesByDepth(listing.Entries, *depth)
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(listing); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		return 0
	}

	printListTree(listing, *depth)
	return 0
}

func runTo(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, toHelpText())
		return 0
	}

	switch args[0] {
	case "html":
		return runToHTML(args[1:])
	case "json":
		return runToJSON(args[1:])
	case "tar":
		return runToTar(args[1:])
	case "graph":
		return runToGraph(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown to target: %s\n\n", args[0])
		fmt.Fprint(os.Stderr, toHelpText())
		return 2
	}
}

type toOptions struct {
	path       string
	out        string
	spec       string
	graphType  string
	plain      bool
	headHTML   string
	headFile   string
	scriptSrcs []string
}

func runToHTML(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, toHTMLHelpText())
		return 0
	}
	options, err := parseToOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.out == "" {
		fmt.Fprintln(os.Stderr, "openknowledge to html requires --out <folder>")
		return 2
	}
	if options.plain {
		if flag := options.headFlag(); flag != "" {
			fmt.Fprintf(os.Stderr, "%s requires the default viewer export; remove --plain\n", flag)
			return 2
		}
	}
	if options.graphType != "" {
		fmt.Fprintln(os.Stderr, "unknown flag: --type")
		return 2
	}

	var result okf.HTMLResult
	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.plain {
		result, err = okf.WritePlainHTMLWithVersion(root, options.out, options.spec)
	} else {
		headInjection, loadErr := loadHeadInjection(options.headInjectionOptions())
		if loadErr != nil {
			fmt.Fprintln(os.Stderr, loadErr)
			return 2
		}
		result, err = writeViewerHTMLWithOptions(root, options.out, options.spec, viewerHTMLExportOptions{HeadHTML: headInjection})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	terminal.success("Exported HTML")
	fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(result.Root))
	fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(result.Out))
	fmt.Printf("%s %d files\n", terminal.muted("wrote"), len(result.Written))
	return 0
}

func runToJSON(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, toJSONHelpText())
		return 0
	}
	options, err := parseToOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.plain {
		fmt.Fprintln(os.Stderr, "unknown flag: --plain")
		return 2
	}
	if options.graphType != "" {
		fmt.Fprintln(os.Stderr, "unknown flag: --type")
		return 2
	}
	if flag := options.headFlag(); flag != "" {
		fmt.Fprintf(os.Stderr, "unknown flag: %s\n", flag)
		return 2
	}

	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	bundle, err := okf.ParseBundleWithVersion(root, options.spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data = append(data, '\n')

	if options.out != "" {
		if err := writeOutputFileAtomically(options.out, data); err != nil {
			fmt.Fprintln(os.Stderr, err)
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

func runToTar(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, toTarHelpText())
		return 0
	}
	options, err := parseToOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.plain {
		fmt.Fprintln(os.Stderr, "unknown flag: --plain")
		return 2
	}
	if options.graphType != "" {
		fmt.Fprintln(os.Stderr, "unknown flag: --type")
		return 2
	}
	if flag := options.headFlag(); flag != "" {
		fmt.Fprintf(os.Stderr, "unknown flag: %s\n", flag)
		return 2
	}
	if options.out == "" {
		fmt.Fprintln(os.Stderr, "openknowledge to tar requires --out <file>")
		return 2
	}

	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	result, err := okf.WriteBundleTarGzipWithVersion(root, options.out, options.spec, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
func runToGraph(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, toGraphHelpText())
		return 0
	}
	options, err := parseToOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.plain {
		fmt.Fprintln(os.Stderr, "unknown flag: --plain")
		return 2
	}
	if flag := options.headFlag(); flag != "" {
		fmt.Fprintf(os.Stderr, "unknown flag: %s\n", flag)
		return 2
	}

	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	graph, err := okf.BuildGraphWithType(root, options.spec, options.graphType)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data = append(data, '\n')

	if options.out != "" {
		if err := writeOutputFileAtomically(options.out, data); err != nil {
			fmt.Fprintln(os.Stderr, err)
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

func parseToOptions(args []string) (toOptions, error) {
	options := toOptions{path: ".", spec: "latest"}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--out":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return toOptions{}, fmt.Errorf("--out requires a value")
			}
			options.out = args[index]
		case strings.HasPrefix(arg, "--out="):
			options.out = strings.TrimPrefix(arg, "--out=")
			if strings.TrimSpace(options.out) == "" {
				return toOptions{}, fmt.Errorf("--out requires a value")
			}
		case arg == "--spec":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return toOptions{}, fmt.Errorf("--spec requires a value")
			}
			options.spec = args[index]
		case strings.HasPrefix(arg, "--spec="):
			options.spec = strings.TrimPrefix(arg, "--spec=")
			if strings.TrimSpace(options.spec) == "" {
				return toOptions{}, fmt.Errorf("--spec requires a value")
			}
		case arg == "--type":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return toOptions{}, fmt.Errorf("--type requires a value")
			}
			options.graphType = args[index]
		case strings.HasPrefix(arg, "--type="):
			options.graphType = strings.TrimPrefix(arg, "--type=")
			if strings.TrimSpace(options.graphType) == "" {
				return toOptions{}, fmt.Errorf("--type requires a value")
			}
		case arg == "--plain":
			options.plain = true
		case arg == "--head-file":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return toOptions{}, fmt.Errorf("--head-file requires a value")
			}
			options.headFile = args[index]
		case strings.HasPrefix(arg, "--head-file="):
			options.headFile = strings.TrimPrefix(arg, "--head-file=")
			if strings.TrimSpace(options.headFile) == "" {
				return toOptions{}, fmt.Errorf("--head-file requires a value")
			}
		case arg == "--head-html":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return toOptions{}, fmt.Errorf("--head-html requires a value")
			}
			options.headHTML = args[index]
		case strings.HasPrefix(arg, "--head-html="):
			options.headHTML = strings.TrimPrefix(arg, "--head-html=")
			if strings.TrimSpace(options.headHTML) == "" {
				return toOptions{}, fmt.Errorf("--head-html requires a value")
			}
		case arg == "--script-src":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return toOptions{}, fmt.Errorf("--script-src requires a value")
			}
			options.scriptSrcs = append(options.scriptSrcs, args[index])
		case strings.HasPrefix(arg, "--script-src="):
			src := strings.TrimPrefix(arg, "--script-src=")
			if strings.TrimSpace(src) == "" {
				return toOptions{}, fmt.Errorf("--script-src requires a value")
			}
			options.scriptSrcs = append(options.scriptSrcs, src)
		case strings.HasPrefix(arg, "-"):
			return toOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			if options.path != "." {
				return toOptions{}, fmt.Errorf("to accepts at most one path")
			}
			options.path = arg
		}
	}
	return options, nil
}

func (options toOptions) headFlag() string {
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

func (options toOptions) headInjectionOptions() headInjectionOptions {
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
		fmt.Fprintln(os.Stderr, "usage: openknowledge version")
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
	fmt.Fprint(os.Stderr, helpText())
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

func helpText() string {
	return `openknowledge creates and validates Open Knowledge Format v0.1 bundles.

Usage:
  openknowledge --help
  openknowledge <command> --help
  openknowledge setup
  openknowledge setup --rules <rules>
  openknowledge from <source> --out <folder>
  openknowledge from <source> --out <folder> --type understanding
  openknowledge from <source> --out <folder> --type custom --about <goal>
  openknowledge rules
  openknowledge rules <rules> --path <path>
  openknowledge rules apply <rules> --path <path>
  openknowledge rules --list
  openknowledge review rules [path]
  openknowledge review rules --rules <rules> --path <path>
  openknowledge review rules --all [path]
  openknowledge agents new
  openknowledge agents new <template> --out <file>
  openknowledge agents list [path]
  openknowledge agents validate <job-or-dir>
  openknowledge agents run <job.md> --dry-run
  openknowledge agents run <job.md>
  openknowledge agents daemon [jobs-dir] --once
  openknowledge new [folder]
  openknowledge new --name <name> [folder]
  openknowledge new --bundle-name <id> --bundle-purpose <text> [folder]
  openknowledge new --no-agents --no-setup [folder]
  openknowledge connect <source>
  openknowledge connect <source> --as <key>
  openknowledge disconnect <key|path>
  openknowledge get <name|path> [entry-or-file]
  openknowledge get <name|path> --info
  openknowledge search <name|path> <query>
  openknowledge search <name|path> <query> --budget <tokens>
  openknowledge search <name|path> <query> --format json
  openknowledge search <name|path> <query> --matches
  openknowledge search <name|path> <query> --no-expand
  openknowledge mcp [key-or-path]
  openknowledge mcp --spec <version> [key-or-path]
  openknowledge ast [path]
  openknowledge ast --out <file> [path]
  openknowledge registry connect <source>
  openknowledge registry connect <source> --as <key>
  openknowledge registry disconnect <key|path>
  openknowledge registry refresh <key|path> [--force]
  openknowledge registry list
  openknowledge registry list --json
  openknowledge registry status [key|path]
  openknowledge registry status [key|path] --json
  openknowledge registry where <name|path>
  openknowledge view [path]
  openknowledge view --name <alias-name> [path]
  openknowledge view --host <host> --port <port> [path]
  openknowledge view --allow-network --host <host> [path]
  openknowledge view --head-file <file> [path]
  openknowledge view --script-src <src> [path]
  openknowledge view --no-browser [path]
  openknowledge to html --out <folder> [path]
  openknowledge to html --head-file <file> --out <folder> [path]
  openknowledge to html --script-src <src> --out <folder> [path]
  openknowledge to json [path]
  openknowledge to json --out <file> [path]
  openknowledge to tar --out <file> [path]
  openknowledge to graph [path]
  openknowledge to graph --out <file> [path]
  openknowledge to graph --type search [path]
  openknowledge spec latest|<version>
  openknowledge validate [key-or-path]
  openknowledge validate --spec <version> [key-or-path]
  openknowledge validate --format json [key-or-path]
  openknowledge validate --rule <rule=off|warn|error> [key-or-path]
  openknowledge list [key-or-path]
  openknowledge list --spec <version> [key-or-path]
  openknowledge list --depth <n> [key-or-path]
  openknowledge list --json [key-or-path]
  openknowledge version

Commands:
  setup      Print an agent setup prompt.
  from       Print an agent source-to-wiki generation prompt.
  rules      Print agent maintenance rules.
  review     Print advisory AI review prompts.
  agents     Experimental: run scheduled local agent jobs from Markdown specs.
  new        Scaffold a local Open Knowledge bundle.
  connect    Connect a local or remote knowledge bundle.
  disconnect Remove a knowledge bundle connection.
  get        Print a Markdown file or bundle entrypoint.
  search     Build source-grounded Markdown context from a bundle.
  mcp        Serve one knowledge base to MCP clients over stdio.
  ast        Print parsed OKF AST JSON.
  registry   Manage knowledge bundle connections.
  view       Start the registry or knowledge base Markdown viewer.
  to         Convert a bundle to another format.
  spec       Print an embedded OKF spec.
  validate   Validate a bundle against an OKF spec.
  list       Print a bundle tree, with optional depth and JSON output.
  version    Print the CLI version.

Flags:
  -h, --help  Show this help.

Run openknowledge <command> --help for command-specific help.

Examples:
  openknowledge from https://github.com/openknowledge-sh/openknowledge --out Wiki --type understanding
  openknowledge from https://openknowledge.sh/wiki/ --out Wiki --type custom --about "Create an onboarding wiki"
  openknowledge rules docs,changelog --path Wiki
  openknowledge rules apply docs,changelog --path Wiki --file AGENTS.md
  openknowledge review rules --rules docs,changelog --path Wiki
  openknowledge agents new docs-audit --out .openknowledge/agents/jobs/docs-audit.md
  openknowledge agents validate .openknowledge/agents/jobs
  openknowledge agents run .openknowledge/agents/jobs/docs.md --dry-run
  openknowledge setup --rules docs,changelog
  openknowledge new ./project-memory
  openknowledge new --no-agents --no-setup ./source-wiki
  openknowledge new --name "Accessibility Review" --bundle-name accessibility --bundle-tag accessibility ./accessibility
  openknowledge connect ./accessibility --as accessibility
  openknowledge get accessibility --info
  openknowledge get accessibility
  openknowledge search accessibility "validation workflow"
  openknowledge mcp accessibility
  openknowledge ast ./project-memory
  openknowledge disconnect accessibility
  openknowledge registry connect ./team-wiki --as team
  openknowledge registry refresh team
  openknowledge registry where accessibility
  openknowledge list personal
  openknowledge validate ./project-memory
  openknowledge to html --out ./site ./project-memory
  openknowledge to json ./project-memory
  openknowledge to tar --out ./bundle.tar.gz ./project-memory
  openknowledge to graph ./project-memory
  openknowledge list --json ./project-memory
  openknowledge list --depth 2 ./project-memory
  openknowledge view
  openknowledge view ./project-memory
`
}

func getHelpText() string {
	return `openknowledge get

Print an exact Markdown file or bundle entrypoint.

Usage:
  openknowledge get <name|path>
  openknowledge get <name|path> <entry-or-file>
  openknowledge get <name|path> --info
  openknowledge get <name|path> <entry-or-file> --info
  openknowledge get --help

Arguments:
  name|path      Local Markdown file, registry key, or local bundle path.
  entry-or-file  Optional entrypoint name from okf_bundle_entry_<name> or
                 bundle-relative Markdown file path inside the selected bundle.

Flags:
  --info         Print bundle and selected-file metadata instead of Markdown body.

Behavior:
  With one argument that points at a local Markdown file, get prints that exact
  file.
  With a bundle path or registry key, get prints okf_bundle_entry_default when
  declared. If no default entrypoint exists, it prints the bundle root index.md.
  With a second argument, get first checks root index.md metadata, then treats
  the value as a path inside the bundle.

  Use openknowledge search when you need query-based, token-budgeted Markdown
  context with source ranges and related authored links.

Examples:
  openknowledge get README.md
  openknowledge get accessibility --info
  openknowledge get accessibility
  openknowledge get accessibility review
  openknowledge get accessibility agents/review.md
`
}

func searchHelpText() string {
	return fmt.Sprintf(`openknowledge search

Build source-grounded Markdown context from an Open Knowledge bundle.

Usage:
  openknowledge search <name|path> <query>
  openknowledge search <name|path> <query> --budget <tokens>
  openknowledge search <name|path> <query> --format json
  openknowledge search <name|path> <query> --matches
  openknowledge search <name|path> <query> --no-expand
  openknowledge search <name|path> <query> --limit <count>
  openknowledge search <name|path> <query> --spec <version>
  openknowledge search --help

Arguments:
  name|path      Registry key or local bundle path.
  query          Search text. Quote multi-word queries in shells.

Flags:
  --budget       Approximate context token budget. Defaults to %d.
                 Context mode only; cannot be combined with --matches.
  --format       Output format: markdown or json. Defaults to markdown.
  --limit        Maximum context source or match count. Defaults to 12.
  --matches      Print ranked match diagnostics instead of packed context.
  --no-expand    Exclude one-hop outgoing-link and backlink context.
  --spec         OKF spec version. Defaults to latest.

Behavior:
  Search builds Markdown chunks from parsed heading sections, preserves source
  line ranges and heading paths, scores chunks with BM25-style lexical ranking
  across title, path, type, description, frontmatter, headings, and body text,
  then packs original Markdown under the requested token budget. Fuzzy and
  diacritic-insensitive matching are enabled for local CLI search.

  Direct evidence is packed first. By default, remaining budget can include
  one-hop outgoing local links and backlinks with their relation. Use
  --no-expand for direct lexical matches only, or --matches to inspect scores,
  matched fields, snippets, and relations instead of context Markdown.

Examples:
  openknowledge search Wiki "validation workflow"
  openknowledge search personal "release checklist" --budget 1200
  openknowledge search personal "MCP auth" --matches
  openknowledge search personal "MCP auth" --no-expand
  openknowledge search personal "MCP auth" --format json

Versions:
  %s
`, okf.DefaultContextBudget, supportedSpecVersionsText())
}

func disconnectHelpText(command string) string {
	return fmt.Sprintf(`%s

Remove a knowledge bundle connection from the user registry.

Usage:
  %[1]s <key|path>
  %[1]s <key|path> --keep-files
  %[1]s <key|path> --delete-files
  %[1]s --help

Arguments:
  key|path        Connection key or connected local path.

Flags:
  --keep-files    Keep files after removing the connection. This is the default.
  --delete-files  Delete the complete cache only for CLI-managed remote sources.

Examples:
  %[1]s accessibility
  %[1]s ./project-memory --keep-files
`, command)
}

func connectHelpText(command string) string {
	return fmt.Sprintf(`%s

Connect an Open Knowledge bundle to the user registry.

Usage:
  %[1]s <source>
  %[1]s <source> --as <key>
  %[1]s <source> --access read|write
  %[1]s <source> --no-validate
  %[1]s --help

Arguments:
  source         Local knowledge base root, registry key, Open Knowledge
                 manifest URL, tar archive URL, or Git URL.

Flags:
  --as           Connection key. Defaults to okf_bundle_name, then the folder name.
  --access       Access capability for local connections, read or write. Remote sources are read-only. Defaults to read.
  --no-validate  Skip the validation status check in the success output.

Remote manifests and tar archives are downloaded into the Open Knowledge cache.
Git sources are cloned into the same cache before registration.

Examples:
  %[1]s ./project-memory
  %[1]s ./accessibility --as accessibility
  %[1]s https://openknowledge.sh/wiki/
  %[1]s https://openknowledge.sh/openknowledge-bundle.tar.gz
  %[1]s https://github.com/openknowledge-sh/accessibility.git --as accessibility
  %[1]s ./team-wiki --access write
`, command)
}

func registryHelpText() string {
	return `openknowledge registry

Manage knowledge bundle connections.

Usage:
  openknowledge registry connect <source>
  openknowledge registry connect <source> --as <key>
  openknowledge registry disconnect <key|path>
  openknowledge registry disconnect <key|path> --keep-files
  openknowledge registry refresh <key|path>
  openknowledge registry refresh <key|path> --force
  openknowledge registry list
  openknowledge registry list --json
  openknowledge registry status [key|path]
  openknowledge registry status [key|path] --json
  openknowledge registry where <name|path>
  openknowledge registry --help

Registry keys are shortcuts for local or cached knowledge bundle paths.
Path-based commands continue to work directly, for example openknowledge list
./project-memory.

Top-level openknowledge connect and openknowledge disconnect are aliases for
the registry subcommands.

Examples:
  openknowledge registry connect ./project-memory --as personal
  openknowledge registry list
  openknowledge registry list --json
  openknowledge registry refresh personal
  openknowledge registry status personal
  openknowledge registry where personal
  openknowledge list personal
`
}

func registryListHelpText() string {
	return `openknowledge registry list

List connected knowledge bases without inspecting their contents.

Usage:
  openknowledge registry list
  openknowledge registry list --json
  openknowledge registry list --help

Flags:
  --json  Print the versioned machine-readable registry inventory.

JSON output uses schemaVersion "1" and includes the registry path, sorted
connection names and paths, effective access, managed state, and source provenance
when present. Use registry status when content health is required.
`
}

func registryRefreshHelpText() string {
	return `openknowledge registry refresh

Fetch and verify a new generation of a managed remote knowledge bundle.

Usage:
  openknowledge registry refresh <key|path>
  openknowledge registry refresh <key|path> --force
  openknowledge registry refresh --help

Flags:
  --force  Discard local changes in the managed cache.

The current generation remains registered until the replacement has been
downloaded, validated, and recorded. Local connections cannot be refreshed.
`
}

func registryStatusHelpText() string {
	return `openknowledge registry status

Check registered bundle and managed-cache integrity without contacting remotes.

Usage:
  openknowledge registry status
  openknowledge registry status [key|path]
  openknowledge registry status [key|path] --json
  openknowledge registry status --help

States:
  ok          Bundle validation and recorded identity pass.
  warnings    Validation passes with warnings.
  unverified  Legacy managed cache has no recorded content identity.
  modified    Content, Git state, or provenance differs from the registry.
  invalid     Bundle validation fails.
  missing     Registered bundle or managed root is unavailable.

The command is offline. It checks local content identity and does not determine
whether a newer remote version exists. JSON output uses schemaVersion "1".
`
}

func registryWhereHelpText() string {
	return `openknowledge registry where

Print the absolute path for a named knowledge base or path.

Usage:
  openknowledge registry where <name|path>
  openknowledge registry where --help

Examples:
  openknowledge registry where personal
  openknowledge registry where ./project-memory
`
}

func toHelpText() string {
	return fmt.Sprintf(`openknowledge to

Convert an Open Knowledge bundle to another format.

Usage:
  openknowledge to html --out <folder> [path]
  openknowledge to html --plain --out <folder> [path]
  openknowledge to html --head-file <file> --out <folder> [path]
  openknowledge to html --script-src <src> --out <folder> [path]
  openknowledge to json [path]
  openknowledge to json --out <file> [path]
  openknowledge to tar --out <file> [path]
  openknowledge to graph [path]
  openknowledge to graph --out <file> [path]
  openknowledge to graph --type search [path]
  openknowledge to --help

Targets:
  html       Write a static HTML site. Defaults to the viewer app bundle.
  json       Write normalized bundle JSON.
  tar        Write a portable bundle tar.gz archive.
  graph      Write node and edge graph JSON by graph type.

Flags:
  --spec       OKF spec version. Defaults to latest.
  --out        Output folder for html, optional output file for json/graph, archive file for tar.
  --head-file  Trusted HTML fragment file to inject into default viewer HTML <head>.
  --head-html  Trusted HTML fragment to inject into default viewer HTML <head>.
  --script-src Script src to inject into default viewer HTML <head>. May be repeated.

Versions:
  %s
`, supportedSpecVersionsText())
}

func toHTMLHelpText() string {
	return fmt.Sprintf(`openknowledge to html

Write a static HTML site for an Open Knowledge bundle.

Usage:
  openknowledge to html --out <folder> [path]
  openknowledge to html --plain --out <folder> [path]
  openknowledge to html --head-file <file> --out <folder> [path]
  openknowledge to html --script-src <src> --out <folder> [path]
  openknowledge to html --spec <version> --out <folder> [path]
  openknowledge to html --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out        Output folder for generated HTML files. Required.
  --head-file  Trusted HTML fragment file to inject into default viewer HTML
                <head>. Defaults to OPENKNOWLEDGE_HEAD_FILE when set.
  --head-html  Trusted HTML fragment to inject into default viewer HTML <head>.
                Defaults to OPENKNOWLEDGE_HEAD_HTML when set.
  --plain      Generate plain semantic HTML without CSS, JavaScript, or viewer chrome.
  --script-src Script src to inject into default viewer HTML <head>. May be
                repeated. Defaults to comma- or newline-separated
                OPENKNOWLEDGE_SCRIPT_SRC when set.
  --spec       OKF spec version. Defaults to latest.

Examples:
  openknowledge to html --head-file ./head.html --out ./site ./project-memory
  openknowledge to html --script-src /analytics.js --out ./site ./project-memory
  openknowledge to html --head-html '<meta name="robots" content="noindex">' --out ./site ./project-memory

Connect:
  Default viewer exports include openknowledge.json and
  assets/openknowledge-bundle.tar.gz for remote openknowledge connect.

Theme:
  Default viewer exports read [html.theme] from openknowledge.toml in the
  bundle root. Set stylesheet = "assets/wiki-theme.css" to link theme CSS.
  Built-in variables are defined in viewer_theme.css as --ok-* tokens.

Versions:
  %s
`, supportedSpecVersionsText())
}

func toJSONHelpText() string {
	return fmt.Sprintf(`openknowledge to json

Write normalized JSON for an Open Knowledge bundle.

Usage:
  openknowledge to json [path]
  openknowledge to json --out <file> [path]
  openknowledge to json --spec <version> [path]
  openknowledge to json --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output file. Defaults to stdout.
  --spec      OKF spec version. Defaults to latest.

Versions:
  %s
`, supportedSpecVersionsText())
}

func toTarHelpText() string {
	return fmt.Sprintf(`openknowledge to tar

Write a portable tar.gz archive for an Open Knowledge bundle.

Usage:
  openknowledge to tar --out <file> [path]
  openknowledge to tar --spec <version> --out <file> [path]
  openknowledge to tar --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output archive file. Required.
  --spec      OKF spec version. Defaults to latest.

Versions:
  %s
`, supportedSpecVersionsText())
}

func toGraphHelpText() string {
	return fmt.Sprintf(`openknowledge to graph

Write node and edge graph JSON for an Open Knowledge bundle.

Usage:
  openknowledge to graph [path]
  openknowledge to graph --out <file> [path]
  openknowledge to graph --type source [path]
  openknowledge to graph --type search [path]
  openknowledge to graph --spec <version> [path]
  openknowledge to graph --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output file. Defaults to stdout.
  --spec      OKF spec version. Defaults to latest.
  --type      Graph type: source or search. Defaults to source.

Behavior:
  Source graphs contain one node per parsed bundle file. Edges are deduplicated
  existing local Markdown links and are sourced from the AST-backed parser.

  Search graphs are derivative retrieval artifacts. They include source file
  nodes, Markdown heading chunk nodes, contains edges, chunk reading-order
  edges, and chunk-level local-link edges for graph-expanded search.

Versions:
  %s
`, supportedSpecVersionsText())
}

func setupHelpText() string {
	return `openknowledge setup

Print an agent setup prompt for creating and customizing a knowledge base.

Usage:
  openknowledge setup
  openknowledge setup --rules <rules>
  openknowledge setup --help

The prompt tells an agent to inspect the current workspace, ask tailored
questions, create a bundle with openknowledge new, customize the scaffold, and
validate the result.

Options:
  --rules     Suggest comma-separated maintenance rules for setup.

Available rules:
  project, docs, decisions, changelog, research, bugs, schemas, summary, agents.
  Run openknowledge rules --list for descriptions.
`
}

func fromHelpText() string {
	return `openknowledge from

Print an agent task prompt for turning a source into an Open Knowledge wiki.

The command does not fetch, crawl, call an LLM, or write the wiki itself. It
prints a prompt for Codex, Claude Code, Cursor, Cowork, or another local agent
that can access the source and write files.

Usage:
  openknowledge from <source> --out <folder>
  openknowledge from <source> --out <folder> --type understanding
  openknowledge from <source> --out <folder> --type custom
  openknowledge from <source> --out <folder> --type custom --about <goal>
  openknowledge from <source> --out <folder> --depth <count>
  openknowledge from --help

Arguments:
  source      Source URL or local path. Examples include GitHub repositories,
              local repositories, and website documentation roots.

Options:
  --out       Output Open Knowledge wiki folder. Required.
  --type      Generation recipe: understanding or custom.
              Defaults to understanding.
  --about     Custom goal for --type custom, avoiding the interview step.
  --depth     Website crawl depth or source traversal depth hint.
              Defaults to 0, meaning the agent should choose the minimum depth.

Behavior:
  The generated prompt tells the agent to inspect the source, ask only missing
  questions, create or update the OKF bundle at --out, preserve provenance such
  as source URLs or commit IDs, run openknowledge validate, and finish with
  list/search/get/view commands for the generated wiki. Copy the printed prompt
  into your agent; avoid shell command substitution or piping for interactive
  agent CLIs.

Examples:
  openknowledge from https://github.com/openknowledge-sh/openknowledge --out Wiki
  openknowledge from https://github.com/openknowledge-sh/openknowledge --out Wiki --type custom
  openknowledge from https://github.com/openknowledge-sh/openknowledge --out Wiki --type custom --about "Help new contributors understand the release workflow"
  openknowledge from https://openknowledge.sh/wiki/ --out Wiki --type understanding --depth 2
`
}

func rulesHelpText() string {
	return `openknowledge rules

Print maintenance instructions for AI agents.

The command does not edit files. It prints a Markdown block you can paste into
AGENTS.md, CLAUDE.md, Cursor rules, or any project instruction file.
Built-in rules are always available, and local custom rules can be added as
OKF Markdown files under rules/ in the selected wiki.
The selected wiki's openknowledge.toml may configure [rules].paths for custom
rule directories and [rules].enabled for default selected rules.
It checks the wiki path and prints non-blocking warnings after the rendered
rules when the path does not exist, has no Markdown, or does not validate as
OKF. Each warning includes an agent action. In a terminal warnings print after
the rules on stdout; with pipes or redirection they print to stderr.

Usage:
  openknowledge rules
  openknowledge rules <rules>
  openknowledge rules <rules> --path <path>
  openknowledge rules --target generic|codex|claude|cursor
  openknowledge rules apply <rules> --path <path>
  openknowledge rules --list
  openknowledge rules --help

Arguments:
  rules       Comma-separated maintenance rules to include.
              Defaults to project.

Options:
  --path      Open Knowledge wiki path used in generated rules.
              Defaults to .openknowledge.
  --target    Instruction target: generic, codex, claude, or cursor.
              Defaults to generic.
  --list      List available rules.

Examples:
  openknowledge rules docs,changelog --path Wiki
  openknowledge rules changelog --path Wiki --target codex
  openknowledge rules apply docs,changelog --path Wiki --file AGENTS.md
`
}

func rulesApplyHelpText() string {
	return `openknowledge rules apply

Write generated maintenance instructions into an agent instruction file.

The command updates a managed block between openknowledge:rules markers, so
running it again replaces the previous generated block instead of duplicating it.
It still checks the wiki path and prints non-blocking warnings with agent actions.

Usage:
  openknowledge rules apply
  openknowledge rules apply <rules>
  openknowledge rules apply <rules> --path <path>
  openknowledge rules apply <rules> --path <path> --file <file>
  openknowledge rules apply <rules> --path <path> --dry-run
  openknowledge rules apply <rules> --path <path> --yes
  openknowledge rules apply --help

Arguments:
  rules       Comma-separated maintenance rules to include.
              Defaults to project.

Options:
  --file      Agent instruction file to update.
  --path      Open Knowledge wiki path used in generated rules.
              Defaults to .openknowledge.
  --target    Instruction target: generic, codex, claude, or cursor.
              Defaults to the target inferred from --file when possible.
  --yes       Use the nearest detected agent instruction file without prompting,
              create AGENTS.md when none exists, and skip confirmation.
  --dry-run   Print the managed block that would be written without editing.

Examples:
  openknowledge rules apply docs,changelog --path Wiki --file AGENTS.md
  openknowledge rules apply changelog --path Wiki --yes
  openknowledge rules apply docs --path Wiki --dry-run
`
}

func reviewHelpText() string {
	return `openknowledge review

Print advisory AI review prompts for Open Knowledge workflows.

The command does not call a model, edit files, or decide validation status.
Use openknowledge validate for deterministic CI-safe checks.

Usage:
  openknowledge review rules [path]
  openknowledge review rules --rules <rules> --path <path>
  openknowledge review rules --all [path]
  openknowledge review --help

Subcommands:
  rules      Print an AI review prompt for selected maintenance rules.

Examples:
  openknowledge review rules Wiki
  openknowledge review rules --rules docs,changelog --path Wiki
  openknowledge review rules --all Wiki
`
}

func reviewRulesHelpText() string {
	return `openknowledge review rules

Print an advisory AI review prompt for Open Knowledge maintenance rules.

The prompt tells an agent to inspect evidence, run deterministic validation,
and report source-backed findings. It does not call a model or edit files.

Usage:
  openknowledge review rules [path]
  openknowledge review rules --path <path>
  openknowledge review rules --rules <rules> --path <path>
  openknowledge review rules --all [path]
  openknowledge review rules --help

Arguments:
  path       Open Knowledge wiki path. Defaults to .openknowledge.

Options:
  --path     Open Knowledge wiki path.
  --rules    Comma-separated maintenance rules to review.
             Defaults to [rules].enabled, then project.
  --all      Review every built-in and local custom rule.

Examples:
  openknowledge review rules Wiki
  openknowledge review rules --rules docs,changelog --path Wiki
  openknowledge review rules --all Wiki
`
}

func newHelpText() string {
	return `openknowledge new

Scaffold a local Open Knowledge bundle.

Usage:
  openknowledge new [folder]
  openknowledge new --name <name> [folder]
  openknowledge new --bundle-name <id> --bundle-purpose <text> [folder]
  openknowledge new --no-agents --no-setup [folder]
  openknowledge new --help

Arguments:
  folder       Destination folder. Defaults to a slug derived from the name.

Flags:
  --name       Knowledge base name. If omitted, the CLI prompts for one.
  --bundle-name
               Optional stable bundle id written as okf_bundle_name.
  --bundle-title
               Optional display title written as okf_bundle_title.
  --bundle-purpose
               Optional purpose written as okf_bundle_purpose.
  --bundle-tag
               Optional tag written into okf_bundle_tags. Repeatable.
  --bundle-entry
               Optional entrypoint as name=path, for example
               default=agents/checker.md. Repeatable.
  --no-agents
               Do not create AGENTS.md starter agent rules.
  --no-setup
               Do not create SETUP.MD or print the setup handoff prompt.

Examples:
  openknowledge new ./project-memory
  openknowledge new --no-agents --no-setup ./source-wiki
  openknowledge new --name "Project Memory" ./project-memory
  openknowledge new --name "Accessibility Review" --bundle-name accessibility --bundle-purpose "Accessibility review guidance." --bundle-tag accessibility --bundle-entry default=agents/accessibility-checker.md ./accessibility
`
}

func viewHelpText() string {
	return `openknowledge view

Start a local HTTP Markdown viewer.

Usage:
  openknowledge view [path]
  openknowledge view --name <alias-name> [path]
  openknowledge view --host <host> --port <port> [path]
  openknowledge view --allow-network --host <host> [path]
  openknowledge view --allow-network --host <host> --token <token> [path]
  openknowledge view --head-file <file> [path]
  openknowledge view --script-src <src> [path]
  openknowledge view --no-browser [path]
  openknowledge view --help

Arguments:
  path         Optional knowledge base root or registry name. When omitted,
               the viewer opens the Open Knowledge Registry workspace selector.

Flags:
  --host       Host to bind. Defaults to 127.0.0.1.
  --port       Port to bind. Defaults to 0, which selects a free port.
  --allow-network
               Permit a non-loopback bind. Every route is then protected by a
               generated token or --token/OPENKNOWLEDGE_VIEW_TOKEN.
  --head-file  Trusted HTML fragment file to inject into <head>. Defaults to
               OPENKNOWLEDGE_HEAD_FILE when set.
  --head-html  Trusted HTML fragment to inject into <head>. Defaults to
               OPENKNOWLEDGE_HEAD_HTML when set.
  --name       Alias name for direct path mode. Defaults to the registry name
               or folder name.
  --no-browser
               Print URLs without opening the default browser.
  --script-src Script src to inject into <head>. May be repeated. Defaults to
               comma- or newline-separated OPENKNOWLEDGE_SCRIPT_SRC when set.
  --token      URL-safe viewer token (16-256 characters). Prefer the
               OPENKNOWLEDGE_VIEW_TOKEN environment variable over command-line
               input when process arguments may be visible to other users.

Examples:
  openknowledge view
  openknowledge view personal
  openknowledge view ./project-memory
  openknowledge view --head-file ./head.html ./project-memory
  openknowledge view --script-src /analytics.js ./project-memory
  openknowledge view --port 8080 ./project-memory
  openknowledge view --name project-memory --port 3000 ./project-memory
  openknowledge view --allow-network --host 0.0.0.0 ./project-memory
`
}

func specHelpText() string {
	return fmt.Sprintf(`openknowledge spec

Print an embedded Open Knowledge Format spec.

Usage:
  openknowledge spec latest|<version>
  openknowledge spec --help

Versions:
  %s

Examples:
  openknowledge spec latest
  openknowledge spec 0.1
`, supportedSpecVersionsText())
}

func validateHelpText() string {
	return fmt.Sprintf(`openknowledge validate

Validate a bundle against an Open Knowledge Format spec.

Usage:
  openknowledge validate [key-or-path]
  openknowledge validate --spec <version> [key-or-path]
  openknowledge validate --format json [key-or-path]
  openknowledge validate --format json --out <file> [key-or-path]
  openknowledge validate --rule <rule=off|warn|error> [key-or-path]
  openknowledge validate --quiet [key-or-path]
  openknowledge validate --help

Arguments:
  key-or-path  Registry key or knowledge base root. Defaults to the current directory.

Flags:
  --spec       OKF spec version. Defaults to latest.
  --format     Output format: text or json. Defaults to text.
  --json       Alias for --format json.
  --out        Write a JSON validation report to a file. Requires JSON output.
  --rule       Override one validation rule severity as rule=off|warn|error.
               May be repeated and overrides [validation.rules] config.
  --quiet      Print only validation errors.

Config:
  openknowledge.toml may define [validation.rules] with rule severities:
    link-target = "error"
    markdown-syntax = "off"

Versions:
  %s

Exit codes:
  0            Validation passed, with or without warnings.
  1            Validation found errors after configured severity overrides.
  2            Usage or setup error.
`, supportedSpecVersionsText())
}

func listHelpText() string {
	return fmt.Sprintf(`openknowledge list

Print a bundle tree with inline validation issues.

Usage:
  openknowledge list [key-or-path]
  openknowledge list --spec <version> [key-or-path]
  openknowledge list --depth <n> [key-or-path]
  openknowledge list --json [key-or-path]
  openknowledge list --help

Arguments:
  key-or-path  Registry key or knowledge base root. Defaults to the current directory.

Flags:
  --spec       OKF spec version. Defaults to latest.
  --depth      Maximum tree depth. Defaults to 0 for unlimited depth.
  --json       Print machine-readable inventory JSON.

Versions:
  %s
`, supportedSpecVersionsText())
}

func versionHelpText() string {
	return `openknowledge version

Print the CLI version.

Usage:
  openknowledge version
  openknowledge version --help
`
}

func supportedSpecVersionsText() string {
	return "latest, " + strings.Join(okf.SupportedSpecVersions(), ", ")
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
