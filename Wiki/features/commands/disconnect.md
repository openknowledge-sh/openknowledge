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
| `--delete-files` | flag | Delete files only for CLI-managed remote clones. |

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
marked managed. If a CLI-managed clone is removed from the registry but file
deletion fails, the command leaves the registry update in place, prints a
warning, and exits with status `1`.

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
