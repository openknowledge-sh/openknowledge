---
type: Command Documentation
title: openknowledge view
description: Browse a local or connected knowledge base in the web viewer.
tags: [openknowledge, cli, command, viewer]
timestamp: 2026-07-30T00:00:00Z
---

# `openknowledge view`

Start the local web viewer. Specify a path or registry key to open one
knowledge base. Omit the target to open the registry workspace selector.

## Usage

```sh
okn view [key-or-path]
okn view --no-browser Wiki
okn view --host 127.0.0.1 --port 8080 Wiki
okn view --allow-network --host 0.0.0.0 Wiki
```

| Option | Default | Description |
| --- | --- | --- |
| `--host <host>` | `127.0.0.1` | Listener host. |
| `--port <port>` | free port | Listener port. |
| `--allow-network` | off | Permit a non-loopback bind and require authentication. |
| `--token <token>` | environment/generated | Network access token. Prefer `OPENKNOWLEDGE_VIEW_TOKEN`. |
| `--name <name>` | registry/folder name | Alias used for a direct path. |
| `--no-browser` | off | Print the URL. Do not open a browser. |
| `--head-file <file>` | environment | Trusted HTML injected into every page head. |
| `--head-html <html>` | environment | Trusted inline HTML injected into every page head. |
| `--script-src <src>` | environment | Trusted script URL. Repeatable. |

Head injection also reads `OPENKNOWLEDGE_HEAD_FILE`,
`OPENKNOWLEDGE_HEAD_HTML`, and `OPENKNOWLEDGE_SCRIPT_SRC`.

## Viewer features

- The viewer renders Markdown and follows local links. It renders fenced
  `mermaid` blocks as diagrams. It also shows note panels, source graphs,
  validation context, highlighted assets, and media or PDF previews.
- The link behavior control is next to viewer settings. Open-beside mode is the
  default. The control can select the current panel mode.
  The browser stores this selection. Hold Shift during activation to use the
  other mode one time.
  Each panel keeps its own focus and close controls.
  The open file explorer remains visible during link navigation and uses the
  first grid column at `25vw` by default. You can resize it within its minimum
  and maximum limits. The viewer header and note workspace remain in the
  second grid column.
- AST-based search uses the same section ranking as
  [`okn search`](search.md). It also uses the same one-level link
  expansion. Results group section matches by document. They can link to and
  highlight the matching text.
- The viewer shows typed YAML frontmatter, tag filters, sortable tables, and
  directory breadcrumbs. The file explorer expands the active file branch.
  Directory rows can collapse, and the explorer has a **Collapse all** action.
- For OKF 0.2 concepts, the viewer derives trust, status, and freshness. It
  shows generated and verified provenance. Source footnotes open the matching
  structured source entry. Attested Computation pages show their runtime,
  parameters, computation, executor receipt fields, and attester contract.
  The viewer never executes these resources automatically.
- The knowledge graph reports the selected note and its connection count.
  A left detail panel keeps this information outside the graph canvas.
  The file tree stays in the file explorer. Arrow keys move the selection.
  Enter opens the selected note.
- You can change browser-local themes, typography, line length, contrast,
  motion, and link settings. **Reset to defaults** restores the viewer
  defaults. These preferences do not change source Markdown.
- Direct paths and writable local connections provide local editor links.
  Static exports use configured repository source links.
- The local viewer serves the same Vite-built JavaScript and CSS bundle as
  static exports. Local routes supply live API data. Static pages load one
  shared generated data file.

| Shortcut | Action |
| --- | --- |
| `⌘K` / `Ctrl+K` | Focus search. |
| `⌘⌥S` / `Ctrl+Alt+S` | Toggle the file explorer. |
| `⌘⌥W` / `Ctrl+Alt+W` | Close the focused note. |

The viewer selects the displayed brand from root metadata in this order:
`okf_bundle_title`, `okf_bundle_name`, `title`, then the first H1.

## Network and file safety

Loopback mode does not require a token. A non-loopback or wildcard bind
requires
`--allow-network`. All routes then use token authentication.

The initial URL exchanges the token for an HttpOnly, SameSite cookie. It then
redirects to a clean URL. A remote client can also send
`Authorization: Bearer <token>`.

Raw routes serve only regular non-Markdown bundle assets. They exclude
dotfiles, `.git`, `.openknowledge.toml`, legacy `openknowledge.toml`, and symlinks. Markdown and asset
resolution cannot leave the bundle root.

The viewer inserts trusted head fragments in their original form. Use only
content that you control.

In registry mode, the viewer rebuilds routes after the validated registry
snapshot changes. It refreshes content-hashed search indexes after source
edits. A registry or fingerprint failure returns an error. The viewer does not
serve stale or partly trusted state.

Theme and source-link configuration comes from
[`.openknowledge.toml`](/features/configuration.md). For deployment, use the
[HTML exporter](/features/exporters/html.md).

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/viewer.go`
> - `packages/cli/cmd/openknowledge/viewer_frontmatter.go`
> - `packages/cli/cmd/openknowledge/viewer_templates.go`
> - `packages/cli/cmd/openknowledge/viewer_assets/`
> - `packages/web/src/viewer/`
> - `packages/cli/cmd/openknowledge/viewer_test.go`
> - `packages/cli/internal/okf/search.go`
>
> **Update notes**
>
> Update this page when viewer flags, routing, authentication, navigation, or
> file-serving behavior changes.
