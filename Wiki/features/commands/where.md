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

In the candidate connections model, `where` resolves connected bundle keys as
well as paths. It remains the command agents use when they need the real
filesystem root for a connected knowledge base.

## Usage

```sh
openknowledge where <key|path>
openknowledge where --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `key|path` | argument | Registry name, connected bundle key, or filesystem path to resolve. |

## Candidate Connection Resolution

Resolution order:

1. If the value looks like a path, expand `~`, normalize it, and print the
   absolute path.
2. Otherwise, look for a connected bundle key.
3. If the key exists, print the connection's stored absolute path.
4. If no key exists, fail with an unknown-knowledge-base error and print the
   available keys.

The output should be only the absolute path so scripts and agents can use it
directly.

## Use Cases

* Locate a standalone or shared knowledge base by name or connected bundle key.
* Normalize relative paths before validation or file reads.
* Fail clearly when a name is unknown.

## Source Anchors

* `packages/cli/cmd/openknowledge/main.go`
* `packages/cli/internal/okf/registry.go`

## Update Notes

Update this page when registry lookup semantics or path detection rules change.
