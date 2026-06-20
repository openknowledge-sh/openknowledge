---
type: Command Documentation
title: openknowledge registry
description: Manages local knowledge bundle connections.
tags: [openknowledge, cli, command, registry]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge registry`

`openknowledge registry` manages local knowledge bundle connections. Registry
keys are shortcuts only; commands still work with normal filesystem paths. The
local viewer also uses the registry as its default workspace list when
`openknowledge open` is run without a path.

The top-level [openknowledge connect](connect.md) and
[openknowledge disconnect](disconnect.md) commands are aliases for
`openknowledge registry connect` and `openknowledge registry disconnect`.
Lookup lives under the same namespace as `openknowledge registry where`.

## Usage

```sh
openknowledge registry connect <path>
openknowledge registry connect <path> --as <key>
openknowledge registry connect <path> --access read|write
openknowledge registry connect <path> --no-validate
openknowledge registry disconnect <key|path>
openknowledge registry disconnect <key|path> --keep-files
openknowledge registry disconnect <key|path> --delete-files
openknowledge registry list
openknowledge registry where <name|path>
openknowledge registry --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `connect` | subcommand | Add or update a local bundle connection. |
| `disconnect` | subcommand | Remove a local bundle connection. |
| `list` | subcommand | Print registered knowledge bases. |
| `where` | subcommand | Print the absolute path for a registry key or path. |
| `key` | argument | Connection key using letters, numbers, dots, underscores, or dashes. |
| `path` | argument | Knowledge bundle folder path. |
| `--as` | flag | Explicit connection key for `connect`. |
| `--access` | flag | Access label stored with a connection, `read` or `write`. |
| `--no-validate` | flag | Skip validation status output for `connect`. |
| `--keep-files` | flag | Keep bundle files after `disconnect`; this is the default. |
| `--delete-files` | flag | Delete files only when the entry is marked `managed`. |

## Behavior

Registry entries are stored as JSON under the user config directory at
`openknowledge/registry.json`. Set `OPENKNOWLEDGE_REGISTRY_FILE` to use a
specific registry file, which is useful in tests and isolated agent runs.

`registry connect` expands `~`, resolves the target to an absolute path,
requires it to be an existing directory, derives or accepts a key, reads
optional root bundle metadata, writes the registry entry, and prints validation
status unless `--no-validate` is set.

`registry disconnect` removes entries by key or absolute path and keeps files
by default. `--delete-files` is guarded and only applies to future managed
entries.

`registry list` prints the registry file path and sorted entries.

`registry where` prints only an absolute path. If the value looks like a path,
it is expanded and normalized directly. Otherwise it must match a registry key.

## Use Cases

* Give shared or standalone wikis stable names.
* Open `openknowledge open` and switch between registered knowledge bases in the
  viewer sidebar.
* Let agents resolve a named wiki before reading files.
* Keep path aliases outside the bundle content.

## Top-Level Aliases

`openknowledge connect` and `openknowledge disconnect` are retained as
convenience aliases for the corresponding `openknowledge registry` subcommands.
They share parsing, output, and exit-code behavior.

`openknowledge where` is not part of the command surface. Use
`openknowledge registry where`.

## Connection Semantics

`registry connect` accepts a local folder, derives a key from `--as`,
`openknowledge.toml` `[bundle].name`, or the folder name, resolves the folder
to an absolute path, and writes a registry entry. If the key was implicit and
already points to a different path, it chooses a suffixed key such as
`project-2`. If the key was explicit and collides with another path, it fails.

Registry entries may now include optional `access` and `managed` fields. The
current local connection command stores `access` as `read` or `write` and
leaves `managed` unset. `registry disconnect` removes matching entries by key
or absolute path, keeps files by default, and refuses `--delete-files` for
non-managed entries. `use` resolves the same keys before reading bundle
entrypoint metadata.

## Future Storage Candidate

Future registry storage should be path-keyed instead of name-keyed:

```json
{
  "connections": {
    "/Users/me/.openknowledge/bundles/accessibility": {
      "key": "accessibility",
      "name": "Accessibility Review",
      "access": "read",
      "source": {
        "type": "github",
        "url": "https://github.com/openknowledge-sh/accessibility",
        "ref": "main"
      },
      "managed": true
    }
  }
}
```

Bundle metadata such as purpose, tags, and entrypoints remains in bundle
content under `openknowledge.toml` `[bundle]` and `[bundle.entries]`. A future
registry migration may store local state as path-keyed connections with source
and managed-cache metadata.

## Source Anchors

* `packages/cli/internal/okf/registry.go`
* `packages/cli/cmd/openknowledge/main.go`

## Command Change History

### 2026-06-20

`openknowledge registry connect`, `openknowledge registry disconnect`, and
`openknowledge registry where` replaced the previous `registry add` and
top-level `where` surface. The top-level `connect` and `disconnect` commands
remain as aliases.

Registry entries gained optional `access` and `managed` fields so
`openknowledge connect` can store local connection labels while preserving the
existing `entries` storage shape.

`openknowledge disconnect` shipped as the user-facing removal path for registry
entries, with key/path resolution and guarded managed-file deletion.

`openknowledge use` shipped as the registry-aware agent entrypoint reader for
connected bundles.

## Update Notes

Update this page when registry storage, key validation, output format,
resolution behavior, or connection command aliases change.
