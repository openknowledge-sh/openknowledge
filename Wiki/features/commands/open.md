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

In direct knowledge base mode, Markdown links open into a horizontally
scrollable stack of panels. The viewer does not switch into a separate
single-page focus mode; the panel stack is the default and only document
browsing layout. The file explorer can be opened from the header and stays open
while selecting files.

## Usage

```sh
openknowledge open [path]
openknowledge open --name <alias-name> [path]
openknowledge open --host <host> --port <port> [path]
openknowledge open --local-domain <domain> [path]
openknowledge open --no-browser [path]
openknowledge open --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `path` | argument | Optional knowledge base root or registry name. When omitted, the viewer uses the Open Knowledge Registry. |
| `--host` | flag | Host to bind. Defaults to `127.0.0.1`. |
| `--port` | flag | Port to bind. Defaults to `0`, which selects a free port. |
| `--name` | flag | Alias name for direct path mode. Defaults to the registry name or folder name. |
| `--local-domain` | flag | Local alias domain to print. Defaults to `open.knowledge`; set it to an empty string to hide the alias URL. |
| `--no-browser` | flag | Print URLs without opening the default browser. |

## URL Output

`Open Knowledge view` is the primary URL and uses the actual listener host,
defaulting to `127.0.0.1`. Direct path mode and single-workspace registry mode
include the alias path in that loopback URL, for example:

```text
Open Knowledge view: http://127.0.0.1:57475/wiki/
```

When `--local-domain` is not empty, the command also prints the
`Open Knowledge alias` line with the configured local domain:

```text
Open Knowledge alias: http://open.knowledge:57475/wiki/
```

The CLI does not create hostname aliases. If the alias URL is unreachable, use
the printed `127.0.0.1` view URL or map the alias hostname to loopback with
`/etc/hosts`, local DNS, or a reverse proxy.

## Use Cases

* Open the registry view and switch between registered knowledge bases from the
  left selector.
* Inspect the wiki locally after setup.
* Review validation warnings alongside the bundle tree.
* Browse local Markdown links as adjacent panels without leaving the current
  context.
* Browse command and feature docs without leaving the repo.

## Source Anchors

* `packages/cli/cmd/openknowledge/viewer.go`
* `packages/cli/cmd/openknowledge/main.go`
* `packages/cli/internal/okf/markdown.go`

## Update Notes

Viewer rendering, routing, validation display, or link rewriting changes should
update this page and [CLI changelog](/changelog/cli.md).
