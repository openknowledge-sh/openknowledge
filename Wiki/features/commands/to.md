---
type: Command Documentation
title: openknowledge to
description: Converts an Open Knowledge bundle to supported output formats.
tags: [openknowledge, cli, command, export]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge to`

`openknowledge to` is the conversion command group. Current shipped targets are
`html` and `json`.

## Usage

```sh
openknowledge to html --out <folder> [path]
openknowledge to html --plain --out <folder> [path]
openknowledge to html --spec <version> --out <folder> [path]
openknowledge to json [path]
openknowledge to json --out <file> [path]
openknowledge to json --spec <version> [path]
openknowledge to --help
```

## Targets

| Target | Status | Details |
| --- | --- | --- |
| `html` | shipped | See [HTML exporter](/features/exporters/html.md). |
| `json` | shipped | See [JSON exporter](/features/exporters/json.md). |
| `graph` | candidate | See [Graph exporter candidate](/features/exporters/graph.md). |

## Common Flags

| Name | Kind | Applies To | Required | Default | Description |
| --- | --- | --- | --- | --- | --- |
| `path` | argument | `html`, `json` | no | current directory | Knowledge base root. |
| `--spec` | flag | `html`, `json` | no | `latest` | OKF spec version. |
| `--out` | flag | `html`, `json` | HTML yes, JSON no | stdout for JSON | Output folder for HTML and optional output file for JSON. |
| `--plain` | flag | `html` only | no | off | Write plain semantic HTML without viewer chrome, CSS, or JavaScript. |

## Quick Examples

```sh
openknowledge to html --out ./site ./project-memory
openknowledge to html --plain --out ./plain-site ./project-memory
openknowledge to json ./project-memory
openknowledge to json --out ./bundle.json ./project-memory
```

## Behavior

`to html` requires `--out`. Without `--plain`, it writes static viewer pages
that include the file tree, search, stacked-panel browsing, and embedded note
manifest. Markdown tables in the default viewer export are horizontally
scrollable and get whole-table filtering, dropdown column filters, sortable
headers, row counts, and a clear filters action. Fenced code blocks use the
same syntax-highlighted, subtly language-labeled code card treatment as the local
viewer, and soft-wrapped list continuation lines stay inside their bullet or
numbered item. The default viewer export
reads optional `[html.theme]` settings from `openknowledge.toml`, links the
configured stylesheet after built-in viewer CSS, and copies local theme CSS
files into the output folder. Files listed in `[publish] exclude` are omitted
from HTML output; concept documents may also use `okf_publish: false`
frontmatter for the same public-view exclusion.
The built-in theme contract lives in
`packages/cli/cmd/openknowledge/viewer_theme.css`, and the exported viewer
derives colors, fonts, and viewer dimensions from its `--ok-*` variables. The
static viewer does not render local editor deeplinks. When
`openknowledge.toml` includes `[html.source]` with `github_base` and optional
`entry`, exported Markdown panels render a single GitHub source button instead;
without that config, no source action is shown. With `--plain`, it writes
unstyled semantic HTML pages and does not include viewer CSS, JavaScript,
theme links, or rich table controls.

`to json` prints the normalized bundle model to stdout by default and writes to
`--out <file>` when provided. `--plain` is not valid for JSON. Unknown targets
and unknown flags exit with status `2`.

## Use Cases

* Publish a static HTML copy of a wiki.
* Produce a normalized JSON model for downstream tools or agents.
* Keep future exporter targets grouped under one command family.
* Deploy documentation that visually matches a landing page through a themed
  viewer export.
* Link exported viewer panels back to GitHub source files through
  `[html.source]`.
* Publish table-heavy command or reference docs with browser-side table
  filtering dropdowns and sorting in the default viewer export.

## Source Anchors

* `packages/cli/cmd/openknowledge/main.go`
* `packages/cli/internal/okf/html.go`
* `packages/cli/internal/okf/bundle.go`
* `packages/cli/cmd/openknowledge/viewer.go`
* `packages/cli/cmd/openknowledge/viewer_theme.go`
* `packages/cli/cmd/openknowledge/viewer_theme.css`
* `packages/cli/internal/okf/export_test.go`
* `packages/cli/cmd/openknowledge/viewer_test.go`

## Update Notes

When a new target is added, update this page, the exporter section, root help,
README command tables, and [CLI changelog](/changelog/cli.md).
