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
		"openknowledge registry add <name> <path>",
		"openknowledge where <name|path>",
		"openknowledge open --name <alias-name> [path]",
		"openknowledge open --host <host> --port <port> [path]",
		"openknowledge open --no-browser [path]",
		"openknowledge to html --out <folder> [path]",
		"openknowledge to json --out <file> [path]",
		"openknowledge validate --spec <version> [path]",
		"openknowledge list --spec <version> [path]",
		"openknowledge list --json [path]",
		"Commands:",
		"setup      Print an agent setup prompt.",
		"new        Scaffold a local Open Knowledge bundle.",
		"registry   Manage named knowledge base paths.",
		"where      Print the path for a named knowledge base or path.",
		"open       Start the registry or knowledge base Markdown viewer.",
		"to         Convert a bundle to another format.",
		"spec       Print an embedded OKF spec.",
		"validate   Validate a bundle against an OKF spec.",
		"list       Print a bundle tree, with optional JSON output.",
		"version    Print the CLI version.",
		"Flags:",
		"-h, --help  Show this help.",
		"Examples:",
		"openknowledge validate ./project-memory",
		"openknowledge to html --out ./site ./project-memory",
		"openknowledge to json ./project-memory",
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
				"openknowledge new --bundle-name <id> --bundle-purpose <text> [folder]",
				"Arguments:",
				"--name",
				"--bundle-entry",
			},
		},
		"registry": {
			help: registryHelpText(),
			required: []string{
				"openknowledge registry add <name> <path>",
				"Registry names are shortcuts",
				"openknowledge list personal",
			},
		},
		"where": {
			help: whereHelpText(),
			required: []string{
				"openknowledge where <name|path>",
				"Print the absolute path",
			},
		},
		"open": {
			help: openHelpText(),
			required: []string{
				"openknowledge open --host <host> --port <port> [path]",
				"openknowledge open --name <alias-name> [path]",
				"openknowledge open --no-browser [path]",
				"Open Knowledge Registry workspace selector",
				"openknowledge open personal",
				"--host",
				"--port",
				"--name",
				"--no-browser",
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
		"to": {
			help: toHelpText(),
			required: []string{
				"openknowledge to html --out <folder> [path]",
				"openknowledge to html --plain --out <folder> [path]",
				"openknowledge to json --out <file> [path]",
				"Targets:",
			},
		},
		"to html": {
			help: toHTMLHelpText(),
			required: []string{
				"openknowledge to html --plain --out <folder> [path]",
				"openknowledge to html --spec <version> --out <folder> [path]",
				"Output folder for generated HTML files. Required.",
				"Generate plain semantic HTML without CSS, JavaScript, or viewer chrome.",
				"Default viewer exports read [html.theme] from openknowledge.toml",
				"Built-in variables are defined in viewer_theme.css",
			},
		},
		"to json": {
			help: toJSONHelpText(),
			required: []string{
				"openknowledge to json --out <file> [path]",
				"Output file. Defaults to stdout.",
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

func TestParseBundleEntryFlags(t *testing.T) {
	entries, err := parseBundleEntryFlags([]string{
		"default=agents/checker.md",
		"review=agents/review.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name != "default" || entries[0].Path != "agents/checker.md" || entries[1].Name != "review" || entries[1].Path != "agents/review.md" {
		t.Fatalf("unexpected entries: %#v", entries)
	}

	if _, err := parseBundleEntryFlags([]string{"missing-separator"}); err == nil {
		t.Fatal("expected missing separator to fail")
	}
}

func TestParseToOptionsAllowsPathBeforeFlags(t *testing.T) {
	options, err := parseToOptions([]string{"./project-memory", "--out", "./site", "--spec", "0.1", "--plain"})
	if err != nil {
		t.Fatal(err)
	}
	if options.path != "./project-memory" || options.out != "./site" || options.spec != "0.1" || !options.plain {
		t.Fatalf("unexpected options: %#v", options)
	}
}
