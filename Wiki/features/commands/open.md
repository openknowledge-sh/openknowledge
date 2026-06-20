---
type: Command Documentation
title: openknowledge open
description: Starts a local HTTP Markdown viewer for a knowledge base.
tags: [openknowledge, cli, command, viewer]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge open`

`openknowledge open` starts a local HTTP viewer. Without a path, it opens a
registry-backed workspace selector. With a filesystem path or registry name, it
opens that knowledge base directly.

The viewer renders Markdown without frontmatter, rewrites local Markdown links,
shows validation context, supports search, opens linked notes in a horizontal
panel stack, and provides a graph overview when no note panels are open. The
header brand comes from root `index.md` metadata in this order:
`okf_bundle_title`, `okf_bundle_name`, `title`, then the first Markdown `#`
heading.

## Usage

```sh
openknowledge open [path]
openknowledge open --name <alias-name> [path]
openknowledge open --host <host> --port <port> [path]
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
| `--no-browser` | flag | Print URLs without opening the default browser. |

## URL Output

`Open Knowledge view` is the primary URL and uses the actual listener host,
defaulting to `127.0.0.1`. Direct path mode and single-workspace registry mode
include the alias path in that loopback URL, for example:

```text
Open Knowledge view: http://127.0.0.1:57475/wiki/
```

The CLI does not print or configure custom hostname aliases. Use the printed
loopback URL; stable knowledge base names are represented as path segments such
as `/wiki/` or `/personal/`.

## Behavior

* Registry names and normal filesystem paths resolve through the same
  key-or-path model used by other commands.
* `Command+K` on macOS or `Ctrl+K` elsewhere focuses search.
* Markdown tables keep semantic table markup and are enhanced with scrolling,
  filtering, sorting, and row counts when viewer JavaScript is active.
* HTML comments are not rendered as visible text. The
  `<!-- okf-footer: agent-maintenance -->` marker turns the remaining document
  content into a visually subdued maintenance footer.
* Local code and text asset links open escaped syntax-highlighted previews.
  Local PDF, image, audio, and video links are served from bundle-scoped raw
  URLs for the browser's native viewer.
* `[html.theme]` in `openknowledge.toml` applies the same theme name and
  stylesheet behavior as `openknowledge to html`. Local theme stylesheets are
  validated before rendering.
* The local viewer includes editor deeplinks for opening Markdown files in
  installed local editors. Static HTML exports replace that behavior with
  optional GitHub source links.

## Use Cases

* Browse a local or connected knowledge base.
* Inspect validation warnings next to the bundle tree.
* Follow local Markdown links without leaving the current context.
* Search files and rendered content from the top bar.
* Preview bundled source and media assets in the browser.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/viewer.go`
> * `packages/cli/cmd/openknowledge/viewer_app.css`
> * `packages/cli/cmd/openknowledge/viewer_app.js`
> * `packages/cli/cmd/openknowledge/viewer_search.js`
> * `packages/cli/cmd/openknowledge/viewer_theme.go`
> * `packages/cli/cmd/openknowledge/viewer_theme.css`
> * `packages/cli/cmd/openknowledge/viewer_test.go`
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/markdown.go`
> * `packages/cli/internal/okf/markdown_test.go`
>
> **Update notes**
>
> Viewer rendering, routing, validation display, or link rewriting changes should
> update this page and [CLI changelog](/changelog/cli.md).
