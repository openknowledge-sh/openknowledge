---
type: Command Documentation
title: openknowledge list
description: Inspect a knowledge-base tree and machine-readable inventory.
tags: [openknowledge, cli, command, inventory]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge list`

Print a bundle tree containing Markdown documents, assets, and validation
context.

## Usage

```sh
openknowledge list [key-or-path]
openknowledge list --depth 2 Wiki
openknowledge list --json Wiki
```

| Option | Default | Description |
| --- | --- | --- |
| `key-or-path` | `.` | Registry key or bundle directory. |
| `--spec <version>` | `latest` | OKF version. |
| `--depth <n>` | `0` | Maximum tree depth; `0` is unlimited. |
| `--json` | off | Print the v1 inventory contract. |

Text output attaches the first validation issue to each affected Markdown file
and labels non-Markdown files as assets. Folders at the depth boundary remain
visible when deeper entries exist.

JSON output contains `schemaVersion`, resolved `root`, and a path-sorted
`entries` array. Each entry retains all validation issues. Its contract is
published as `list.schema.json`.

```text
project-memory/
|-- agents/
|   `-- default.md  Agent Entrypoint  Default Agent Guide
|-- assets/
|   `-- logo.txt  asset
|-- index.md  index
`-- log.md  log
```

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/list.go`
> - `packages/cli/internal/okf/list_types.go`
> - `packages/cli/schemas/v1/list.schema.json`
