package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/telemetry"
)

func runTelemetry(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, telemetryHelpText())
		return 0
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			fmt.Fprintln(stderrOutput(), "telemetry status accepts no arguments")
			return 2
		}
		config, exists, err := telemetry.Status()
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		state := "enabled"
		if !config.Enabled {
			state = "disabled"
		}
		fmt.Fprintf(os.Stdout, "Telemetry:      %s\n", state)
		if exists {
			fmt.Fprintln(os.Stdout, "Configuration: saved")
		} else {
			fmt.Fprintln(os.Stdout, "Configuration: default")
		}
		fmt.Fprintln(os.Stdout, "Data:           anonymous usage and sanitized errors")
		return 0
	case "enable", "disable":
		if len(args) != 1 {
			fmt.Fprintf(stderrOutput(), "telemetry %s accepts no arguments\n", args[0])
			return 2
		}
		enabled := args[0] == "enable"
		if _, err := telemetry.SetEnabled(enabled); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		if enabled {
			fmt.Fprintln(os.Stdout, "Anonymous usage and sanitized error telemetry is enabled.")
		} else {
			fmt.Fprintln(os.Stdout, "Telemetry is disabled. The random installation ID was deleted.")
		}
		return 0
	case "show-payload":
		if len(args) != 1 {
			fmt.Fprintln(stderrOutput(), "telemetry show-payload accepts no arguments")
			return 2
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(telemetry.SamplePayload()); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderrOutput(), "unknown telemetry subcommand: %s\n", args[0])
		return 2
	}
}

func telemetryHelpText() string {
	return `openknowledge telemetry

Inspect or change anonymous product telemetry.

Usage:
  openknowledge telemetry status
  openknowledge telemetry enable
  openknowledge telemetry disable
  openknowledge telemetry show-payload

Telemetry is enabled by default after a first-run disclosure. It sends only
allowlisted command, outcome, duration, version, platform, and random
installation identifiers. It does not send command arguments, paths, content,
repository or user identity, output, hostnames, IP addresses, or raw user
agents. Use --no-telemetry before a command to disable telemetry persistently.
`
}
