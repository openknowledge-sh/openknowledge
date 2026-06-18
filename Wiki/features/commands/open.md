---
type: Command Documentation
title: openknowledge open
description: Starts a local HTTP Markdown viewer for a knowledge base.
tags: [openknowledge, cli, command, viewer]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge open`

`openknowledge open` starts a local HTTP viewer in the browser. Without a path,
it opens a registry-backed workspace selector. With a path or registry name, it
opens that knowledge base directly. The viewer renders Markdown, strips
frontmatter from document pages, rewrites local Markdown links, and shows
validation issues in the index.

## Usage

```sh
openknowledge open [path]
openknowledge open --host <host> --port <port> [path]
openknowledge open --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `path` | argument | Optional knowledge base root or registry name. When omitted, the viewer uses the Open Knowledge Registry. |
| `--host` | flag | Host to bind. Defaults to `127.0.0.1`. |
| `--port` | flag | Port to bind. Defaults to `0`, which selects a free port. |

## Use Cases

* Open the registry view and switch between registered knowledge bases from the
  left selector.
* Inspect the wiki locally after setup.
* Review validation warnings alongside the bundle tree.
* Browse command and feature docs without leaving the repo.

## Source Anchors

* `packages/cli/cmd/openknowledge/viewer.go`
* `packages/cli/cmd/openknowledge/main.go`
* `packages/cli/internal/okf/markdown.go`

## Update Notes

Viewer rendering, routing, validation display, or link rewriting changes should
update this page and [CLI changelog](/changelog/cli.md).
