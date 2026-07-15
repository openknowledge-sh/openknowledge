---
type: Command Documentation
title: openknowledge disconnect
description: Removes a connected knowledge bundle from the local registry.
tags: [openknowledge, cli, command, registry, disconnect]
timestamp: 2026-06-20T00:00:00Z
---

# `openknowledge disconnect`

`openknowledge disconnect` removes one knowledge bundle connection from the
user registry. It unregisters the connection and keeps files by default. It is
a top-level alias for `openknowledge registry disconnect`.

Local `connect` targets create non-managed connections. Remote manifest, tar,
and Git targets create CLI-managed cache entries. `disconnect --delete-files`
refuses to delete ordinary local folders.

## Usage

```sh
openknowledge disconnect <key-or-path>
openknowledge disconnect <key-or-path> --keep-files
openknowledge disconnect <key-or-path> --delete-files
openknowledge disconnect --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `key-or-path` | argument | Connection key or connected local path. |
| `--keep-files` | flag | Keep bundle files after removing the registry entry. This is the default. |
| `--delete-files` | flag | Delete the complete cache only for CLI-managed remote sources. |

`--keep-files` and `--delete-files` cannot be used together.

## Behavior

`disconnect` resolves its target against the local registry:

* a non-path value is treated as a connection key;
* a path-like value is expanded, normalized to an absolute path, and matched
  against stored connection paths;
* unknown targets fail clearly and print available keys when the registry has
  entries.

Disconnect flags may appear before or after the required `<key-or-path>`
argument. Top-level and `registry disconnect` forms use the same parsing.

After resolution, the command removes the registry entry and prints the key,
path, and file action. Default and `--keep-files` output uses `files kept`.

`--delete-files` requires the recorded managed root to be a direct child of the
Open Knowledge cache and requires the registered bundle path to be inside that
root. This prevents a forged or stale `managed` flag from authorizing deletion
of an arbitrary local folder. Legacy records without `managedRoot` may use the
registered path only when it passes the same cache-boundary check.

Deletion takes the source-specific cache lock, verifies that the registry entry
has not changed, renames the complete cache container to a sibling tombstone,
and removes that exact snapshot from the registry. A registry failure renames
the cache and provenance sidecar back. This removes top-level archive files as
well as a possible nested bundle root while leaving unrelated cache siblings
untouched. If final tombstone deletion fails, the command leaves the registry
update in place, prints a warning, and exits with status `1`.

## Quick Examples

```sh
openknowledge disconnect accessibility
openknowledge disconnect ./project-memory --keep-files
```

## Example Output

```text
OK Disconnected knowledge bundle
key    personal
path   /work/project-memory
files  kept
```

## Command Change History

### 2026-07-15 - Transactional managed-root deletion

`--delete-files` now validates the managed root against the cache boundary,
locks that source, tombstones the entire cache container and provenance sidecar,
and removes only an unchanged registry snapshot. Registry failure rolls the
filesystem move back; nested archive roots no longer leave top-level cache
files behind. Source anchors: `packages/cli/internal/okf/registry.go`,
`packages/cli/internal/okf/registry_test.go`,
`packages/cli/cmd/openknowledge/main.go`, and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Transactional managed-file guard

The `--delete-files` managed check and registry removal now operate on one
locked registry snapshot, preventing a concurrent key or path change from
causing deletion of files that were never approved as CLI-managed. Source
anchors: `packages/cli/internal/okf/registry.go`,
`packages/cli/internal/okf/registry_test.go`, and
`packages/cli/cmd/openknowledge/main.go`.

### 2026-07-15 - Positional-first flags

`openknowledge disconnect` and `openknowledge registry disconnect` now parse
`--keep-files` and `--delete-files` after `<key-or-path>` as documented while
continuing to accept flag-first forms. Source anchors:
`packages/cli/cmd/openknowledge/main.go` and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-06-20

`openknowledge disconnect` became the top-level alias for the registry
`disconnect` subcommand after connection management moved under the registry
namespace.

`openknowledge disconnect` shipped with key/path target resolution,
`--keep-files`, guarded `--delete-files`, unknown-target key hints, and
non-managed deletion refusal.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/registry.go`
> * `packages/cli/internal/okf/registry_test.go`
>
> **Update notes**
>
> Update this page when target resolution, registry removal, managed-file
> deletion, output, or exit-code behavior changes. CLI behavior changes also
> require [CLI changelog](/changelog/cli.md) updates.
