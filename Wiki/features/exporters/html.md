---
type: Exporter Documentation
title: HTML Exporter
description: Publish an Open Knowledge bundle as a static site.
tags: [openknowledge, cli, exporter, html]
timestamp: 2026-07-18T00:00:00Z
---

# HTML Exporter

Export a validated bundle as a full static viewer or as plain semantic HTML.
Both modes require `[publish] enabled = true`.

## Usage

```sh
okn export html --out <folder> [key-or-path]
okn export html --plain --out <folder> [key-or-path]
okn export html --head-file <file> --out <folder> [key-or-path]
okn export html --script-src <src> --out <folder> [key-or-path]
```

| Option | Default | Description |
| --- | --- | --- |
| `key-or-path` | `.` | Registry key or bundle root. |
| `--out <folder>` | required | Output directory. |
| `--plain` | off | Omit viewer CSS, JavaScript, search, graph, and chrome. |
| `--spec <version>` | `latest` | OKF spec used for validation. |
| `--head-file <file>` | environment | Trusted head fragment for viewer mode. |
| `--head-html <html>` | environment | Trusted inline head fragment for viewer mode. |
| `--script-src <src>` | environment | Script URL for viewer mode. Repeatable. |

Head injection also reads `OPENKNOWLEDGE_HEAD_FILE`,
`OPENKNOWLEDGE_HEAD_HTML`, and `OPENKNOWLEDGE_SCRIPT_SRC`.
Plain mode does not support head injection.

## Output

Viewer mode includes:

- static Markdown pages with file navigation, Mermaid diagrams, search, graph
  data, stacked panels, metadata inspectors, table controls, themes, and mobile layout
- `llms.txt` for pages enabled for both `viewer` and `llms`
- `sitemap.xml` when the configuration contains `[html.site].base_url`
- `openknowledge.json` and `assets/openknowledge-bundle.tar.gz` for remote
  `okn connect`
- allowed public assets at their bundle-relative paths

Viewer mode writes its executable JavaScript below `assets/openknowledge/`.
Every page references one shared `viewer.js`, `viewer.css`,
`viewer-theme.js`, and `viewer-data.js`. The data file contains the rendered
note collection, graph, and deterministic editor catalog. Individual HTML
pages do not embed a copy of the complete collection.

The shared JavaScript includes the pinned Mermaid runtime. Vite builds the
viewer from the TypeScript and JavaScript sources in `packages/web/src/viewer`.
Generated pages do not require `unsafe-inline` in `script-src`.
The deployment owns trusted inline scripts from `--head-file` or `--head-html`.
These scripts can require a CSP nonce or hash.
Use `--script-src` for trusted external scripts.

Plain mode writes only semantic HTML pages.
It omits viewer assets, discovery files, search data, source controls, and frontmatter chrome.

## Publication rules

```toml
[publish]
enabled = true
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
  projections. They default to `true` after you enable publication.
- A non-Markdown file is public only when it matches `publish.assets`.
- Keep local stylesheets in the bundle.
- The exporter links HTTP(S) stylesheets as configured.
- `html.site.base_url` must be an absolute HTTP(S) URL without query or
  fragment.
- Without `html.site.base_url`, `llms.txt` uses relative links.
- Without `html.site.base_url`, the exporter does not build a sitemap.

See [`openknowledge.toml`](/features/configuration.md) for the strict field
contract.

## Build behavior

The source must validate without errors.
The exporter permits warnings.
The exporter builds a complete sibling generation.
It replaces the destination only when all pages, assets, manifests, and archives are complete.
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
They show content after `<!-- okf-footer: agent-maintenance -->` as subdued maintenance metadata.
The portable archive contains only publishable Markdown and allowed assets.
It excludes project configuration and `.openknowledge` job or run state.
It also excludes Markdown with `okf_publish: false`, including private insights.
It excludes assets that do not match the asset list.

`okn connect <site-url>` validates the strict manifest and archive digest.
It also validates the extracted bundle and declared OKF version.
Then, it registers the materialized source.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/html.go`
> - `packages/cli/internal/okf/atomic_output.go`
> - `packages/cli/cmd/openknowledge/viewer_export.go`
> - `packages/cli/cmd/openknowledge/viewer_templates.go`
> - `packages/cli/cmd/openknowledge/viewer_test.go`
> - `packages/cli/cmd/openknowledge/viewer_discovery.go`
> - `packages/cli/cmd/openknowledge/viewer_theme.go`
> - `packages/web/src/viewer/`
> - `packages/web/vite.viewer.config.ts`
> - `packages/cli/internal/okf/export_test.go`
>
> **Update notes**
>
> Update this page after a change to publication selection or generated files.
> Also update it after a change to viewer mode, plain mode, or build atomicity.
