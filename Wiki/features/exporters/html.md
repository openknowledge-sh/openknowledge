---
type: Exporter Documentation
title: HTML Exporter
description: Static HTML export behavior for Open Knowledge bundles.
tags: [openknowledge, cli, exporter, html]
timestamp: 2026-06-18T00:00:00Z
---

# HTML Exporter

The HTML exporter writes one `.html` file for each Markdown file in a bundle.
By default, those pages use the same static viewer app bundle as
`openknowledge open`: file tree, search, stacked-panel browsing, and embedded
note data are available without a local server. The shared viewer CSS keeps the
top-bar search field responsive on narrow mobile widths so it does not overlap
the knowledge base brand. The `--plain` flag switches to unstyled semantic HTML
without CSS, JavaScript, or viewer chrome.

## Command

```sh
openknowledge to html --out <folder> [path]
openknowledge to html --plain --out <folder> [path]
openknowledge to html --spec <version> --out <folder> [path]
```

## Arguments And Flags

| Name | Kind | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `path` | argument | no | current directory | Knowledge base root. |
| `--out` | flag | yes | none | Output folder for generated HTML files. |
| `--plain` | flag | no | off | Generate plain semantic HTML without CSS, JavaScript, or viewer chrome. |
| `--spec` | flag | no | `latest` | OKF spec version. |

## Behavior

Both modes strip YAML frontmatter from rendered pages and rewrite local
Markdown links to generated `.html` targets. Files with `okf_publish: false`
frontmatter are skipped and do not get generated HTML pages. The default viewer
export embeds a static note manifest and graph data in each generated page so
search and panel navigation work in exported output; unpublished files are
omitted from that manifest and graph data. The plain export keeps only the
rendered document structure.

The default viewer export also reads an optional `openknowledge.toml` file from
the bundle root. A `[html.theme]` section can set a theme name and stylesheet:

```toml
[html.theme]
name = "landing"
stylesheet = "assets/wiki-theme.css"
```

The exported pages include `data-openknowledge-theme="<name>"` on `<html>` and
link the configured stylesheet after the built-in viewer CSS. Local stylesheets
must stay inside the bundle; they are copied into the output folder and linked
relatively from every generated page. External `http` and `https` stylesheet
URLs are linked as-is. `openknowledge open` serves the same local stylesheet
through the raw file endpoint for local preview, applies the same theme name on
viewer HTML, and validates local stylesheet paths before rendering.

Default viewer exports leave the internal `data-note-root` attribute empty, so
portable static pages do not expose the local filesystem path used during the
build. The browser-side viewer falls back to the page URL for static-only
storage keys.

The canonical default theme source is
`packages/cli/cmd/openknowledge/viewer_theme.css`. The local viewer and default
HTML export derive colors, fonts, and viewer dimensions from this theme layer.
Supported theme variables are:

```css
--ok-font-body
--ok-font-mono
--ok-header-height
--ok-mobile-header-height
--ok-sidebar-width
--ok-note-panel-default-width
--ok-note-panel-min-width
--ok-color-text
--ok-color-document-text
--ok-color-muted
--ok-color-border
--ok-color-page
--ok-color-surface
--ok-color-accent
--ok-color-accent-rgb
--ok-color-accent-strong
--ok-color-accent-soft
--ok-color-accent-softer
--ok-color-accent-selected
--ok-color-accent-focus
--ok-color-accent-focus-strong
--ok-color-accent-border
--ok-color-accent-border-strong
--ok-color-shadow
--ok-color-danger
--ok-color-header-bg
--ok-color-viewer-canvas
--ok-color-viewer-header-bg
--ok-color-control-text
--ok-color-control-hover-text
--ok-color-control-hover-border
--ok-color-control-hover-bg
--ok-color-close-text
--ok-color-close-hover-border
--ok-color-close-hover-bg
--ok-color-sidebar
--ok-color-sidebar-header
--ok-color-sidebar-row
--ok-color-sidebar-border
--ok-color-sidebar-text
--ok-color-sidebar-tree-hover-bg
--ok-color-search-input-border
--ok-color-search-input-bg
--ok-color-search-shortcut-border
--ok-color-search-shortcut-bg
--ok-color-search-shortcut-text
--ok-color-search-popover-border
--ok-color-search-popover-bg
--ok-color-search-popover-shadow
--ok-color-search-result-border
--ok-color-search-result-hover-border
--ok-color-search-result-hover-bg
--ok-color-card-border
--ok-color-card-bg
--ok-color-card-hover-bg
--ok-color-rail-track
--ok-color-rail-thumb
--ok-color-rail-thumb-hover
--ok-color-tree-text
--ok-color-tree-directory-bg
--ok-color-tree-directory-text
--ok-color-tree-directory-marker
--ok-color-tree-badge-border
--ok-color-tree-badge-text
--ok-color-note-resize-hitarea
--ok-color-note-resize-active
--ok-color-note-chrome-bg
--ok-color-note-close-text
--ok-color-note-close-hover-border
--ok-color-note-close-hover-text
--ok-color-editor-trigger-border
--ok-color-editor-trigger-bg
--ok-color-editor-trigger-text
--ok-color-editor-trigger-shadow
--ok-color-editor-trigger-separator
--ok-color-editor-trigger-focus-border
--ok-color-editor-mark-border
--ok-color-editor-mark-bg
--ok-color-editor-mark-text
--ok-color-editor-caret
--ok-color-editor-menu-border
--ok-color-editor-menu-bg
--ok-color-editor-menu-shadow
--ok-color-editor-menu-item-text
--ok-color-editor-menu-item-hover-bg
--ok-color-editor-option-border
--ok-color-editor-option-bg
--ok-color-editor-option-text
--ok-color-editor-menu-separator
--ok-color-code-inline-bg
--ok-color-code-block-bg
--ok-color-code-block-text
--ok-color-syntax-keyword
--ok-color-syntax-string
--ok-color-syntax-number
--ok-color-syntax-comment
--ok-color-graph-edge
--ok-color-graph-edge-muted
--ok-color-graph-edge-active
--ok-color-graph-node-bg
--ok-color-graph-node-border
--ok-color-graph-node-active-border
--ok-color-graph-node-shadow
--ok-color-graph-node-active-shadow
--ok-color-graph-label-halo
--ok-color-graph-label
--ok-color-graph-label-active
```

## Use Cases

* Publish a portable static copy of a wiki.
* Review generated pages outside the local viewer.
* Test Markdown rendering and local link rewriting.
* Produce minimal HTML for systems that should not include viewer JavaScript.
* Match an exported documentation wiki to a landing page by overriding viewer
  theme variables in a deployable CSS file.

## Source Anchors

* `packages/cli/internal/okf/html.go`
* `packages/cli/internal/okf/markdown.go`
* `packages/cli/internal/okf/export_test.go`
* `packages/cli/cmd/openknowledge/viewer.go`
* `packages/cli/cmd/openknowledge/viewer_theme.go`
* `packages/cli/cmd/openknowledge/viewer_theme.css`
* `packages/cli/cmd/openknowledge/viewer_test.go`

## Update Notes

When template structure, CSS, link rewriting, file naming, or export reporting
changes, update this page and [CLI changelog](/changelog/cli.md).
