package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
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
		"openknowledge connect <source>",
		"openknowledge connect <source> --as <key>",
		"openknowledge disconnect <key|path>",
		"openknowledge use <name|path> [entry]",
		"openknowledge use <name|path> --info",
		"openknowledge use <name|path> --query <text>",
		"openknowledge use <name|path> --query <text> --format json",
		"openknowledge registry connect <source>",
		"openknowledge registry connect <source> --as <key>",
		"openknowledge registry disconnect <key|path>",
		"openknowledge registry where <name|path>",
		"openknowledge open --name <alias-name> [path]",
		"openknowledge open --host <host> --port <port> [path]",
		"openknowledge open --no-browser [path]",
		"openknowledge to html --out <folder> [path]",
		"openknowledge to json --out <file> [path]",
		"openknowledge to tar --out <file> [path]",
		"openknowledge validate --spec <version> [path]",
		"openknowledge list --spec <version> [path]",
		"openknowledge list --json [path]",
		"Commands:",
		"setup      Print an agent setup prompt.",
		"new        Scaffold a local Open Knowledge bundle.",
		"connect    Connect a local or remote knowledge bundle.",
		"disconnect Remove a knowledge bundle connection.",
		"use        Print an agent entrypoint or query-focused excerpts from a bundle.",
		"registry   Manage knowledge bundle connections.",
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
		"openknowledge use accessibility --query \"validation workflow\"",
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
		"openknowledge context",
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
		"use": {
			help: useHelpText(),
			required: []string{
				"openknowledge use <name|path> <entry> --info",
				"openknowledge use <name|path> --query <text>",
				"--budget",
				"--format",
				"okf_bundle_entry_<name>",
				"prints the bundle root index.md",
				"not use embeddings",
				"generate summaries",
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
				"openknowledge to tar --out <file> [path]",
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

func TestParseUseOptionsAllowsQueryFlags(t *testing.T) {
	options, err := parseUseOptions([]string{"./project-memory", "--query", "validation workflow", "--budget", "1200", "--limit=5", "--format=json", "--spec", "0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if options.target != "./project-memory" || !options.queryMode || options.query != "validation workflow" || options.budget != 1200 || options.limit != 5 || options.format != "json" || options.spec != "0.1" {
		t.Fatalf("unexpected use query options: %#v", options)
	}
	if _, err := parseUseOptions([]string{"./project-memory", "--budget", "1200"}); err == nil {
		t.Fatal("expected query-only flag without query to fail")
	}
	if _, err := parseUseOptions([]string{"./project-memory", "--query", "x", "--budget", "0"}); err == nil {
		t.Fatal("expected invalid budget to fail")
	}
	if _, err := parseUseOptions([]string{"./project-memory", "review", "--query", "x"}); err == nil {
		t.Fatal("expected query mode with entry to fail")
	}
}

func TestRunUseQueryPrintsMarkdownSections(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Home\n")
	writeMainTestFile(t, root, "guides/validate.md", "---\ntype: Guide\ntitle: Validation Workflow\n---\n\n# Validate\n\nRun `openknowledge validate` before sharing.\n")

	output, code := captureMainStdout(t, func() int {
		return runUse([]string{root, "--query", "validation workflow", "--budget", "400"})
	})
	if code != 0 {
		t.Fatalf("expected use query to succeed, got exit code %d", code)
	}
	for _, expected := range []string{"# Open Knowledge Query", "guides/validate.md:", "Run `openknowledge validate`"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected markdown use query to include %q:\n%s", expected, output)
		}
	}
}

func TestRunUseQueryPrintsJSON(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Home\n")
	writeMainTestFile(t, root, "guides/release.md", "---\ntype: Guide\ntitle: Release Checklist\n---\n\n# Release\n\nShip the release notes.\n")

	output, code := captureMainStdout(t, func() int {
		return runUse([]string{"--query=release checklist", "--format", "json", root})
	})
	if code != 0 {
		t.Fatalf("expected use query json to succeed, got exit code %d", code)
	}
	var payload okf.ContextResult
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected JSON use query output: %v\n%s", err, output)
	}
	if payload.Query != "release checklist" || len(payload.Results) == 0 || payload.Results[0].Path != "guides/release.md" {
		t.Fatalf("unexpected use query payload: %#v", payload)
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
	selection, err = selectUseEntrypoint(root, info, "guides/manual.md")
	if err != nil {
		t.Fatal(err)
	}
	if selection.name != "guides/manual.md" || selection.rel != "guides/manual.md" {
		t.Fatalf("unexpected path selection: %#v", selection)
	}
	if _, err := selectUseEntrypoint(root, info, "missing"); err == nil {
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
	selection, err = selectUseEntrypoint(fallbackRoot, fallbackInfo, "")
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
	if _, err := os.Stat(filepath.Join(entry.Path, "index.md")); err != nil {
		t.Fatalf("expected materialized index.md: %v", err)
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
