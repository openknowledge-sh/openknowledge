package main

import (
	"fmt"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func getHelpText() string {
	return `openknowledge get

Print an exact Markdown file or bundle entrypoint.

Usage:
  openknowledge get <name|path>
  openknowledge get <name|path> <entry-or-file>
  openknowledge get <name|path> --info
  openknowledge get <name|path> <entry-or-file> --info
  openknowledge get --help

Arguments:
  name|path      Local Markdown file, registry key, or local bundle path.
  entry-or-file  Optional entrypoint name from okf_bundle_entry_<name> or
                 bundle-relative Markdown file path inside the selected bundle.

Flags:
  --info         Print bundle and selected-file metadata instead of Markdown body.

Behavior:
  With one argument that points at a local Markdown file, get prints that exact
  file.
  With a bundle path or registry key, get prints okf_bundle_entry_default when
  declared. If no default entrypoint exists, it prints the bundle root index.md.
  With a second argument, get first checks root index.md metadata, then treats
  the value as a path inside the bundle.

  Use openknowledge search when you need query-based, token-budgeted Markdown
  context with source ranges and related authored links.

Examples:
  openknowledge get README.md
  openknowledge get accessibility --info
  openknowledge get accessibility
  openknowledge get accessibility review
  openknowledge get accessibility agents/review.md
`
}

func searchHelpText() string {
	return fmt.Sprintf(`openknowledge search

Build source-grounded Markdown context from an Open Knowledge bundle.

Usage:
  openknowledge search <name|path> <query>
  openknowledge search <name|path> <query> --budget <tokens>
  openknowledge search <name|path> <query> --format json
  openknowledge search <name|path> <query> --matches
  openknowledge search <name|path> <query> --no-expand
  openknowledge search <name|path> <query> --limit <count>
  openknowledge search <name|path> <query> --spec <version>
  openknowledge search --all <query>
  openknowledge search --all <query> --matches --format json
  openknowledge search --help

Arguments:
  name|path      Registry key or local bundle path.
  query          Search text. Quote multi-word queries in shells.

Flags:
  --budget       Approximate context token budget. Defaults to %d.
                 Context mode only; cannot be combined with --matches.
  --all          Search every registry entry and fuse per-bundle ranks.
  --format       Output format: markdown or json. Defaults to markdown.
  --limit        Maximum context source or match count. Defaults to 12.
  --matches      Print ranked match diagnostics instead of packed context.
  --no-expand    Exclude structural document, outgoing-link, and backlink context.
  --spec         OKF spec version. Defaults to latest.

Behavior:
  Search builds Markdown chunks from parsed heading sections, preserves source
  line ranges and heading paths, scores chunks with BM25-style lexical ranking
  across title, path, type, description, frontmatter, headings, and body text,
  then packs original Markdown under the requested token budget. Fuzzy and
  diacritic-insensitive matching are enabled for local CLI search.

  Direct evidence is packed first. By default, remaining budget can include
  parent/child document context, one-hop outgoing local links, and backlinks
  with their relation. Use --no-expand for direct lexical matches only, or
  --matches to inspect scores, matched fields, snippets, and relations instead
  of context Markdown.

  Both output modes identify the indexed Markdown revision and give every
  section a content-addressed locator so stored citations can detect refreshes.

  --all uses reciprocal-rank fusion with rank constant 60 because BM25 scores
  from different bundle corpora are not directly comparable. Budget and limit
  are global. One broken bundle is reported without hiding healthy results.

Examples:
  openknowledge search Wiki "validation workflow"
  openknowledge search personal "release checklist" --budget 1200
  openknowledge search personal "MCP auth" --matches
  openknowledge search personal "MCP auth" --no-expand
  openknowledge search personal "MCP auth" --format json

Versions:
  %s
`, okf.DefaultContextBudget, supportedSpecVersionsText())
}

func disconnectHelpText(command string) string {
	return fmt.Sprintf(`%s

Remove a knowledge bundle connection from the user registry.

Usage:
  %[1]s <key|path>
  %[1]s <key|path> --keep-files
  %[1]s <key|path> --delete-files
  %[1]s --help

Arguments:
  key|path        Connection key or connected local path.

Flags:
  --keep-files    Keep files after removing the connection. This is the default.
  --delete-files  Delete the complete cache only for CLI-managed remote sources.

Examples:
  %[1]s accessibility
  %[1]s ./project-memory --keep-files
`, command)
}

func connectHelpText(command string) string {
	return fmt.Sprintf(`%s

Connect an Open Knowledge bundle to the user registry.

Usage:
  %[1]s <source>
  %[1]s <source> --as <key>
  %[1]s <source> --access read|write
  %[1]s <source> --no-validate
  %[1]s --help

Arguments:
  source         Local knowledge base root, registry key, Open Knowledge
                 manifest URL, tar archive URL, or Git URL.

Flags:
  --as           Connection key. Defaults to okf_bundle_name, then the folder name.
  --access       Access capability for local connections, read or write. Remote sources are read-only. Defaults to read.
  --git-ref      Git branch, tag, or commit to fetch instead of the remote default.
  --git-subdir   Slash-separated OKF bundle root below a Git repository root.
  --no-validate  Skip the validation status check in the success output.

Remote manifests and tar archives are downloaded into the Open Knowledge cache.
Git sources are cloned into the same cache before registration. Git selectors
are recorded in provenance, included in cache identity, and retained by refresh.
Each Git materialization has a two-minute process budget, disables interactive
credential prompts, and retains at most 256 KiB of subprocess diagnostics.
After every Git step, the staging generation is limited to 100,000 entries,
256 MiB per file, and 2 GiB total before validation, hashing, or publication.
Remote URLs must not embed userinfo or credential query parameters. Git
credentials must resolve through SSH keys or a credential helper; HTTP sources
must be directly accessible without URL-embedded authentication.

Examples:
  %[1]s ./project-memory
  %[1]s ./accessibility --as accessibility
  %[1]s https://openknowledge.sh/wiki/
  %[1]s https://openknowledge.sh/openknowledge-bundle.tar.gz
  %[1]s https://github.com/openknowledge-sh/accessibility.git --as accessibility
  %[1]s https://github.com/example/monorepo.git --git-ref docs-v2 --git-subdir knowledge
  %[1]s ./team-wiki --access write
`, command)
}

func registryHelpText() string {
	return `openknowledge registry

Manage knowledge bundle connections.

Usage:
  openknowledge registry refresh <key|path>
  openknowledge registry refresh <key|path> --force
  openknowledge registry list
  openknowledge registry list --json
  openknowledge registry status [key|path]
  openknowledge registry status [key|path] --json
  openknowledge registry where <name|path>
  openknowledge registry --help

Registry keys are shortcuts for local or cached knowledge bundle paths.
Path-based commands continue to work directly, for example openknowledge list
./project-memory.

Use the canonical top-level openknowledge connect and openknowledge disconnect
commands to mutate the registry.

Examples:
  openknowledge connect ./project-memory --as personal
  openknowledge registry list
  openknowledge registry list --json
  openknowledge registry refresh personal
  openknowledge registry status personal
  openknowledge registry where personal
  openknowledge list personal
`
}

func registryListHelpText() string {
	return `openknowledge registry list

List connected knowledge bases without inspecting their contents.

Usage:
  openknowledge registry list
  openknowledge registry list --json
  openknowledge registry list --help

Flags:
  --json  Print the versioned machine-readable registry inventory.

JSON output uses schemaVersion "1" and includes the registry path, sorted
connection names and paths, effective access, managed state, and source provenance
when present. Use registry status when content health is required.
`
}

func registryRefreshHelpText() string {
	return `openknowledge registry refresh

Fetch and verify a new generation of a managed remote knowledge bundle.

Usage:
  openknowledge registry refresh <key|path>
  openknowledge registry refresh <key|path> --force
  openknowledge registry refresh --help

Flags:
  --force  Discard local changes in the managed cache.

The current generation remains registered until the replacement has been
downloaded, validated, and recorded. Local connections cannot be refreshed.
`
}

func registryStatusHelpText() string {
	return `openknowledge registry status

Check registered bundle and managed-cache integrity without contacting remotes.

Usage:
  openknowledge registry status
  openknowledge registry status [key|path]
  openknowledge registry status [key|path] --json
  openknowledge registry status --help

States:
  ok          Bundle validation and recorded identity pass.
  warnings    Validation passes with warnings.
  unverified  Legacy managed cache has no recorded content identity.
  modified    Content, Git state, or provenance differs from the registry.
  invalid     Bundle validation fails.
  missing     Registered bundle or managed root is unavailable.

The command is offline. It checks local content identity and does not determine
whether a newer remote version exists. JSON output uses schemaVersion "1".
`
}

func registryWhereHelpText() string {
	return `openknowledge registry where

Print the absolute path for a named knowledge base or path.

Usage:
  openknowledge registry where <name|path>
  openknowledge registry where --help

Examples:
  openknowledge registry where personal
  openknowledge registry where ./project-memory
`
}

func exportHelpText() string {
	return fmt.Sprintf(`openknowledge export

Convert an Open Knowledge bundle to another format.

Usage:
  openknowledge export html --out <folder> [path]
  openknowledge export html --plain --out <folder> [path]
  openknowledge export html --no-source-archive --out <folder> [path]
  openknowledge export html --head-file <file> --out <folder> [path]
  openknowledge export html --script-src <src> --out <folder> [path]
  openknowledge export json [path]
  openknowledge export json --out <file> [path]
  openknowledge export tar --out <file> [path]
  openknowledge export graph [path]
  openknowledge export graph --out <file> [path]
  openknowledge export graph --type search [path]
  openknowledge export --help

Targets:
  html       Write a static HTML site. Defaults to the viewer app bundle.
  json       Write normalized bundle JSON.
  tar        Write a portable bundle tar.gz archive.
  graph      Write node and edge graph JSON by graph type.

Flags:
  --spec       OKF spec version. Defaults to latest.
  --out        Output folder for html, optional output file for json/graph, archive file for tar.
  --head-file  Trusted HTML fragment file to inject into default viewer HTML <head>.
  --head-html  Trusted HTML fragment to inject into default viewer HTML <head>.
  --no-source-archive
               Omit the portable source archive and connect manifest from HTML viewer output.
  --script-src Script src to inject into default viewer HTML <head>. May be repeated.

Versions:
  %s
`, supportedSpecVersionsText())
}

func exportHTMLHelpText() string {
	return fmt.Sprintf(`openknowledge export html

Write a static HTML site for an Open Knowledge bundle.

Usage:
  openknowledge export html --out <folder> [path]
  openknowledge export html --plain --out <folder> [path]
  openknowledge export html --no-source-archive --out <folder> [path]
  openknowledge export html --head-file <file> --out <folder> [path]
  openknowledge export html --script-src <src> --out <folder> [path]
  openknowledge export html --spec <version> --out <folder> [path]
  openknowledge export html --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out        Output folder for generated HTML files. Required.
  --head-file  Trusted HTML fragment file to inject into default viewer HTML
                <head>. Defaults to OPENKNOWLEDGE_HEAD_FILE when set.
  --head-html  Trusted HTML fragment to inject into default viewer HTML <head>.
                Defaults to OPENKNOWLEDGE_HEAD_HTML when set.
  --no-source-archive
               Omit the portable source archive and connect manifest.
  --plain      Generate plain semantic HTML without CSS, JavaScript, or viewer chrome.
  --script-src Script src to inject into default viewer HTML <head>. May be
                repeated. Defaults to comma- or newline-separated
                OPENKNOWLEDGE_SCRIPT_SRC when set.
  --spec       OKF spec version. Defaults to latest.

Examples:
  openknowledge export html --head-file ./head.html --out ./site ./project-memory
  openknowledge export html --script-src /analytics.js --out ./site ./project-memory
  openknowledge export html --head-html '<meta name="robots" content="noindex">' --out ./site ./project-memory

Connect:
  Viewer exports include openknowledge.json and assets/openknowledge-bundle.tar.gz
  for remote openknowledge connect unless --no-source-archive is set.

Theme:
  Default viewer exports read [html.theme] from .openknowledge.toml in the
  bundle root. Set stylesheet = "assets/wiki-theme.css" to link theme CSS.
  Built-in variables are defined in viewer_theme.css as --ok-* tokens.

Versions:
  %s
`, supportedSpecVersionsText())
}

func exportJSONHelpText() string {
	return fmt.Sprintf(`openknowledge export json

Write normalized JSON for an Open Knowledge bundle.

Usage:
  openknowledge export json [path]
  openknowledge export json --out <file> [path]
  openknowledge export json --spec <version> [path]
  openknowledge export json --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output file. Defaults to stdout.
  --spec      OKF spec version. Defaults to latest.

Versions:
  %s
`, supportedSpecVersionsText())
}

func exportTarHelpText() string {
	return fmt.Sprintf(`openknowledge export tar

Write a portable tar.gz archive for an Open Knowledge bundle.

Usage:
  openknowledge export tar --out <file> [path]
  openknowledge export tar --spec <version> --out <file> [path]
  openknowledge export tar --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output archive file. Required.
  --spec      OKF spec version. Defaults to latest.

Versions:
  %s
`, supportedSpecVersionsText())
}

func exportGraphHelpText() string {
	return fmt.Sprintf(`openknowledge export graph

Write node and edge graph JSON for an Open Knowledge bundle.

Usage:
  openknowledge export graph [path]
  openknowledge export graph --out <file> [path]
  openknowledge export graph --type source [path]
  openknowledge export graph --type search [path]
  openknowledge export graph --spec <version> [path]
  openknowledge export graph --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output file. Defaults to stdout.
  --spec      OKF spec version. Defaults to latest.
  --type      Graph type: source or search. Defaults to source.

Behavior:
  Source graphs contain one node per parsed bundle file. Edges are deduplicated
  existing local Markdown links and are sourced from the AST-backed parser.

  Search graphs are derivative retrieval artifacts. They include source file
  nodes, Markdown heading chunk nodes, contains edges, chunk reading-order
  edges, and chunk-level local-link edges for graph-expanded search.

Versions:
  %s
`, supportedSpecVersionsText())
}

func rulesHelpText() string {
	return `openknowledge prompt rules

Print maintenance instructions for AI agents.

The command does not edit files. It prints a Markdown block you can paste into
AGENTS.md, CLAUDE.md, Cursor rules, or any project instruction file.
Built-in rules are always available, and local custom rules can be added as
OKF Markdown files under rules/ in the selected wiki.
The selected wiki's .openknowledge.toml may configure [rules].paths for custom
rule directories and [rules].enabled for default selected rules.
It checks the wiki path and prints non-blocking warnings after the rendered
rules when the path does not exist, has no Markdown, or does not validate as
OKF. Each warning includes an agent action. In a terminal warnings print after
the rules on stdout; with pipes or redirection they print to stderr.

Usage:
  openknowledge prompt rules
  openknowledge prompt rules <rules>
  openknowledge prompt rules <rules> --path <path>
  openknowledge prompt rules --target generic|codex|claude|cursor
  openknowledge prompt rules apply <rules> --path <path>
  openknowledge prompt rules --list
  openknowledge prompt rules --help

Arguments:
  rules       Comma-separated maintenance rules to include.
              Defaults to project.

Options:
  --path      Open Knowledge wiki path used in generated rules.
              Defaults to .openknowledge.
  --target    Instruction target: generic, codex, claude, or cursor.
              Defaults to generic.
  --list      List available rules.

Examples:
  openknowledge prompt rules docs,changelog --path Wiki
  openknowledge prompt rules changelog --path Wiki --target codex
  openknowledge prompt rules apply docs,changelog --path Wiki --file AGENTS.md
`
}

func rulesApplyHelpText() string {
	return `openknowledge prompt rules apply

Write generated maintenance instructions into an agent instruction file.

The command updates a managed block between openknowledge:rules markers, so
running it again replaces the previous generated block instead of duplicating it.
It still checks the wiki path and prints non-blocking warnings with agent actions.

Usage:
  openknowledge prompt rules apply
  openknowledge prompt rules apply <rules>
  openknowledge prompt rules apply <rules> --path <path>
  openknowledge prompt rules apply <rules> --path <path> --file <file>
  openknowledge prompt rules apply <rules> --path <path> --dry-run
  openknowledge prompt rules apply <rules> --path <path> --yes
  openknowledge prompt rules apply --help

Arguments:
  rules       Comma-separated maintenance rules to include.
              Defaults to project.

Options:
  --file      Agent instruction file to update.
  --path      Open Knowledge wiki path used in generated rules.
              Defaults to .openknowledge.
  --target    Instruction target: generic, codex, claude, or cursor.
              Defaults to the target inferred from --file when possible.
  --yes       Use the nearest detected agent instruction file without prompting,
              create AGENTS.md when none exists, and skip confirmation.
  --dry-run   Print the managed block that would be written without editing.

Examples:
  openknowledge prompt rules apply docs,changelog --path Wiki --file AGENTS.md
  openknowledge prompt rules apply changelog --path Wiki --yes
  openknowledge prompt rules apply docs --path Wiki --dry-run
`
}

func reviewHelpText() string {
	return `openknowledge prompt review

Print advisory AI review prompts for Open Knowledge workflows.

The command does not call a model, edit files, or decide validation status.
Use openknowledge validate for deterministic CI-safe checks.

Usage:
  openknowledge prompt review rules [path]
  openknowledge prompt review rules --rules <rules> --path <path>
  openknowledge prompt review rules --all [path]
  openknowledge prompt review --help

Subcommands:
  rules      Print an AI review prompt for selected maintenance rules.

Examples:
  openknowledge prompt review rules Wiki
  openknowledge prompt review rules --rules docs,changelog --path Wiki
  openknowledge prompt review rules --all Wiki
`
}

func reviewRulesHelpText() string {
	return `openknowledge prompt review rules

Print an advisory AI review prompt for Open Knowledge maintenance rules.

The prompt tells an agent to inspect evidence, run deterministic validation,
and report source-backed findings. It does not call a model or edit files.

Usage:
  openknowledge prompt review rules [path]
  openknowledge prompt review rules --path <path>
  openknowledge prompt review rules --rules <rules> --path <path>
  openknowledge prompt review rules --all [path]
  openknowledge prompt review rules --help

Arguments:
  path       Open Knowledge wiki path. Defaults to .openknowledge.

Options:
  --path     Open Knowledge wiki path.
  --rules    Comma-separated maintenance rules to review.
             Defaults to [rules].enabled, then project.
  --all      Review every built-in and local custom rule.

Examples:
  openknowledge prompt review rules Wiki
  openknowledge prompt review rules --rules docs,changelog --path Wiki
  openknowledge prompt review rules --all Wiki
`
}

func scaffoldHelpText() string {
	return fmt.Sprintf(`openknowledge scaffold

Scaffold a local Open Knowledge bundle.

Usage:
  openknowledge scaffold [folder]
  openknowledge scaffold --name <name> [folder]
  openknowledge scaffold --spec <version> [folder]
  openknowledge scaffold --bundle-name <id> --bundle-purpose <text> [folder]
  openknowledge scaffold --no-agents --no-setup [folder]
  openknowledge scaffold --help

Arguments:
  folder       Destination folder. Defaults to a slug derived from the name.

Flags:
  --name       Knowledge base name. If omitted, the CLI prompts for one.
  --spec       OKF spec version. Defaults to latest.
  --bundle-name
               Optional stable bundle id written as okf_bundle_name.
  --bundle-title
               Optional display title written as okf_bundle_title.
  --bundle-purpose
               Optional purpose written as okf_bundle_purpose.
  --bundle-tag
               Optional tag written into okf_bundle_tags. Repeatable.
  --bundle-entry
               Optional entrypoint as name=path, for example
               default=agents/checker.md. Repeatable.
  --no-agents
               Do not create AGENTS.md starter agent rules.
  --no-setup
               Do not create SETUP.MD or print the setup handoff prompt.

Examples:
  openknowledge scaffold ./project-memory
  openknowledge scaffold --spec 0.1 ./legacy-wiki
  openknowledge scaffold --no-agents --no-setup ./source-wiki
  openknowledge scaffold --name "Project Memory" ./project-memory
  openknowledge scaffold --name "Accessibility Review" --bundle-name accessibility --bundle-purpose "Accessibility review guidance." --bundle-tag accessibility --bundle-entry default=agents/accessibility-checker.md ./accessibility

Versions:
  %s
`, supportedSpecVersionsText())
}

func viewHelpText() string {
	return `openknowledge view

Start a local HTTP Markdown viewer.

Usage:
  openknowledge view [path]
  openknowledge view --name <alias-name> [path]
  openknowledge view --host <host> --port <port> [path]
  openknowledge view --allow-network --host <host> [path]
  openknowledge view --allow-network --host <host> --token <token> [path]
  openknowledge view --head-file <file> [path]
  openknowledge view --script-src <src> [path]
  openknowledge view --no-browser [path]
  openknowledge view --help

Arguments:
  path         Optional knowledge base root or registry name. When omitted,
               the viewer opens the Open Knowledge Registry workspace selector.

Flags:
  --host       Host to bind. Defaults to 127.0.0.1.
  --port       Port to bind. Defaults to 0, which selects a free port.
  --allow-network
               Permit a non-loopback bind. Every route is then protected by a
               generated token or --token/OPENKNOWLEDGE_VIEW_TOKEN.
  --head-file  Trusted HTML fragment file to inject into <head>. Defaults to
               OPENKNOWLEDGE_HEAD_FILE when set.
  --head-html  Trusted HTML fragment to inject into <head>. Defaults to
               OPENKNOWLEDGE_HEAD_HTML when set.
  --name       Alias name for direct path mode. Defaults to the registry name
               or folder name.
  --no-browser
               Print URLs without opening the default browser.
  --script-src Script src to inject into <head>. May be repeated. Defaults to
               comma- or newline-separated OPENKNOWLEDGE_SCRIPT_SRC when set.
  --token      URL-safe viewer token (16-256 characters). Prefer the
               OPENKNOWLEDGE_VIEW_TOKEN environment variable over command-line
               input when process arguments may be visible to other users.

Examples:
  openknowledge view
  openknowledge view personal
  openknowledge view ./project-memory
  openknowledge view --head-file ./head.html ./project-memory
  openknowledge view --script-src /analytics.js ./project-memory
  openknowledge view --port 8080 ./project-memory
  openknowledge view --name project-memory --port 3000 ./project-memory
  openknowledge view --allow-network --host 0.0.0.0 ./project-memory
`
}

func specHelpText() string {
	return fmt.Sprintf(`openknowledge spec

Print an embedded Open Knowledge Format spec.

Usage:
  openknowledge spec latest|<version>
  openknowledge spec --help

Versions:
  %s

Examples:
  openknowledge spec latest
  openknowledge spec 0.1
`, supportedSpecVersionsText())
}

func validateHelpText() string {
	return fmt.Sprintf(`openknowledge validate

Validate a bundle against an Open Knowledge Format spec.

Usage:
  openknowledge validate [key-or-path]
  openknowledge validate --spec <version> [key-or-path]
  openknowledge validate --format json [key-or-path]
  openknowledge validate --format json --out <file> [key-or-path]
  openknowledge validate --rule <rule=off|warn|error> [key-or-path]
  openknowledge validate --quiet [key-or-path]
  openknowledge validate --help

Arguments:
  key-or-path  Registry key or knowledge base root. Defaults to the current directory.

Flags:
  --spec       OKF spec version. Defaults to latest.
  --format     Output format: text or json. Defaults to text.
  --json       Alias for --format json.
  --out        Write a JSON validation report to a file. Requires JSON output.
  --rule       Override one validation rule severity as rule=off|warn|error.
               May be repeated and overrides [validation.rules] config.
               The rule must belong to the selected OKF spec version.
  --quiet      Print only validation errors.

Config:
  .openknowledge.toml may define [validation.rules] with rule severities:
    link-target = "error"
    markdown-syntax = "off"

Versions:
  %s

Exit codes:
  0            Validation passed, with or without warnings.
  1            Validation found errors after configured severity overrides.
  2            Usage or setup error.
`, supportedSpecVersionsText())
}

func listHelpText() string {
	return fmt.Sprintf(`openknowledge list

Print a bundle tree with inline validation issues.

Usage:
  openknowledge list [key-or-path]
  openknowledge list --spec <version> [key-or-path]
  openknowledge list --depth <n> [key-or-path]
  openknowledge list --json [key-or-path]
  openknowledge list --help

Arguments:
  key-or-path  Registry key or knowledge base root. Defaults to the current directory.

Flags:
  --spec       OKF spec version. Defaults to latest.
  --depth      Maximum tree depth. Defaults to 0 for unlimited depth.
  --json       Print machine-readable inventory JSON.

Versions:
  %s
`, supportedSpecVersionsText())
}

func versionHelpText() string {
	return `openknowledge version

Print the CLI version.

Usage:
  openknowledge version
  openknowledge version --help
`
}

func supportedSpecVersionsText() string {
	return "latest, " + strings.Join(okf.SupportedSpecVersions(), ", ")
}
