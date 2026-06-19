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

## Source Anchors

* `packages/cli/internal/okf/registry.go`
* `packages/cli/cmd/openknowledge/main.go`
* `docs/cli.md`

## Update Notes

Update this page when registry storage, name validation, output format, or
resolution behavior changes.
