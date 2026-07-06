---
type: Command Documentation
title: openknowledge list
description: Prints a depth-limited bundle tree and optional machine-readable inventory, including assets.
tags: [openknowledge, cli, command, inventory]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge list`

`openknowledge list` prints the bundle tree with inline validation issues. It
includes Markdown documents and non-Markdown assets, and can emit JSON
inventory output for tools and agents.

The optional target uses the registry-aware key-or-path model. Without a target,
`list` prints the current directory tree.

## Usage

```sh
openknowledge list [key-or-path]
openknowledge list --spec <version> [key-or-path]
openknowledge list --depth <n> [key-or-path]
openknowledge list --json [key-or-path]
openknowledge list --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `key-or-path` | argument | Registry key or knowledge base root. Defaults to the current directory. |
| `--spec` | flag | OKF spec version. Defaults to latest. |
| `--depth` | flag | Maximum tree depth. Defaults to `0` for unlimited depth. |
| `--json` | flag | Print machine-readable inventory entries. |

## Behavior

`list` resolves the optional target the same way as other path-based commands:
path-like values are used as paths, existing local directories win over same
named registry keys, and otherwise a registry key resolves to its stored bundle
path. With no target, the current directory is listed.

Text output prints the bundle tree with validation issues attached to affected
Markdown files. Non-Markdown files are listed as `asset` entries. `--depth`
limits how deep the displayed tree expands; folder rows at the depth boundary
are still shown when deeper files exist.

JSON output prints machine-readable inventory entries. With `--depth`, JSON
keeps file entries whose path depth is within the requested limit.

## Use Cases

* Inspect a wiki from the terminal.
* Inspect a connected bundle by registry key without resolving its path first.
* Give agents a compact file inventory before opening files.
* Limit initial exploration with `--depth` before drilling into deeper folders.
* Check validation issues in context.

## Command Change History

### 2026-07-06

`openknowledge list` gained `--depth <n>` and now includes non-Markdown bundle
files as `asset` entries so the command describes the whole knowledge base
structure, not only OKF Markdown documents.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/list.go`
> * `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> Update this page when tree formatting, JSON fields, depth behavior, asset
> inclusion, validation attachment, or sorting behavior changes.
