---
type: Command Documentation
title: openknowledge disconnect
description: Removes a connected knowledge bundle from the local registry.
tags: [openknowledge, cli, command, registry, disconnect]
timestamp: 2026-06-20T00:00:00Z
---

# `openknowledge disconnect`

`openknowledge disconnect` removes one knowledge bundle connection from the
user registry. It unregisters the connection and keeps files by default.

The shipped local `connect` command creates non-managed connections, so
`disconnect --delete-files` is reserved for future managed remote-cache entries
and refuses to delete ordinary local folders.

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
| `--delete-files` | flag | Delete files only when the registry entry is marked `managed`. |

`--keep-files` and `--delete-files` cannot be used together.

## Behavior

`disconnect` resolves its target against the local registry:

* a non-path value is treated as a connection key;
* a path-like value is expanded, normalized to an absolute path, and matched
  against stored connection paths;
* unknown targets fail clearly and print available keys when the registry has
  entries.

After resolution, the command removes the registry entry and prints the key,
path, and file action. Default and `--keep-files` output uses `files kept`.

`--delete-files` fails before unregistering when the matched entry is not
marked managed. If a future managed entry is removed but file deletion fails,
the command leaves the registry update in place, prints a warning, and exits
with status `1`.

## Quick Examples

```sh
openknowledge disconnect accessibility
openknowledge disconnect ./project-memory --keep-files
```

## Source Anchors

* `packages/cli/cmd/openknowledge/main.go`
* `packages/cli/internal/okf/registry.go`
* `packages/cli/internal/okf/registry_test.go`

## Command Change History

### 2026-06-20

`openknowledge disconnect` shipped with key/path target resolution,
`--keep-files`, guarded `--delete-files`, unknown-target key hints, and
non-managed deletion refusal.

## Update Notes

Update this page when target resolution, registry removal, managed-file
deletion, output, or exit-code behavior changes. CLI behavior changes also
require [CLI changelog](/changelog/cli.md) updates.
