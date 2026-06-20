package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestHelpTextIncludesCommandsFlagsAndExamples(t *testing.T) {
	help := helpText()
	required := []string{
		"Usage:",
		"openknowledge --help",
		"openknowledge <command> --help",
		"openknowledge setup",
		"openknowledge new --name <name> [folder]",
		"openknowledge connect <path>",
		"openknowledge connect <path> --as <key>",
		"openknowledge disconnect <key|path>",
		"openknowledge use <name|path> [entry]",
		"openknowledge use <name|path> --info",
		"openknowledge registry connect <path>",
		"openknowledge registry connect <path> --as <key>",
		"openknowledge registry disconnect <key|path>",
		"openknowledge registry where <name|path>",
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
		"connect    Connect a local knowledge bundle.",
		"disconnect Remove a knowledge bundle connection.",
		"use        Print an agent entrypoint from a bundle.",
		"registry   Manage local knowledge bundle connections.",
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

	forbidden := []string{
		"openknowledge registry add <name> <path>",
		"openknowledge where <name|path>",
		"where      Print the path for a named knowledge base or path.",
	}
	for _, unexpected := range forbidden {
		if strings.Contains(help, unexpected) {
			t.Fatalf("did not expect help text to include %q:\n%s", unexpected, help)
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
		"connect": {
			help: connectHelpText("openknowledge connect"),
			required: []string{
				"openknowledge connect <path> --as <key>",
				"--access",
				"--no-validate",
				"Remote URL sources are not supported yet",
			},
		},
		"registry connect": {
			help: connectHelpText("openknowledge registry connect"),
			required: []string{
				"openknowledge registry connect <path> --as <key>",
				"openknowledge registry connect <path> --access read|write",
				"openknowledge registry connect --help",
			},
		},
		"disconnect": {
			help: disconnectHelpText("openknowledge disconnect"),
			required: []string{
				"openknowledge disconnect <key|path> --keep-files",
				"openknowledge disconnect <key|path> --delete-files",
				"reserved for future managed remote-cache entries",
			},
		},
		"registry disconnect": {
			help: disconnectHelpText("openknowledge registry disconnect"),
			required: []string{
				"openknowledge registry disconnect <key|path> --keep-files",
				"openknowledge registry disconnect <key|path> --delete-files",
				"openknowledge registry disconnect --help",
			},
		},
		"use": {
			help: useHelpText(),
			required: []string{
				"openknowledge use <name|path> <entry> --info",
				"[bundle.entries]",
				"prints the bundle root index.md",
			},
		},
		"registry": {
			help: registryHelpText(),
			required: []string{
				"openknowledge registry connect <path> --as <key>",
				"openknowledge registry disconnect <key|path> --keep-files",
				"openknowledge registry where <name|path>",
				"Registry keys are shortcuts",
				"openknowledge list personal",
			},
		},
		"registry where": {
			help: registryWhereHelpText(),
			required: []string{
				"openknowledge registry where <name|path>",
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

func TestParseUseOptionsAllowsInfoAfterEntry(t *testing.T) {
	options, err := parseUseOptions([]string{"accessibility", "review", "--info"})
	if err != nil {
		t.Fatal(err)
	}
	if options.target != "accessibility" || options.entry != "review" || !options.info {
		t.Fatalf("unexpected options: %#v", options)
	}
}

func TestSelectUseEntrypointUsesDefaultNamedAndRootFallback(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", `---
okf_version: "0.1"
---

# Bundle
`)
	writeMainTestFile(t, root, "openknowledge.toml", `[bundle.entries]
default = "agents/default.md"
review = "agents/review.md"
`)
	writeMainTestFile(t, root, "agents/default.md", "---\ntype: Agent Entrypoint\n---\n\n# Default\n")
	writeMainTestFile(t, root, "agents/review.md", "---\ntype: Agent Entrypoint\n---\n\n# Review\n")

	info, err := okf.ReadBundleInfo(root)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := selectUseEntrypoint(root, info, "")
	if err != nil {
		t.Fatal(err)
	}
	if selection.name != "default" || selection.rel != "agents/default.md" {
		t.Fatalf("unexpected default selection: %#v", selection)
	}
	selection, err = selectUseEntrypoint(root, info, "review")
	if err != nil {
		t.Fatal(err)
	}
	if selection.name != "review" || selection.rel != "agents/review.md" {
		t.Fatalf("unexpected review selection: %#v", selection)
	}
	if _, err := selectUseEntrypoint(root, info, "missing"); err == nil {
		t.Fatal("expected missing named entrypoint to fail")
	}

	fallbackRoot := t.TempDir()
	writeMainTestFile(t, fallbackRoot, "index.md", "# Root\n")
	fallbackInfo, err := okf.ReadBundleInfo(fallbackRoot)
	if err != nil {
		t.Fatal(err)
	}
	selection, err = selectUseEntrypoint(fallbackRoot, fallbackInfo, "")
	if err != nil {
		t.Fatal(err)
	}
	if selection.name != "index" || selection.rel != "index.md" {
		t.Fatalf("unexpected root fallback selection: %#v", selection)
	}
}

func writeMainTestFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
