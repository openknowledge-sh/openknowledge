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
		"openknowledge <command> --help",
		"openknowledge setup",
		"openknowledge new --name <name> [folder]",
		"openknowledge open --host <host> --port <port> [path]",
		"openknowledge open --head-file <file> [path]",
		"openknowledge open --script-src <src> [path]",
		"openknowledge validate --spec <version> [path]",
		"openknowledge list --spec <version> [path]",
		"openknowledge list --json [path]",
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

func TestCommandHelpTextIncludesCommandSpecificDetails(t *testing.T) {
	tests := map[string]struct {
		help     string
		required []string
	}{
		"setup": {
			help: setupHelpText(),
			required: []string{
				"openknowledge setup --help",
				"Print an agent setup prompt",
				"create a bundle with",
			},
		},
		"new": {
			help: newHelpText(),
			required: []string{
				"openknowledge new --name <name> [folder]",
				"Arguments:",
				"--name",
			},
		},
		"open": {
			help: openHelpText(),
			required: []string{
				"openknowledge open --host <host> --port <port> [path]",
				"openknowledge open --head-file <file> [path]",
				"--host",
				"--port",
				"--head-html",
				"--script-src",
			},
		},
		"spec": {
			help: specHelpText(),
			required: []string{
				"openknowledge spec latest|<version>",
				"Versions:",
				"latest, 0.1",
			},
		},
		"validate": {
			help: validateHelpText(),
			required: []string{
				"openknowledge validate --quiet [path]",
				"Exit codes:",
				"Validation found errors.",
			},
		},
		"list": {
			help: listHelpText(),
			required: []string{
				"openknowledge list --json [path]",
				"Print machine-readable inventory JSON.",
			},
		},
		"version": {
			help: versionHelpText(),
			required: []string{
				"openknowledge version --help",
				"Print the CLI version.",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			for _, expected := range test.required {
				if !strings.Contains(test.help, expected) {
					t.Fatalf("expected %s help to include %q:\n%s", name, expected, test.help)
				}
			}
		})
	}
}

func TestHasHelpFlagRecognizesCommonHelpForms(t *testing.T) {
	if !hasHelpFlag([]string{"--help"}) {
		t.Fatal("expected --help to be recognized")
	}
	if !hasHelpFlag([]string{"-h"}) {
		t.Fatal("expected -h to be recognized")
	}
	if !hasHelpFlag([]string{"-help"}) {
		t.Fatal("expected -help to be recognized")
	}
	if !hasHelpFlag([]string{"--spec", "0.1", "--help"}) {
		t.Fatal("expected help flag to be recognized after other flags")
	}
	if hasHelpFlag([]string{"./project-memory"}) {
		t.Fatal("did not expect normal arguments to be recognized as help")
	}
}
