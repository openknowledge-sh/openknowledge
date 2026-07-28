package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
)

func runIntegration(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, integrationHelpText())
		return 0
	}
	switch args[0] {
	case "install":
		return runIntegrationInstall(args[1:])
	case "status":
		return runIntegrationStatus(args[1:])
	case "remove":
		return runIntegrationRemove(args[1:])
	default:
		fmt.Fprintf(stderrOutput(), "unknown integration command: %s\n", args[0])
		return 2
	}
}

func runIntegrationInstall(args []string) int {
	global := false
	observe := false
	runtime := ""
	path := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--global":
			global = true
		case arg == "--observe":
			observe = true
		case arg == "--runtime":
			value, next, err := nextFlagValue(args, index, arg)
			if err != nil {
				fmt.Fprintln(stderrOutput(), err)
				return 2
			}
			runtime = value
			index = next
		case strings.HasPrefix(arg, "--runtime="):
			runtime = strings.TrimPrefix(arg, "--runtime=")
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(stderrOutput(), "unknown integration install option: %s\n", arg)
			return 2
		case path == "":
			path = arg
		default:
			fmt.Fprintln(stderrOutput(), "integration install accepts one knowledge base path")
			return 2
		}
	}
	if strings.TrimSpace(runtime) == "" {
		fmt.Fprintln(stderrOutput(), "integration install requires --runtime codex, claude, or opencode")
		return 2
	}
	if global && path != "" {
		fmt.Fprintln(stderrOutput(), "--global cannot be combined with a knowledge base path")
		return 2
	}
	if global && observe {
		fmt.Fprintln(stderrOutput(), "--observe is project-scoped and cannot be combined with --global")
		return 2
	}
	if !global && path == "" {
		fmt.Fprintln(stderrOutput(), "integration install requires a knowledge base path or --global")
		return 2
	}

	var result integration.InstallResult
	var err error
	if global {
		result, err = integration.InstallGlobalForRuntime("", runtime)
	} else {
		result, err = integration.InstallProjectWithOptions(path, integration.InstallOptions{
			Runtime: runtime,
			Observe: observe,
		})
	}
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if global {
		fmt.Fprintf(os.Stdout, "Installed the %s discovery skill (no hooks).\n", runtime)
	} else {
		fmt.Fprintf(os.Stdout, "Integrated %s with %s in %s.\n", path, runtime, result.Root)
		if !observe {
			fmt.Fprintln(os.Stdout, "Session observation is off. Re-run with --observe to enable it.")
		}
	}
	printIntegrationFiles(result.Root, result.Files)
	return 0
}

func runIntegrationStatus(args []string) int {
	if len(args) > 1 || (len(args) == 1 && strings.HasPrefix(args[0], "-")) {
		fmt.Fprintln(stderrOutput(), "integration status accepts at most one repository path")
		return 2
	}
	start := "."
	if len(args) == 1 {
		start = args[0]
	}
	status, err := integration.Status(start)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Repository: %s\n", status.Root)
	fmt.Fprintf(os.Stdout, "Knowledge base: %s\n", status.KnowledgeBase)
	if status.Runtime == "" {
		fmt.Fprintln(os.Stdout, "Runtime: legacy integration")
	} else {
		fmt.Fprintf(os.Stdout, "Runtime: %s\n", status.Runtime)
	}
	fmt.Fprintf(os.Stdout, "Observation: %s\n", enabledLabel(status.Observe))
	for _, file := range status.Files {
		fmt.Fprintf(os.Stdout, "  %-9s %s\n", file.State, file.Path)
	}
	return 0
}

func runIntegrationRemove(args []string) int {
	if len(args) > 1 || (len(args) == 1 && strings.HasPrefix(args[0], "-")) {
		fmt.Fprintln(stderrOutput(), "integration remove accepts at most one repository path")
		return 2
	}
	start := "."
	if len(args) == 1 {
		start = args[0]
	}
	result, err := integration.Remove(start)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Removed Open Knowledge integration from %s.\n", result.Root)
	for _, file := range result.Removed {
		fmt.Fprintf(os.Stdout, "  removed   %s\n", file)
	}
	for _, file := range result.Preserved {
		fmt.Fprintf(os.Stdout, "  preserved %s\n", file)
	}
	return 0
}

// runIntegrate preserves the old agent subcommand while the canonical command
// is `openknowledge integration install`.
func runIntegrate(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, integrateHelpText())
		return 0
	}
	return runIntegration(append([]string{"install"}, args...))
}

func printIntegrationFiles(root string, files []string) {
	for _, file := range files {
		if filepath.IsAbs(file) {
			fmt.Fprintf(os.Stdout, "  %s\n", file)
		} else {
			fmt.Fprintf(os.Stdout, "  %s\n", filepath.Join(root, filepath.FromSlash(file)))
		}
	}
}

func enabledLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func integrationHelpText() string {
	return `openknowledge integration

Install and manage one agent-runtime integration.

Usage:
  openknowledge integration install <wiki> --runtime <name> [--observe]
  openknowledge integration install --global --runtime <name>
  openknowledge integration status [repository]
  openknowledge integration remove [repository]

Supported runtimes: codex, claude, opencode.

Project installation writes the selected runtime's skill only. Session
observation is disabled unless --observe is present. Status never changes
files. Remove deletes unchanged managed files and preserves modified files.
Global installation adds one discovery skill and never installs hooks.
`
}

func integrateHelpText() string {
	return `openknowledge agent integrate

Deprecated alias for "openknowledge integration install".

Usage:
  openknowledge agent integrate <wiki> --runtime <name> [--observe]
  openknowledge agent integrate --global --runtime <name>
`
}
