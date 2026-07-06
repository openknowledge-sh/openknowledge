package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

var version = "0.1.0"

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
	case "rules":
		os.Exit(runRules(os.Args[2:]))
	case "new":
		os.Exit(runNew(os.Args[2:]))
	case "connect":
		os.Exit(runConnect(os.Args[2:], "openknowledge connect"))
	case "disconnect":
		os.Exit(runDisconnect(os.Args[2:], "openknowledge disconnect"))
	case "use":
		os.Exit(runUse(os.Args[2:]))
	case "search":
		os.Exit(runSearch(os.Args[2:]))
	case "ast":
		os.Exit(runAST(os.Args[2:]))
	case "registry":
		os.Exit(runRegistry(os.Args[2:]))
	case "open":
		os.Exit(runOpen(os.Args[2:]))
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
		fmt.Print(okf.RenderRulesList())
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

type rulesArgs struct {
	wiki   string
	target string
	rules  []string
	list   bool
}

type rulesApplyArgs struct {
	wiki   string
	target string
	rules  []string
	file   string
	yes    bool
	dryRun bool
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
		Name: name,
		Path: path,
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

	fmt.Println()
	terminal.section("Agent handoff")
	fmt.Println("  Paste this into your agent:")
	fmt.Println()
	fmt.Printf("  Set up an Open Knowledge agentic wiki for this workspace. Read %s,\n", terminal.path(result.SetupPath))
	fmt.Println("  inspect this workspace and any relevant memories, ask only the setup questions still needed,")
	fmt.Println("  run openknowledge validate, and show me how to inspect it with openknowledge open.")
	return 0
}

func runRegistry(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, registryHelpText())
		return 0
	}

	switch args[0] {
	case "list":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "usage: openknowledge registry list")
			return 2
		}
		entries, err := okf.RegistryEntries()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		printRegistryEntries(entries)
		return 0
	case "connect":
		return runConnect(args[1:], "openknowledge registry connect")
	case "disconnect":
		return runDisconnect(args[1:], "openknowledge registry disconnect")
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
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s <source> [--as <key>]\n", command)
		return 2
	}

	source := fs.Arg(0)
	sourceInfo := okf.RegistrySource{}
	if looksLikeRemoteSource(source) {
		var err error
		var materializedRoot string
		materializedRoot, sourceInfo, err = materializeRemoteSource(source, strings.TrimSpace(*keyFlag))
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

	entry, warning, err := okf.ConnectRegistryEntryWithSource(key, root, *accessFlag, explicitKey, sourceInfo)
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

func materializeRemoteSource(source string, key string) (string, okf.RegistrySource, error) {
	source = strings.TrimSpace(source)
	cacheRoot, err := remoteBundleCacheRoot()
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	name := registryCacheName(source, key)
	target := filepath.Join(cacheRoot, name)
	if root, ok := cachedBundleRoot(target); ok {
		return root, okf.RegistrySource{Type: remoteSourceType(source), URL: source}, nil
	}
	if err := os.MkdirAll(cacheRoot, 0755); err != nil {
		return "", okf.RegistrySource{}, err
	}

	if looksLikeManifestSource(source) {
		root, archiveURL, err := materializeManifestSource(source, target)
		if err != nil {
			return "", okf.RegistrySource{}, err
		}
		return root, okf.RegistrySource{Type: "manifest", URL: source, Ref: archiveURL}, nil
	}
	if looksLikeArchiveSource(source) {
		root, err := materializeArchiveSource(source, target, "")
		if err != nil {
			return "", okf.RegistrySource{}, err
		}
		return root, okf.RegistrySource{Type: "tar", URL: source}, nil
	}
	if isHTTPSource(source) {
		for _, candidate := range manifestCandidateURLs(source) {
			manifest, ok, err := fetchBundleManifest(candidate)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			if !ok {
				continue
			}
			archiveURL, err := resolveManifestArchiveURL(candidate, manifest.Archive)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			root, err := materializeArchiveSource(archiveURL, target, manifest.ArchiveSHA256)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			return root, okf.RegistrySource{Type: "manifest", URL: candidate, Ref: archiveURL}, nil
		}
		if root, ok, err := tryMaterializeDirectArchive(source, target); err != nil {
			return "", okf.RegistrySource{}, err
		} else if ok {
			return root, okf.RegistrySource{Type: "tar", URL: source}, nil
		}
	}

	cmd := exec.Command("git", "clone", "--depth", "1", source, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return "", okf.RegistrySource{}, fmt.Errorf("could not clone remote bundle %s: %s", source, detail)
	}
	return target, okf.RegistrySource{Type: "git", URL: source}, nil
}

type remoteBundleManifest struct {
	Type          string `json:"type"`
	Version       int    `json:"version"`
	Spec          string `json:"spec"`
	Name          string `json:"name"`
	Title         string `json:"title"`
	Archive       string `json:"archive"`
	ArchiveSHA256 string `json:"archiveSha256"`
	ArchiveFormat string `json:"archiveFormat"`
}

func materializeManifestSource(source string, target string) (string, string, error) {
	manifest, ok, err := fetchBundleManifest(source)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", fmt.Errorf("Open Knowledge manifest not found: %s", source)
	}
	archiveURL, err := resolveManifestArchiveURL(source, manifest.Archive)
	if err != nil {
		return "", "", err
	}
	root, err := materializeArchiveSource(archiveURL, target, manifest.ArchiveSHA256)
	if err != nil {
		return "", "", err
	}
	return root, archiveURL, nil
}

func materializeArchiveSource(source string, target string, expectedSHA256 string) (string, error) {
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-source-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "bundle.tar.gz")
	contentType, err := downloadRemoteFile(source, archivePath)
	if err != nil {
		return "", err
	}
	if !looksLikeArchiveSource(source) && !downloadedFileLooksLikeArchive(archivePath, contentType) {
		return "", fmt.Errorf("remote source is not a tar archive: %s", source)
	}
	if strings.TrimSpace(expectedSHA256) != "" {
		actual, err := okf.SHA256File(archivePath)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(actual, strings.TrimSpace(expectedSHA256)) {
			return "", fmt.Errorf("archive checksum mismatch for %s", source)
		}
	}

	extractRoot := filepath.Join(tempDir, "extract")
	if err := okf.ExtractBundleArchive(archivePath, extractRoot); err != nil {
		return "", err
	}
	bundleRoot, err := validatedExtractedBundleRoot(extractRoot)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	if err := os.Rename(extractRoot, target); err != nil {
		return "", err
	}
	if bundleRoot == extractRoot {
		return target, nil
	}
	rel, err := filepath.Rel(extractRoot, bundleRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(target, rel), nil
}

func tryMaterializeDirectArchive(source string, target string) (string, bool, error) {
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-probe-*")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "probe")
	contentType, err := downloadRemoteFile(source, archivePath)
	if err != nil {
		return "", false, nil
	}
	if !downloadedFileLooksLikeArchive(archivePath, contentType) {
		return "", false, nil
	}
	root, err := materializeArchiveFile(archivePath, target, "")
	if err != nil {
		return "", false, err
	}
	return root, true, nil
}

func materializeArchiveFile(archivePath string, target string, expectedSHA256 string) (string, error) {
	if strings.TrimSpace(expectedSHA256) != "" {
		actual, err := okf.SHA256File(archivePath)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(actual, strings.TrimSpace(expectedSHA256)) {
			return "", fmt.Errorf("archive checksum mismatch")
		}
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-extract-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)
	extractRoot := filepath.Join(tempDir, "extract")
	if err := okf.ExtractBundleArchive(archivePath, extractRoot); err != nil {
		return "", err
	}
	bundleRoot, err := validatedExtractedBundleRoot(extractRoot)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	if err := os.Rename(extractRoot, target); err != nil {
		return "", err
	}
	if bundleRoot == extractRoot {
		return target, nil
	}
	rel, err := filepath.Rel(extractRoot, bundleRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(target, rel), nil
}

func fetchBundleManifest(source string) (remoteBundleManifest, bool, error) {
	tempDir, err := os.MkdirTemp("", "openknowledge-manifest-*")
	if err != nil {
		return remoteBundleManifest{}, false, err
	}
	defer os.RemoveAll(tempDir)
	manifestPath := filepath.Join(tempDir, "openknowledge.json")
	if _, err := downloadRemoteFile(source, manifestPath); err != nil {
		return remoteBundleManifest{}, false, nil
	}
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return remoteBundleManifest{}, false, err
	}
	var manifest remoteBundleManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return remoteBundleManifest{}, false, err
	}
	if manifest.Type != okf.BundleManifestType {
		return remoteBundleManifest{}, false, fmt.Errorf("unsupported Open Knowledge manifest type: %s", manifest.Type)
	}
	if strings.TrimSpace(manifest.Archive) == "" {
		return remoteBundleManifest{}, false, fmt.Errorf("Open Knowledge manifest is missing archive")
	}
	return manifest, true, nil
}

func downloadRemoteFile(source string, target string) (string, error) {
	parsed, err := url.Parse(source)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "file" {
		inputPath, err := url.PathUnescape(parsed.Path)
		if err != nil {
			return "", err
		}
		return "", copyFile(inputPath, target)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported archive URL scheme: %s", parsed.Scheme)
	}
	client := http.Client{Timeout: 30 * time.Second}
	response, err := client.Get(source)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", fmt.Errorf("GET %s returned %s", source, response.Status)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", err
	}
	output, err := os.Create(target)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(output, response.Body); err != nil {
		_ = output.Close()
		return "", err
	}
	if err := output.Close(); err != nil {
		return "", err
	}
	return response.Header.Get("Content-Type"), nil
}

func copyFile(source string, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	output, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func validatedExtractedBundleRoot(root string) (string, error) {
	if result, err := okf.Validate(root); err == nil && len(result.Errors) == 0 {
		return result.Root, nil
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
		if result, err := okf.Validate(directories[0]); err == nil && len(result.Errors) == 0 {
			return result.Root, nil
		}
	}
	return "", fmt.Errorf("archive does not contain a valid Open Knowledge bundle")
}

func cachedBundleRoot(target string) (string, bool) {
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		return "", false
	}
	root, err := validatedExtractedBundleRoot(target)
	if err != nil {
		_ = os.RemoveAll(target)
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

func remoteSourceType(source string) string {
	if looksLikeManifestSource(source) {
		return "manifest"
	}
	if looksLikeArchiveSource(source) {
		return "tar"
	}
	return "git"
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

func registryCacheName(source string, key string) string {
	base := okf.RegistryKeyFromNameForCache(key)
	if base == "" {
		trimmed := strings.TrimRight(source, "/")
		base = okf.RegistryKeyFromNameForCache(filepath.Base(trimmed))
	}
	if base == "" {
		base = "bundle"
	}
	sum := sha256.Sum256([]byte(source))
	return base + "-" + hex.EncodeToString(sum[:])[:12]
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
	if err := fs.Parse(args); err != nil {
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
	entry, ok, err := okf.ResolveRegistryTarget(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !ok {
		printUnknownConnection(target)
		return 1
	}
	if *deleteFilesFlag && !entry.Managed {
		fmt.Fprintf(os.Stderr, "refusing to delete non-managed files: %s\n", entry.Path)
		return 1
	}

	entry, ok, err = okf.RemoveRegistryEntry(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !ok {
		printUnknownConnection(target)
		return 1
	}

	files := "kept"
	if *deleteFilesFlag {
		if err := os.RemoveAll(entry.Path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: disconnected but could not delete %s: %v\n", entry.Path, err)
			files = "delete failed"
		} else {
			files = "deleted"
		}
	}
	printDisconnectResult(entry, files)
	if files == "delete failed" {
		return 1
	}
	return 0
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

type useOptions struct {
	target string
	entry  string
	info   bool
}

type searchOptions struct {
	target      string
	query       string
	format      string
	spec        string
	limit       int
	expandGraph bool
}

type useSelection struct {
	name string
	rel  string
	abs  string
}

// use remains the deterministic entrypoint/file reader. Query retrieval moved
// to runSearch so callers do not mix "load this document" with "rank chunks".
func runUse(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, useHelpText())
		return 0
	}
	options, err := parseUseOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
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
		if err := printUseInfo(root, info, options.entry); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	selection, err := selectUseEntrypoint(root, info, options.entry)
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

// search is the CLI retrieval surface: resolve a key/path, build the
// section-level knowledge index, then print ranked chunks as text or JSON.
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
	result, err := okf.SearchKnowledgeWithVersion(root, options.spec, okf.SearchOptions{
		Query:       options.query,
		Limit:       options.limit,
		Fuzzy:       true,
		ExpandGraph: options.expandGraph,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := printSearchResult(result, options.format); err != nil {
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

func parseUseOptions(args []string) (useOptions, error) {
	options := useOptions{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--info":
			options.info = true
		case arg == "--query":
			return useOptions{}, removedUseQueryError()
		case strings.HasPrefix(arg, "--query="):
			return useOptions{}, removedUseQueryError()
		case arg == "--budget" || strings.HasPrefix(arg, "--budget=") ||
			arg == "--limit" || strings.HasPrefix(arg, "--limit=") ||
			arg == "--format" || strings.HasPrefix(arg, "--format=") ||
			arg == "--spec" || strings.HasPrefix(arg, "--spec="):
			// These were query-mode flags before search became a standalone
			// command. Keep the failure explicit so scripts migrate cleanly.
			return useOptions{}, fmt.Errorf("%s is a search option; use openknowledge search <bundle> <query>", strings.Split(arg, "=")[0])
		case strings.HasPrefix(arg, "-"):
			return useOptions{}, fmt.Errorf("unknown flag: %s", arg)
		case options.target == "":
			options.target = arg
		case options.entry == "":
			options.entry = arg
		default:
			return useOptions{}, fmt.Errorf("use accepts at most one entry")
		}
	}
	if options.target == "" {
		return useOptions{}, fmt.Errorf("usage: openknowledge use <name|path> [entry]")
	}
	return options, nil
}

func removedUseQueryError() error {
	return fmt.Errorf("openknowledge use --query has been removed. Use: openknowledge search <bundle> <query>")
}

func parseSearchOptions(args []string) (searchOptions, error) {
	options := searchOptions{
		format: "text",
		spec:   "latest",
		limit:  12,
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
		case arg == "--expand":
			value, next, err := nextFlagValue(args, index, "--expand")
			if err != nil {
				return searchOptions{}, err
			}
			if err := applySearchExpand(&options, value); err != nil {
				return searchOptions{}, err
			}
			index = next
		case strings.HasPrefix(arg, "--expand="):
			if err := applySearchExpand(&options, strings.TrimPrefix(arg, "--expand=")); err != nil {
				return searchOptions{}, err
			}
		case strings.HasPrefix(arg, "-"):
			return searchOptions{}, fmt.Errorf("unknown search option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	if options.format == "" {
		options.format = "text"
	}
	if options.format != "text" && options.format != "json" {
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
	return options, nil
}

func applySearchExpand(options *searchOptions, value string) error {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "graph":
		options.expandGraph = true
	case "none", "off":
		options.expandGraph = false
	default:
		return fmt.Errorf("unsupported search expansion %q; use graph", value)
	}
	return nil
}

func printSearchResult(result okf.SearchResultSet, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case "text":
		printSearchTextResult(result)
	default:
		return fmt.Errorf("unsupported search format: %s", format)
	}
	return nil
}

func printSearchTextResult(result okf.SearchResultSet) {
	terminal.title("Open Knowledge Search", "source-grounded chunks")
	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Root: %s\n", terminal.path(result.Root))
	fmt.Printf("Results: %d\n", len(result.Results))
	if len(result.Results) == 0 {
		fmt.Println()
		fmt.Println("No matching chunks found.")
		return
	}
	fmt.Println()
	for index, match := range result.Results {
		location := match.Path
		if match.LineStart > 0 {
			location = fmt.Sprintf("%s:%d-%d", match.Path, match.LineStart, match.LineEnd)
		}
		relation := "direct"
		if match.Neighbor {
			relation = match.Relation
		}
		fmt.Printf("%d. %s\n", index+1, location)
		if strings.TrimSpace(match.Heading) != "" {
			fmt.Printf("   heading: %s\n", match.Heading)
		}
		if len(match.HeadingPath) > 0 {
			fmt.Printf("   path: %s\n", strings.Join(match.HeadingPath, " > "))
		}
		if strings.TrimSpace(match.Type) != "" {
			fmt.Printf("   type: %s\n", match.Type)
		}
		fmt.Printf("   score: %.2f (%s)\n", match.Score, relation)
		if strings.TrimSpace(match.Snippet) != "" {
			fmt.Printf("   %s\n", match.Snippet)
		}
	}
}

func selectUseEntrypoint(root string, info okf.BundleInfo, entryName string) (useSelection, error) {
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
				return useSelection{}, fmt.Errorf("entrypoint or path %q does not exist; this bundle has no declared entrypoints", name)
			}
			return useSelection{}, fmt.Errorf("entrypoint or path %q does not exist; available entries: %s", name, strings.Join(available, ", "))
		}
		return useSelection{}, err
	}
	return useSelection{name: name, rel: normalizedRel, abs: abs}, nil
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
	abs := filepath.Join(root, rel)
	relative, err := filepath.Rel(root, abs)
	if err != nil {
		return "", "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("entrypoint path must stay inside the bundle: %s", rel)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("entrypoint path is a directory: %s", rel)
	}
	return abs, filepath.ToSlash(relative), nil
}

func printUseInfo(root string, info okf.BundleInfo, entryName string) error {
	terminal.title("Open Knowledge Use", "agent entrypoint metadata")
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
		selection, err := selectUseEntrypoint(root, info, entryName)
		if err != nil {
			return err
		}
		document, err := okf.ReadMarkdownDocumentInfo(selection.abs, selection.rel)
		if err != nil {
			return err
		}
		printUseEntrypointInfo(selection, document)
		return nil
	}

	if len(info.Metadata.Entries) == 0 {
		selection, err := selectUseEntrypoint(root, info, "")
		if err != nil {
			return err
		}
		document, err := okf.ReadMarkdownDocumentInfo(selection.abs, selection.rel)
		if err != nil {
			return err
		}
		printUseEntrypointInfo(selection, document)
		return nil
	}

	terminal.section("Entrypoints")
	for _, entry := range info.Metadata.Entries {
		selection, err := selectUseEntrypoint(root, info, entry.Name)
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

func printUseEntrypointInfo(selection useSelection, document okf.MarkdownDocumentInfo) {
	terminal.section("Entrypoint")
	fmt.Printf("%-12s %s\n", "entry", selection.name)
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
	if err := os.WriteFile(out, data, 0644); err != nil {
		return err
	}
	terminal.success("Wrote validation report")
	fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(result.Root))
	fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(out))
	return nil
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
	if err := fs.Parse(args); err != nil {
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
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(listing.Entries); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		return 0
	}

	printListTree(listing)
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
		if err := os.WriteFile(options.out, data, 0644); err != nil {
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
		if err := os.WriteFile(options.out, data, 0644); err != nil {
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

func printListTree(listing okf.ListResult) {
	terminal.title("Open Knowledge List", "bundle tree")
	fmt.Printf("%s %s\n", terminal.muted("target"), terminal.path(listing.Root))
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
	printListChildren(children, "")
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

func printListChildren(nodes []*listTreeNode, prefix string) {
	for index, node := range nodes {
		last := index == len(nodes)-1
		connector := "|-- "
		nextPrefix := prefix + "|   "
		if last {
			connector = "`-- "
			nextPrefix = prefix + "    "
		}
		fmt.Println(prefix + connector + formatListNode(node))
		if len(node.children) > 0 {
			printListChildren(sortedListChildren(node), nextPrefix)
		}
	}
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
  openknowledge rules
  openknowledge rules <rules> --path <path>
  openknowledge rules apply <rules> --path <path>
  openknowledge rules --list
  openknowledge new [folder]
  openknowledge new --name <name> [folder]
  openknowledge new --bundle-name <id> --bundle-purpose <text> [folder]
  openknowledge connect <source>
  openknowledge connect <source> --as <key>
  openknowledge disconnect <key|path>
  openknowledge use <name|path> [entry]
  openknowledge use <name|path> --info
  openknowledge search <name|path> <query>
  openknowledge search <name|path> <query> --format json
  openknowledge search <name|path> <query> --expand graph
  openknowledge ast [path]
  openknowledge ast --out <file> [path]
  openknowledge registry connect <source>
  openknowledge registry connect <source> --as <key>
  openknowledge registry disconnect <key|path>
  openknowledge registry list
  openknowledge registry where <name|path>
  openknowledge open [path]
  openknowledge open --name <alias-name> [path]
  openknowledge open --host <host> --port <port> [path]
  openknowledge open --head-file <file> [path]
  openknowledge open --script-src <src> [path]
  openknowledge open --no-browser [path]
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
  openknowledge list --json [key-or-path]
  openknowledge version

Commands:
  setup      Print an agent setup prompt.
  rules      Print agent maintenance rules.
  new        Scaffold a local Open Knowledge bundle.
  connect    Connect a local or remote knowledge bundle.
  disconnect Remove a knowledge bundle connection.
  use        Print an agent entrypoint from a bundle.
  search     Search source-grounded Markdown chunks in a bundle.
  ast        Print parsed OKF AST JSON.
  registry   Manage knowledge bundle connections.
  open       Start the registry or knowledge base Markdown viewer.
  to         Convert a bundle to another format.
  spec       Print an embedded OKF spec.
  validate   Validate a bundle against an OKF spec.
  list       Print a bundle tree, with optional JSON output.
  version    Print the CLI version.

Flags:
  -h, --help  Show this help.

Run openknowledge <command> --help for command-specific help.

Examples:
  openknowledge rules docs,changelog --path Wiki
  openknowledge rules apply docs,changelog --path Wiki --file AGENTS.md
  openknowledge setup --rules docs,changelog
  openknowledge new ./project-memory
  openknowledge new --name "Accessibility Review" --bundle-name accessibility --bundle-tag accessibility ./accessibility
  openknowledge connect ./accessibility --as accessibility
  openknowledge use accessibility --info
  openknowledge use accessibility
  openknowledge search accessibility "validation workflow"
  openknowledge ast ./project-memory
  openknowledge disconnect accessibility
  openknowledge registry connect ./team-wiki --as team
  openknowledge registry where accessibility
  openknowledge list personal
  openknowledge validate ./project-memory
  openknowledge to html --out ./site ./project-memory
  openknowledge to json ./project-memory
  openknowledge to tar --out ./bundle.tar.gz ./project-memory
  openknowledge to graph ./project-memory
  openknowledge list --json ./project-memory
  openknowledge open
  openknowledge open ./project-memory
`
}

func useHelpText() string {
	return `openknowledge use

Print an agent-facing entrypoint from a knowledge bundle.

Usage:
  openknowledge use <name|path>
  openknowledge use <name|path> <entry>
  openknowledge use <name|path> --info
  openknowledge use <name|path> <entry> --info
  openknowledge use --help

Arguments:
  name|path      Registry key or local bundle path.
  entry          Optional entrypoint name from okf_bundle_entry_<name> or
                 bundle-relative file path.

Flags:
  --info         Print bundle and entrypoint metadata instead of Markdown body.

Behavior:
  Without an entry, use prints okf_bundle_entry_default when declared. If no
  default entrypoint exists, it prints the bundle root index.md. With an entry,
  use first checks root index.md metadata, then treats the value as a path
  inside the bundle.

  Use openknowledge search when you need query-based retrieval across Markdown
  sections, heading paths, snippets, and optional graph expansion.

Examples:
  openknowledge use accessibility --info
  openknowledge use accessibility
  openknowledge use accessibility review
  openknowledge use accessibility agents/review.md
`
}

func searchHelpText() string {
	return fmt.Sprintf(`openknowledge search

Search source-grounded Markdown chunks in an Open Knowledge bundle.

Usage:
  openknowledge search <name|path> <query>
  openknowledge search <name|path> <query> --format json
  openknowledge search <name|path> <query> --expand graph
  openknowledge search <name|path> <query> --limit <count>
  openknowledge search <name|path> <query> --spec <version>
  openknowledge search --help

Arguments:
  name|path      Registry key or local bundle path.
  query          Search text. Quote multi-word queries in shells.

Flags:
  --expand       Optional expansion mode. Use graph to include outgoing local
                 links and backlinks as lower-ranked neighbor results.
  --format       Output format: text or json. Defaults to text.
  --limit        Maximum result count. Defaults to 12.
  --spec         OKF spec version. Defaults to latest.

Behavior:
  Search builds Markdown chunks from parsed heading sections, preserves source
  line ranges and heading paths, scores chunks with BM25-style lexical ranking
  across title, path, type, description, frontmatter, headings, and body text,
  and returns source snippets. Fuzzy and diacritic-insensitive matching are
  enabled for local CLI search.

  With --expand graph, direct matches are followed by linked neighbor chunks
  and backlinks. Neighbor results are marked with their relation.

Examples:
  openknowledge search Wiki "validation workflow"
  openknowledge search personal "release checklist" --limit 5
  openknowledge search personal "MCP auth" --expand graph
  openknowledge search personal "MCP auth" --format json

Versions:
  %s
`, supportedSpecVersionsText())
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
  --delete-files  Delete files only for CLI-managed remote clones.

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
  --access       Access label stored with the connection, read or write. Defaults to read.
  --no-validate  Skip the validation status check in the success output.

Remote manifests and tar archives are downloaded into the Open Knowledge cache.
Git sources are cloned into the same cache before registration.

Examples:
  %[1]s ./project-memory
  %[1]s ./accessibility --as accessibility
  %[1]s https://openknowledge.sh/wiki/
  %[1]s https://example.com/openknowledge-bundle.tar.gz
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
  openknowledge registry list
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
  openknowledge registry where personal
  openknowledge list personal
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

func rulesHelpText() string {
	return `openknowledge rules

Print maintenance instructions for AI agents.

The command does not edit files. It prints a Markdown block you can paste into
AGENTS.md, CLAUDE.md, Cursor rules, or any project instruction file.
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

func newHelpText() string {
	return `openknowledge new

Scaffold a local Open Knowledge bundle.

Usage:
  openknowledge new [folder]
  openknowledge new --name <name> [folder]
  openknowledge new --bundle-name <id> --bundle-purpose <text> [folder]
  openknowledge new --help

Arguments:
  folder       Destination folder. Defaults to the current directory.

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

Examples:
  openknowledge new ./project-memory
  openknowledge new --name "Project Memory" ./project-memory
  openknowledge new --name "Accessibility Review" --bundle-name accessibility --bundle-purpose "Accessibility review guidance." --bundle-tag accessibility --bundle-entry default=agents/accessibility-checker.md ./accessibility
`
}

func openHelpText() string {
	return `openknowledge open

Start a local HTTP Markdown viewer.

Usage:
  openknowledge open [path]
  openknowledge open --name <alias-name> [path]
  openknowledge open --host <host> --port <port> [path]
  openknowledge open --head-file <file> [path]
  openknowledge open --script-src <src> [path]
  openknowledge open --no-browser [path]
  openknowledge open --help

Arguments:
  path         Optional knowledge base root or registry name. When omitted,
               the viewer opens the Open Knowledge Registry workspace selector.

Flags:
  --host       Host to bind. Defaults to 127.0.0.1.
  --port       Port to bind. Defaults to 0, which selects a free port.
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

Examples:
  openknowledge open
  openknowledge open personal
  openknowledge open ./project-memory
  openknowledge open --head-file ./head.html ./project-memory
  openknowledge open --script-src /analytics.js ./project-memory
  openknowledge open --port 8080 ./project-memory
  openknowledge open --name project-memory --port 3000 ./project-memory
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
  openknowledge list --json [key-or-path]
  openknowledge list --help

Arguments:
  key-or-path  Registry key or knowledge base root. Defaults to the current directory.

Flags:
  --spec       OKF spec version. Defaults to latest.
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
