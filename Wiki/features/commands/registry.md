---
type: Command Documentation
title: openknowledge registry
description: Manages named local paths for knowledge bases.
tags: [openknowledge, cli, command, registry]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge registry`

`openknowledge registry` stores local aliases for knowledge base paths. Names
are shortcuts only; commands still work with normal filesystem paths. The local
viewer also uses the registry as its default workspace list when
`openknowledge open` is run without a path.

This is the shipped low-level command. Normal local connection workflows should
use [openknowledge connect](connect.md) and
[openknowledge disconnect](disconnect.md), while `registry add` remains the
internal persistence primitive and compatibility surface.

## Usage

```sh
openknowledge registry list
openknowledge registry add <name> <path>
openknowledge registry --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `list` | subcommand | Print registered knowledge bases. |
| `add` | subcommand | Add or replace a registry entry. |
| `name` | argument | Alias using letters, numbers, dots, underscores, or dashes. |
| `path` | argument | Knowledge base folder path. |

## Behavior

Registry entries are stored as JSON under the user config directory at
`openknowledge/registry.json`. Set `OPENKNOWLEDGE_REGISTRY_FILE` to use a
specific registry file, which is useful in tests and isolated agent runs.

`registry add` expands `~`, resolves the target to an absolute path, requires it
to be an existing directory, and replaces an existing entry with the same name.
`registry list` prints the registry file path and sorted entries.

## Use Cases

* Give shared or standalone wikis stable names.
* Open `openknowledge open` and switch between registered knowledge bases in the
  viewer sidebar.
* Let agents resolve a named wiki before reading files.
* Keep path aliases outside the bundle content.

## Relationship To Connect And Disconnect

`connect` accepts a local folder, derives a key from `--as`,
`okf_bundle_name`, or the folder name, resolves the folder to an absolute path,
and writes a registry entry. If the key was implicit and already points to a
different path, `connect` chooses a suffixed key such as `project-2`. If the
key was explicit and collides with another path, `connect` fails.

Registry entries may now include optional `access` and `managed` fields. The
current local `connect` command stores `access` as `read` or `write` and leaves
`managed` unset. `disconnect` removes matching entries by key or absolute path,
keeps files by default, and refuses `--delete-files` for non-managed entries.

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
content as `okf_bundle_*` root metadata. A future registry migration may store
local state as path-keyed connections with source and managed-cache metadata.

## Source Anchors

* `packages/cli/internal/okf/registry.go`
* `packages/cli/cmd/openknowledge/main.go`

## Command Change History

### 2026-06-20

Registry entries gained optional `access` and `managed` fields so
`openknowledge connect` can store local connection labels while preserving the
existing `entries` storage shape.

`openknowledge disconnect` shipped as the user-facing removal path for registry
entries, with key/path resolution and guarded managed-file deletion.

## Update Notes

Update this page when registry storage, name validation, output format,
resolution behavior, or the connect/disconnect transition changes.
