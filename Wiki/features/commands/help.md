---
type: Command Documentation
title: openknowledge --help
description: Discover commands, global flags, and command-specific help.
tags: [openknowledge, cli, command, help]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge --help`

## Usage

```sh
openknowledge --help
openknowledge -h
openknowledge <command> --help
openknowledge jobs <subcommand> --help
openknowledge --error-format json <command> [args...]
```

Command-specific help also accepts `-h` and `-help`. Nested job commands expose
subcommand-specific help; other groups may provide a group overview. The
[command reference](index.md) provides task-oriented behavior and examples.

## Command groups

| Group | Commands |
| --- | --- |
| Create and maintain | `setup`, `agent`, `insights`, `jobs` |
| Use and publish | `get`, `search`, `list`, `view`, `mcp`, `export` |
| Run as a service | `runtime`, `deploy` |
| Validate and connect | `validate`, `connect`, `disconnect`, `registry` |
| Advanced | `scaffold`, `prompt`, `ast`, `spec`, `version` |

Unknown commands print root usage to stderr and exit with status `2`.

The global `--error-format text|json` option must precede the command. JSON mode
wraps failing diagnostics on stderr without changing command stdout or
command-specific semantic JSON. See
[Machine-readable contracts](/features/machine-contracts.md).

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/main.go`
