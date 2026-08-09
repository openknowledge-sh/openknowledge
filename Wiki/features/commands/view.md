---
type: Command Documentation
title: openknowledge view
description: Browse a local or connected knowledge base in the web viewer.
tags: [openknowledge, cli, command, viewer]
timestamp: 2026-08-09T00:00:00Z
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
- Click a rendered Mermaid diagram to open a viewport-filling dialog.
  You can also focus the diagram and press Enter or Space.
  Use the toolbar, wheel, or pinch gesture to zoom.
  Drag the diagram, or use the arrow keys, to pan.
  Select **Fit** to fit the complete diagram in the viewport.
  Select **100%** to center the diagram at its original scale.
  Press Escape to close the dialog.
- The sidebar has separate **Documents**, **Graph**, and **Settings** items.
  **Documents** shows the file tree. **Graph** shows the graph workspace.
  **Settings** opens the viewer preferences.
- Open-beside mode is the default. The link behavior control can select the
  current panel mode.
  The browser stores this selection. Hold Shift during activation to use the
  other mode one time.
  Each panel keeps its own focus and close controls.
  The open file explorer remains visible during link navigation and uses the
  first grid column at `25vw` by default. You can resize it within its minimum
  and maximum limits. The viewer header, note workspace, and horizontal scroll
  rail remain in the second grid column.
- On viewports up to 680 px, sidebar open and close actions have no motion.
  Note stack navigation has no View Transition or fallback panel entrance
  effect. A sidebar link closes the sidebar and shows its destination
  immediately.
- AST-based search uses the same section ranking as
  [`okn search`](search.md). It also uses the same one-level link
  expansion. Results group section matches by document. They can link to and
  highlight the matching text. Search excerpts strip common Markdown markup.
  The viewer adds folder context to `Index` titles.
  The search panel shows progress, result counts, no-result messages,
  and errors. Select **Retry** after an error. Use the arrow keys to select a
  result. Press Enter to open it. Press Shift+Enter to use the other panel
  mode. Press Escape to close search.
- Each note uses a compact header. The full-width **Frontmatter** disclosure is
  immediately below the header and starts collapsed. Select **Frontmatter** to
  expand the full typed YAML table.
  The viewer also provides tag filters, sortable tables, and directory
  breadcrumbs. The **Documents** view expands the active file branch.
  Directory rows can collapse, and the explorer has a **Collapse all** action.
- For OKF 0.2 concepts, the expanded frontmatter area groups derived trust,
  status, freshness, provenance, structured sources, and Attested Computation
  data.
  Source footnotes open the matching structured source entry. The viewer never
  executes declared resources automatically.
- The knowledge graph uses a theme-aware canvas. The left detail panel shows
  only the connection count for the selected or hovered node. The canvas
  `aria-label` keeps the node name and Enter hint.
  On viewports wider than 680 px, the graph workspace fills the viewport below
  the header. The canvas is 380 px high on smaller viewports. On viewports up
  to 680 px, the **Graph settings** disclosure starts collapsed and contains
  all graph controls and actions.
  Drag the canvas to pan. Use the wheel or trackpad to zoom.
  Drag a node to move it. Select **Fit** to show the complete graph.
  Filter notes by title or path. The **Color nodes by folder** control applies
  theme-aware colors. Hovered and selected nodes keep their folder colors.
  Display controls adjust arrows, labels, node size, and link thickness.
  Force controls adjust center, repel, and link forces.
  Select **Pause** to stop graph motion. Select **Resume** to start motion.
  Select **Reset graph** to restore the graph defaults.
  Arrow keys move the selection. Enter opens the selected note.
  The file tree stays in the file explorer.
- Use **Settings** to change browser-local themes, typography, line length,
  contrast, motion, and link settings. **Reset to defaults** restores the viewer
  defaults. The first visit uses the light blue default theme. A saved theme
  preference overrides this default. These preferences do not change source
  Markdown.
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
The viewer truncates long brand names with an ellipsis when header space is
limited.

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
> - `packages/cli/cmd/openknowledge/viewer_assets.go`
> - `packages/cli/cmd/openknowledge/viewer_frontmatter.go`
> - `packages/cli/cmd/openknowledge/viewer_templates.go`
> - `packages/web/src/viewer/`
> - `packages/web/vite.viewer.config.ts`
> - `packages/web/vite.theme.config.ts`
> - `packages/web/scripts/browser.e2e.mjs`
> - `packages/cli/cmd/openknowledge/viewer_test.go`
> - `packages/cli/internal/okf/search.go`
>
> **Update notes**
>
> Update this page when viewer flags, routing, authentication, navigation, or
> file-serving behavior changes.
