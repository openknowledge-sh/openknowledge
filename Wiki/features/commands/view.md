---
type: Command Documentation
title: openknowledge view
description: Browse a local or connected knowledge base in the web viewer.
tags: [openknowledge, cli, command, viewer]
timestamp: 2026-08-24T00:00:00Z
---

# `openknowledge view`

Start the local web viewer. Specify a path or registry key to open one
knowledge base. Omit the target to open all connected knowledge bases in one
registry workspace.

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
- Source and text files open as standard note cards. They use the same
  breadcrumbs, panel controls, syntax highlighting, and open-beside behavior.
  Other assets keep their raw or dedicated media previews.
- Click a rendered Mermaid diagram to open a viewport-filling dialog.
  You can also focus the diagram and press Enter or Space.
  Use the toolbar, wheel, or pinch gesture to zoom.
  Drag the diagram, or use the arrow keys, to pan.
  Select **Fit** to fit the complete diagram in the viewport.
  Select **100%** to center the diagram at its original scale.
  Press Escape to close the dialog.
- The sidebar has separate **Documents**, **Claims**, **Graph**, and
  **Knowledge bases** items. **Documents** shows the active file tree.
  **Claims** shows the typed claim workspace. **Graph** shows the graph
  workspace. **Knowledge bases** lists every connected knowledge base and its
  file tree in registry mode. Use the collapse icon to close all expanded
  trees. Use the plus icon to connect another local folder. **Settings** opens
  the viewer preferences from the sidebar footer.
  Select the shortcut badge in the viewer header to open or close the file
  explorer.
- Open-beside mode is the default. The link behavior control can select the
  current panel mode.
  The browser stores this selection. Hold Shift during activation to use the
  other mode one time.
  Each panel keeps its own focus and close controls.
  The open file explorer remains visible during link navigation and uses the
  first grid column at `25vw` by default. You can resize it within its minimum
  and maximum limits. The viewer header, note workspace, and horizontal scroll
  rail remain in the second grid column.
  With multiple panels open, drag the bottom horizontal scroll thumb to move
  the document stack directly.
- On viewports up to 680 px, sidebar open and close actions have no motion.
  Note stack navigation has no View Transition or fallback panel entrance
  effect. A sidebar link closes the sidebar and shows its destination
  immediately.
- AST-based search uses the same section ranking as
  [`okn search`](search.md). It also uses the same one-level link
  expansion. Results group section matches by document. They can link to and
  highlight the matching text. Search excerpts strip common Markdown markup.
  The search control is on the right side of the viewer header.
  Registry mode searches all connected knowledge bases. Each result shows its
  source knowledge base.
  The viewer adds folder context to `Index` titles.
  The search panel shows progress, result counts, no-result messages,
  and errors. Select **Retry** after an error. Use the arrow keys to select a
  result. Press Enter to open it. Press Shift+Enter to use the other panel
  mode. Press Escape to close search.
- Each note uses a compact header. The full-width **Frontmatter** disclosure is
  immediately below the header and starts collapsed. Select **Frontmatter** to
  expand the full typed YAML table.
  Each note with typed claims also has a collapsed **Claims** panel. The panel
  shows typed statements, status, scope, evidence totals, and occurrence
  relations. It uses ontology labels and keeps technical IDs in claim details.
  For metric claims, the panel shows the quantity-kind label with the value and
  unit.
  A `section_ref` links a claim to its Markdown section. The matching heading
  shows the claim count. Select the marker to open the first bound claim. The
  panel is read-only and does not change canonical YAML.
  When the registry workspace contains multiple knowledge bases, the header
  prefixes the path breadcrumb with the source knowledge-base name. The prefix
  uses the same breadcrumb style and slash separator.
  The viewer also provides tag filters, sortable tables, and directory
  breadcrumbs. The **Documents** view expands the active file branch.
  Directory rows can collapse, and the explorer has a **Collapse all** action.
- Markdown note headers include a **Listen** control when the browser provides
  speech synthesis. Open Knowledge does not require an OpenAI API key, consume
  Codex credits, or make its own speech API request. The browser selects the
  voice; depending on the browser, operating system, and installed voice, its
  speech service may be local or network-backed. Select the control again to
  pause or resume, and select **Stop** to end narration. Starting another note
  stops the current note first.
  Narration reads the rendered reader-facing article. It skips frontmatter,
  agent-only annotations, code blocks, Mermaid diagrams, and hidden controls.
  Browsers without speech synthesis hide the control.
- For OKF 0.2 concepts, the expanded frontmatter area groups derived trust,
  status, freshness, provenance, structured sources, and Attested Computation
  data. Each derived signal uses the same two-column field-and-value rows as
  the frontmatter table. The viewer marks
  inferred `unverified` trust and `stable` status values as **Default**.
  Source footnotes open the matching structured source entry. The viewer never
  executes declared resources automatically.
- The knowledge graph uses a theme-aware canvas. The left detail panel shows
  only the connection count for the selected or hovered node. The canvas
  `aria-label` keeps the node name and Enter hint.
  The graph preserves typed claim occurrence nodes. It shows declaration,
  reference, supersession, contradiction, and derivation relations.
  The top-bar Graph icon toggles between the graph and the open documents.
  On viewports wider than 680 px, the graph workspace fills the viewport below
  the header. The canvas is 380 px high on smaller viewports. On viewports up
  to 680 px, the **Graph settings** disclosure starts collapsed and contains
  graph filters, display and force controls, and reset and animation actions.
  Drag the canvas to pan. Use the wheel or trackpad to zoom. Icon-only zoom,
  fit, and settings controls sit in the canvas's lower-left corner.
  Graph settings are hidden by default. The settings control toggles a
  vertically centered card on the canvas's left.
  Drag a node to move it. Use the fit control to show the complete graph.
  Filter notes by title or path. The **Color nodes by folder** control applies
  theme-aware colors. Hovered and selected nodes keep their folder colors.
  Registry mode combines the graphs from all connected knowledge bases. It
  does not add links between knowledge bases. File-name labels use the source
  knowledge-base color. The file nodes keep their current graph colors.
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
  The **Horizontal stack** switch is enabled by default in the **Document**
  section. Disable it to replace the current document when following a link.
- The browser assigns a color to each connected knowledge base. Use the color
  control beside its name to change the color. The browser saves this choice
  locally and reuses it in search results and graph labels.
- Direct paths and writable local connections provide local editor links.
  Static exports use configured repository source links.
- The local viewer serves the same Vite-built JavaScript and CSS bundle as
  static exports. Local routes supply live API data. Static pages load one
  shared generated data file.
- The local viewer watches visible Markdown and asset files. It refreshes the
  open page after an add, update, rename, move, or deletion.
  One save burst causes one refresh after a short delay.
  Existing open paths, the active path, the graph view, and scroll positions
  survive when their targets still exist.
  The viewer removes deleted open paths before refresh.
  Static HTML exports do not include this live reload client.

## Claims workspace

Select **Claims** to browse all typed claim occurrences in the active knowledge
base. Registry mode combines claims and identifies each source knowledge base.

Filter claims by status, entity, predicate, owner, evidence stance, document,
or validation state. Select a claim to inspect its typed value, evidence,
provenance, time, and local relationships. This workspace is read-only.
Select **Relationships** to inspect authored relations around the selected
claim.
For metric claims, **Browse** and **Relationships** show the quantity-kind label
with the value and unit.
The workspace uses compact spacing in its header, filters, claim rows,
**Browse**, and **Relationships**.

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

The live reload event stream uses the same authentication. Events contain a
content revision and optional knowledge-base aliases. They do not contain file
contents or filesystem paths.

Raw routes serve only regular non-Markdown bundle assets. They exclude
dotfiles, `.git`, `.openknowledge.toml`, legacy `openknowledge.toml`, and symlinks. Markdown and asset
resolution cannot leave the bundle root.

The viewer inserts trusted head fragments in their original form. Use only
content that you control.

In registry mode, the viewer rebuilds routes after the validated registry
snapshot changes. It refreshes content-hashed search indexes after source
edits. A registry or fingerprint failure returns an error. The viewer does not
serve stale or partly trusted state.

The registry workspace can connect an existing local knowledge base. The
connection is read-only unless you enable editor links. If the selected folder
is not a knowledge base, the viewer shows the `okn setup` command that creates
one. Direct-path and static-export views do not provide the registry connection
control.

Theme and source-link configuration comes from
[`.openknowledge.toml`](/features/configuration.md). For deployment, use the
[HTML exporter](/features/exporters/html.md).

Markdown `agent-context` annotations remain visible in the document flow. A
subdued text color distinguishes their content from reader content. Their
content remains in canonical source and the AST, but does not enter ordinary
reader search. The viewer also accepts the legacy agent-maintenance footer
marker.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/viewer.go`
> - `packages/cli/cmd/openknowledge/viewer_live_reload.go`
> - `packages/cli/cmd/openknowledge/viewer_assets.go`
> - `packages/cli/cmd/openknowledge/viewer_frontmatter.go`
> - `packages/cli/cmd/openknowledge/viewer_claims.go`
> - `packages/cli/cmd/openknowledge/viewer_export.go`
> - `packages/cli/cmd/openknowledge/viewer_templates.go`
> - `packages/web/src/viewer/`
> - `packages/web/vite.viewer.config.js`
> - `packages/web/vite.viewer-live-reload.config.js`
> - `packages/web/vite.theme.config.js`
> - `packages/web/scripts/browser.e2e.mjs`
> - `packages/web/scripts/viewer-live-reload.e2e.mjs`
> - `packages/cli/cmd/openknowledge/viewer_test.go`
> - `packages/cli/cmd/openknowledge/viewer_live_reload_test.go`
> - `packages/cli/internal/okf/search.go`
>
> **Update notes**
>
> Update this page when viewer flags, routing, authentication, navigation, or
> file-serving behavior changes.
