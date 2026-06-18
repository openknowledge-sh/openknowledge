---
type: Command Documentation
title: openknowledge where
description: Resolves a registry name or path to an absolute bundle path.
tags: [openknowledge, cli, command, registry]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge where`

`openknowledge where` prints the absolute path for a registry name or a path.
Agent workflows should use it to resolve a named wiki, then read and edit files
with normal filesystem tools.

## Usage

```sh
openknowledge where <name|path>
openknowledge where --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `name|path` | argument | Registry name or filesystem path to resolve. |

## Use Cases

* Locate a standalone or shared knowledge base by name.
* Normalize relative paths before validation or file reads.
* Fail clearly when a name is unknown.

## Source Anchors

* `packages/cli/cmd/openknowledge/main.go`
* `packages/cli/internal/okf/registry.go`

## Update Notes

Update this page when registry lookup semantics or path detection rules change.
