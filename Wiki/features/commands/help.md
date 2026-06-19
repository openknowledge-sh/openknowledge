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
the command-specific help convention. Unknown commands print the root usage to
stderr and exit with status `2`.

## Use Cases

* Discover the current command surface.
* Verify examples after adding or changing a command.
* Give agents a stable entry point before setup.

## Source Anchors

* `packages/cli/cmd/openknowledge/main.go`
* `README.md`

## Update Notes

When adding commands, flags, or examples, update root help, command-specific
help, README command tables, and this wiki.
