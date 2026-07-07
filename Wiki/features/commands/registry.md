---
type: Command Documentation
title: openknowledge registry
description: Manages knowledge bundle connections.
tags: [openknowledge, cli, command, registry]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge registry`

`openknowledge registry` manages knowledge bundle connections. Registry
keys are shortcuts only; commands still work with normal filesystem paths. The
local viewer also uses the registry as its default workspace list when
`openknowledge view` is run without a path.

The top-level [openknowledge connect](connect.md) and
[openknowledge disconnect](disconnect.md) commands are aliases for
`openknowledge registry connect` and `openknowledge registry disconnect`.
Lookup lives under the same namespace as `openknowledge registry where`.

## Usage

```sh
openknowledge registry connect <source>
openknowledge registry connect <source> --as <key>
openknowledge registry connect <source> --access read|write
openknowledge registry connect <source> --no-validate
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
| `connect` | subcommand | Add or update a local or remote bundle connection. |
| `disconnect` | subcommand | Remove a local bundle connection. |
| `list` | subcommand | Print registered knowledge bases. |
| `where` | subcommand | Print the absolute path for a registry key or path. |
| `key` | argument | Connection key using letters, numbers, dots, underscores, or dashes. |
| `source` | argument | Knowledge bundle folder path, existing registry key, Open Knowledge manifest URL, tar archive URL, or Git URL. |
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
status unless `--no-validate` is set. For remote sources, it first materializes
the source into the Open Knowledge cache and stores source metadata on the
connection. HTTP(S) sources try Open Knowledge manifests, direct tar archives,
then Git fallback.

`registry disconnect` removes entries by key or absolute path and keeps files
by default. `--delete-files` is guarded and only applies to CLI-managed remote
clones.

`registry list` prints the registry file path and sorted entries.

`registry where` prints only an absolute path. If the value looks like a path,
it is expanded and normalized directly. Otherwise it must match a registry key.

## Example Output

After connecting a bundle as `personal`, `openknowledge registry list` prints
the registry file and sorted entries:

```text
Open Knowledge Registry
known knowledge bases

config /home/user/.config/openknowledge/registry.json

  personal           /work/project-memory
```

`openknowledge registry where personal` prints only the resolved path, which is
useful for scripts:

```text
/work/project-memory
```

## Top-Level Aliases

`openknowledge connect` and `openknowledge disconnect` are retained as
convenience aliases for the corresponding `openknowledge registry` subcommands.
They share parsing, output, and exit-code behavior.

`openknowledge where` is not part of the command surface. Use
`openknowledge registry where`.

## Storage

Registry storage is path-keyed under `connections`. Entries store the stable
key, display name, optional access label, optional source metadata, and whether
the files are CLI-managed. Local connections normally leave `managed` unset;
remote manifest, tar, and Git connections are marked managed because their
files live in the Open Knowledge cache.

Bundle metadata such as purpose, tags, and entrypoints remains in bundle
content as `okf_bundle_*` root metadata.

Use the registry to give shared or standalone wikis stable names while keeping
aliases outside the bundle content.

## Command Change History

### 2026-06-20

`openknowledge registry connect`, `openknowledge registry disconnect`, and
`openknowledge registry where` replaced the previous `registry add` and
top-level `where` surface. The top-level `connect` and `disconnect` commands
remain as aliases.

Registry storage now writes path-keyed `connections` and records access,
managed-file, and source metadata for local or remote connections.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/registry.go`
> * `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> Update this page when registry storage, key validation, output format,
> resolution behavior, or connection command aliases change.
