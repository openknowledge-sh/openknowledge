---
type: Exporter Documentation
title: JSON Exporter
description: Normalized JSON export behavior for Open Knowledge bundles.
tags: [openknowledge, cli, exporter, json]
timestamp: 2026-06-18T00:00:00Z
---

# JSON Exporter

The JSON exporter serializes a normalized bundle model. It includes file
metadata, typed YAML frontmatter values, Markdown body content, links, and
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

## Behavior

`to json` parses and validates the bundle before serialization. Validation
errors and warnings are included in the top-level `issues` array and attached to
matching files. When `--out` is omitted, the JSON is printed to stdout. The
HTML-only `--plain` flag is rejected for JSON.

Link entries include their kind, source line, local target path and ID, and
whether the target exists. Directory links are marked existing when they resolve
to an `index.md` file in that directory.

Each file's `frontmatter` preserves YAML mappings and sequences as JSON objects
and arrays, scalar types as JSON-compatible values, and block scalar content as
strings. Export no longer flattens nested values or substitutes YAML syntax
markers such as `|` for decoded content.

## Use Cases

* Feed bundle content to tools and agents.
* Inspect parsed frontmatter and link extraction.
* Validate output contracts in tests.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/bundle.go`
> * `packages/cli/internal/okf/bundle_types.go`
> * `packages/cli/internal/okf/export_test.go`
> * `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> When JSON fields, issue inclusion, link metadata, frontmatter handling, or
> stdout versus file behavior changes, update this page and [CLI changelog](/changelog/cli.md).
