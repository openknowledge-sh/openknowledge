package main

import (
	"fmt"
	"os"
	"strings"
)

type rootCommand struct {
	Name        string
	Group       string
	Summary     string
	Subcommands map[string]struct{}
	Run         func([]string) int
}

type commandGroup struct {
	Name string
}

var commandGroups = []commandGroup{
	{Name: "Create and maintain"},
	{Name: "Use and publish"},
	{Name: "Run as a service"},
	{Name: "Validate and connect"},
	{Name: "Advanced and portable tools"},
}

var rootCommandCatalog = []rootCommand{
	{Name: "setup", Group: "Create and maintain", Summary: "Launch an agent to create, validate, and integrate a knowledge base.", Run: runSetup},
	{Name: "agent", Group: "Create and maintain", Summary: "Run, integrate, and review knowledge with an agent.", Subcommands: commandNames("exec", "integrate", "doctor"), Run: runAgent},
	{Name: "insights", Group: "Create and maintain", Summary: "Capture and resolve knowledge-maintenance insights.", Subcommands: commandNames("create", "list", "run", "dismiss", "verify", "observe"), Run: runInsights},
	{Name: "jobs", Group: "Create and maintain", Summary: "Run repeatable isolated maintenance jobs from Markdown specs.", Subcommands: commandNames("new", "list", "status", "runs", "start", "stop", "kill", "validate", "run", "daemon"), Run: runJobs},

	{Name: "get", Group: "Use and publish", Summary: "Read an exact Markdown file or bundle entrypoint.", Run: runGet},
	{Name: "search", Group: "Use and publish", Summary: "Build source-grounded context from one or more knowledge bases.", Run: runSearch},
	{Name: "list", Group: "Use and publish", Summary: "Inspect knowledge-base structure.", Run: runList},
	{Name: "view", Group: "Use and publish", Summary: "Browse knowledge locally.", Run: runView},
	{Name: "mcp", Group: "Use and publish", Summary: "Connect an MCP client to read-only knowledge tools.", Run: runMCP},
	{Name: "export", Group: "Use and publish", Summary: "Export HTML, JSON, graph, or portable tar views.", Subcommands: commandNames("html", "json", "tar", "graph"), Run: runExport},

	{Name: "runtime", Group: "Run as a service", Summary: "Build, serve, and maintain an isolated knowledge runtime.", Subcommands: commandNames("plan", "build", "serve", "worker"), Run: runRuntime},
	{Name: "deploy", Group: "Run as a service", Summary: "Provision that runtime on a supported provider.", Subcommands: commandNames("railway"), Run: runDeploy},

	{Name: "validate", Group: "Validate and connect", Summary: "Validate a bundle against an OKF spec.", Run: runValidate},
	{Name: "connect", Group: "Validate and connect", Summary: "Connect a local or remote knowledge base.", Run: func(args []string) int {
		return runConnect(args, "openknowledge connect")
	}},
	{Name: "disconnect", Group: "Validate and connect", Summary: "Remove a knowledge-base connection.", Run: func(args []string) int {
		return runDisconnect(args, "openknowledge disconnect")
	}},
	{Name: "registry", Group: "Validate and connect", Summary: "Refresh, inspect, and resolve connected knowledge bases.", Subcommands: commandNames("refresh", "list", "status", "where"), Run: runRegistry},

	{Name: "scaffold", Group: "Advanced and portable tools", Summary: "Create a deterministic local OKF knowledge base.", Run: runScaffold},
	{Name: "prompt", Group: "Advanced and portable tools", Summary: "Print or install portable agent instructions.", Subcommands: commandNames("setup", "from", "rules", "review"), Run: runPrompt},
	{Name: "ast", Group: "Advanced and portable tools", Summary: "Print parsed OKF AST JSON.", Run: runAST},
	{Name: "spec", Group: "Advanced and portable tools", Summary: "Print an embedded OKF spec.", Run: runSpec},
	{Name: "version", Group: "Advanced and portable tools", Summary: "Print the CLI version.", Run: runVersion},
}

var rootCommandsByName = func() map[string]rootCommand {
	commands := make(map[string]rootCommand, len(rootCommandCatalog))
	for _, command := range rootCommandCatalog {
		if command.Name == "" || command.Run == nil {
			panic("invalid root command catalog entry")
		}
		if _, exists := commands[command.Name]; exists {
			panic("duplicate root command catalog entry: " + command.Name)
		}
		commands[command.Name] = command
	}
	return commands
}()

func commandNames(names ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		result[name] = struct{}{}
	}
	return result
}

func cliErrorCommand(args []string) string {
	if len(args) == 0 {
		return "openknowledge"
	}
	command, ok := rootCommandsByName[args[0]]
	if !ok {
		return "openknowledge"
	}
	if len(args) > 1 {
		if _, ok := command.Subcommands[args[1]]; ok {
			return command.Name + " " + args[1]
		}
	}
	return command.Name
}

func dispatchCLI(args []string) int {
	if len(args) < 1 {
		usage()
		return 2
	}
	if isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, helpText())
		return 0
	}
	command, ok := rootCommandsByName[args[0]]
	if !ok {
		fmt.Fprintf(stderrOutput(), "unknown command: %s\n\n", args[0])
		usage()
		return 2
	}
	return command.Run(args[1:])
}

func helpText() string {
	var output strings.Builder
	output.WriteString(`openknowledge builds, uses, and runs self-maintaining OKF knowledge bases.

Usage:
  openknowledge --help
  openknowledge --error-format json <command> [args...]
  openknowledge <command> --help

`)
	for _, group := range commandGroups {
		output.WriteString(group.Name)
		output.WriteString(":\n")
		for _, command := range rootCommandCatalog {
			if command.Group != group.Name {
				continue
			}
			fmt.Fprintf(&output, "  %-13s%s\n", command.Name, command.Summary)
		}
		output.WriteByte('\n')
	}
	output.WriteString(`Flags:
  -h, --help                Show this help.
  --error-format text|json  Format command failures on stderr (default text).

Start with a workflow above, then run openknowledge <command> --help.

Common flows:
  openknowledge setup Wiki --from .
  openknowledge insights create "Document the deployment rollback workflow"
  openknowledge validate Wiki
  openknowledge search Wiki "deployment model"
  openknowledge view Wiki
  openknowledge export html --out ./site Wiki
  openknowledge deploy railway Wiki --dry-run
`)
	return output.String()
}
