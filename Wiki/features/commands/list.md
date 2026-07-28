---
type: Command Documentation
title: openknowledge list
description: Inspect a knowledge-base tree and machine-readable inventory.
tags: [openknowledge, cli, command, inventory]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge list`

Print a bundle tree of Markdown documents and assets. The output also contains
validation context.

## Usage

```sh
okn list [key-or-path]
okn list --depth 2 Wiki
okn list --json Wiki
```

| Option | Default | Description |
| --- | --- | --- |
| `key-or-path` | `.` | Registry key or bundle directory. |
| `--spec <version>` | `latest` | OKF version. |
| `--depth <n>` | `0` | Maximum tree depth. `0` is unlimited. |
| `--json` | off | Print the v1 inventory contract. |

Text output shows the first validation issue for each affected Markdown file.
It identifies non-Markdown files as assets. A folder stays visible at the
depth limit when it contains deeper entries.

JSON output contains `schemaVersion`, the resolved `root`, and a path-sorted
`entries` array. Each entry contains all validation issues. The
`list.schema.json` file defines the contract.

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
