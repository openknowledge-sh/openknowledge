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
note data are available without a local server. The `--plain` flag switches to
unstyled semantic HTML without CSS, JavaScript, or viewer chrome.

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
Markdown links to generated `.html` targets. The default viewer export embeds a
static note manifest and graph data in each generated page so search and panel
navigation work in exported output. The plain export keeps only the rendered
document structure.

## Use Cases

* Publish a portable static copy of a wiki.
* Review generated pages outside the local viewer.
* Test Markdown rendering and local link rewriting.
* Produce minimal HTML for systems that should not include viewer JavaScript.

## Source Anchors

* `packages/cli/internal/okf/html.go`
* `packages/cli/internal/okf/markdown.go`
* `packages/cli/internal/okf/export_test.go`
* `packages/cli/cmd/openknowledge/viewer.go`
* `packages/cli/cmd/openknowledge/viewer_test.go`

## Update Notes

When template structure, CSS, link rewriting, file naming, or export reporting
changes, update this page and [CLI changelog](/changelog/cli.md).
