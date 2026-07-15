---
type: Command Documentation
title: openknowledge view
description: Starts a local HTTP Markdown viewer for a knowledge base.
tags: [openknowledge, cli, command, viewer]
timestamp: 2026-07-09T00:00:00Z
---

# `openknowledge view`

`openknowledge view` starts a local HTTP viewer. Without a path, it opens a
registry-backed workspace selector. With a filesystem path or registry name, it
opens that knowledge base directly.

The viewer renders Markdown bodies together with a typed, collapsible inspector
for each note's YAML frontmatter, rewrites local Markdown links, shows
validation context, supports search, opens linked notes in a horizontal panel
stack, and provides a graph overview when no note panels are open. The header
brand comes from root `index.md` metadata in this order:
`okf_bundle_title`, `okf_bundle_name`, `title`, then the first parsed Markdown
`#` heading.

Think of `view` as the interactive OKF view. It presents the same bundle as a
file tree, rendered Markdown panels, local search results, validation context,
and a source-link graph. The derivative search graph used for retrieval is
exported with [`openknowledge to graph --type search`](/features/exporters/graph.md).

## Usage

```sh
openknowledge view [path]
openknowledge view --name <alias-name> [path]
openknowledge view --host <host> --port <port> [path]
openknowledge view --head-file <file> [path]
openknowledge view --script-src <src> [path]
openknowledge view --no-browser [path]
openknowledge view --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `path` | argument | Optional knowledge base root or registry name. When omitted, the viewer uses the Open Knowledge Registry. |
| `--host` | flag | Host to bind. Defaults to `127.0.0.1`. |
| `--port` | flag | Port to bind. Defaults to `0`, which selects a free port. |
| `--head-file` | flag | Trusted HTML fragment file to inject into every viewer page `<head>`. Defaults to `OPENKNOWLEDGE_HEAD_FILE` when set. |
| `--head-html` | flag | Trusted inline HTML fragment to inject into every viewer page `<head>`. Defaults to `OPENKNOWLEDGE_HEAD_HTML` when set. |
| `--name` | flag | Alias name for direct path mode. Defaults to the registry name or folder name. |
| `--no-browser` | flag | Print URLs without opening the default browser. |
| `--script-src` | flag | Script `src` to inject into every viewer page `<head>`. May be repeated and defaults to comma- or newline-separated `OPENKNOWLEDGE_SCRIPT_SRC` when set. |

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

## Example Output

`openknowledge view --no-browser ./project-memory` starts a long-running local
server and prints the URL plus direct-mode details:

```text
Open Knowledge view: http://127.0.0.1:57475/project-memory/
root /work/project-memory
Press Ctrl+C to stop.
```

Running `openknowledge view` without a path starts the registry workspace
selector instead:

```text
Open Knowledge view: http://127.0.0.1:57475/
registry /home/user/.config/openknowledge/registry.json
knowledge bases 2
Press Ctrl+C to stop.
```

## Behavior

* Registry names and normal filesystem paths resolve through the same
  key-or-path model used by other commands.
* `Command+K` focuses search. `Ctrl+K` is still accepted as a non-macOS
  fallback, but the visible search shortcut stays `⌘K`.
* Top-bar search opens ranked default files when focused, supports Arrow-key
  navigation and Enter activation, and dismisses its results and query on
  Escape, outside pointer interaction, or focus moving elsewhere. Results use a
  clear title, path/type metadata, and optional snippet hierarchy.
* `Command+Option+S` toggles the file explorer sidebar. `Ctrl+Alt+S` is still
  accepted as a non-macOS fallback, but the shortcut shown beside the file
  explorer button stays `⌘⌥S`. The sidebar shortcut is ignored while focus is in
  editable controls.
* `Command+Option+W` closes the focused note panel. `Ctrl+Alt+W` is still
  accepted as a non-macOS fallback. The close button exposes the shortcut in
  its hover/focus tooltip, and after a panel closes, focus moves to the previous
  panel when one exists.
* The local search API returns `highlightText` and `highlightURL` when a result
  has a reliable visible text match. `highlightURL` points at the Markdown file
  with `?ok-highlight=<text>`, and the viewer opens, scrolls to, and marks the
  first matching text in the active note panel. This deep-link contract is for
  the local viewer; static HTML exports keep their existing search links.
* Markdown tables keep semantic table markup and are enhanced with scrolling,
  filtering, sorting, and row counts when viewer JavaScript is active.
* Notes with YAML frontmatter show a collapsed-by-default, per-note collapsible
  metadata inspector above the Markdown body. Values use content-aware
  presentations without datatype badges: booleans retain a state treatment,
  simple lists render as chips, and nested lists and maps render recursively.
  Top-level `tags` chips are navigable facets: selecting one opens the existing
  search surface with exact same-tag matches from other notes, rather than
  fuzzy body-text matches.
  The inspector consumes the same typed YAML mapping as the shared OKF parser,
  so valid nested mappings and sequences, flow collections, and block scalars
  retain their structure and content. Invalid frontmatter is surfaced by
  validation without hiding the Markdown body.
* HTML comments are not rendered as visible text. The
  `<!-- okf-footer: agent-maintenance -->` marker turns the remaining document
  content into a visually subdued maintenance footer.
* Local code and text asset links open escaped syntax-highlighted previews.
  Local PDF, image, audio, and video links are served from bundle-scoped raw
  URLs for the browser's native viewer. Raw and Markdown path resolution rejects
  traversal and every symbolic link below the resolved bundle root.
* Asset pages and `/raw/` serve only regular non-Markdown bundle assets. Dotfile
  paths, `.git`, `openknowledge.toml`, Markdown source files, missing paths, and
  non-regular entries are not exposed or listed as assets. Raw content accepts
  only `GET` and `HEAD` and sends `nosniff`, no-referrer, and sandboxed content
  policy headers.
* `[html.theme]` in `openknowledge.toml` applies the same theme name and
  stylesheet behavior as `openknowledge to html`. Local theme stylesheets are
  validated before rendering and cannot be symbolic links.
* Trusted head injection is intended for local analytics, verification meta
  tags, or small loader scripts. Inline HTML and file content are inserted
  without escaping; `--script-src` escapes the attribute value and accepts only
  relative, `http:`, or `https:` URLs.
* The local viewer includes editor deeplinks for opening Markdown files in
  installed local editors. Static HTML exports replace that behavior with
  optional GitHub source links.
* The file explorer sidebar renders folder rows as lightweight bold text
  without filled row blocks, keeping the tree visually quiet while preserving
  file hover states.
* Reserved `index.md` and `log.md` entries show their `system` badge directly
  beside the file name instead of pinning the badge to the far edge of the tree
  row.
* Note paths render as segmented breadcrumbs. Directory segments link to their
  real `index.md` or `index.markdown` document when one exists; missing indexes
  remain plain text. The current-file segment always links to a clean
  single-panel view, so it closes any other open note panels.
* The file viewer header includes a settings menu with five built-in visual
  themes: Night, Light, Paper, Ocean, and Rose, plus a custom theme editor for
  page, surface, text, muted, accent, and border colors. Night is the first-run
  theme when no valid browser-local preference exists; an explicit saved theme
  selection takes precedence on later visits. The same system-level menu
  includes `Show frontmatter`, font, text size, line spacing, motion, readable
  line length, high contrast, and link-underlining controls. These choices
  affect the viewer presentation, never the authored Markdown or editor
  deeplinks. Theme, frontmatter, and accessibility choices are browser-local
  and persist through `localStorage` with a cookie fallback. `Show frontmatter`
  is enabled by default and controls inspector visibility for every currently
  open and newly opened note panel without expanding it; each inspector remains
  independently collapsible and starts collapsed.

## Use Cases

* Browse a local or connected knowledge base.
* Inspect a note's OKF metadata and nested frontmatter without opening its raw
  Markdown source.
* Inspect validation warnings next to the bundle tree.
* Follow local Markdown links without leaving the current context.
* Search files and rendered content from the top bar.
* Let an agent search `/api/search`, navigate a browser to `highlightURL`, and
  show the exact matched text in context.
* Inspect the authored source graph as an interactive view of the same OKF
  bundle that can also be exported with `openknowledge to graph`.
* Preview bundled source and media assets in the browser.
* Inject trusted custom `<head>` snippets that match the web deploy contract.

## Command Change History

### 2026-07-15 - Bundle-asset-only raw serving

Viewer asset pages and `/raw/` no longer act as general file reads over the
selected directory. They reject private/config paths and Markdown sources,
hide private assets from the tree, require regular bundle-contained files, and
limit raw requests to `GET`/`HEAD` with defensive response headers. Source
anchors: `packages/cli/cmd/openknowledge/viewer.go` and
`packages/cli/cmd/openknowledge/viewer_test.go`.

### 2026-07-15 - Complete YAML frontmatter inspector

The frontmatter inspector now uses the shared complete YAML parser. Valid block
scalars, flow mappings, and nested collections render from typed data instead
of falling back because of parser-subset limitations.

### 2026-07-06

`openknowledge view` replaced the previous viewer command name as the clean
pre-1.0 API. The command owns the interactive local application, while
[`openknowledge get`](get.md) owns exact Markdown retrieval and
[`openknowledge list`](list.md) owns structure inspection.

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
> * `packages/cli/internal/okf/search.go`
> * `packages/cli/internal/okf/search_types.go`
> * `packages/cli/internal/okf/markdown.go`
> * `packages/cli/internal/okf/markdown_test.go`
> * `packages/cli/internal/okf/frontmatter_yaml.go`
>
> **Update notes**
>
> Viewer rendering, routing, validation display, or link rewriting changes should
> update this page and [CLI changelog](/changelog/cli.md).
