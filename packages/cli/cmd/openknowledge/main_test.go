package main

import (
	"strings"
	"testing"
)

func TestHelpTextIncludesCommandsFlagsAndExamples(t *testing.T) {
	help := helpText()
	required := []string{
		"Usage:",
		"openknowledge --help",
		"openknowledge setup",
		"openknowledge new --name <name> [folder]",
		"openknowledge open --host <host> --port <port> [path]",
		"openknowledge validate --spec <version> [path]",
		"openknowledge list --spec <version> [path]",
		"Commands:",
		"setup      Print an agent setup prompt.",
		"new        Scaffold a local Open Knowledge bundle.",
		"open       Start a local Markdown viewer.",
		"spec       Print an embedded OKF spec.",
		"validate   Validate a bundle against an OKF spec.",
		"list       Print a bundle tree, with optional JSON output.",
		"version    Print the CLI version.",
		"Flags:",
		"-h, --help  Show this help.",
		"Examples:",
		"openknowledge validate ./project-memory",
	}

	for _, expected := range required {
		if !strings.Contains(help, expected) {
			t.Fatalf("expected help text to include %q:\n%s", expected, help)
		}
	}
}
