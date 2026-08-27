---
type: Exporter Documentation
title: HTML Exporter
description: Publish an Open Knowledge bundle as a static site.
tags: [openknowledge, cli, exporter, html]
timestamp: 2026-08-24T00:00:00Z
---

# HTML Exporter

Export a validated bundle as a full static viewer or as plain semantic HTML.
Both modes require `viewer` in `[release].outputs`.

## Usage

```sh
okn export html --out <folder> [key-or-path]
okn export html --no-source-archive --out <folder> [key-or-path]
okn export html --plain --out <folder> [key-or-path]
okn export html --head-file <file> --out <folder> [key-or-path]
okn export html --script-src <src> --out <folder> [key-or-path]
```

| Option | Default | Description |
| --- | --- | --- |
| `key-or-path` | `.` | Registry key or bundle root. |
| `--out <folder>` | required | Output directory. |
| `--plain` | off | Omit viewer CSS, JavaScript, search, graph, and chrome. |
| `--no-source-archive` | off | Omit the portable source archive and connect manifest. |
| `--spec <version>` | `latest` | OKF spec used for validation. |
| `--head-file <file>` | environment | Trusted head fragment for viewer mode. |
| `--head-html <html>` | environment | Trusted inline head fragment for viewer mode. |
| `--script-src <src>` | environment | Script URL for viewer mode. Repeatable. |

Head injection also reads `OPENKNOWLEDGE_HEAD_FILE`,
`OPENKNOWLEDGE_HEAD_HTML`, and `OPENKNOWLEDGE_SCRIPT_SRC`.
Plain mode does not support head injection.

## Output

Viewer mode includes:

- static Markdown pages with **Documents**, **Claims**, **Graph**, and
  **Settings** sidebar items
- read-only claim panels, claim filters, claim details, and relationship views
- claim occurrence nodes and typed claim relations in the graph
- Mermaid diagrams, search, stacked panels, metadata inspectors, table
  controls, browser-native text-to-speech, themes, and mobile layout
- a light blue default theme that matches the Open Knowledge website
- OKF 0.2 trust, status, freshness, provenance, structured source, and
  Attested Computation contract views
- `llms.txt` for pages enabled for both `viewer` and `llms`
- `sitemap.xml` when the configuration contains `[html.site].base_url`
- `openknowledge.json` and `assets/openknowledge-bundle.tar.gz` for remote
  `okn connect`, unless `--no-source-archive` is set
- allowed public assets at their bundle-relative paths

Markdown images retain source-relative paths in static pages. This behavior
supports hosted output and direct `file://` use from nested pages. Add each
local image to `publish.assets` so the exporter copies it.

Use `--no-source-archive` when the published source would make the site too
large. A site without these files does not support remote `okn connect` from
its site URL.

Viewer mode writes its executable JavaScript below `assets/openknowledge/`.
Every page references one shared `viewer.js`, `viewer.css`,
`viewer-theme.js`, and `viewer-data.js`. The data file contains the rendered
note collection, graph, claim projection, and deterministic editor catalog.
Individual HTML pages do not embed a copy of the complete collection.

Viewer mode keeps typed claims in canonical YAML frontmatter. It also provides
the same read-only claim panel and Claims workspace as `okn view`.

Viewer-mode search strips common Markdown markup from excerpts. It adds folder
context to `Index` titles. It keeps arrow key, Enter, Shift+Enter, and Escape
controls.

Viewer mode provides the graph controls that the [`view`](../commands/view.md)
command provides.

Viewer-mode Markdown pages expose **Listen**, pause/resume, and stop controls
when the browser supports speech synthesis. Narration uses the browser or
operating system voice. Open Knowledge makes no speech API request and manages
no speech credential, but the browser-selected voice may itself be local or
network-backed. Narration skips frontmatter, agent-only annotations, code
blocks, Mermaid diagrams, and hidden controls. The controls remain hidden for
unsupported browsers and for source or text asset cards. Plain mode does not
include narration controls.

The shared JavaScript includes the pinned Mermaid runtime. Vite builds the
viewer from JavaScript and CSS modules in `packages/web/src/viewer`.
Workspace and release builds generate the embedded viewer assets on demand.
Git does not track the compiled viewer files.
Generated pages do not require `unsafe-inline` in `script-src`.
The deployment owns trusted inline scripts from `--head-file` or `--head-html`.
These scripts can require a CSP nonce or hash.
Use `--script-src` for trusted external scripts.

The local viewer and interactive HTML exports use the same diagram controls.
Both viewer modes provide zoom, pan, **Fit**, and **100%** controls in a
viewport-filling dialog.

The first visit uses the default light theme. A browser-local theme preference
overrides the default theme. The **Night** theme remains available in settings.

Viewer mode displays executor and attester declarations. It does not execute
either resource.

Plain mode writes only semantic HTML pages. It includes frontmatter in native
`details`, `dl`, and list elements. Claims remain in this frontmatter view.
OKF 0.2 source footnotes link to matching source metadata entries. Plain mode
omits the claim panel, Claims workspace, and other interactive viewer controls.
It also omits viewer assets, discovery files, and search data.

## Publication rules

```toml
[release]
outputs = ["viewer"]

[publish]
assets = ["assets/public/**", "whitepapers/*.pdf"]

[html.site]
base_url = "https://docs.example.com/"

[html.theme]
name = "custom"
stylesheet = "assets/public/wiki.css"

[html.source]
github_base = "https://github.com/example/project/blob/main"
entry = "Wiki"
```

- The exporter excludes files with `okf_publish: false`.
- `okf_targets.viewer`, `search`, `llms`, and `sitemap` control individual
  projections. They default to `true` after you enable the viewer output.
- A non-Markdown file is public only when it matches `publish.assets`.
- Keep local stylesheets in the bundle.
- The exporter links HTTP(S) stylesheets as configured.
- `html.site.base_url` must be an absolute HTTP(S) URL without query or
  fragment.
- Without `html.site.base_url`, `llms.txt` uses relative links.
- Without `html.site.base_url`, the exporter does not build a sitemap.

See [`.openknowledge.toml`](/features/configuration.md) for the strict field
contract.

## Build behavior

The source must validate without errors.
The exporter permits warnings.
The exporter builds a complete sibling generation.
It replaces the destination only when all selected files are complete.
A failed build preserves the previous site.
A successful build removes stale output.

An identical source produces identical viewer files. The static editor catalog
does not inspect installed applications or copy machine-local icons. Relative
asset paths support a hosted site and direct `file://` use, including nested
pages.

The output can be in the source bundle.
In this case, the portable archive excludes the output.
The output must not equal or contain the source root.

Viewer pages rewrite local links and hide HTML comments.
They keep bounded `agent-context` annotations visible and use a subdued text
color. The legacy `<!-- okf-footer: agent-maintenance -->` marker uses the same
presentation and extends to the end of its source file.
When included, the portable archive contains only publishable Markdown and allowed assets.
It excludes project configuration and `.openknowledge` job or run state.
It also excludes Markdown with `okf_publish: false`, including private insights.
It excludes assets that do not match the asset list.

When these files are included, `okn connect <site-url>` validates the strict manifest and archive digest.
It also validates the extracted bundle and declared OKF version.
Then, it registers the materialized source.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/html.go`
> - `packages/cli/internal/okf/html_frontmatter.go`
> - `packages/cli/internal/okf/atomic_output.go`
> - `packages/cli/cmd/openknowledge/viewer_export.go`
> - `packages/cli/cmd/openknowledge/viewer_frontmatter.go`
> - `packages/cli/cmd/openknowledge/viewer_claims.go`
> - `packages/cli/cmd/openknowledge/viewer_templates.go`
> - `packages/cli/cmd/openknowledge/viewer_test.go`
> - `packages/cli/cmd/openknowledge/viewer_discovery.go`
> - `packages/cli/cmd/openknowledge/viewer_theme.go`
> - `packages/web/src/viewer/`
> - `packages/web/scripts/browser.e2e.mjs`
> - `packages/web/vite.viewer.config.js`
> - `packages/web/vite.theme.config.js`
> - `packages/cli/internal/okf/export_test.go`
>
> **Update notes**
>
> Update this page after a change to publication selection or generated files.
> Also update it after a change to viewer mode, plain mode, or build atomicity.
