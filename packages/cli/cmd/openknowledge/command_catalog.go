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
	{Name: "Work locally"},
	{Name: "Share and connect"},
	{Name: "Automate and operate"},
	{Name: "Advanced and portable tools"},
}

var rootCommandCatalog = []rootCommand{
	{Name: "setup", Group: "Start here", Summary: "Set up a knowledge base and its agent instructions.", Subcommands: commandNames("complete", "status", "repair", "observe"), Run: runSetup},
	{Name: "search", Group: "Start here", Summary: "Build source-grounded context from one or more knowledge bases.", Run: runSearch},
	{Name: "validate", Group: "Start here", Summary: "Validate a bundle against an OKF spec.", Run: runValidate},

	{Name: "agent", Group: "Work locally", Summary: "Run a local knowledge task with an agent.", Subcommands: commandNames("exec", "doctor"), Run: runAgent},
	{Name: "get", Group: "Work locally", Summary: "Read an exact Markdown file or bundle entrypoint.", Run: runGet},
	{Name: "list", Group: "Work locally", Summary: "Inspect knowledge-base structure.", Run: runList},
	{Name: "view", Group: "Work locally", Summary: "Browse knowledge locally.", Run: runView},

	{Name: "export", Group: "Share and connect", Summary: "Export HTML, JSON, graph, or portable tar views.", Subcommands: commandNames("html", "json", "tar", "graph"), Run: runExport},
	{Name: "mcp", Group: "Share and connect", Summary: "Connect an MCP client to read-only knowledge tools.", Run: runMCP},
	{Name: "connect", Group: "Share and connect", Summary: "Connect a local or remote knowledge base.", Run: func(args []string) int {
		return runConnect(args, "openknowledge connect")
	}},
	{Name: "disconnect", Group: "Share and connect", Summary: "Remove a knowledge-base connection.", Run: func(args []string) int {
		return runDisconnect(args, "openknowledge disconnect")
	}},
	{Name: "registry", Group: "Share and connect", Summary: "Refresh, inspect, and resolve connected knowledge bases.", Subcommands: commandNames("refresh", "list", "status", "where"), Run: runRegistry},

	{Name: "automation", Group: "Automate and operate", Summary: "Run jobs, insights, runtimes, and deployments.", Subcommands: commandNames("jobs", "insights", "runtime", "deploy"), Run: runAutomation},

	{Name: "scaffold", Group: "Advanced and portable tools", Summary: "Create a deterministic local OKF knowledge base.", Run: runScaffold},
	{Name: "prompt", Group: "Advanced and portable tools", Summary: "Print or install maintenance instructions.", Subcommands: commandNames("rules", "review"), Run: runPrompt},
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
	for _, alias := range legacyAutomationCommands {
		commands[alias.Name] = alias
	}
	return commands
}()

var legacyAutomationCommands = []rootCommand{
	{Name: "jobs", Subcommands: commandNames("new", "list", "status", "runs", "start", "stop", "kill", "validate", "run", "daemon"), Run: runJobs},
	{Name: "insights", Subcommands: commandNames("create", "list", "run", "dismiss", "verify", "observe"), Run: runInsights},
	{Name: "runtime", Subcommands: commandNames("plan", "build", "serve", "worker"), Run: runRuntime},
	{Name: "deploy", Subcommands: commandNames("railway"), Run: runDeploy},
}

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
	for _, alias := range legacyAutomationCommands {
		if args[0] == alias.Name {
			return cliErrorCommand(append([]string{"automation"}, args...))
		}
	}
	command, ok := rootCommandsByName[args[0]]
	if !ok {
		return "openknowledge"
	}
	if command.Name == "automation" && len(args) > 1 {
		if _, ok := command.Subcommands[args[1]]; ok {
			return "automation " + args[1]
		}
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
	output.WriteString(`Flexible knowledge bases in Markdown that your agents can create, retrieve, validate, and publish.

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

Start with setup for an interactive onboarding. Use setup --prompt when an
existing agent should run the onboarding task. Run openknowledge <command>
--help when you need another workflow.

Get started:
  openknowledge setup
`)
	return output.String()
}
