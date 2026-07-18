---
type: Exporter Documentation
title: HTML Exporter
description: Publish an Open Knowledge bundle as a static site.
tags: [openknowledge, cli, exporter, html]
timestamp: 2026-07-18T00:00:00Z
---

# HTML Exporter

Export a validated bundle as either the full static viewer or plain semantic
HTML. Both modes require `[publish] enabled = true`.

## Usage

```sh
openknowledge export html --out <folder> [key-or-path]
openknowledge export html --plain --out <folder> [key-or-path]
openknowledge export html --head-file <file> --out <folder> [key-or-path]
openknowledge export html --script-src <src> --out <folder> [key-or-path]
```

| Option | Default | Description |
| --- | --- | --- |
| `key-or-path` | `.` | Registry key or bundle root. |
| `--out <folder>` | required | Output directory. |
| `--plain` | off | Omit viewer CSS, JavaScript, search, graph, and chrome. |
| `--spec <version>` | `latest` | OKF spec used for validation. |
| `--head-file <file>` | environment | Trusted head fragment for viewer mode. |
| `--head-html <html>` | environment | Trusted inline head fragment for viewer mode. |
| `--script-src <src>` | environment | Script URL for viewer mode; repeatable. |

Head injection also reads `OPENKNOWLEDGE_HEAD_FILE`,
`OPENKNOWLEDGE_HEAD_HTML`, and `OPENKNOWLEDGE_SCRIPT_SRC`. Plain mode does not
support head injection.

## Output

Viewer mode includes:

- static Markdown pages with file navigation, search, graph data, stacked
  panels, metadata inspectors, table controls, themes, and mobile layout;
- `llms.txt` for pages enabled for both `viewer` and `llms`;
- `sitemap.xml` when `[html.site].base_url` is configured;
- `openknowledge.json` and `assets/openknowledge-bundle.tar.gz` for remote
  `openknowledge connect`;
- allowlisted public assets at their bundle-relative paths.

Viewer mode writes its executable JavaScript as same-origin files below
`assets/openknowledge/`; generated pages do not require `unsafe-inline` in
`script-src`. Trusted inline scripts supplied through `--head-file` or
`--head-html` remain deployment-owned and may require a CSP nonce or hash; use
`--script-src` for trusted external scripts.

Plain mode writes only semantic HTML pages. It omits viewer assets, discovery
files, search data, source controls, and frontmatter chrome.

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

- Files with `okf_publish: false` are excluded.
- `okf_targets.viewer`, `search`, `llms`, and `sitemap` control individual
  projections and default to `true` after publication is enabled.
- Non-Markdown files are public only when matched by `publish.assets`.
- Local stylesheets must remain inside the bundle; HTTP(S) stylesheets are
  linked as configured.
- `html.site.base_url` must be an absolute HTTP(S) URL without query or
  fragment. Without it, `llms.txt` uses relative links and no sitemap is built.

See [`openknowledge.toml`](/features/configuration.md) for the strict field
contract.

## Build behavior

The source must validate without errors; warnings are allowed. The exporter
builds a complete sibling generation and atomically replaces the destination
only after every page, asset, manifest, and archive succeeds. Failed builds
preserve the previous site; successful builds remove stale output.

The output may be inside the source bundle and is then excluded from the
portable archive. It may not equal or contain the source root.

Viewer pages rewrite local links, hide HTML comments, and render content after
`<!-- okf-footer: agent-maintenance -->` as subdued maintenance metadata. The
portable archive contains only publishable Markdown and allowlisted assets.
Project configuration, `.openknowledge` job/run state, Markdown marked
`okf_publish: false` (including private insights), and non-allowlisted assets
are excluded.

`openknowledge connect <site-url>` validates the strict manifest, archive
digest, extracted bundle, and declared OKF version before registering the
materialized source.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/html.go`
> - `packages/cli/internal/okf/atomic_output.go`
> - `packages/cli/cmd/openknowledge/viewer.go`
> - `packages/cli/cmd/openknowledge/viewer_test.go`
> - `packages/cli/cmd/openknowledge/viewer_discovery.go`
> - `packages/cli/cmd/openknowledge/viewer_theme.go`
> - `packages/cli/internal/okf/export_test.go`
>
> **Update notes**
>
> Update this page when publication selection, generated files, viewer/plain
> behavior, or build atomicity changes.
