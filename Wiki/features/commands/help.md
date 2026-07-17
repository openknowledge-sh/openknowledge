---
type: Command Documentation
title: openknowledge --help
description: Root and command-specific help behavior.
tags: [openknowledge, cli, command, help]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge --help`

The root help command prints supported commands, usage forms, command summaries,
global help flags, and examples. Command-specific help is available through
`openknowledge <command> --help`.

## Usage

```sh
openknowledge --help
openknowledge -h
openknowledge --error-format json <command> [args...]
openknowledge <command> --help
openknowledge <command> -h
openknowledge jobs <subcommand> --help
openknowledge runtime <subcommand> --help
openknowledge deploy railway --help
```

Command-specific help also accepts `-help`, because the command dispatcher
recognizes the common Go flag help spelling after a subcommand.

## Behavior

Root help organizes the supported command surface around creating and
maintaining knowledge, using and publishing it, running it as a service, and
validating or connecting it. It shows only representative end-to-end flows;
command-specific help owns exhaustive flags and variants.
Unknown commands print the root usage to stderr and exit with status `2`.

The root-only `--error-format text|json` option must precede the command. Its
default `text` value preserves human diagnostics. `json` converts a failing
command's diagnostic stderr into the versioned CLI error envelope without
changing command stdout or command-specific JSON result contracts. Semantic
nonzero results that already provide complete machine output, such as an
invalid JSON validation report, are not wrapped as CLI failures.

Nested job commands dispatch help at the subcommand level. For example,
`openknowledge jobs run --help` prints the run-specific flags rather than
the general `jobs` overview. `openknowledge agent --help` and
`openknowledge agent exec --help` describe the multi-harness human-facing
direct and isolated modes separately.

## Example Output

Root help uses progressive disclosure: it names the complete capability groups
and representative commands, while command-specific help owns detailed flags
and variants.

```text
openknowledge builds, uses, and runs self-maintaining OKF knowledge bases.

Usage:
  openknowledge --help
  openknowledge --error-format json <command> [args...]
  openknowledge <command> --help

Create and maintain:
  setup        Create, validate, and integrate a knowledge base with an agent.
  agent        Run, integrate, and review knowledge with an agent.
  jobs         Run repeatable isolated maintenance jobs from Markdown specs.

Use and publish:
  get          Read an exact Markdown file or bundle entrypoint.
  search       Build source-grounded context from one or more knowledge bases.
  list         Inspect knowledge-base structure.
  view         Browse knowledge locally.
  mcp          Connect an MCP client to read-only knowledge tools.
  export       Export HTML, JSON, graph, or portable tar views.

Run as a service:
  runtime      Build, serve, and maintain an isolated knowledge runtime.
  deploy       Provision that runtime on a supported provider.

Validate and connect:
  validate     Validate a bundle against an OKF spec.
  connect      Connect a local or remote knowledge base.
  disconnect   Remove a knowledge-base connection.
  registry     Refresh, inspect, and resolve connected knowledge bases.

Advanced and portable tools:
  scaffold     Create a deterministic local OKF knowledge base.
  prompt       Print or install portable agent instructions.
  ast          Print parsed OKF AST JSON.
  spec         Print an embedded OKF spec.
  version      Print the CLI version.
```
## Use Cases

* Discover the current command surface.
* Verify examples after adding or changing a command.
* Give agents a stable entry point before setup.

## Command Change History

### 2026-07-17 - Workflow consolidation

Root help now presents `setup`, `agent`, and `jobs` as the maintenance workflow,
uses `export` as the only publishing namespace, reserves `connect` and
`disconnect` for registry mutation, and places print-only tools under the
advanced `prompt` namespace. Removed pre-1.0 command forms are not aliases.

The low-level deterministic bundle command is named `scaffold`; the former
top-level `new` name is not retained as an alias.

### 2026-07-17 - Workflow-oriented root help

Root help now presents the CLI through four product workflows instead of an
exhaustive flat usage matrix. Project integration and suggestion review are
nested under `openknowledge agent` without duplicate top-level forms.

### 2026-07-17

Root help now separates the human-facing `agent` command from declarative
`jobs`, exposes `agent exec` and `--isolate`, uses `jobs start` for detached
runs, and removes the former `agents` and `spawn` names without aliases.

### 2026-07-15

Root help now documents global `--error-format text|json`. JSON mode emits a
versioned, bounded command-error envelope on stderr while preserving stdout
contracts and human-readable errors by default. Source anchors:
`packages/cli/cmd/openknowledge/main.go`,
`packages/cli/cmd/openknowledge/main_test.go`, and
`packages/cli/schemas/v1/cli-error.schema.json`.

Root and search help now include `openknowledge search --all <query>` and its
RRF, global budget/limit, local-snapshot, and partial-failure semantics. Source
anchors: `packages/cli/cmd/openknowledge/main.go` and
`packages/cli/cmd/openknowledge/main_test.go`.

Root and registry help now include `openknowledge registry list --json`, and
`registry list --help` has dedicated discovery-contract guidance distinct from
the deeper offline integrity checks in `registry status`. Source anchors:
`packages/cli/cmd/openknowledge/main.go` and
`packages/cli/cmd/openknowledge/main_test.go`.

Root and command-specific help now include `openknowledge mcp [key-or-path]`
and `--spec <version>` for the read-only MCP stdio server. The help records the
protocol revision, exact resources plus search/validation surface, stdout
transport boundary, and 4 MiB resource-read limit. Source anchors:
`packages/cli/cmd/openknowledge/main.go`,
`packages/cli/cmd/openknowledge/mcp.go`, and
`packages/cli/cmd/openknowledge/main_test.go`.

Root help now includes `openknowledge registry refresh <key|path> [--force]`
and a refresh example, keeping the discoverable command surface aligned with
the registry command group. Source anchors:
`packages/cli/cmd/openknowledge/main.go` and
`packages/cli/cmd/openknowledge/main_test.go`.

Nested `openknowledge jobs <subcommand> --help` now dispatches to dedicated
help for `new`, `list`, `status`, `runs`, `spawn`, `stop`, `kill`, `validate`,
`run`, and `daemon`. Source anchors:
`packages/cli/cmd/openknowledge/agents_command.go` and
`packages/cli/cmd/openknowledge/agents_command_test.go`.

### 2026-07-09

Root and command-specific help changed `openknowledge search` to emit a
source-preserving Markdown context packet by default. Search help added
`--budget <tokens>`, `--no-expand`, and `--matches`; changed `--format` to
`markdown|json` with `markdown` as the default; and removed `--expand graph`
and the previous text-as-default description. Help also records the `2400`
default token budget, the `12` source or match limit, and default one-hop local
link and backlink expansion.

### 2026-07-07

Root help marks `openknowledge jobs` as experimental while the local job
schema and scheduler behavior are still settling.

Root help added `openknowledge jobs new` and
`openknowledge jobs new <template> --out <file>` for built-in local agent
job templates and job-file creation.

Root help added `openknowledge jobs list [path]`,
`openknowledge jobs validate <job-or-dir>`,
`openknowledge jobs run <job.md> --dry-run`, and
`openknowledge jobs daemon [jobs-dir] --once` for local scheduled agent job
automation.

Root help added `openknowledge review rules [path]`,
`openknowledge review rules --rules <rules> --path <path>`, and
`openknowledge review rules --all [path]` for advisory AI review prompt
generation.

Command-specific help for `openknowledge rules` and
`openknowledge review rules` now describes `[rules]` defaults from
`openknowledge.toml`.

Root help added `openknowledge from <source> --out <folder>`,
`openknowledge from <source> --out <folder> --type understanding`, and
`openknowledge from <source> --out <folder> --type custom --about <goal>` for
source-to-wiki prompt generation.

Root and command-specific help added
`openknowledge new --no-agents --no-setup [folder]` for scaffolding bundles
that do not need starter agent rules or a setup handoff document.

### 2026-07-06

Root help replaced the previous deterministic read and viewer command names
with `openknowledge get <name|path> [entry-or-file]` and
`openknowledge view [path]`. The old command names are no longer part of the
pre-1.0 command surface.

Root help added `openknowledge list --depth <n> [key-or-path]` for bounded
bundle tree inspection.

Root help added `openknowledge search <name|path> <query>`,
`openknowledge search <name|path> <query> --format json`, and
`openknowledge search <name|path> <query> --expand graph`. It removed
the previous query-mode usage forms and keeps search as the standalone
retrieval command. This was the pre-2026-07-09 search surface.

Root help added `openknowledge export graph --type search [path]` for derivative
search graph exports.

### 2026-07-05

Root help added `openknowledge rules <rules> --path <path>`,
`openknowledge rules apply <rules> --path <path>`,
`openknowledge rules --list`, and `openknowledge setup --rules <rules>` usage
forms with examples for printing, applying, and preselecting agent maintenance
rules.

### 2026-06-28

Root and command-specific help described the previous query mode as a
source-grounded query briefing instead of an excerpt-only mode.

### 2026-06-20

Root help removed top-level `where` and the `registry add` subcommand, added
`openknowledge registry connect`, `openknowledge registry disconnect`, and
`openknowledge registry where`, and reframed `registry` as the
connection-management namespace.

Root help added the previous deterministic entrypoint-loading command summary
and quick examples for inspecting and printing an entrypoint.

Root help added `openknowledge disconnect <key|path>`, a `disconnect` command
summary, and a quick example for removing a connection.

Root help added `openknowledge connect <source>`,
`openknowledge connect <source> --as <key>`, a `connect` command summary, and a
quick example for connecting a bundle with an explicit key.

Root help added `openknowledge export tar --out <file> [path]` and the `tar`
converter target for portable bundle archives.

Root help added `openknowledge export graph [path]`,
`openknowledge export graph --out <file> [path]`, and the `graph` converter target
for AST-backed link graph JSON.

Root help added the previous query usage, JSON output usage, and a quick
example for query-focused bundle excerpts under the then-current deterministic
read command.

Root help added `openknowledge ast [path]`, file output usage, an `ast` command
summary, and a quick example for printing parsed OKF AST JSON.

---

<!-- okf-footer: job-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `README.md`
>
> **Update notes**
>
> When adding commands, flags, or examples, update root help, command-specific
> help, README command tables, and this wiki.
