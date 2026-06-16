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
	case "open":
		os.Exit(runOpen(os.Args[2:]))
	case "spec":
		os.Exit(runSpec(os.Args[2:]))
	case "validate":
		os.Exit(runValidate(os.Args[2:]))
	case "list":
		os.Exit(runList(os.Args[2:]))
	case "version":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func runSetup(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: openknowledge setup")
		return 2
	}

	fmt.Print(okf.SetupPrompt())
	return 0
}

func runSpec(args []string) int {
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
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	nameFlag := fs.String("name", "", "knowledge base name")
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

	result, err := okf.NewProject(okf.NewProjectOptions{Name: name, Path: path})
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
	fmt.Printf("  Read %s and set up this local Open Knowledge wiki.\n", terminal.path(result.SetupPath))
	fmt.Println("  Start by interviewing me about what the knowledge base should cover, then create")
	fmt.Println("  the initial structure, rules, indexes, and seed pages described there.")
	return 0
}

func runValidate(args []string) int {
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

func helpText() string {
	return `openknowledge creates and validates Open Knowledge Format v0.1 bundles.

Usage:
  openknowledge --help
  openknowledge setup
  openknowledge new [folder]
  openknowledge new --name <name> [folder]
  openknowledge open [path]
  openknowledge open --host <host> --port <port> [path]
  openknowledge spec latest|<version>
  openknowledge validate [path]
  openknowledge validate --spec <version> [path]
  openknowledge list [path]
  openknowledge list --spec <version> [path]
  openknowledge version

Commands:
  setup      Print an agent setup prompt.
  new        Scaffold a local Open Knowledge bundle.
  open       Start a local Markdown viewer.
  spec       Print an embedded OKF spec.
  validate   Validate a bundle against an OKF spec.
  list       Print a bundle tree, with optional JSON output.
  version    Print the CLI version.

Flags:
  -h, --help  Show this help.

Examples:
  openknowledge new ./project-memory
  openknowledge validate ./project-memory
  openknowledge list --json ./project-memory
  openknowledge open ./project-memory
`
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
