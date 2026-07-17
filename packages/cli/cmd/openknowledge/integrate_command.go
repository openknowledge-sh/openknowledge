package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
)

func runIntegrate(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, integrateHelpText())
		return 0
	}
	global := false
	path := ""
	for _, arg := range args {
		switch {
		case arg == "--global":
			global = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "unknown integrate option: %s\n", arg)
			return 2
		case path == "":
			path = arg
		default:
			fmt.Fprintln(os.Stderr, "integrate accepts one knowledge base path")
			return 2
		}
	}
	if global && path != "" {
		fmt.Fprintln(os.Stderr, "--global cannot be combined with a knowledge base path")
		return 2
	}
	if !global && path == "" {
		fmt.Fprintln(os.Stderr, "integrate requires a knowledge base path or --global")
		return 2
	}
	var result integration.InstallResult
	var err error
	if global {
		result, err = integration.InstallGlobal("")
	} else {
		result, err = integration.InstallProject(path)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if global {
		fmt.Fprintln(os.Stdout, "Installed global Open Knowledge discovery skill (no hooks).")
	} else {
		fmt.Fprintf(os.Stdout, "Integrated %s with repository %s.\n", path, result.Root)
	}
	for _, file := range result.Files {
		if filepath.IsAbs(file) {
			fmt.Fprintf(os.Stdout, "  %s\n", file)
		} else {
			fmt.Fprintf(os.Stdout, "  %s\n", filepath.Join(result.Root, filepath.FromSlash(file)))
		}
	}
	return 0
}

func integrateHelpText() string {
	return `openknowledge agent integrate

Connect agent harnesses to an Open Knowledge knowledge base.

Usage:
  openknowledge agent integrate <wiki>
  openknowledge agent integrate --global

Project integration writes .openknowledge/integration.toml, installs a
project-scoped skill, and installs observation hooks for Codex, Claude Code,
and OpenCode. Global integration installs only discovery skills and never
installs hooks or observes sessions.
`
}
