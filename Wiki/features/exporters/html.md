---
type: Exporter Documentation
title: HTML Exporter
description: Static HTML export behavior for Open Knowledge bundles.
tags: [openknowledge, cli, exporter, html]
timestamp: 2026-06-18T00:00:00Z
---

# HTML Exporter

The HTML exporter turns an OKF bundle into static pages. The default mode ships
the same viewer used by `openknowledge open`, so exported docs keep file
browsing, search, stacked panels, graph data, table controls, syntax
highlighting, and mobile layout behavior without a local server. It also writes
connection assets so a deployed wiki can be added back to the local registry.

Use `--plain` when the output should be only semantic HTML without viewer CSS,
JavaScript, search, graph data, or table controls.

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

Both modes strip YAML frontmatter from rendered pages, rewrite local Markdown
links to generated `.html` targets, and skip files with `okf_publish: false`.
Rendered Markdown keeps list continuations inside their parent item and emits
semantic tables with alignment metadata.

Default viewer exports embed a static note manifest and graph data in each page
so search, panel navigation, source links, and enhanced table controls work on a
static host. The viewer can resolve pretty URLs such as `/agents` or
`/features/` back to generated notes, and the header brand links to the exported
`index.html` with a relative URL for subpath deployments. Portable static pages
do not expose the local filesystem path used during the build.

Default viewer exports include remote-connect assets:

* `openknowledge.json` - an Open Knowledge manifest with type
  `openknowledge.bundle`, archive path, archive format, spec version, bundle
  name/title metadata when present, and archive SHA-256.
* `assets/openknowledge-bundle.tar.gz` - a portable source bundle archive
  generated with the same writer as `openknowledge to tar`.

`openknowledge connect <deployed-wiki-url>` discovers the manifest, verifies the
archive hash when present, extracts the archive safely, validates the extracted
bundle, and registers the materialized bundle in the local registry.

Default viewer exports read optional settings from `openknowledge.toml`.
`[html.theme]` sets a theme name and stylesheet:

```toml
[html.theme]
name = "landing"
stylesheet = "assets/wiki-theme.css"
```

Exported pages include `data-openknowledge-theme="<name>"` on `<html>` and link
the stylesheet after the built-in viewer CSS. Local stylesheets must stay inside
the bundle; they are copied into the output and linked relatively from every
page. External `http` and `https` stylesheet URLs are linked as-is.

Default viewer exports also suppress the local editor dropdown because deployed
HTML cannot open the build machine's local files. To show a source action in
exported pages, configure `[html.source]` in `openknowledge.toml`:

```toml
[html.source]
github_base = "https://github.com/openknowledge-sh/openknowledge/blob/main"
entry = "Wiki"
```

When `github_base` is present, each exported Markdown panel shows a GitHub
source button built from `github_base`, optional `entry`, and the Markdown file
path. When `[html.source]` is absent, exported pages render no local editor or
source action. `html.source.entry` is a repository path prefix, not the viewer
title.

The deployed viewer brand comes from root `index.md` metadata in this order:
`okf_bundle_title`, `okf_bundle_name`, `title`, then the first Markdown `#`
heading. The built-in theme contract lives in
`packages/cli/cmd/openknowledge/viewer_theme.css`; override `--ok-*` variables
there through a configured stylesheet instead of changing generated HTML.

## Use Cases

* Publish a portable static wiki.
* Connect a deployed wiki back into a local registry.
* Produce minimal HTML for systems that should not include viewer JavaScript.
* Apply a deployable theme stylesheet without changing source Markdown.
* Link deployed pages to GitHub source without exposing local editor deeplinks.

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
