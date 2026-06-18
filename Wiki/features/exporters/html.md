---
type: Exporter Documentation
title: HTML Exporter
description: Static HTML export behavior for Open Knowledge bundles.
tags: [openknowledge, cli, exporter, html]
timestamp: 2026-06-18T00:00:00Z
---

# HTML Exporter

The HTML exporter writes one `.html` file for each Markdown file in a bundle.
It strips YAML frontmatter from rendered pages and rewrites local Markdown links
to generated HTML targets.

## Command

```sh
openknowledge to html --out <folder> [path]
openknowledge to html --spec <version> --out <folder> [path]
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `path` | argument | Knowledge base root. Defaults to the current directory. |
| `--out` | flag | Required output folder for generated HTML files. |
| `--spec` | flag | OKF spec version. Defaults to latest. |

## Use Cases

* Publish a portable static copy of a wiki.
* Review generated pages outside the local viewer.
* Test Markdown rendering and local link rewriting.

## Source Anchors

* `packages/cli/internal/okf/html.go`
* `packages/cli/internal/okf/markdown.go`
* `packages/cli/internal/okf/export_test.go`

## Update Notes

When template structure, CSS, link rewriting, file naming, or export reporting
changes, update this page and [CLI changelog](/changelog/cli.md).
