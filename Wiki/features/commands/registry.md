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

This is the current shipped low-level command. The candidate product direction
is to move normal users to [openknowledge connect](connect.md) and
[openknowledge disconnect](disconnect.md), while keeping the registry as the
internal persistence layer and compatibility surface.

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

## Candidate Direction

The future user-facing model is "connected knowledge bundles" rather than a
manual registry. In that model:

* `openknowledge connect <path-or-url>` adds a local or remote OKF bundle to
  the user's local knowledge registry.
* The registry stores canonical absolute paths so agents can resolve bundles
  from any working directory.
* The canonical storage key is the normalized absolute path; the short command
  key is a user-facing shortcut derived from `okf_bundle_name`, `--as`, or the
  folder/repository basename.
* `openknowledge list` with no arguments lists connected bundles.
* `openknowledge where <key-or-path>` prints the absolute path.
* `openknowledge disconnect <key-or-path>` removes a connection and only
  deletes managed cached files with an explicit deletion flag.

`registry add` currently accepts a name and existing local folder path, resolves
that folder to an absolute path, and writes it to the registry JSON file. That
behavior is still useful as the primitive behind `connect`, but it should not
remain the primary documented workflow once `connect` ships.

## Candidate Storage Model

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
content as `okf_bundle_*` root metadata. The registry stores local state:
absolute path, key, access, source, and whether the files are managed by Open
Knowledge.

## Source Anchors

* `packages/cli/internal/okf/registry.go`
* `packages/cli/cmd/openknowledge/main.go`
* `docs/cli.md`

## Update Notes

Update this page when registry storage, name validation, output format,
resolution behavior, or the connect/disconnect transition changes.
