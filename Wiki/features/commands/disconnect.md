---
type: Command Documentation
title: openknowledge disconnect
description: Remove a knowledge-base connection from the local registry.
tags: [openknowledge, cli, command, registry, disconnect]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge disconnect`

Remove one connected bundle from the registry. The command keeps files by
default.

## Usage

```sh
okn disconnect <key-or-path>
okn disconnect <key-or-path> --keep-files
okn disconnect <key-or-path> --delete-files
```

`--keep-files` and `--delete-files` are mutually exclusive.

Use `--delete-files` only for a CLI-managed manifest, archive, or Git cache.
The command rejects an ordinary local folder. It verifies the location of the
managed root. The root must belong directly to the Open Knowledge cache. The
registered bundle must be inside the root.

For managed deletion, the command renames the complete cache to a sibling
tombstone. It then updates the registry. If the registry write fails, the
command restores the cache.

If final tombstone removal fails, the connection stays removed. The command
exits with status `1` and prints a cleanup warning.

A target can be a connection key or registered path. An unknown target stops
the command. When possible, the error lists available keys.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/main.go`
> - `packages/cli/internal/okf/registry.go`
> - `packages/cli/internal/okf/registry_test.go`
