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
frontmatter from document pages, rewrites local Markdown links, preserves inline
formatting inside link labels such as code spans, syntax-highlights fenced code
blocks for common languages, and shows validation issues in the index.

The document header brand is the knowledge base display name, not the product
name. It prefers root `index.md` metadata in this order:
`okf_bundle_title`, `okf_bundle_name`, then root index title metadata or the
first H1. If none of those are present, it falls back to `Open Knowledge`.

In direct knowledge base mode, Markdown links open into a horizontally
scrollable stack of panels. The viewer does not switch into a separate
single-page focus mode; the panel stack is the default and only document
browsing layout. A single open panel is exactly centered, keeps symmetric
viewport gutters, and keeps the same top and bottom canvas gaps as multi-panel
stacks. When that lone panel is resized, it grows or shrinks around its center
so both vertical edges move symmetrically. Opening another panel returns the
stack to the left-to-right browsing layout. Multi-panel stacks follow the Andy
Matuschak-style pane pattern: the workspace is the horizontal flex scroll
container and panels keep their own vertical scrolling without showing native
horizontal scrollbars inside individual panels. The viewer adds a custom
always-visible bottom rail for horizontal panel movement; dragging the rail
thumb, clicking the rail track, keyboarding the focused thumb, or dragging the
gray workspace gaps scrolls left or right without taking over text selection
inside note panels. Holding `Space` switches mouse devices into a canvas-style
pan mode, so dragging sideways across panels scrolls the stack without
activating links. Each note panel has left and right resize handles for
horizontal width changes. Resized widths are stored per note and restored when
the note is opened again, while notes without a saved width keep the default
panel size. The resize handles remain aligned with the visible panel edges
while the note content is scrolled vertically. Panels enforce a minimum width so
a note cannot be collapsed into an unreadable strip. The file explorer can be
opened from the header and stays open while selecting files. File rows show
only the filename; reserved Markdown files such as `index.md` and `log.md` are
marked with a right-aligned `system` badge.

When all panels are closed, the empty workspace shows a split overview: a
narrow file tree on the left and a wider connected graph of Markdown files on
the right. The tree uses roughly 30% of the available width on desktop, leaving
the rest for the graph. The graph is built from local Markdown links, uses a
deterministic force layout so well-connected notes cluster together, and
renders into an animated canvas that continues to apply lightweight physics
after the initial layout. Graph labels use smaller sans-serif typography so
node names read more quietly under each note. Hovering or focusing a graph node
expands that node's label, gently pushes nearby nodes away from the active label
with eased damping, keeps non-active nodes in their default visual style, and
highlights the links connected to the active node. Generic `index` labels
include path context such as `commands/index` so nested index files can be
distinguished.

The top bar includes the primary search field, and `Command+K` on macOS or
`Ctrl+K` elsewhere focuses it. In the local server viewer, search uses the
search API; in exported static HTML it searches the embedded static note
manifest in the browser. The result dropdown opens while the search field is
focused, shows top file entries for an empty query, updates in place while
typing, closes after a result is activated, and supports `ArrowDown`, `ArrowUp`,
and `Enter` keyboard selection while keeping focus in the search field.
Reserved `index.md` files remain searchable but rank below comparable regular
pages. The document viewer also keeps a bottom-right
`Powered by OpenKnowledge.sh` link to the project website.

Panel changes use the browser View Transitions API when it is available and a
single CSS entry animation as a fallback when it is not.

When the knowledge base root has `openknowledge.toml` with `[html.theme]`,
the local viewer links the configured stylesheet through the raw file endpoint
so the same theme can be previewed before running `openknowledge to html`.
The built-in theme contract lives in
`packages/cli/cmd/openknowledge/viewer_theme.css`; viewer colors, fonts, and
viewer dimensions derive from its `--ok-*` variables.
The built-in viewer app CSS and JavaScript live in normal asset files next to
the command source (`viewer_app.css`, `viewer_app.js`, and `viewer_search.js`)
and are embedded into the Go binary at build time.

Local links to code and text assets, such as `.go`, `.ts`, `.json`, `.yaml`, or
`.txt` files, open lightweight asset preview pages with escaped source text and
syntax highlighting. Local PDF, image, audio, and video references are served
from a bundle-scoped raw URL so the browser can use its native PDF or media
viewer. Direct `/file/<asset>` URLs also render an asset page; PDF asset pages
embed the raw PDF URL in the browser.

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
* Distinguish reserved system Markdown files in the file explorer without
  duplicating each row's full path.
* Browse local Markdown links as adjacent panels without leaving the current
  context.
* Move across multi-panel stacks with the bottom rail, rail keyboard controls,
  by dragging the workspace gaps, or by holding `Space` and dragging sideways
  across panels on mouse devices.
* Resize note panels horizontally from either edge and keep the customized
  width when reopening the same note.
* Search the knowledge base from the top bar with pointer or keyboard result
  selection.
* Open bundled source files with syntax highlighting and bundled PDFs in the
  browser's native PDF viewer.
* Follow the bottom-right `Powered by OpenKnowledge.sh` attribution to the
  project website.
* Browse command and feature docs without leaving the repo.

## Source Anchors

* `packages/cli/cmd/openknowledge/viewer.go`
* `packages/cli/cmd/openknowledge/viewer_app.css`
* `packages/cli/cmd/openknowledge/viewer_app.js`
* `packages/cli/cmd/openknowledge/viewer_search.js`
* `packages/cli/cmd/openknowledge/viewer_theme.go`
* `packages/cli/cmd/openknowledge/viewer_theme.css`
* `packages/cli/cmd/openknowledge/viewer_test.go`
* `packages/cli/cmd/openknowledge/main.go`
* `packages/cli/internal/okf/markdown.go`
* `packages/cli/internal/okf/markdown_test.go`

## Update Notes

Viewer rendering, routing, validation display, or link rewriting changes should
update this page and [CLI changelog](/changelog/cli.md).
