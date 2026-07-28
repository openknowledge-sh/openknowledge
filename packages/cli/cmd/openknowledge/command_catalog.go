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
	{Name: "Start here"},
	{Name: "Maintain and automate"},
	{Name: "Browse and publish"},
	{Name: "Connect and operate"},
	{Name: "Advanced and portable tools"},
}

var rootCommandCatalog = []rootCommand{
	{Name: "setup", Group: "Start here", Summary: "Print setup instructions, or run them with --agent.", Run: runSetup},
	{Name: "search", Group: "Start here", Summary: "Build source-grounded context from one or more knowledge bases.", Run: runSearch},
	{Name: "validate", Group: "Start here", Summary: "Validate a bundle against an OKF spec.", Run: runValidate},

	{Name: "agent", Group: "Maintain and automate", Summary: "Run, integrate, and review knowledge with an agent.", Subcommands: commandNames("exec", "integrate", "doctor"), Run: runAgent},
	{Name: "integration", Group: "Maintain and automate", Summary: "Install and manage one local agent-runtime integration.", Subcommands: commandNames("install", "status", "remove"), Run: runIntegration},
	{Name: "insights", Group: "Maintain and automate", Summary: "Capture and resolve knowledge-maintenance insights.", Subcommands: commandNames("create", "list", "run", "dismiss", "verify", "observe"), Run: runInsights},
	{Name: "jobs", Group: "Maintain and automate", Summary: "Run repeatable isolated maintenance jobs from Markdown specs.", Subcommands: commandNames("new", "list", "status", "runs", "start", "stop", "kill", "validate", "run", "daemon"), Run: runJobs},

	{Name: "get", Group: "Browse and publish", Summary: "Read an exact Markdown file or bundle entrypoint.", Run: runGet},
	{Name: "list", Group: "Browse and publish", Summary: "Inspect knowledge-base structure.", Run: runList},
	{Name: "view", Group: "Browse and publish", Summary: "Browse knowledge locally.", Run: runView},
	{Name: "mcp", Group: "Browse and publish", Summary: "Connect an MCP client to read-only knowledge tools.", Run: runMCP},
	{Name: "export", Group: "Browse and publish", Summary: "Export HTML, JSON, graph, or portable tar views.", Subcommands: commandNames("html", "json", "tar", "graph"), Run: runExport},

	{Name: "connect", Group: "Connect and operate", Summary: "Connect a local or remote knowledge base.", Run: func(args []string) int {
		return runConnect(args, "openknowledge connect")
	}},
	{Name: "disconnect", Group: "Connect and operate", Summary: "Remove a knowledge-base connection.", Run: func(args []string) int {
		return runDisconnect(args, "openknowledge disconnect")
	}},
	{Name: "registry", Group: "Connect and operate", Summary: "Refresh, inspect, and resolve connected knowledge bases.", Subcommands: commandNames("refresh", "list", "status", "where"), Run: runRegistry},
	{Name: "runtime", Group: "Connect and operate", Summary: "Build, serve, and maintain an isolated knowledge runtime.", Subcommands: commandNames("plan", "build", "serve", "worker"), Run: runRuntime},
	{Name: "deploy", Group: "Connect and operate", Summary: "Provision that runtime on a supported provider.", Subcommands: commandNames("railway"), Run: runDeploy},

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

Start with setup. Add --agent to run the printed instructions. Run
openknowledge <command> --help when you need another workflow.

Get started:
  openknowledge setup
`)
	return output.String()
}
