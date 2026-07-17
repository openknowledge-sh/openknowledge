---
type: Command Documentation
title: openknowledge disconnect
description: Remove a knowledge-base connection from the local registry.
tags: [openknowledge, cli, command, registry, disconnect]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge disconnect`

Unregister one connected bundle. Files are kept by default.

## Usage

```sh
openknowledge disconnect <key-or-path>
openknowledge disconnect <key-or-path> --keep-files
openknowledge disconnect <key-or-path> --delete-files
```

`--keep-files` and `--delete-files` are mutually exclusive.

`--delete-files` is available only for CLI-managed manifest, archive, or Git
caches. It refuses ordinary local folders. The command verifies that the
recorded managed root belongs directly to the Open Knowledge cache and that the
registered bundle is inside it.

Managed deletion is transactional: the complete cache is renamed to a sibling
tombstone, the registry is updated, and a failed registry write restores the
cache. If final tombstone removal fails, the connection remains removed and
the command exits `1` with a cleanup warning.

Targets may be connection keys or registered paths. Unknown targets fail and
list available keys when possible.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/main.go`
> - `packages/cli/internal/okf/registry.go`
> - `packages/cli/internal/okf/registry_test.go`
