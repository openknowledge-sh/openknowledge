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

The optional target uses the registry-aware key-or-path model. Without a target,
`list` prints the current directory tree.

## Usage

```sh
openknowledge list [key-or-path]
openknowledge list --spec <version> [key-or-path]
openknowledge list --json [key-or-path]
openknowledge list --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `key-or-path` | argument | Registry key or knowledge base root. Defaults to the current directory. |
| `--spec` | flag | OKF spec version. Defaults to latest. |
| `--json` | flag | Print machine-readable inventory entries. |

## Behavior

`list` resolves the optional target the same way as other path-based commands:
path-like values are used as paths, existing local directories win over same
named registry keys, and otherwise a registry key resolves to its stored bundle
path. With no target, the current directory is listed.

Text output prints the bundle tree with validation issues attached to affected
files. JSON output prints the same inventory entries in machine-readable form.

## Use Cases

* Inspect a wiki from the terminal.
* Inspect a connected bundle by registry key without resolving its path first.
* Give agents a compact file inventory before opening files.
* Check validation issues in context.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/list.go`
> * `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> Update this page when tree formatting, JSON fields, validation attachment, or
> sorting behavior changes.
