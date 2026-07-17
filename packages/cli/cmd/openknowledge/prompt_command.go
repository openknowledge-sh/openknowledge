package main

import (
	"fmt"
	"os"
)

func runPrompt(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, promptHelpText())
		return 0
	}
	switch args[0] {
	case "setup":
		return runPromptSetup(args[1:])
	case "from":
		return runPromptFrom(args[1:])
	case "rules":
		return runRules(args[1:])
	case "review":
		return runReview(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown prompt subcommand: %s\n", args[0])
		return 2
	}
}

func promptHelpText() string {
	return `openknowledge prompt

Print or install portable agent instructions without starting a model runtime.

Usage:
  openknowledge prompt setup [--rules <rules>]
  openknowledge prompt from <source> --out <folder>
  openknowledge prompt rules [<rules>] [--path <wiki>]
  openknowledge prompt rules apply [<rules>] [--path <wiki>]
  openknowledge prompt review rules [wiki]

Use openknowledge setup for the managed create, validate, and integrate
workflow. The prompt namespace is the advanced portable surface for users who
want to pass instructions to another agent host themselves.
`
}
