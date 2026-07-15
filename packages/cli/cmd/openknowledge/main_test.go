package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestHelpTextIncludesCommandsFlagsAndExamples(t *testing.T) {
	help := helpText()
	required := []string{
		"Usage:",
		"openknowledge --help",
		"openknowledge <command> --help",
		"openknowledge setup",
		"openknowledge setup --rules <rules>",
		"openknowledge from <source> --out <folder>",
		"openknowledge from <source> --out <folder> --type custom --about <goal>",
		"openknowledge rules",
		"openknowledge rules <rules> --path <path>",
		"openknowledge rules apply <rules> --path <path>",
		"openknowledge rules --list",
		"openknowledge review rules [path]",
		"openknowledge review rules --rules <rules> --path <path>",
		"openknowledge agents new",
		"openknowledge agents new <template> --out <file>",
		"openknowledge agents list [path]",
		"openknowledge agents validate <job-or-dir>",
		"openknowledge agents run <job.md> --dry-run",
		"openknowledge agents daemon [jobs-dir] --once",
		"openknowledge new --name <name> [folder]",
		"openknowledge new --no-agents --no-setup [folder]",
		"openknowledge connect <source>",
		"openknowledge connect <source> --as <key>",
		"openknowledge disconnect <key|path>",
		"openknowledge get <name|path> [entry-or-file]",
		"openknowledge get <name|path> --info",
		"openknowledge search <name|path> <query>",
		"openknowledge search <name|path> <query> --budget <tokens>",
		"openknowledge search <name|path> <query> --format json",
		"openknowledge search <name|path> <query> --matches",
		"openknowledge search <name|path> <query> --no-expand",
		"openknowledge registry connect <source>",
		"openknowledge registry connect <source> --as <key>",
		"openknowledge registry disconnect <key|path>",
		"openknowledge registry where <name|path>",
		"openknowledge view --name <alias-name> [path]",
		"openknowledge view --host <host> --port <port> [path]",
		"openknowledge view --head-file <file> [path]",
		"openknowledge view --script-src <src> [path]",
		"openknowledge view --no-browser [path]",
		"openknowledge to html --out <folder> [path]",
		"openknowledge to html --head-file <file> --out <folder> [path]",
		"openknowledge to html --script-src <src> --out <folder> [path]",
		"openknowledge to json --out <file> [path]",
		"openknowledge to tar --out <file> [path]",
		"openknowledge to graph --out <file> [path]",
		"openknowledge to graph --type search [path]",
		"openknowledge validate --spec <version> [key-or-path]",
		"openknowledge validate --format json [key-or-path]",
		"openknowledge validate --rule <rule=off|warn|error> [key-or-path]",
		"openknowledge list --spec <version> [key-or-path]",
		"openknowledge list --depth <n> [key-or-path]",
		"openknowledge list --json [key-or-path]",
		"Commands:",
		"setup      Print an agent setup prompt.",
		"from       Print an agent source-to-wiki generation prompt.",
		"rules      Print agent maintenance rules.",
		"review     Print advisory AI review prompts.",
		"agents     Experimental: run scheduled local agent jobs from Markdown specs.",
		"new        Scaffold a local Open Knowledge bundle.",
		"connect    Connect a local or remote knowledge bundle.",
		"disconnect Remove a knowledge bundle connection.",
		"get        Print a Markdown file or bundle entrypoint.",
		"search     Build source-grounded Markdown context from a bundle.",
		"registry   Manage knowledge bundle connections.",
		"view       Start the registry or knowledge base Markdown viewer.",
		"to         Convert a bundle to another format.",
		"spec       Print an embedded OKF spec.",
		"validate   Validate a bundle against an OKF spec.",
		"list       Print a bundle tree, with optional depth and JSON output.",
		"version    Print the CLI version.",
		"Flags:",
		"-h, --help  Show this help.",
		"Examples:",
		"openknowledge from https://github.com/openknowledge-sh/openknowledge --out Wiki --type understanding",
		"openknowledge from https://openknowledge.sh/wiki/ --out Wiki --type custom --about \"Create an onboarding wiki\"",
		"openknowledge rules docs,changelog --path Wiki",
		"openknowledge rules apply docs,changelog --path Wiki --file AGENTS.md",
		"openknowledge review rules --rules docs,changelog --path Wiki",
		"openknowledge agents new docs-audit --out .openknowledge/agents/jobs/docs-audit.md",
		"openknowledge agents validate .openknowledge/agents/jobs",
		"openknowledge agents run .openknowledge/agents/jobs/docs.md --dry-run",
		"openknowledge setup --rules docs,changelog",
		"openknowledge new --no-agents --no-setup ./source-wiki",
		"openknowledge validate ./project-memory",
		"openknowledge search accessibility \"validation workflow\"",
		"openknowledge list --depth 2 ./project-memory",
		"openknowledge to html --out ./site ./project-memory",
		"openknowledge to json ./project-memory",
		"openknowledge to graph ./project-memory",
	}

	for _, expected := range required {
		if !strings.Contains(help, expected) {
			t.Fatalf("expected help text to include %q:\n%s", expected, help)
		}
	}

	forbidden := []string{
		"openknowledge registry add <name> <path>",
		"openknowledge where <name|path>",
		"openknowledge use <name|path>",
		"openknowledge open [path]",
		"use        Print an agent entrypoint from a bundle.",
		"open       Start the registry or knowledge base Markdown viewer.",
		"openknowledge context",
		"openknowledge search <name|path> <query> --expand graph",
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
				"openknowledge setup --rules <rules>",
				"Print an agent setup prompt",
				"create a bundle with",
				"--rules",
				"project, docs, decisions, changelog, research, bugs, schemas, summary, agents",
			},
		},
		"from": {
			help: fromHelpText(),
			required: []string{
				"openknowledge from <source> --out <folder>",
				"openknowledge from <source> --out <folder> --type custom --about <goal>",
				"Print an agent task prompt",
				"The command does not fetch, crawl, call an LLM, or write the wiki itself",
				"--type",
				"understanding or custom",
				"--about",
				"--depth",
				"Copy the printed prompt",
				"avoid shell command substitution or piping",
			},
		},
		"rules": {
			help: rulesHelpText(),
			required: []string{
				"openknowledge rules <rules> --path <path>",
				"openknowledge rules apply <rules> --path <path>",
				"openknowledge rules --target generic|codex|claude|cursor",
				"Print maintenance instructions for AI agents",
				"The command does not edit files",
				"prints non-blocking warnings after the rendered",
				"with pipes or",
				"--list",
				"--path",
			},
		},
		"rules apply": {
			help: rulesApplyHelpText(),
			required: []string{
				"openknowledge rules apply <rules> --path <path> --file <file>",
				"managed block",
				"--dry-run",
				"--yes",
				"skip confirmation",
			},
		},
		"review": {
			help: reviewHelpText(),
			required: []string{
				"openknowledge review rules [path]",
				"Print advisory AI review prompts",
				"does not call a model",
				"Use openknowledge validate",
			},
		},
		"review rules": {
			help: reviewRulesHelpText(),
			required: []string{
				"openknowledge review rules --rules <rules> --path <path>",
				"advisory AI review prompt",
				"--rules",
				"--all",
				"Defaults to [rules].enabled, then project",
			},
		},
		"agents": {
			help: agentsHelpText(),
			required: []string{
				"openknowledge agents new <template> --out <file>",
				"openknowledge agents list [path]",
				"openknowledge agents validate <job-or-dir>",
				"openknowledge agents run <job.md> --dry-run",
				"openknowledge agents daemon [jobs-dir] --once",
				"Experimental command group for deterministic local agent jobs",
				"may still",
			},
		},
		"agents new": {
			help: agentsNewHelpText(),
			required: []string{
				"openknowledge agents new --reference",
				"openknowledge agents new <template> --out <file>",
				"--list",
				"--force",
				"built-in agent job templates",
			},
		},
		"agents run": {
			help: agentsRunHelpText(),
			required: []string{
				"openknowledge agents run <job.md> --at <time>",
				"--dry-run",
				"--executor",
				"deterministic run ID",
			},
		},
		"new": {
			help: newHelpText(),
			required: []string{
				"openknowledge new --name <name> [folder]",
				"openknowledge new --bundle-name <id> --bundle-purpose <text> [folder]",
				"openknowledge new --no-agents --no-setup [folder]",
				"Arguments:",
				"--name",
				"--bundle-entry",
				"--no-agents",
				"--no-setup",
			},
		},
		"connect": {
			help: connectHelpText("openknowledge connect"),
			required: []string{
				"openknowledge connect <source> --as <key>",
				"--access",
				"--no-validate",
				"manifest URL, tar archive URL, or Git URL",
			},
		},
		"registry connect": {
			help: connectHelpText("openknowledge registry connect"),
			required: []string{
				"openknowledge registry connect <source> --as <key>",
				"openknowledge registry connect <source> --access read|write",
				"openknowledge registry connect --help",
			},
		},
		"disconnect": {
			help: disconnectHelpText("openknowledge disconnect"),
			required: []string{
				"openknowledge disconnect <key|path> --keep-files",
				"openknowledge disconnect <key|path> --delete-files",
				"Delete files only for CLI-managed remote clones",
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
		"get": {
			help: getHelpText(),
			required: []string{
				"openknowledge get <name|path> <entry-or-file> --info",
				"Local Markdown file, registry key, or local bundle path.",
				"okf_bundle_entry_<name>",
				"prints the bundle root index.md",
				"Use openknowledge search",
			},
		},
		"search": {
			help: searchHelpText(),
			required: []string{
				"openknowledge search <name|path> <query>",
				"openknowledge search <name|path> <query> --budget <tokens>",
				"openknowledge search <name|path> <query> --format json",
				"openknowledge search <name|path> <query> --matches",
				"openknowledge search <name|path> <query> --no-expand",
				"Defaults to 2400",
				"--limit",
				"--spec",
				"BM25-style",
				"heading paths",
				"backlinks",
				"original Markdown",
			},
		},
		"registry": {
			help: registryHelpText(),
			required: []string{
				"openknowledge registry connect <source> --as <key>",
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
		"view": {
			help: viewHelpText(),
			required: []string{
				"openknowledge view --host <host> --port <port> [path]",
				"openknowledge view --head-file <file> [path]",
				"openknowledge view --name <alias-name> [path]",
				"openknowledge view --no-browser [path]",
				"Open Knowledge Registry workspace selector",
				"openknowledge view personal",
				"--host",
				"--port",
				"--head-html",
				"--name",
				"--no-browser",
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
		"to": {
			help: toHelpText(),
			required: []string{
				"openknowledge to html --out <folder> [path]",
				"openknowledge to html --plain --out <folder> [path]",
				"openknowledge to html --head-file <file> --out <folder> [path]",
				"openknowledge to html --script-src <src> --out <folder> [path]",
				"openknowledge to json --out <file> [path]",
				"openknowledge to tar --out <file> [path]",
				"openknowledge to graph --out <file> [path]",
				"openknowledge to graph --type search [path]",
				"Targets:",
			},
		},
		"to html": {
			help: toHTMLHelpText(),
			required: []string{
				"openknowledge to html --plain --out <folder> [path]",
				"openknowledge to html --spec <version> --out <folder> [path]",
				"--head-file",
				"--head-html",
				"--script-src",
				"Output folder for generated HTML files. Required.",
				"Generate plain semantic HTML without CSS, JavaScript, or viewer chrome.",
				"openknowledge.json",
				"assets/openknowledge-bundle.tar.gz",
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
		"to tar": {
			help: toTarHelpText(),
			required: []string{
				"openknowledge to tar --out <file> [path]",
				"Write a portable tar.gz archive",
				"Output archive file. Required.",
			},
		},
		"to graph": {
			help: toGraphHelpText(),
			required: []string{
				"openknowledge to graph --out <file> [path]",
				"openknowledge to graph --type search [path]",
				"Write node and edge graph JSON",
				"source",
				"search",
				"AST-backed parser",
			},
		},
		"validate": {
			help: validateHelpText(),
			required: []string{
				"openknowledge validate --quiet [key-or-path]",
				"openknowledge validate --format json --out <file> [key-or-path]",
				"--rule",
				"[validation.rules]",
				"Exit codes:",
				"Validation found errors after configured severity overrides.",
			},
		},
		"list": {
			help: listHelpText(),
			required: []string{
				"openknowledge list --depth <n> [key-or-path]",
				"openknowledge list --json [key-or-path]",
				"Maximum tree depth.",
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

func TestRulesCommandPrintsSelectedRules(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "Wiki")
	writeMainTestFile(t, wiki, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Wiki\n")

	output, stderr, code := captureMainOutput(t, func() int {
		return runRules([]string{"docs,changelog", "--path", wiki, "--target", "codex"})
	})
	if code != 0 {
		t.Fatalf("expected rules command to succeed, got exit code %d\n%s", code, output)
	}
	required := []string{
		"Open Knowledge wiki at `" + wiki + "`",
		"repository `AGENTS.md` file for Codex",
		"- docs: Keep docs in sync",
		"Docs rules:",
		"Changelog rules:",
		"openknowledge validate \"" + wiki + "\"",
	}
	for _, expected := range required {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected rules output to include %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "Project rules:") {
		t.Fatalf("did not expect default project rules when explicit rules were selected:\n%s", output)
	}
	if stderr != "" {
		t.Fatalf("did not expect warnings for valid wiki, got:\n%s", stderr)
	}
}

func TestRulesCommandListsRules(t *testing.T) {
	output, code := captureMainStdout(t, func() int {
		return runRules([]string{"--list"})
	})
	if code != 0 {
		t.Fatalf("expected rules --list to succeed, got exit code %d\n%s", code, output)
	}
	for _, expected := range []string{
		"openknowledge rules prints maintenance instructions",
		"does not edit files",
		"Available rules:",
		"project",
		"docs",
		"changelog",
		"agents",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected rules list to include %q:\n%s", expected, output)
		}
	}
}

func TestRulesCommandListsAndPrintsCustomRules(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "Wiki")
	writeMainTestFile(t, wiki, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Wiki\n")
	writeMainTestFile(t, wiki, "rules/security.md", `---
type: Rule
title: Security
description: Keep security-sensitive changes documented.
rule_id: security
---

# Security

## Instructions

- When auth or permissions change, update security notes.
`)

	list, _, code := captureMainOutput(t, func() int {
		return runRules([]string{"--list", "--path", wiki})
	})
	if code != 0 {
		t.Fatalf("expected rules --list with custom rules to succeed, got %d\n%s", code, list)
	}
	if !strings.Contains(list, "security") || !strings.Contains(list, "Keep security-sensitive changes documented.") {
		t.Fatalf("expected custom rule in list:\n%s", list)
	}

	output, stderr, code := captureMainOutput(t, func() int {
		return runRules([]string{"security", "--path", wiki})
	})
	if code != 0 {
		t.Fatalf("expected custom rules command to succeed, got %d\nstdout:\n%s\nstderr:\n%s", code, output, stderr)
	}
	for _, expected := range []string{
		"- security: Keep security-sensitive changes documented.",
		"Security rules:",
		"When auth or permissions change, update security notes.",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected custom rules output to include %q:\n%s", expected, output)
		}
	}
}

func TestRulesCommandUsesConfiguredEnabledRules(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "Wiki")
	writeMainTestFile(t, wiki, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Wiki\n")
	writeMainTestFile(t, wiki, "openknowledge.toml", "[rules]\nenabled = [\"docs\", \"changelog\"]\n")

	output, stderr, code := captureMainOutput(t, func() int {
		return runRules([]string{"--path", wiki})
	})
	if code != 0 {
		t.Fatalf("expected configured rules command to succeed, got %d\nstdout:\n%s\nstderr:\n%s", code, output, stderr)
	}
	for _, expected := range []string{
		"Docs rules:",
		"Changelog rules:",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected configured rules output to include %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "Project rules:") {
		t.Fatalf("did not expect project default when rules.enabled is configured:\n%s", output)
	}
}

func TestReviewRulesCommandPrintsPrompt(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "Wiki")
	writeMainTestFile(t, wiki, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Wiki\n")
	writeMainTestFile(t, wiki, "rules/security.md", `---
type: Rule
title: Security
description: Keep security-sensitive changes documented.
rule_id: security
rule_review_prompt: Check auth and permission changes against security notes.
rule_review_evidence: [git diff, Wiki/security/]
---

# Security

## Instructions

- When auth or permissions change, update security notes.
`)

	output, stderr, code := captureMainOutput(t, func() int {
		return runReview([]string{"rules", "--rules", "security", "--path", wiki})
	})
	if code != 0 {
		t.Fatalf("expected review rules to succeed, got %d\nstdout:\n%s\nstderr:\n%s", code, output, stderr)
	}
	for _, expected := range []string{
		"Open Knowledge Rule Review",
		"advisory AI review",
		"security: Keep security-sensitive changes documented.",
		"Check auth and permission changes against security notes.",
		"git diff",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected review output to include %q:\n%s", expected, output)
		}
	}
}

func TestReviewRulesHelpUsesSubcommandHelp(t *testing.T) {
	output, code := captureMainStdout(t, func() int {
		return runReview([]string{"rules", "--help"})
	})
	if code != 0 {
		t.Fatalf("expected review rules help to succeed, got %d", code)
	}
	if !strings.Contains(output, "openknowledge review rules --rules <rules> --path <path>") {
		t.Fatalf("expected review rules subcommand help:\n%s", output)
	}
}

func TestRulesCommandRejectsUnknownRule(t *testing.T) {
	_, code := captureMainStdout(t, func() int {
		return runRules([]string{"release-changelog"})
	})
	if code != 2 {
		t.Fatalf("expected unknown rule to exit 2, got %d", code)
	}
}

func TestRulesCommandRejectsRemovedModeFlag(t *testing.T) {
	_, code := captureMainStdout(t, func() int {
		return runRules([]string{"--mode", "docs"})
	})
	if code != 2 {
		t.Fatalf("expected removed --mode flag to exit 2, got %d", code)
	}
}

func TestRulesCommandRejectsInvalidRulesList(t *testing.T) {
	_, code := captureMainStdout(t, func() int {
		return runRules([]string{"docs,"})
	})
	if code != 2 {
		t.Fatalf("expected invalid comma-separated rules list to exit 2, got %d", code)
	}
}

func TestRulesCommandWarnsWhenWikiPathIsMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	output, stderr, code := captureMainOutput(t, func() int {
		return runRules([]string{"docs", "--path", missing})
	})
	if code != 0 {
		t.Fatalf("expected missing wiki path to warn but still print rules, got %d", code)
	}
	if !strings.Contains(output, "Open Knowledge Maintenance") {
		t.Fatalf("expected rules on stdout:\n%s", output)
	}
	if !strings.Contains(stderr, "⚠ Warning: Open Knowledge wiki path does not exist") || !strings.Contains(stderr, "Agent action: create the wiki first") {
		t.Fatalf("expected missing path warning on stderr:\n%s", stderr)
	}
}

func TestWarningTextIncludesIconAndMessage(t *testing.T) {
	message := warningText(os.Stderr, "Open Knowledge wiki path does not exist: Wiki")
	if !strings.Contains(message, "⚠ Warning:") || !strings.Contains(message, "Open Knowledge wiki path does not exist: Wiki") {
		t.Fatalf("unexpected warning text: %q", message)
	}
}

func TestWarningTextUsesYellowWhenColorIsForced(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm")
	t.Setenv("CLICOLOR_FORCE", "1")
	message := warningText(os.Stderr, "Open Knowledge wiki path does not exist: Wiki")
	if !strings.Contains(message, "\x1b[33m") || !strings.Contains(message, "\x1b[0m") {
		t.Fatalf("expected yellow ANSI warning text, got %q", message)
	}
}

func TestRulesApplyWritesManagedBlockToFile(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "Wiki")
	agentFile := filepath.Join(root, "AGENTS.md")
	writeMainTestFile(t, wiki, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Wiki\n")
	writeMainTestFile(t, root, "AGENTS.md", "# Agent Rules\n")

	output, stderr, code := captureMainOutput(t, func() int {
		return runRules([]string{"apply", "docs", "--path", wiki, "--file", agentFile, "--yes"})
	})
	if code != 0 {
		t.Fatalf("expected rules apply to succeed, got %d\nstdout:\n%s\nstderr:\n%s", code, output, stderr)
	}
	if !strings.Contains(output, "Updated "+agentFile) {
		t.Fatalf("expected update message, got:\n%s", output)
	}
	content := string(readMainTestFile(t, agentFile))
	for _, expected := range []string{
		okf.RulesBlockStart,
		"This Codex instruction block is managed by `openknowledge rules apply`.",
		"Docs rules:",
		okf.RulesBlockEnd,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected AGENTS.md to include %q:\n%s", expected, content)
		}
	}

	_, _, code = captureMainOutput(t, func() int {
		return runRules([]string{"apply", "changelog", "--path", wiki, "--file", agentFile, "--yes"})
	})
	if code != 0 {
		t.Fatalf("expected second rules apply to succeed, got %d", code)
	}
	content = string(readMainTestFile(t, agentFile))
	if strings.Count(content, okf.RulesBlockStart) != 1 || strings.Contains(content, "Docs rules:") || !strings.Contains(content, "Changelog rules:") {
		t.Fatalf("expected managed block replacement:\n%s", content)
	}
}

func TestRulesApplyDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "Wiki")
	agentFile := filepath.Join(root, "AGENTS.md")
	writeMainTestFile(t, wiki, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Wiki\n")
	writeMainTestFile(t, root, "AGENTS.md", "# Agent Rules\n")

	output, _, code := captureMainOutput(t, func() int {
		return runRules([]string{"apply", "docs", "--path", wiki, "--file", agentFile, "--dry-run"})
	})
	if code != 0 {
		t.Fatalf("expected rules apply dry-run to succeed, got %d", code)
	}
	if !strings.Contains(output, "Would update "+agentFile) || !strings.Contains(output, okf.RulesBlockStart) {
		t.Fatalf("expected dry-run managed block output:\n%s", output)
	}
	content := string(readMainTestFile(t, agentFile))
	if strings.Contains(content, okf.RulesBlockStart) {
		t.Fatalf("dry-run should not write managed block:\n%s", content)
	}
}

func TestRulesApplyConfirmationMessagesDescribeWriteType(t *testing.T) {
	file := filepath.Join("repo", "AGENTS.md")
	tests := []struct {
		name     string
		existing string
		expected string
	}{
		{
			name:     "replace managed block",
			existing: okf.RulesBlockStart + "\nold\n" + okf.RulesBlockEnd + "\n",
			expected: "replace that block",
		},
		{
			name:     "append to existing file",
			existing: "# Agent Rules\n",
			expected: "append an Open Knowledge rules block",
		},
		{
			name:     "partial marker",
			existing: okf.RulesBlockStart + "\nold\n",
			expected: "partial Open Knowledge rules marker",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message := rulesApplyConfirmationMessage(file, test.existing)
			if !strings.Contains(message, file) || !strings.Contains(message, test.expected) {
				t.Fatalf("unexpected confirmation message:\n%s", message)
			}
		})
	}
}

func TestSetupCommandAcceptsRules(t *testing.T) {
	output, code := captureMainStdout(t, func() int {
		return runSetup([]string{"--rules", "docs,changelog"})
	})
	if code != 0 {
		t.Fatalf("expected setup command with rules to succeed, got exit code %d\n%s", code, output)
	}
	for _, expected := range []string{
		"Selected maintenance rules:",
		"- docs: Keep docs in sync",
		"- changelog: Track user-facing changes",
		"After the user answers:",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected setup output to include %q:\n%s", expected, output)
		}
	}
}

func TestFromCommandPrintsSourceToWikiPrompt(t *testing.T) {
	output, code := captureMainStdout(t, func() int {
		return runFrom([]string{
			"https://github.com/openknowledge-sh/openknowledge",
			"--out", "Wiki",
			"--type", "custom",
			"--about", "Help contributors understand releases",
			"--depth", "2",
		})
	})
	if code != 0 {
		t.Fatalf("expected from command to succeed, got exit code %d\n%s", code, output)
	}
	for _, expected := range []string{
		"source URL or path -> local agent task -> OKF Markdown bundle",
		"Source: `https://github.com/openknowledge-sh/openknowledge`",
		"Output wiki path: `Wiki`",
		"Wiki type: `custom`",
		"Custom goal: `Help contributors understand releases`",
		"Depth: 2",
		"openknowledge new --name \"<clear wiki name>\" --no-agents --no-setup \"Wiki\"",
		"okf_generated_from",
		"openknowledge validate \"Wiki\"",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected from output to include %q:\n%s", expected, output)
		}
	}
}

func TestNewCommandCanSkipAgentAndSetupDocs(t *testing.T) {
	target := filepath.Join(t.TempDir(), "source-wiki")

	output, code := captureMainStdout(t, func() int {
		return runNew([]string{
			"--name", "Source Wiki",
			"--no-agents",
			"--no-setup",
			target,
		})
	})
	if code != 0 {
		t.Fatalf("expected new command to succeed, got exit code %d\n%s", code, output)
	}
	for _, expected := range []string{
		"Created knowledge base",
		"+ index.md",
		"+ log.md",
		"+ SPEC.md",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected new output to include %q:\n%s", expected, output)
		}
	}
	for _, unexpected := range []string{
		"+ AGENTS.md",
		"+ SETUP.MD",
		"Agent handoff",
	} {
		if strings.Contains(output, unexpected) {
			t.Fatalf("did not expect new output to include %q:\n%s", unexpected, output)
		}
	}
	for _, name := range []string{"AGENTS.md", "SETUP.MD"} {
		if _, err := os.Stat(filepath.Join(target, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s not to exist, got err=%v", name, err)
		}
	}
}

func TestParseFromOptionsDefaultsToUnderstanding(t *testing.T) {
	options, err := parseFromOptions([]string{"https://openknowledge.sh/wiki/", "--out=Wiki", "--depth=0"})
	if err != nil {
		t.Fatal(err)
	}
	if options.source != "https://openknowledge.sh/wiki/" || options.out != "Wiki" || options.wikiType != okf.DefaultFromType || options.depth != 0 {
		t.Fatalf("unexpected from options: %#v", options)
	}
	if _, err := parseFromOptions([]string{"https://openknowledge.sh/wiki/", "--out", "Wiki", "--depth", "-1"}); err == nil {
		t.Fatal("expected negative depth to fail")
	}
	if _, err := parseFromOptions([]string{"https://openknowledge.sh/wiki/"}); err == nil {
		t.Fatal("expected missing --out to fail")
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
	options, err := parseToOptions([]string{"./project-memory", "--out", "./site", "--spec", "0.1", "--plain", "--head-html", `<meta name="ok-head">`, "--script-src=/analytics.js"})
	if err != nil {
		t.Fatal(err)
	}
	if options.path != "./project-memory" || options.out != "./site" || options.spec != "0.1" || !options.plain || options.headHTML == "" || strings.Join(options.scriptSrcs, ",") != "/analytics.js" {
		t.Fatalf("unexpected options: %#v", options)
	}
}

func TestParseSearchOptionsAcceptsQueryFlags(t *testing.T) {
	defaults, err := parseSearchOptions([]string{"./project-memory", "validation workflow"})
	if err != nil {
		t.Fatal(err)
	}
	if defaults.format != "markdown" || defaults.budget != okf.DefaultContextBudget || defaults.limit != 12 || defaults.matches || defaults.noExpand {
		t.Fatalf("unexpected search defaults: %#v", defaults)
	}

	options, err := parseSearchOptions([]string{"./project-memory", "validation workflow", "--limit=5", "--budget", "900", "--format=json", "--spec", "0.1", "--no-expand"})
	if err != nil {
		t.Fatal(err)
	}
	if options.target != "./project-memory" || options.query != "validation workflow" || options.limit != 5 || options.budget != 900 || !options.budgetSet || options.format != "json" || options.spec != "0.1" || options.matches || !options.noExpand {
		t.Fatalf("unexpected search options: %#v", options)
	}
	if _, err := parseSearchOptions([]string{"./project-memory", "validation workflow", "--limit", "0"}); err == nil {
		t.Fatal("expected invalid limit to fail")
	}
	if _, err := parseSearchOptions([]string{"./project-memory", "validation workflow", "--budget", "0"}); err == nil {
		t.Fatal("expected invalid budget to fail")
	}
	if _, err := parseSearchOptions([]string{"./project-memory", "validation workflow", "--matches", "--budget", "900"}); err == nil {
		t.Fatal("expected matches and budget combination to fail")
	}
	if _, err := parseSearchOptions([]string{"./project-memory", "validation workflow", "--format", "text"}); err == nil {
		t.Fatal("expected removed text format to fail")
	}
	if _, err := parseSearchOptions([]string{"./project-memory", "validation workflow", "--expand", "graph"}); err == nil {
		t.Fatal("expected removed expand flag to fail")
	}
}

func TestRunValidateAcceptsRegistryKey(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Bundle\n")
	writeMainTestFile(t, root, "notes/topic.md", "---\ntype: Note\n---\n\n# Topic\n")
	t.Setenv(okf.RegistryFileEnv, filepath.Join(t.TempDir(), "registry.json"))
	if _, _, err := okf.ConnectRegistryEntry("personal", root, "read", true); err != nil {
		t.Fatal(err)
	}

	_, code := captureMainStdout(t, func() int {
		return runValidate([]string{"--quiet", "personal"})
	})
	if code != 0 {
		t.Fatalf("expected validate registry key to succeed, got exit code %d", code)
	}
}

func TestRunConnectAcceptsFlagsBeforeAndAfterDocumentedSourceArgument(t *testing.T) {
	tests := []struct {
		name string
		run  func(root string, key string) int
	}{
		{
			name: "top-level flags after source",
			run: func(root string, key string) int {
				return runConnect([]string{root, "--as", key, "--access", "write", "--no-validate"}, "openknowledge connect")
			},
		},
		{
			name: "top-level flags before source",
			run: func(root string, key string) int {
				return runConnect([]string{"--as", key, "--access", "write", "--no-validate", root}, "openknowledge connect")
			},
		},
		{
			name: "registry flags after source",
			run: func(root string, key string) int {
				return runRegistry([]string{"connect", root, "--as", key, "--access", "write", "--no-validate"})
			},
		},
		{
			name: "registry flags before source",
			run: func(root string, key string) int {
				return runRegistry([]string{"connect", "--as", key, "--access", "write", "--no-validate", root})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeMainTestFile(t, root, "index.md", "# Bundle\n")
			t.Setenv(okf.RegistryFileEnv, filepath.Join(t.TempDir(), "registry.json"))
			key := "personal"

			output, stderr, code := captureMainOutput(t, func() int {
				return test.run(root, key)
			})
			if code != 0 {
				t.Fatalf("expected connect to accept interspersed flags, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
			}
			entry, ok, err := okf.ResolveRegistryEntry(key)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || entry.Path != root || entry.Access != "write" {
				t.Fatalf("unexpected connected entry: %#v", entry)
			}
			for _, expected := range []string{"key      personal", "access   write", "status   unknown"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("expected connect output to include %q:\n%s", expected, output)
				}
			}
		})
	}
}

func TestRunDisconnectAcceptsFlagsBeforeAndAfterDocumentedTargetArgument(t *testing.T) {
	tests := []struct {
		name string
		run  func(key string) int
	}{
		{
			name: "top-level flag after target",
			run: func(key string) int {
				return runDisconnect([]string{key, "--delete-files"}, "openknowledge disconnect")
			},
		},
		{
			name: "top-level flag before target",
			run: func(key string) int {
				return runDisconnect([]string{"--delete-files", key}, "openknowledge disconnect")
			},
		},
		{
			name: "registry flag after target",
			run: func(key string) int {
				return runRegistry([]string{"disconnect", key, "--delete-files"})
			},
		},
		{
			name: "registry flag before target",
			run: func(key string) int {
				return runRegistry([]string{"disconnect", "--delete-files", key})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "bundles", "personal-cache")
			writeMainTestFile(t, root, "index.md", "# Bundle\n")
			t.Setenv(okf.RegistryFileEnv, filepath.Join(base, "registry.json"))
			key := "personal"
			if _, _, err := okf.ConnectRegistryEntryWithSource(key, root, "read", true, okf.RegistrySource{Type: "git", URL: "https://example.test/bundle.git", ManagedRoot: root}); err != nil {
				t.Fatal(err)
			}

			output, stderr, code := captureMainOutput(t, func() int {
				return test.run(key)
			})
			if code != 0 {
				t.Fatalf("expected disconnect to accept interspersed flags, got %d\nstdout=%s\nstderr=%s", code, output, stderr)
			}
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatalf("expected managed bundle files to be deleted, stat error: %v", err)
			}
			if _, ok, err := okf.ResolveRegistryEntry(key); err != nil || ok {
				t.Fatalf("expected registry entry to be removed, ok=%t err=%v", ok, err)
			}
			if !strings.Contains(output, "files  deleted") {
				t.Fatalf("expected disconnect output to report deleted files:\n%s", output)
			}
		})
	}
}

func TestRunValidatePrintsJSONReportWithConfiguredRules(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Bundle\n\n[Missing](missing.md)\n")
	writeMainTestFile(t, root, "openknowledge.toml", "[validation.rules]\nlink-target = \"error\"\n")

	output, code := captureMainStdout(t, func() int {
		return runValidate([]string{"--json", root})
	})
	if code != 1 {
		t.Fatalf("expected configured validation error exit code, got %d\n%s", code, output)
	}
	var report okf.Result
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("expected JSON validation report: %v\n%s", err, output)
	}
	if report.Summary.Status != "fail" || report.Summary.ErrorCount != 1 {
		t.Fatalf("unexpected JSON validation summary: %#v", report.Summary)
	}
	if report.SchemaVersion != okf.MachineSchemaVersion {
		t.Fatalf("unexpected validation schema version: %#v", report)
	}
	if len(report.Errors) != 1 || report.Errors[0].Rule != "link-target" || report.Errors[0].Severity != okf.ValidationSeverityError {
		t.Fatalf("expected escalated link-target error, got %#v", report.Errors)
	}
	if report.Policy.Overrides["link-target"] != okf.ValidationSeverityError || !strings.HasSuffix(report.Policy.ConfigPath, "openknowledge.toml") {
		t.Fatalf("expected policy metadata in report, got %#v", report.Policy)
	}
}

func TestRunValidateWritesJSONReport(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Bundle\n")
	out := filepath.Join(t.TempDir(), "okf-report.json")

	_, code := captureMainStdout(t, func() int {
		return runValidate([]string{"--format", "json", "--out", out, root})
	})
	if code != 0 {
		t.Fatalf("expected JSON report write to succeed, got exit code %d", code)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var report okf.Result
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("expected written JSON validation report: %v\n%s", err, string(data))
	}
	if report.Summary.Status != "pass" || report.Root == "" {
		t.Fatalf("unexpected written validation report: %#v", report)
	}
}

func TestRunListAcceptsRegistryKey(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Bundle\n")
	writeMainTestFile(t, root, "notes/topic.md", "---\ntype: Note\n---\n\n# Topic\n")
	t.Setenv(okf.RegistryFileEnv, filepath.Join(t.TempDir(), "registry.json"))
	if _, _, err := okf.ConnectRegistryEntry("personal", root, "read", true); err != nil {
		t.Fatal(err)
	}

	output, code := captureMainStdout(t, func() int {
		return runList([]string{"personal"})
	})
	if code != 0 {
		t.Fatalf("expected list registry key to succeed, got exit code %d", code)
	}
	if !strings.Contains(output, "topic.md") {
		t.Fatalf("expected list output to include bundle file:\n%s", output)
	}
}

func TestRunListDepthLimitsTreeAndIncludesAssets(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Bundle\n")
	writeMainTestFile(t, root, "assets/logo.txt", "logo")
	writeMainTestFile(t, root, "notes/topic.md", "---\ntype: Note\n---\n\n# Topic\n")
	writeMainTestFile(t, root, "notes/deep/detail.md", "---\ntype: Note\n---\n\n# Detail\n")

	output, code := captureMainStdout(t, func() int {
		return runList([]string{"--depth", "2", root})
	})
	if code != 0 {
		t.Fatalf("expected depth-limited list to succeed, got exit code %d", code)
	}
	for _, expected := range []string{"depth 2", "assets/", "logo.txt", "notes/", "topic.md"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected list output to include %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "detail.md") {
		t.Fatalf("did not expect depth-limited list output to include deep file:\n%s", output)
	}
}

func TestRunListJSONUsesVersionedEnvelope(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Home\n")
	writeMainTestFile(t, root, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n")

	output, code := captureMainStdout(t, func() int {
		return runList([]string{"--json", root})
	})
	if code != 0 {
		t.Fatalf("expected list JSON to succeed, got %d\n%s", code, output)
	}
	var listing okf.ListResult
	if err := json.Unmarshal([]byte(output), &listing); err != nil {
		t.Fatalf("expected versioned list JSON object: %v\n%s", err, output)
	}
	if listing.SchemaVersion != okf.MachineSchemaVersion || listing.Root != root || len(listing.Entries) != 2 {
		t.Fatalf("unexpected list JSON envelope: %#v", listing)
	}
}

func TestRunGetPrintsDirectFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "note.md")
	writeMainTestFile(t, root, "note.md", "# Note\n\nExact body.\n")

	output, code := captureMainStdout(t, func() int {
		return runGet([]string{file})
	})
	if code != 0 {
		t.Fatalf("expected get file to succeed, got exit code %d", code)
	}
	if output != "# Note\n\nExact body.\n" {
		t.Fatalf("unexpected get output:\n%s", output)
	}
}

func TestRunSearchPrintsMarkdownSections(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Home\n")
	writeMainTestFile(t, root, "guides/validate.md", "---\ntype: Guide\ntitle: Validation Workflow\n---\n\n# Validate\n\nRun `openknowledge validate` before sharing.\n")

	output, code := captureMainStdout(t, func() int {
		return runSearch([]string{root, "validation workflow"})
	})
	if code != 0 {
		t.Fatalf("expected search to succeed, got exit code %d", code)
	}
	for _, expected := range []string{"# Open Knowledge Context", "Query: validation workflow", "Context:", "Sources: 1", "## 1. Validate", "Source: `guides/validate.md:", "Relation: `direct`", "# Validate", "Run `openknowledge validate`"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected search output to include %q:\n%s", expected, output)
		}
	}
}

func TestRunSearchPrintsJSON(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Home\n")
	writeMainTestFile(t, root, "guides/release.md", "---\ntype: Guide\ntitle: Release Checklist\n---\n\n# Release\n\nShip the release notes.\n")

	output, code := captureMainStdout(t, func() int {
		return runSearch([]string{"--format", "json", root, "release checklist"})
	})
	if code != 0 {
		t.Fatalf("expected search json to succeed, got exit code %d", code)
	}
	var payload okf.ContextResult
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected JSON search output: %v\n%s", err, output)
	}
	if payload.Query != "release checklist" || payload.Budget != okf.DefaultContextBudget || len(payload.Sources) == 0 || payload.Sources[0].Path != "guides/release.md" || payload.Sources[0].Heading != "Release" {
		t.Fatalf("unexpected search payload: %#v", payload)
	}
	if payload.SchemaVersion != okf.MachineSchemaVersion {
		t.Fatalf("unexpected search context schema version: %#v", payload)
	}
	if payload.Sources[0].LineStart == 0 || payload.Sources[0].Markdown == "" || payload.Sources[0].Relation != "direct" {
		t.Fatalf("expected source range and Markdown in search context: %#v", payload.Sources[0])
	}
	if !strings.Contains(output, `"sources"`) || !strings.Contains(output, `"issues": []`) {
		t.Fatalf("expected stable context JSON fields:\n%s", output)
	}
}

func TestRunSearchMatchesPrintsRankedDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Home\n")
	writeMainTestFile(t, root, "guides/release.md", "---\ntype: Guide\ntitle: Release Checklist\n---\n\n# Release\n\nShip the release notes.\n")

	output, code := captureMainStdout(t, func() int {
		return runSearch([]string{"--matches", root, "release checklist"})
	})
	if code != 0 {
		t.Fatalf("expected search matches to succeed, got exit code %d", code)
	}
	for _, expected := range []string{"# Open Knowledge Search Matches", "Matches: 1", "## 1. Release", "Relation: `direct`", "Score:", "Ship the release notes"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected search matches output to include %q:\n%s", expected, output)
		}
	}
}

func TestRunSearchMatchesPrintsJSON(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Home\n")
	writeMainTestFile(t, root, "guides/release.md", "---\ntype: Guide\ntitle: Release Checklist\n---\n\n# Release\n\nShip the release notes.\n")

	output, code := captureMainStdout(t, func() int {
		return runSearch([]string{"--matches", "--format", "json", root, "release checklist"})
	})
	if code != 0 {
		t.Fatalf("expected JSON search matches to succeed, got exit code %d", code)
	}
	var payload okf.SearchResultSet
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected JSON match output: %v\n%s", err, output)
	}
	if len(payload.Results) != 1 || payload.Results[0].Path != "guides/release.md" || payload.Results[0].Relation != "direct" || payload.Results[0].Snippet == "" {
		t.Fatalf("unexpected JSON search matches: %#v", payload)
	}
	if payload.SchemaVersion != okf.MachineSchemaVersion {
		t.Fatalf("unexpected search results schema version: %#v", payload)
	}
}

func TestRunSearchExpandsRelatedContextByDefault(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "runbook.md", "---\ntype: Runbook\ntitle: Deploy Runbook\n---\n\n# Deploy\n\nRun the deploy checklist and read [Rollback](rollback.md).\n")
	writeMainTestFile(t, root, "rollback.md", "---\ntype: Runbook\ntitle: Rollback Plan\n---\n\n# Rollback\n\nRestore the previous release.\n")
	writeMainTestFile(t, root, "owners.md", "---\ntype: Team\ntitle: Owners\n---\n\n# Owners\n\nPlatform owns the [Runbook](runbook.md).\n")

	expanded, code := captureMainStdout(t, func() int {
		return runSearch([]string{root, "deploy checklist", "--budget", "1000"})
	})
	if code != 0 {
		t.Fatalf("expected expanded context search to succeed, got exit code %d", code)
	}
	for _, expected := range []string{"Relation: `direct`", "Relation: `outgoing-link`", "Relation: `backlink`", "Restore the previous release", "Platform owns the"} {
		if !strings.Contains(expanded, expected) {
			t.Fatalf("expected default context expansion to include %q:\n%s", expected, expanded)
		}
	}

	directOnly, code := captureMainStdout(t, func() int {
		return runSearch([]string{root, "deploy checklist", "--no-expand"})
	})
	if code != 0 {
		t.Fatalf("expected direct-only context search to succeed, got exit code %d", code)
	}
	if strings.Contains(directOnly, "outgoing-link") || strings.Contains(directOnly, "backlink") || strings.Contains(directOnly, "Restore the previous release") {
		t.Fatalf("expected --no-expand to omit related context:\n%s", directOnly)
	}
}

func TestRunToTarWritesPortableArchive(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Bundle\n")
	writeMainTestFile(t, root, "notes/topic.md", "---\ntype: Note\n---\n\n# Topic\n")
	out := filepath.Join(t.TempDir(), "bundle.tar.gz")

	code := runToTar([]string{"--out", out, root})
	if code != 0 {
		t.Fatalf("expected to tar to succeed, got exit code %d", code)
	}
	extracted := filepath.Join(t.TempDir(), "extracted")
	if err := okf.ExtractBundleArchive(out, extracted); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(extracted, "index.md")); err != nil {
		t.Fatalf("expected extracted index.md: %v", err)
	}
	validation, err := okf.Validate(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if len(validation.Errors) != 0 {
		t.Fatalf("expected extracted archive to validate, got %#v", validation.Errors)
	}
}

func TestRunToHTMLInjectsTrustedHeadHTML(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeMainTestFile(t, root, "index.md", "# Bundle\n\nRead [Topic](notes/topic.md).\n")
	writeMainTestFile(t, root, "notes/topic.md", "# Topic\n")

	code := runToHTML([]string{
		"--head-html", `<meta name="ok-export-head" content="1">`,
		"--script-src", "/analytics.js",
		"--out", out,
		root,
	})
	if code != 0 {
		t.Fatalf("expected to html to succeed, got exit code %d", code)
	}

	index := string(readMainTestFile(t, filepath.Join(out, "index.html")))
	if !strings.Contains(index, `<meta name="ok-export-head" content="1">`) || !strings.Contains(index, `<script src="/analytics.js"></script>`) {
		t.Fatalf("expected exported index to include trusted head HTML:\n%s", index)
	}

	nested := string(readMainTestFile(t, filepath.Join(out, "notes", "topic.html")))
	if !strings.Contains(nested, `<meta name="ok-export-head" content="1">`) || !strings.Contains(nested, `<script src="/analytics.js"></script>`) {
		t.Fatalf("expected nested exported page to include trusted head HTML:\n%s", nested)
	}
}

func TestRunToGraphPrintsGraphJSON(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Home\n\nRead [Topic](notes/topic.md).\n")
	writeMainTestFile(t, root, "notes/topic.md", "---\ntype: Note\ntitle: Topic\n---\n\n# Topic\n")

	output, code := captureMainStdout(t, func() int {
		return runToGraph([]string{root})
	})
	if code != 0 {
		t.Fatalf("expected to graph to succeed, got exit code %d", code)
	}
	var graph okf.Graph
	if err := json.Unmarshal([]byte(output), &graph); err != nil {
		t.Fatalf("expected graph JSON output: %v\n%s", err, output)
	}
	if len(graph.Nodes) != 2 || len(graph.Edges) != 1 {
		t.Fatalf("unexpected graph output: %#v", graph)
	}
	if graph.SchemaVersion != okf.MachineSchemaVersion {
		t.Fatalf("unexpected graph schema version: %#v", graph)
	}
	if graph.Edges[0].Source != "index.md" || graph.Edges[0].Target != "notes/topic.md" || graph.Edges[0].Label != "Topic" {
		t.Fatalf("unexpected graph edge: %#v", graph.Edges[0])
	}
}

func TestRunToGraphPrintsSearchGraphJSON(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Home\n\nRead [Topic](notes/topic.md).\n")
	writeMainTestFile(t, root, "notes/topic.md", "---\ntype: Note\ntitle: Topic\n---\n\n# Topic\n\n## Details\n\nSearchable details.\n")

	output, code := captureMainStdout(t, func() int {
		return runToGraph([]string{"--type", "search", root})
	})
	if code != 0 {
		t.Fatalf("expected to graph --type search to succeed, got exit code %d", code)
	}
	var graph okf.Graph
	if err := json.Unmarshal([]byte(output), &graph); err != nil {
		t.Fatalf("expected graph JSON output: %v\n%s", err, output)
	}
	if graph.Type != okf.GraphTypeSearch {
		t.Fatalf("expected search graph, got %#v", graph)
	}
	hasChunk := false
	for _, node := range graph.Nodes {
		if node.ID == "notes/topic#details" && node.Kind == "chunk" && node.Heading == "Details" {
			hasChunk = true
		}
	}
	if !hasChunk {
		t.Fatalf("expected search graph chunk node, got %#v", graph.Nodes)
	}
}

func TestParseGetOptionsAllowsInfoAfterEntry(t *testing.T) {
	options, err := parseGetOptions([]string{"accessibility", "review", "--info"})
	if err != nil {
		t.Fatal(err)
	}
	if options.target != "accessibility" || options.entry != "review" || !options.info {
		t.Fatalf("unexpected options: %#v", options)
	}
}

func TestSelectGetTargetUsesDefaultNamedAndRootFallback(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", `---
okf_version: "0.1"
okf_bundle_entry_default: "agents/default.md"
okf_bundle_entry_review: "agents/review.md"
---

# Bundle
`)
	writeMainTestFile(t, root, "agents/default.md", "---\ntype: Agent Entrypoint\n---\n\n# Default\n")
	writeMainTestFile(t, root, "agents/review.md", "---\ntype: Agent Entrypoint\n---\n\n# Review\n")
	writeMainTestFile(t, root, "guides/manual.md", "---\ntype: Guide\n---\n\n# Manual\n")

	info, err := okf.ReadBundleInfo(root)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := selectGetTarget(root, info, "")
	if err != nil {
		t.Fatal(err)
	}
	if selection.name != "default" || selection.rel != "agents/default.md" {
		t.Fatalf("unexpected default selection: %#v", selection)
	}
	selection, err = selectGetTarget(root, info, "review")
	if err != nil {
		t.Fatal(err)
	}
	if selection.name != "review" || selection.rel != "agents/review.md" {
		t.Fatalf("unexpected review selection: %#v", selection)
	}
	selection, err = selectGetTarget(root, info, "guides/manual.md")
	if err != nil {
		t.Fatal(err)
	}
	if selection.name != "guides/manual.md" || selection.rel != "guides/manual.md" {
		t.Fatalf("unexpected path selection: %#v", selection)
	}
	if _, err := selectGetTarget(root, info, "missing"); err == nil {
		t.Fatal("expected missing entrypoint path to fail")
	} else if !strings.Contains(err.Error(), `entrypoint or path "missing" does not exist; available entries: default, review`) {
		t.Fatalf("unexpected missing entrypoint error: %v", err)
	}

	fallbackRoot := t.TempDir()
	writeMainTestFile(t, fallbackRoot, "index.md", "# Root\n")
	fallbackInfo, err := okf.ReadBundleInfo(fallbackRoot)
	if err != nil {
		t.Fatal(err)
	}
	selection, err = selectGetTarget(fallbackRoot, fallbackInfo, "")
	if err != nil {
		t.Fatal(err)
	}
	if selection.name != "index" || selection.rel != "index.md" {
		t.Fatalf("unexpected root fallback selection: %#v", selection)
	}
}

func TestRunConnectClonesRemoteSource(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for remote connect test")
	}
	base := t.TempDir()
	remote := filepath.Join(base, "remote")
	runGit(t, base, "init", remote)
	runGit(t, remote, "config", "user.email", "test@example.com")
	runGit(t, remote, "config", "user.name", "Test User")
	writeMainTestFile(t, remote, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_name: remote\n---\n\n# Remote\n")
	runGit(t, remote, "add", "index.md")
	runGit(t, remote, "commit", "-m", "init")

	registryFile := filepath.Join(base, "registry.json")
	t.Setenv(okf.RegistryFileEnv, registryFile)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))

	code := runConnect([]string{"--as", "remote", "--no-validate", "file://" + remote}, "openknowledge connect")
	if code != 0 {
		t.Fatalf("expected remote connect to succeed, got exit code %d", code)
	}
	entry, ok, err := okf.ResolveRegistryEntry("remote")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected remote registry entry")
	}
	if !entry.Managed || entry.Source.Type != "git" || entry.Source.URL != "file://"+remote {
		t.Fatalf("unexpected remote registry entry: %#v", entry)
	}
	if entry.Source.GitCommit == "" || entry.Source.ManagedRoot != entry.Path || entry.Source.Spec != okf.LatestSpecVersion {
		t.Fatalf("expected exact Git provenance and managed root: %#v", entry.Source)
	}
	if _, err := time.Parse(time.RFC3339Nano, entry.Source.FetchedAt); err != nil {
		t.Fatalf("expected RFC3339 fetch time, got %q: %v", entry.Source.FetchedAt, err)
	}
	if _, err := os.Stat(filepath.Join(entry.Path, "index.md")); err != nil {
		t.Fatalf("expected cloned index.md: %v", err)
	}
}

func TestRunConnectUsesRemoteOpenKnowledgeManifest(t *testing.T) {
	base := t.TempDir()
	bundle := filepath.Join(base, "bundle")
	writeMainTestFile(t, bundle, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_name: hosted\n---\n\n# Hosted\n")
	hosted := filepath.Join(base, "hosted")
	archive := filepath.Join(hosted, "assets", "openknowledge-bundle.tar.gz")
	archiveResult, err := okf.WriteBundleTarGzipWithVersion(bundle, archive, "0.1", nil)
	if err != nil {
		t.Fatal(err)
	}
	manifest := okf.BundleManifest{
		Type:          okf.BundleManifestType,
		Version:       1,
		Spec:          "0.1",
		Name:          "hosted",
		Title:         "Hosted",
		Archive:       "assets/openknowledge-bundle.tar.gz",
		ArchiveSHA256: archiveResult.SHA256,
		ArchiveFormat: okf.BundleArchiveFormat,
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hosted, okf.BundleManifestRelPath), manifestData, 0644); err != nil {
		t.Fatal(err)
	}

	registryFile := filepath.Join(base, "registry.json")
	t.Setenv(okf.RegistryFileEnv, registryFile)
	manifestURL := "file://" + filepath.Join(hosted, okf.BundleManifestRelPath)
	code := runConnect([]string{"--no-validate", manifestURL}, "openknowledge connect")
	if code != 0 {
		t.Fatalf("expected manifest connect to succeed, got exit code %d", code)
	}
	entry, ok, err := okf.ResolveRegistryEntry("hosted")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected hosted registry entry")
	}
	expectedArchiveURL := "file://" + filepath.Join(hosted, "assets", "openknowledge-bundle.tar.gz")
	if entry.Source.Type != "manifest" || entry.Source.URL != manifestURL || entry.Source.Ref != expectedArchiveURL {
		t.Fatalf("unexpected manifest source: %#v", entry.Source)
	}
	if entry.Source.ManifestURL != manifestURL || entry.Source.ArchiveURL != expectedArchiveURL || entry.Source.SHA256 != archiveResult.SHA256 || entry.Source.Spec != "0.1" || entry.Source.ManagedRoot == "" {
		t.Fatalf("expected complete manifest provenance: %#v", entry.Source)
	}
	if _, err := os.Stat(filepath.Join(entry.Path, "index.md")); err != nil {
		t.Fatalf("expected materialized index.md: %v", err)
	}
}

func TestRunConnectResolvesManifestArchiveAgainstRedirectDestination(t *testing.T) {
	base := t.TempDir()
	bundle := filepath.Join(base, "bundle")
	writeMainTestFile(t, bundle, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_name: redirected\n---\n\n# Redirected\n")
	archivePath := filepath.Join(base, "bundle.tar.gz")
	archiveResult, err := okf.WriteBundleTarGzipWithVersion(bundle, archivePath, "0.1", nil)
	if err != nil {
		t.Fatal(err)
	}
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	manifestData, err := json.Marshal(okf.BundleManifest{
		Type:          okf.BundleManifestType,
		Version:       okf.BundleManifestVersion,
		Spec:          "0.1",
		Archive:       "bundle.tar.gz",
		ArchiveSHA256: archiveResult.SHA256,
		ArchiveFormat: okf.BundleArchiveFormat,
	})
	if err != nil {
		t.Fatal(err)
	}

	requestedManifestURL := "https://example.test/openknowledge.json"
	finalManifestURL := "https://cdn.example.test/releases/v1/openknowledge.json"
	finalArchiveURL := "https://cdn.example.test/releases/v1/bundle.tar.gz"
	originalHTTPClient := remoteHTTPClient
	remoteHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case requestedManifestURL:
			finalRequest := request.Clone(request.Context())
			finalRequest.URL, _ = url.Parse(finalManifestURL)
			return testHTTPResponse(http.StatusOK, "application/json", manifestData, finalRequest), nil
		case finalArchiveURL:
			return testHTTPResponse(http.StatusOK, "application/gzip", archiveData, request), nil
		default:
			return testHTTPResponse(http.StatusNotFound, "text/plain", []byte("not found"), request), nil
		}
	})}
	defer func() { remoteHTTPClient = originalHTTPClient }()

	t.Setenv(okf.RegistryFileEnv, filepath.Join(base, "registry.json"))
	if code := runConnect([]string{"--as", "redirected", "--no-validate", requestedManifestURL}, "openknowledge connect"); code != 0 {
		t.Fatalf("expected redirected manifest connect to succeed, got %d", code)
	}
	entry, ok, err := okf.ResolveRegistryEntry("redirected")
	if err != nil || !ok {
		t.Fatalf("expected redirected registry entry, ok=%t err=%v", ok, err)
	}
	if entry.Source.URL != requestedManifestURL || entry.Source.Ref != finalArchiveURL || entry.Source.ManifestURL != finalManifestURL || entry.Source.ResolvedURL != finalManifestURL || entry.Source.ArchiveURL != finalArchiveURL || entry.Source.SHA256 != archiveResult.SHA256 {
		t.Fatalf("unexpected redirected manifest provenance: %#v", entry.Source)
	}
}

func TestRemoteCacheIdentityDoesNotDependOnRegistryAlias(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for remote cache identity test")
	}
	base := t.TempDir()
	remote := filepath.Join(base, "remote")
	runGit(t, base, "init", remote)
	runGit(t, remote, "config", "user.email", "test@example.com")
	runGit(t, remote, "config", "user.name", "Test User")
	writeMainTestFile(t, remote, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_name: remote\n---\n\n# Remote\n")
	runGit(t, remote, "add", "index.md")
	runGit(t, remote, "commit", "-m", "init")

	registryFile := filepath.Join(base, "registry.json")
	t.Setenv(okf.RegistryFileEnv, registryFile)
	source := "file://" + remote
	if code := runConnect([]string{"--as", "first", "--no-validate", source}, "openknowledge connect"); code != 0 {
		t.Fatalf("expected first connect, got %d", code)
	}
	first, ok, err := okf.ResolveRegistryEntry("first")
	if err != nil || !ok {
		t.Fatalf("expected first entry, ok=%t err=%v", ok, err)
	}
	if code := runConnect([]string{"--as", "second", "--no-validate", source}, "openknowledge connect"); code != 0 {
		t.Fatalf("expected cached connect, got %d", code)
	}
	second, ok, err := okf.ResolveRegistryEntry("second")
	if err != nil || !ok {
		t.Fatalf("expected renamed cached entry, ok=%t err=%v", ok, err)
	}
	if first.Path != second.Path || first.Source.GitCommit != second.Source.GitCommit || first.Source.FetchedAt != second.Source.FetchedAt {
		t.Fatalf("cache hit must preserve path and exact provenance:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if _, ok, err := okf.ResolveRegistryEntry("first"); err != nil || ok {
		t.Fatalf("same cached path should be renamed, not duplicated, ok=%t err=%v", ok, err)
	}

	cacheRoot := filepath.Join(filepath.Dir(registryFile), "bundles")
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		t.Fatal(err)
	}
	directories := 0
	for _, entry := range entries {
		if entry.IsDir() {
			directories++
		}
	}
	if directories != 1 {
		t.Fatalf("expected one source-addressed cache directory, got %d entries=%v", directories, entries)
	}
	metadataPath := remoteCacheSourcePath(first.Source.ManagedRoot)
	info, err := os.Stat(metadataPath)
	if err != nil {
		t.Fatalf("expected persistent cache provenance sidecar: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("expected owner-only cache provenance, got %04o", info.Mode().Perm())
	}
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var record remoteCacheRecord
	if err := json.Unmarshal(metadata, &record); err != nil {
		t.Fatal(err)
	}
	if record.SchemaVersion != remoteCacheSchemaVersion || record.Source.GitCommit != first.Source.GitCommit {
		t.Fatalf("unexpected versioned cache provenance: %#v", record)
	}
}

func TestConcurrentRemoteMaterializationPublishesOneCompleteCache(t *testing.T) {
	base := t.TempDir()
	bundle := filepath.Join(base, "bundle")
	writeMainTestFile(t, bundle, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_name: concurrent\n---\n\n# Concurrent\n")
	hosted := filepath.Join(base, "hosted")
	archivePath := filepath.Join(hosted, "bundle.tar.gz")
	archiveResult, err := okf.WriteBundleTarGzipWithVersion(bundle, archivePath, "0.1", nil)
	if err != nil {
		t.Fatal(err)
	}
	manifestData, err := json.Marshal(okf.BundleManifest{
		Type:          okf.BundleManifestType,
		Version:       okf.BundleManifestVersion,
		Spec:          "0.1",
		Archive:       "bundle.tar.gz",
		ArchiveSHA256: archiveResult.SHA256,
		ArchiveFormat: okf.BundleArchiveFormat,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(hosted, okf.BundleManifestRelPath)
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(okf.RegistryFileEnv, filepath.Join(base, "registry.json"))
	source := "file://" + manifestPath
	const workers = 12
	start := make(chan struct{})
	errors := make(chan error, workers)
	paths := make(chan string, workers)
	var waitGroup sync.WaitGroup
	for index := 0; index < workers; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			root, sourceInfo, err := materializeRemoteSource(source)
			if err != nil {
				errors <- err
				return
			}
			if sourceInfo.Type != "manifest" || sourceInfo.SHA256 != archiveResult.SHA256 {
				errors <- fmt.Errorf("unexpected provenance: %#v", sourceInfo)
				return
			}
			paths <- root
		}()
	}
	close(start)
	waitGroup.Wait()
	close(errors)
	close(paths)
	for err := range errors {
		t.Error(err)
	}
	var expectedPath string
	for materializedPath := range paths {
		if expectedPath == "" {
			expectedPath = materializedPath
		} else if materializedPath != expectedPath {
			t.Errorf("expected one cache path %s, got %s", expectedPath, materializedPath)
		}
	}
	if t.Failed() {
		return
	}
	if _, err := os.Stat(filepath.Join(expectedPath, "index.md")); err != nil {
		t.Fatalf("expected complete published cache: %v", err)
	}
	cacheRoot := filepath.Join(base, "bundles")
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		t.Fatal(err)
	}
	directories := 0
	for _, entry := range entries {
		if entry.IsDir() {
			directories++
		}
		if strings.HasPrefix(entry.Name(), ".openknowledge-") {
			t.Fatalf("unexpected staging directory after concurrent materialization: %s", entry.Name())
		}
	}
	if directories != 1 {
		t.Fatalf("expected one published cache directory, got %d entries=%v", directories, entries)
	}
}

func TestRunConnectRejectsInvalidGitBundleWithoutPublishingCache(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for remote validation test")
	}
	base := t.TempDir()
	remote := filepath.Join(base, "remote")
	runGit(t, base, "init", remote)
	runGit(t, remote, "config", "user.email", "test@example.com")
	runGit(t, remote, "config", "user.name", "Test User")
	writeMainTestFile(t, remote, "README.md", "# Missing required concept frontmatter\n")
	runGit(t, remote, "add", "README.md")
	runGit(t, remote, "commit", "-m", "invalid")

	registryFile := filepath.Join(base, "registry.json")
	t.Setenv(okf.RegistryFileEnv, registryFile)
	source := "file://" + remote
	_, stderr, code := captureMainOutput(t, func() int {
		return runConnect([]string{"--as", "invalid", "--no-validate", source}, "openknowledge connect")
	})
	if code != 1 || !strings.Contains(stderr, "does not contain a valid Open Knowledge bundle") {
		t.Fatalf("expected invalid Git bundle refusal, code=%d stderr=%s", code, stderr)
	}
	target := filepath.Join(base, "bundles", registryCacheName(source))
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("invalid Git clone must not be published, stat error: %v", err)
	}
	if _, ok, err := okf.ResolveRegistryEntry("invalid"); err != nil || ok {
		t.Fatalf("invalid Git bundle must not be registered, ok=%t err=%v", ok, err)
	}
}

func TestFailedRemoteReplacementPreservesPreviousCache(t *testing.T) {
	base := t.TempDir()
	t.Setenv(okf.RegistryFileEnv, filepath.Join(base, "registry.json"))
	source := "file://" + filepath.Join(base, "missing.git")
	target := filepath.Join(base, "bundles", registryCacheName(source))
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "marker")
	if err := os.WriteFile(marker, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "bad.md"), []byte("# Missing frontmatter\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := materializeRemoteSource(source); err == nil {
		t.Fatal("expected missing remote replacement to fail")
	}
	content, err := os.ReadFile(marker)
	if err != nil || string(content) != "previous" {
		t.Fatalf("failed replacement must preserve previous cache, content=%q err=%v", content, err)
	}
}

func TestPublishRemoteCacheRollsBackFailedReplacement(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "cache")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "marker")
	if err := os.WriteFile(marker, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}

	err := publishRemoteCache(filepath.Join(base, "missing-staging"), target)
	if err == nil {
		t.Fatal("expected publication with missing staging directory to fail")
	}
	content, readErr := os.ReadFile(marker)
	if readErr != nil || string(content) != "previous" {
		t.Fatalf("failed publication must restore previous cache, content=%q err=%v", content, readErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(base, ".openknowledge-previous-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("rollback left previous-cache staging paths: %v", matches)
	}
}

func TestDisconnectDeleteFilesRemovesEntireNestedManagedCache(t *testing.T) {
	base := t.TempDir()
	archivePath := filepath.Join(base, "nested.tar.gz")
	writeMainTestTarGzip(t, archivePath, map[string]string{
		"LICENSE":         "fixture license\n",
		"bundle/index.md": "---\nokf_version: \"0.1\"\nokf_bundle_name: nested\n---\n\n# Nested\n",
	})
	registryFile := filepath.Join(base, "registry.json")
	t.Setenv(okf.RegistryFileEnv, registryFile)
	if code := runConnect([]string{"--as", "nested", "--no-validate", "file://" + archivePath}, "openknowledge connect"); code != 0 {
		t.Fatalf("expected nested archive connect, got %d", code)
	}
	entry, ok, err := okf.ResolveRegistryEntry("nested")
	if err != nil || !ok {
		t.Fatalf("expected nested entry, ok=%t err=%v", ok, err)
	}
	if entry.Source.ManagedRoot == "" || entry.Source.ManagedRoot == entry.Path || filepath.Base(entry.Path) != "bundle" {
		t.Fatalf("expected nested bundle path inside complete managed root: %#v", entry)
	}
	if _, err := os.Stat(filepath.Join(entry.Source.ManagedRoot, "LICENSE")); err != nil {
		t.Fatalf("expected top-level cache sibling before deletion: %v", err)
	}
	unrelated := filepath.Join(filepath.Dir(entry.Source.ManagedRoot), "unrelated-cache-sibling")
	if err := os.Mkdir(unrelated, 0700); err != nil {
		t.Fatal(err)
	}

	output, stderr, code := captureMainOutput(t, func() int {
		return runDisconnect([]string{"nested", "--delete-files"}, "openknowledge disconnect")
	})
	if code != 0 {
		t.Fatalf("expected managed cache deletion, code=%d stdout=%s stderr=%s", code, output, stderr)
	}
	if _, err := os.Stat(entry.Source.ManagedRoot); !os.IsNotExist(err) {
		t.Fatalf("expected complete managed root deletion, stat error: %v", err)
	}
	if _, err := os.Stat(remoteCacheSourcePath(entry.Source.ManagedRoot)); !os.IsNotExist(err) {
		t.Fatalf("expected provenance sidecar deletion, stat error: %v", err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated cache sibling must remain: %v", err)
	}
	if _, ok, err := okf.ResolveRegistryEntry("nested"); err != nil || ok {
		t.Fatalf("expected nested entry removal, ok=%t err=%v", ok, err)
	}
	if !strings.Contains(output, "files  deleted") {
		t.Fatalf("expected deleted output: %s", output)
	}
	entries, err := os.ReadDir(filepath.Dir(entry.Source.ManagedRoot))
	if err != nil {
		t.Fatal(err)
	}
	for _, cacheEntry := range entries {
		if strings.HasPrefix(cacheEntry.Name(), ".openknowledge-delete-") {
			t.Fatalf("unexpected deletion tombstone left behind: %s", cacheEntry.Name())
		}
	}
}

func TestDisconnectDeleteFilesRefusesManagedFlagOutsideCache(t *testing.T) {
	base := t.TempDir()
	registryFile := filepath.Join(base, "registry.json")
	t.Setenv(okf.RegistryFileEnv, registryFile)
	local := filepath.Join(base, "local")
	writeMainTestFile(t, local, "index.md", "# Local\n")
	if _, _, err := okf.ConnectRegistryEntryWithSource("local", local, "read", true, okf.RegistrySource{Type: "git", URL: "https://example.test/forged.git"}); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runDisconnect([]string{"local", "--delete-files"}, "openknowledge disconnect")
	})
	if code != 1 || !strings.Contains(stderr, "outside the Open Knowledge cache") {
		t.Fatalf("expected out-of-cache refusal, code=%d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(local); err != nil {
		t.Fatalf("refused local path must remain: %v", err)
	}
	if _, ok, err := okf.ResolveRegistryEntry("local"); err != nil || !ok {
		t.Fatalf("refused entry must remain registered, ok=%t err=%v", ok, err)
	}
}

func TestRunConnectRejectsManifestAndArchiveSpecMismatch(t *testing.T) {
	base := t.TempDir()
	bundle := filepath.Join(base, "bundle")
	writeMainTestFile(t, bundle, "index.md", "---\nokf_version: \"9.9\"\nokf_bundle_name: mismatch\n---\n\n# Mismatch\n")
	hosted := filepath.Join(base, "hosted")
	archivePath := filepath.Join(hosted, "bundle.tar.gz")
	archiveResult, err := okf.WriteBundleTarGzipWithVersion(bundle, archivePath, "0.1", nil)
	if err != nil {
		t.Fatal(err)
	}
	manifestData, err := json.Marshal(okf.BundleManifest{
		Type:          okf.BundleManifestType,
		Version:       okf.BundleManifestVersion,
		Spec:          "0.1",
		Archive:       "bundle.tar.gz",
		ArchiveSHA256: archiveResult.SHA256,
		ArchiveFormat: okf.BundleArchiveFormat,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(hosted, okf.BundleManifestRelPath)
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(okf.RegistryFileEnv, filepath.Join(base, "registry.json"))
	_, stderr, code := captureMainOutput(t, func() int {
		return runConnect([]string{"--as", "mismatch", "--no-validate", "file://" + manifestPath}, "openknowledge connect")
	})
	if code != 1 || !strings.Contains(stderr, `archive bundle declares okf_version "9.9" but manifest requires "0.1"`) {
		t.Fatalf("expected manifest/archive spec mismatch, code=%d stderr=%s", code, stderr)
	}
	if _, ok, err := okf.ResolveRegistryEntry("mismatch"); err != nil || ok {
		t.Fatalf("mismatched bundle must not be registered, ok=%t err=%v", ok, err)
	}
}

func TestFetchBundleManifestSurfacesServerErrors(t *testing.T) {
	originalHTTPClient := remoteHTTPClient
	remoteHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return testHTTPResponse(http.StatusServiceUnavailable, "text/plain", []byte("unavailable"), request), nil
	})}
	defer func() { remoteHTTPClient = originalHTTPClient }()

	if _, _, ok, err := fetchBundleManifest("https://example.test/openknowledge.json"); err == nil || ok || !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Fatalf("expected manifest server error to be preserved, ok=%t err=%v", ok, err)
	}
}

func writeMainTestTarGzip(t *testing.T, archivePath string, entries map[string]string) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range entries {
		data := []byte(content)
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadRemoteFileEnforcesByteLimitAndCleansPartialFile(t *testing.T) {
	tests := []struct {
		name          string
		contentLength int64
	}{
		{name: "declared length", contentLength: 6},
		{name: "streamed length", contentLength: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalHTTPClient := remoteHTTPClient
			remoteHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				response := testHTTPResponse(http.StatusOK, "application/octet-stream", []byte("123456"), request)
				response.ContentLength = test.contentLength
				return response, nil
			})}
			defer func() { remoteHTTPClient = originalHTTPClient }()

			target := filepath.Join(t.TempDir(), "download")
			if _, err := downloadRemoteFile("https://example.test/bundle", target, 5); err == nil || !strings.Contains(err.Error(), "maximum size of 5 bytes") {
				t.Fatalf("expected bounded download error, got %v", err)
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("bounded download must not leave partial target, stat error: %v", err)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func testHTTPResponse(statusCode int, contentType string, body []byte, request *http.Request) *http.Response {
	return &http.Response{
		StatusCode:    statusCode,
		Status:        fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:        http.Header{"Content-Type": []string{contentType}},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       request,
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func captureMainStdout(t *testing.T, run func() int) (string, int) {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	code := run()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = original
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(output), code
}

func captureMainOutput(t *testing.T, run func() int) (string, string, int) {
	t.Helper()
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	code := run()
	if err := stdoutWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = originalStdout
	os.Stderr = originalStderr
	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatal(err)
	}
	return string(stdout), string(stderr), code
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
