---
type: Command Documentation
title: openknowledge --help
description: Discover commands, global flags, and command-specific help.
tags: [openknowledge, cli, command, help]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge --help`

Installed releases include the shorter `okn` alias. The documentation uses
this alias in shell examples.

## Usage

```sh
okn --help
okn -h
okn <command> --help
okn automation jobs <subcommand> --help
okn --error-format json <command> [args...]
okn --no-telemetry <command> [args...]
```

Command help also accepts `-h` and `-help`. Nested job commands provide
subcommand help. Other groups can provide a group overview. The
[command reference](index.md) gives task-based behavior and examples.

## Command groups

| Group | Commands |
| --- | --- |
| Start here | `setup`, `search`, `validate`, `view` |
| Trust and govern | `audit`, `claims`, `evidence`, `eval`, `quality` |
| Query and interchange | `query`, `export` |
| Publish and operate | `mcp`, `connect`, `disconnect`, `registry`, `automation` |
| Advanced internals | `agent`, `get`, `list`, `scaffold`, `prompt`, `ast`, `spec`, `version`, `telemetry` |

An unknown command prints root usage to stderr and exits with status `2`.

Put the global `--error-format text|json` option before the command. JSON mode
wraps error diagnostics on stderr. It does not change command stdout or
command-specific JSON. See
[Machine-readable contracts](/features/machine-contracts.md).

Put the global `--no-telemetry` option before the command. This option disables
telemetry for the current command and saves the disabled preference. See
[telemetry](telemetry.md).

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/main.go`
