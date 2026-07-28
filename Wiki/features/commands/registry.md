---
type: Command Documentation
title: openknowledge registry
description: Inspects, resolves, and refreshes connected knowledge bases.
tags: [openknowledge, cli, command, registry]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge registry`

The registry gives stable local names to connected bundles. Use this namespace
only for management and inspection. Use the top-level
[`openknowledge connect`](connect.md) and
[`openknowledge disconnect`](disconnect.md) commands to change membership.

## Usage

```sh
openknowledge registry list
openknowledge registry list --json
openknowledge registry status [key-or-path]
openknowledge registry status [key-or-path] --json
openknowledge registry refresh <key-or-path>
openknowledge registry refresh <key-or-path> --force
openknowledge registry where <key-or-path>
```

| Subcommand | Effect |
| --- | --- |
| `list` | List sorted connections and their local paths. JSON includes access, managed state, and provenance. |
| `status` | Verify local bundle, cache, Git, and provenance integrity. Do not contact remotes. |
| `refresh` | Fetch and validate a new managed remote generation, then switch the connection atomically. |
| `where` | Resolve a key or path to its absolute bundle root. |

`refresh` preserves the recorded Git ref and subdirectory selectors. It stages
downloads or clones in a separate location. It validates the new generation
before activation. If validation fails, it keeps the previous generation.

`--force` lets refresh discard local changes in the managed cache. `status`
works offline. It reports source drift, cache integrity, Git state, and
validation state.

Open Knowledge removed `registry connect` and `registry disconnect` before
1.0. No compatibility aliases remain.


---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/registry.go`
