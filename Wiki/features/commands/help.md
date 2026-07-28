---
type: Command Documentation
title: openknowledge --help
description: Discover commands, global flags, and command-specific help.
tags: [openknowledge, cli, command, help]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge --help`

Installed releases include the shorter `okn` alias. You can use `okn` instead
of `openknowledge` in each example.

## Usage

```sh
openknowledge --help
openknowledge -h
openknowledge <command> --help
openknowledge jobs <subcommand> --help
openknowledge --error-format json <command> [args...]
```

Command help also accepts `-h` and `-help`. Nested job commands provide
subcommand help. Other groups can provide a group overview. The
[command reference](index.md) gives task-based behavior and examples.

## Command groups

| Group | Commands |
| --- | --- |
| Start here | `setup`, `search`, `validate` |
| Maintain and automate | `agent`, `insights`, `jobs` |
| Browse and publish | `get`, `list`, `view`, `mcp`, `export` |
| Connect and operate | `connect`, `disconnect`, `registry`, `runtime`, `deploy` |
| Advanced and portable tools | `scaffold`, `prompt`, `ast`, `spec`, `version` |

An unknown command prints root usage to stderr and exits with status `2`.

Put the global `--error-format text|json` option before the command. JSON mode
wraps error diagnostics on stderr. It does not change command stdout or
command-specific JSON. See
[Machine-readable contracts](/features/machine-contracts.md).

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/main.go`
