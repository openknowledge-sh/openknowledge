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
the command-specific help convention. Its examples include a minimal
`openknowledge new` scaffold, a scaffold with optional bundle metadata, and a
`connect` example that registers the generated bundle under a stable key.
Unknown commands print the root usage to stderr and exit with status `2`.

## Use Cases

* Discover the current command surface.
* Verify examples after adding or changing a command.
* Give agents a stable entry point before setup.

## Command Change History

### 2026-06-20

Root help removed top-level `where` and the `registry add` subcommand, added
`openknowledge registry connect`, `openknowledge registry disconnect`, and
`openknowledge registry where`, and reframed `registry` as the
connection-management namespace.

Root help added `openknowledge use <name|path> [entry]`,
`openknowledge use <name|path> --info`, a `use` command summary, and quick
examples for inspecting and printing an entrypoint.

Root help added `openknowledge disconnect <key|path>`, a `disconnect` command
summary, and a quick example for removing a connection.

Root help added `openknowledge connect <source>`,
`openknowledge connect <source> --as <key>`, a `connect` command summary, and a
quick example for connecting a bundle with an explicit key.

Root help added `openknowledge to tar --out <file> [path]` and the `tar`
converter target for portable bundle archives.

Root help added `openknowledge context [name|path] --query <text>`, JSON output
usage, a `context` command summary, and a quick example for query-focused bundle
context.

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
