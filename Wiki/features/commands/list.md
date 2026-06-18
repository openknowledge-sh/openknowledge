---
type: Command Documentation
title: openknowledge list
description: Prints a bundle tree and optional machine-readable inventory.
tags: [openknowledge, cli, command, inventory]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge list`

`openknowledge list` prints the bundle tree with inline validation issues. It
can also emit JSON inventory output for tools and agents.

## Usage

```sh
openknowledge list [path]
openknowledge list --spec <version> [path]
openknowledge list --json [path]
openknowledge list --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `path` | argument | Knowledge base root. Defaults to the current directory. |
| `--spec` | flag | OKF spec version. Defaults to latest. |
| `--json` | flag | Print machine-readable inventory entries. |

## Use Cases

* Inspect a wiki from the terminal.
* Give agents a compact bundle map before opening files.
* Check validation issues in context.

## Source Anchors

* `packages/cli/internal/okf/list.go`
* `packages/cli/cmd/openknowledge/main.go`

## Update Notes

Update this page when tree formatting, JSON fields, validation attachment, or
sorting behavior changes.
