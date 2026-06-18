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
openknowledge to json [path]
openknowledge to json --out <file> [path]
openknowledge to --help
```

## Targets

| Target | Status | Details |
| --- | --- | --- |
| `html` | shipped | See [HTML exporter](/features/exporters/html.md). |
| `json` | shipped | See [JSON exporter](/features/exporters/json.md). |
| `graph` | candidate | See [Graph exporter candidate](/features/exporters/graph.md). |

## Common Flags

| Name | Kind | Description |
| --- | --- | --- |
| `path` | argument | Knowledge base root. Defaults to the current directory. |
| `--spec` | flag | OKF spec version. Defaults to latest. |
| `--out` | flag | Output folder for HTML and optional output file for JSON. |

## Use Cases

* Publish a static HTML copy of a wiki.
* Produce a normalized JSON model for downstream tools or agents.
* Keep future exporter targets grouped under one command family.

## Source Anchors

* `packages/cli/cmd/openknowledge/main.go`
* `packages/cli/internal/okf/html.go`
* `packages/cli/internal/okf/bundle.go`

## Update Notes

When a new target is added, update this page, the exporter section, root help,
README command tables, and [CLI changelog](/changelog/cli.md).
