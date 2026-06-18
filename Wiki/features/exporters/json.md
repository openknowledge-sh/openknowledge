---
type: Exporter Documentation
title: JSON Exporter
description: Normalized JSON export behavior for Open Knowledge bundles.
tags: [openknowledge, cli, exporter, json]
timestamp: 2026-06-18T00:00:00Z
---

# JSON Exporter

The JSON exporter serializes a normalized bundle model. It includes file
metadata, frontmatter scalar values, Markdown body content, links, and
validation issues.

## Command

```sh
openknowledge to json [path]
openknowledge to json --out <file> [path]
openknowledge to json --spec <version> [path]
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `path` | argument | Knowledge base root. Defaults to the current directory. |
| `--out` | flag | Optional output file. Defaults to stdout. |
| `--spec` | flag | OKF spec version. Defaults to latest. |

## Use Cases

* Feed bundle content to tools and agents.
* Inspect parsed frontmatter and link extraction.
* Validate output contracts in tests.

## Source Anchors

* `packages/cli/internal/okf/bundle.go`
* `packages/cli/internal/okf/export_test.go`
* `packages/cli/cmd/openknowledge/main.go`

## Update Notes

When JSON fields, issue inclusion, link metadata, frontmatter handling, or
stdout versus file behavior changes, update this page and [CLI changelog](/changelog/cli.md).
