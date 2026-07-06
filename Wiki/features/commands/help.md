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
openknowledge <command> --help
openknowledge <command> -h
```

Command-specific help also accepts `-help`, because the command dispatcher
recognizes the common Go flag help spelling after a subcommand.

## Behavior

Root help prints the supported command surface, global help flag, examples, and
the command-specific help convention. Its examples include rule generation,
setup rule selection, a minimal `openknowledge new` scaffold, a scaffold with
optional bundle metadata, and a `connect` example that registers the generated
bundle under a stable key.
Unknown commands print the root usage to stderr and exit with status `2`.

## Use Cases

* Discover the current command surface.
* Verify examples after adding or changing a command.
* Give agents a stable entry point before setup.

## Command Change History

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
retrieval command.

Root help added `openknowledge to graph --type search [path]` for derivative
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

Root help added `openknowledge to tar --out <file> [path]` and the `tar`
converter target for portable bundle archives.

Root help added `openknowledge to graph [path]`,
`openknowledge to graph --out <file> [path]`, and the `graph` converter target
for AST-backed link graph JSON.

Root help added the previous query usage, JSON output usage, and a quick
example for query-focused bundle excerpts under the then-current deterministic
read command.

Root help added `openknowledge ast [path]`, file output usage, an `ast` command
summary, and a quick example for printing parsed OKF AST JSON.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `README.md`
>
> **Update notes**
>
> When adding commands, flags, or examples, update root help, command-specific
> help, README command tables, and this wiki.
