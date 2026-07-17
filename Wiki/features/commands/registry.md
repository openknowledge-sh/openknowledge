---
type: Command Documentation
title: openknowledge registry
description: Inspects, resolves, and refreshes connected knowledge bases.
tags: [openknowledge, cli, command, registry]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge registry`

The registry gives connected bundles stable local names. This namespace owns
management and inspection only. Use the top-level
[`openknowledge connect`](connect.md) and
[`openknowledge disconnect`](disconnect.md) commands to mutate membership.

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
| `list` | List sorted connections and their local paths; JSON includes access, managed state, and provenance. |
| `status` | Verify local bundle, cache, Git, and provenance integrity without contacting remotes. |
| `refresh` | Fetch and validate a new managed remote generation, then switch the connection atomically. |
| `where` | Resolve a key or path to its absolute bundle root. |

`refresh` preserves the recorded Git ref and subdirectory selectors. It stages
downloads or clones separately, validates them, and keeps the previous
generation on failure. `--force` rematerializes even when source identity has
not advanced. Status remains offline and reports source drift, cache integrity,
Git commit/cleanliness, and validation state.

The former `registry connect` and `registry disconnect` subcommands were
removed before 1.0; no compatibility aliases remain.

## Command Change History

### 2026-07-17 - Inspection-only namespace

Removed duplicate connection mutation from `registry`. Top-level `connect` and
`disconnect` are now the only public mutation entry points.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/registry.go`
