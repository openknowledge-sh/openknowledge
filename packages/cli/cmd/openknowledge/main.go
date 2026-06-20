package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	case "new":
		os.Exit(runNew(os.Args[2:]))
	case "connect":
		os.Exit(runConnect(os.Args[2:]))
	case "registry":
		os.Exit(runRegistry(os.Args[2:]))
	case "where":
		os.Exit(runWhere(os.Args[2:]))
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
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: openknowledge setup")
		return 2
	}

	fmt.Print(okf.SetupPrompt())
	return 0
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
	if len(args) == 0 || hasHelpFlag(args) {
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
	case "add":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: openknowledge registry add <name> <path>")
			return 2
		}
		entry, err := okf.AddRegistryEntry(args[1], args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		terminal.success("Registered knowledge base")
		fmt.Printf("%s %s\n", terminal.muted("name"), entry.Name)
		fmt.Printf("%s %s\n", terminal.muted("path"), terminal.path(entry.Path))
		return 0
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

func runConnect(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, connectHelpText())
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
		fmt.Fprintln(os.Stderr, "usage: openknowledge connect <path> [--as <key>]")
		return 2
	}

	source := fs.Arg(0)
	if looksLikeRemoteSource(source) {
		fmt.Fprintln(os.Stderr, "remote bundle sources are not supported yet; clone the bundle locally and connect its directory")
		return 2
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

	entry, warning, err := okf.ConnectRegistryEntry(key, root, *accessFlag, explicitKey)
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
		strings.HasPrefix(value, "git@")
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

func runWhere(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, whereHelpText())
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: openknowledge where <name|path>")
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
	if err := fs.Parse(args); err != nil {
		return 2
	}

	root := "."
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "validate accepts at most one path")
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

	result, err := okf.ValidateWithVersion(root, *specVersion)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
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
		fmt.Fprintln(os.Stderr, "list accepts at most one path")
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
	default:
		fmt.Fprintf(os.Stderr, "unknown to target: %s\n\n", args[0])
		fmt.Fprint(os.Stderr, toHelpText())
		return 2
	}
}

type toOptions struct {
	path  string
	out   string
	spec  string
	plain bool
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

	var result okf.HTMLResult
	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.plain {
		result, err = okf.WritePlainHTMLWithVersion(root, options.out, options.spec)
	} else {
		result, err = writeViewerHTMLWithVersion(root, options.out, options.spec)
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
		case arg == "--plain":
			options.plain = true
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
  openknowledge new [folder]
  openknowledge new --name <name> [folder]
  openknowledge new --bundle-name <id> --bundle-purpose <text> [folder]
  openknowledge connect <path>
  openknowledge connect <path> --as <key>
  openknowledge registry list
  openknowledge registry add <name> <path>
  openknowledge where <name|path>
  openknowledge open [path]
  openknowledge open --name <alias-name> [path]
  openknowledge open --host <host> --port <port> [path]
  openknowledge open --no-browser [path]
  openknowledge to html --out <folder> [path]
  openknowledge to json [path]
  openknowledge to json --out <file> [path]
  openknowledge spec latest|<version>
  openknowledge validate [path]
  openknowledge validate --spec <version> [path]
  openknowledge list [path]
  openknowledge list --spec <version> [path]
  openknowledge list --json [path]
  openknowledge version

Commands:
  setup      Print an agent setup prompt.
  new        Scaffold a local Open Knowledge bundle.
  connect    Connect a local knowledge bundle.
  registry   Manage named knowledge base paths.
  where      Print the path for a named knowledge base or path.
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
  openknowledge new ./project-memory
  openknowledge new --name "Accessibility Review" --bundle-name accessibility --bundle-tag accessibility ./accessibility
  openknowledge connect ./accessibility --as accessibility
  openknowledge registry add personal ~/knowledge
  openknowledge where personal
  openknowledge list personal
  openknowledge validate ./project-memory
  openknowledge to html --out ./site ./project-memory
  openknowledge to json ./project-memory
  openknowledge list --json ./project-memory
  openknowledge open
  openknowledge open ./project-memory
`
}

func connectHelpText() string {
	return `openknowledge connect

Connect a local Open Knowledge bundle to the user registry.

Usage:
  openknowledge connect <path>
  openknowledge connect <path> --as <key>
  openknowledge connect <path> --access read|write
  openknowledge connect <path> --no-validate
  openknowledge connect --help

Arguments:
  path           Local knowledge base root. Registry names are also accepted
                 and resolve to their stored local path.

Flags:
  --as           Connection key. Defaults to okf_bundle_name, then the folder name.
  --access       Access label stored with the connection, read or write. Defaults to read.
  --no-validate  Skip the validation status check in the success output.

Remote URL sources are not supported yet. Clone remote bundles locally, then
connect the local directory.

Examples:
  openknowledge connect ./project-memory
  openknowledge connect ./accessibility --as accessibility
  openknowledge connect ./team-wiki --access write
`
}

func registryHelpText() string {
	return `openknowledge registry

Manage named knowledge base paths.

Usage:
  openknowledge registry list
  openknowledge registry add <name> <path>
  openknowledge registry --help

Registry names are shortcuts for normal filesystem paths. Path-based commands
continue to work directly, for example openknowledge list ./project-memory.

Examples:
  openknowledge registry add personal ~/knowledge
  openknowledge registry list
  openknowledge list personal
`
}

func whereHelpText() string {
	return `openknowledge where

Print the absolute path for a named knowledge base or path.

Usage:
  openknowledge where <name|path>
  openknowledge where --help

Examples:
  openknowledge where personal
  openknowledge where ./project-memory
`
}

func toHelpText() string {
	return fmt.Sprintf(`openknowledge to

Convert an Open Knowledge bundle to another format.

Usage:
  openknowledge to html --out <folder> [path]
  openknowledge to html --plain --out <folder> [path]
  openknowledge to json [path]
  openknowledge to json --out <file> [path]
  openknowledge to --help

Targets:
  html       Write a static HTML site. Defaults to the viewer app bundle.
  json       Write normalized bundle JSON.

Flags:
  --spec     OKF spec version. Defaults to latest.
  --out      Output folder for html, optional output file for json.

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
  openknowledge to html --spec <version> --out <folder> [path]
  openknowledge to html --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output folder for generated HTML files. Required.
  --plain     Generate plain semantic HTML without CSS, JavaScript, or viewer chrome.
  --spec      OKF spec version. Defaults to latest.

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

func setupHelpText() string {
	return `openknowledge setup

Print an agent setup prompt for creating and customizing a knowledge base.

Usage:
  openknowledge setup
  openknowledge setup --help

The prompt tells an agent to inspect the current workspace, ask tailored
questions, create a bundle with openknowledge new, customize the scaffold, and
validate the result.
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
  openknowledge open --no-browser [path]
  openknowledge open --help

Arguments:
  path         Optional knowledge base root or registry name. When omitted,
               the viewer opens the Open Knowledge Registry workspace selector.

Flags:
  --host       Host to bind. Defaults to 127.0.0.1.
  --port       Port to bind. Defaults to 0, which selects a free port.
  --name       Alias name for direct path mode. Defaults to the registry name
               or folder name.
  --no-browser
               Print URLs without opening the default browser.

Examples:
  openknowledge open
  openknowledge open personal
  openknowledge open ./project-memory
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
  openknowledge validate [path]
  openknowledge validate --spec <version> [path]
  openknowledge validate --quiet [path]
  openknowledge validate --help

Arguments:
  path         Knowledge base root. Defaults to the current directory.

Flags:
  --spec       OKF spec version. Defaults to latest.
  --quiet      Print only validation errors.

Versions:
  %s

Exit codes:
  0            Validation passed, with or without warnings.
  1            Validation found errors.
  2            Usage or setup error.
`, supportedSpecVersionsText())
}

func listHelpText() string {
	return fmt.Sprintf(`openknowledge list

Print a bundle tree with inline validation issues.

Usage:
  openknowledge list [path]
  openknowledge list --spec <version> [path]
  openknowledge list --json [path]
  openknowledge list --help

Arguments:
  path         Knowledge base root. Defaults to the current directory.

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
