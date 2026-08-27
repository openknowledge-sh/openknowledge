package main

import (
	"fmt"
	"os"
)

func runAutomation(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, automationHelpText())
		return 0
	}
	switch args[0] {
	case "jobs":
		return runJobs(args[1:])
	case "insights":
		return runInsights(args[1:])
	case "runtime":
		return runRuntime(args[1:])
	case "deploy":
		return runDeploy(args[1:])
	case "github":
		return runAutomationGitHub(args[1:])
	default:
		fmt.Fprintf(stderrOutput(), "unknown automation command: %s\n\n", args[0])
		fmt.Fprint(stderrOutput(), automationHelpText())
		return 2
	}
}

func automationHelpText() string {
	return `openknowledge automation

Run unattended knowledge maintenance and hosted runtime workflows.

Usage:
  openknowledge automation jobs <command> [args...]
  openknowledge automation insights <command> [args...]
  openknowledge automation runtime <command> [args...]
  openknowledge automation deploy <provider> [args...]
  openknowledge automation github <plan|run> [args...]

Commands:
  jobs       Run repeatable isolated maintenance jobs from Markdown specs.
  insights   Capture and resolve knowledge-maintenance insights.
  runtime    Build, serve, and maintain an isolated knowledge runtime.
  deploy     Provision a runtime on a supported provider.
  github     Plan or run the config-driven GitHub Action bridge.

Use openknowledge automation <command> --help for command details.
`
}
