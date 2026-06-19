---
type: Changelog
title: CLI Changelog
description: Maintained changelog memory for Open Knowledge CLI package changes.
tags: [openknowledge, cli, changelog]
timestamp: 2026-06-18T00:00:00Z
---

# CLI Changelog

This page records CLI-facing package changes in a developer-focused format.
Entries should summarize what changed, why it matters, source anchors, and docs
that were updated.

## Unreleased

### 2026-06-20 - Viewer app assets split from Go source

* The built-in `openknowledge open` viewer app CSS and JavaScript now live in
  normal source files (`viewer_app.css`, `viewer_app.js`, and
  `viewer_search.js`) instead of large raw string constants in `viewer.go`.
* The files are still embedded into the Go binary at build time, preserving the
  existing single-binary viewer behavior while making syntax highlighting and
  editing practical.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_assets.go`,
  `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_search.js`,
  `packages/cli/cmd/openknowledge/viewer.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer website attribution

* `openknowledge open` document pages and default viewer HTML exports now show
  a bottom-right `Powered by OpenKnowledge.sh` link to the project website.
* The attribution sits alongside the viewer's bottom chrome and shifts with the
  file sidebar so it remains visible without covering the panel scroll rail.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer overview graph spacing

* `openknowledge open` now gives the empty-workspace file tree roughly 30% of
  the desktop overview width, leaving more room for the knowledge graph.
* Knowledge graph labels now use smaller sans-serif typography instead of the
  heavier monospace style, making labels under nodes read more quietly.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer resize handles follow panel scroll

* `openknowledge open` now keeps note panel resize handles aligned with the
  visible panel edges when the note content is scrolled vertically.
* This prevents the resize bars from disappearing at the top of long notes
  after a user scrolls inside a panel.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer knowledge base brand

* `openknowledge open` document and asset pages now show the knowledge base
  display name in the header instead of always showing `Open Knowledge`.
* The viewer prefers root `index.md` metadata in this order:
  `okf_bundle_title`, `okf_bundle_name`, root index title metadata, then the
  first root index H1, with `Open Knowledge` reserved as the final fallback.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - CLI docs moved into wiki

* The remaining operational notes from `docs/cli.md` now live in
  `Wiki/features/operations.md`, with install deployment notes kept in
  `Wiki/features/installation.md`.
* The wiki feature-docs workflow now points future docs work at the canonical
  wiki pages instead of the retired `docs/cli.md` file.
* Source anchors: `Wiki/features/operations.md`,
  `Wiki/features/installation.md`, `Wiki/workflows/feature-docs.md`.
* Docs updated: `Wiki/features/index.md`, `Wiki/log.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Export publish metadata

* `openknowledge validate` now accepts `okf_publish` metadata on `index.md`
  files, so public-view-only exclusions such as `okf_publish: false` do not
  make reserved index files invalid.
* `openknowledge to html` and `openknowledge to html --plain` now skip files
  whose frontmatter declares `okf_publish: false`; the default viewer export
  also omits unpublished files from its static note manifest and graph data.
* Nested `index.md` files still reject concept-style frontmatter such as
  `type: Index`.
* Source anchors: `packages/cli/internal/okf/validate.go`,
  `packages/cli/internal/okf/bundle.go`,
  `packages/cli/internal/okf/html.go`,
  `packages/cli/internal/okf/export_test.go`,
  `packages/cli/internal/okf/validate_test.go`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/validate.md`,
  `Wiki/features/exporters/html.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer single-panel centering and resize

* A lone open note panel now uses symmetric viewport gutters so its center
  aligns exactly with the workspace center instead of drifting from asymmetric
  stack padding.
* Resizing a lone panel now expands or shrinks it around that center, so the
  dragged edge follows the pointer and the opposite edge moves the same amount
  in the opposite direction.
* Multi-panel resize behavior keeps the existing edge-anchored scroll handling
  for left-to-right pane browsing.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - HTML viewer export theming

* `openknowledge to html` default viewer exports now read optional
  `[html.theme]` settings from `openknowledge.toml` in the bundle root.
* Theme config supports `name` for `data-openknowledge-theme` and `stylesheet`
  (or `css`) for a deployable theme CSS file. Local stylesheets are constrained
  to the bundle, copied into the output folder, and linked relatively from every
  generated page; external `http` and `https` stylesheets are linked as-is.
* The default theme now lives in
  `packages/cli/cmd/openknowledge/viewer_theme.css`, which is embedded into
  the viewer app. The local viewer and default HTML export derive colors, fonts,
  graph colors, syntax colors, and viewer dimensions from its documented
  `--ok-*` variables.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_theme.go`,
  `packages/cli/cmd/openknowledge/viewer_theme.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/open.md`, `Wiki/features/commands/to.md`,
  `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer resizable panels restored

* Note panels in the local viewer can be resized horizontally from either
  vertical edge, with a minimum width to keep notes readable.
* Panel widths are stored per note and restored when that note is opened again;
  notes without a saved width keep the existing default panel size.
* Right-edge resize handles now stay aligned with the panel edge after resizing
  instead of drifting into the note body when the panel has a vertical scrollbar.
* Single-panel workspaces now use the same bottom rail gap as multi-panel
  workspaces and no longer show a native horizontal scrollbar from one-sided
  stack padding.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer asset links and syntax highlighting

* `openknowledge open` now syntax-highlights fenced code blocks in rendered
  Markdown and highlights common code/text files opened through the local
  viewer.
* Local links to code/text assets open escaped source preview pages, while local
  PDF, image, audio, and video references resolve to bundle-scoped raw URLs so
  the browser can use native PDF and media viewers.
* Raw asset responses are constrained to files under the knowledge root and set
  `X-Content-Type-Options: nosniff`; active code-like raw types are served as
  plain text.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `packages/cli/internal/okf/markdown.go`,
  `packages/cli/internal/okf/markdown_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - New bundle metadata flags

* `openknowledge new` now accepts optional `--bundle-name`, `--bundle-title`,
  `--bundle-purpose`, repeatable `--bundle-tag`, and repeatable
  `--bundle-entry name=path` flags.
* The scaffold writes those values into root `index.md` as flat
  `okf_bundle_*` metadata while preserving the default minimal scaffold when no
  metadata flags are provided.
* Validation now accepts `okf_bundle_*` keys in the bundle-root `index.md` as
  an Open Knowledge CLI metadata layer; plain OKF bundles with only
  `okf_version: "0.1"` remain valid, and nested `index.md` files still cannot
  use frontmatter.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/internal/okf/new.go`,
  `packages/cli/internal/okf/validate.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/new.md`,
  `Wiki/features/commands/validate.md`,
  `Wiki/features/commands/help.md`,
  `Wiki/features/commands/use.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer knowledge graph canvas physics

* The empty-state knowledge graph now renders as an animated canvas graph
  instead of static SVG, allowing lightweight physics to keep nodes responsive
  after the deterministic initial layout.
* Hover and keyboard focus now ease the active node label and separation forces
  in and out, with velocity clamping and damping to reduce jitter in displaced
  nodes.
* Non-active nodes keep their default visual style during hover; the emphasis is
  on the active node and its direct connections.
* Default graph lines are visually lighter so the connected-edge highlight is
  the main emphasis during graph exploration.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer markdown links with code labels

* Markdown links whose labels contain inline code spans, for example a React
  docs link whose visible label includes `useEffect` as code, now render as
  clickable anchors in `openknowledge open` instead of leaking the raw Markdown
  syntax.
* Inline code spans that contain link-looking text remain literal code and are
  not converted into anchors.
* Source anchors: `packages/cli/internal/okf/markdown.go`,
  `packages/cli/internal/okf/markdown_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer knowledge graph clustering

* The empty-state knowledge graph now uses a deterministic force-style layout
  so linked notes cluster together instead of being arranged in a fixed circle.
* The graph layout now runs collision passes against node and label bounds to
  reduce overlapping note names when the graph has enough room to separate them.
* Generic `index` graph labels now include path suffix context, such as
  `commands/index`, to distinguish nested index files.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer multi-panel horizontal scrolling

* Multi-panel document stacks now use an Andy Matuschak-style horizontal flex
  scroll container plus a custom always-visible bottom rail for horizontal
  movement on mouse or trackpad devices.
* The rail thumb can be dragged, the rail track can be clicked, and the focused
  thumb supports keyboard scrolling.
* The gray workspace gaps support mouse drag scrolling left and right while
  preserving normal text selection inside note panels.
* Holding `Space` now enables canvas-style mouse panning across note panels, so
  sideways dragging scrolls the stack without opening links under the pointer.
* Browser-aborted View Transition animations no longer surface as viewer app
  errors after the stack DOM update has already completed.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Setup skill subagent guidance

* Updated the setup prompt and generated `SETUP.MD` so repo-scoped or
  user-scoped skills should include guidance for spawning focused subagents
  with lower reasoning effort for bounded wiki maintenance tasks when the
  runtime supports that.
* Source anchors: `packages/cli/internal/okf/setup.go`,
  `packages/cli/internal/okf/new.go`,
  `packages/cli/internal/okf/setup_test.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/setup.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer single-panel centering

* The panel viewer now centers a lone open panel in the workspace.
* Opening a second panel removes the single-panel centering and keeps the
  existing left-to-right stack browsing behavior.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer file tree system badges

* The viewer file explorer now shows only the filename in each file row instead
  of repeating the full relative path on the right.
* Removed the generic `md` badge and replaced it with a right-aligned `system`
  badge only for reserved Markdown files such as `index.md` and `log.md`.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer search dropdown focus and keyboard controls

* The viewer search dropdown now opens on focus with top file entries for an
  empty query and stays open while typed search requests are pending, avoiding
  flicker between keystrokes.
* The dropdown closes when a result is activated, including after pending search
  requests resolve.
* Search now gives `index.md` files lower priority than comparable regular
  pages in both the local search API and exported static HTML.
* The document viewer header keeps its vertical padding so the top-bar search
  aligns with the logo.
* Search results can be selected with `ArrowDown`/`ArrowUp` and opened with
  `Enter` while focus stays in the search field.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `packages/cli/internal/okf/search.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer search moved out of sidebar

* Removed the duplicate search box from the file explorer sidebar; viewer
  search now lives only in the top bar.
* `Command+K` on macOS and `Ctrl+K` elsewhere still focus the top-bar search,
  and exported static HTML keeps the same search behavior.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Empty workspace graph overview

* The panel viewer empty workspace now uses a 50/50 overview with the file tree
  on the left and a connected graph of Markdown files on the right.
* The graph is built from local Markdown links and graph nodes open files as
  panels, including in exported static HTML.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Sidebar search restored in viewer

* The panel viewer file explorer now includes a search box above the file tree.
* The top bar now includes the primary search field, focused by `Command+K` on
  macOS and `Ctrl+K` elsewhere.
* Local `openknowledge open` pages use the existing `/api/search` endpoint, and
  exported static HTML searches the embedded note manifest in-browser.
* Search result clicks open as panels and keep the sidebar open.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Local viewer always uses panel stack

* Removed the local viewer focus-mode toggle so document browsing always uses
  the horizontally scrollable panel stack.
* File-tree and Markdown link navigation now consistently append or replace
  panels instead of switching into a single-page layout.
* Stack View Transitions now clear fallback panel-entry animation classes before
  the live DOM is shown again, avoiding a second flash after the transition.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Reachable local viewer URL

* `openknowledge open` now prints and opens the actual listener URL as the
  `Open Knowledge view` line, defaulting to `127.0.0.1`, so direct path aliases
  such as `/wiki/` remain reachable without local DNS setup.
* The optional local-domain URL is still printed as `Open Knowledge alias` for
  environments that map names such as `open.knowledge` to loopback.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/open.md`, `Wiki/changelog/cli.md`.

### 2026-06-18 - Markdown and frontmatter validation warnings

* `openknowledge validate` now checks Markdown syntax for malformed links,
  unclosed code spans, invalid table separators, and unclosed fenced code blocks.
* Parseable frontmatter formatting issues, such as duplicate keys or delimiter
  whitespace, are reported as warnings; frontmatter that cannot be parsed
  remains an error.
* Source anchors: `packages/cli/internal/okf/validate.go`,
  `packages/cli/internal/okf/frontmatter.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/validate.md`, `Wiki/changelog/cli.md`.

### 2026-06-18 - Registry-backed local viewer

* Changed `openknowledge open` without a path to open the Open Knowledge
  Registry viewer, with a left workspace selector for registered knowledge
  bases.
* Kept `openknowledge open <path-or-name>` as the direct viewer for one
  knowledge base.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/open.md`, `Wiki/features/commands/registry.md`.

### 2026-06-18 - Context-aware setup interview prompt

* Updated `openknowledge setup` so agents inspect the current workspace or
  target folder and relevant runtime-exposed memories before asking questions.
* The setup prompt and generated `SETUP.MD` now tell agents to ask only missing,
  context-specific questions instead of repeating a fixed questionnaire.
* Source anchors: `packages/cli/internal/okf/setup.go`,
  `packages/cli/internal/okf/new.go`, `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`, `packages/web/index.html`,
  `Wiki/features/commands/setup.md`.

### 2026-06-18 - Wiki maintenance loop initialized

* Created a colocated Open Knowledge wiki at `Wiki/`.
* Added command, exporter, installation, workflow, and changelog seed pages.
* Added root `AGENTS.md` and repo skill `.codex/skills/openknowledge-wiki/SKILL.md`
  so future agents update this wiki when touching CLI behavior.

## Baseline Command Surface

As of the wiki setup, the CLI exposes:

* `openknowledge setup`
* `openknowledge new`
* `openknowledge registry list`
* `openknowledge registry add`
* `openknowledge where`
* `openknowledge open`
* `openknowledge to html`
* `openknowledge to json`
* `openknowledge spec`
* `openknowledge validate`
* `openknowledge list`
* `openknowledge version`

## Entry Template

```md
### YYYY-MM-DD - Short change title

* What changed:
* Why it matters:
* Source anchors:
* Docs updated:
```

## Update Rules

Add an entry when a change affects command behavior, arguments or flags, help
text, validation rules, export output, viewer behavior, setup prompts, registry
semantics, release packaging, npm wrapper behavior, or developer-facing docs.

Do not add entries for purely internal refactors unless they alter user-visible
or developer-relevant behavior.
