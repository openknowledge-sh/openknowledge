---
type: Exporter Documentation
title: JSON Exporter
description: Normalized JSON export behavior for Open Knowledge bundles.
tags: [openknowledge, cli, exporter, json]
timestamp: 2026-06-18T00:00:00Z
---

# JSON Exporter

The JSON exporter serializes a normalized model of the bundle's parsed
Markdown documents. It includes document metadata, typed YAML frontmatter
values, Markdown body content, links, and validation issues. Non-Markdown
assets are not part of this projection.

## Command

```sh
openknowledge export json [key-or-path]
openknowledge export json --out <file> [key-or-path]
openknowledge export json --spec <version> [key-or-path]
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `key-or-path` | argument | Registry key or knowledge base root. Defaults to the current directory. |
| `--out` | flag | Optional output file. Defaults to stdout. |
| `--spec` | flag | OKF spec version. Defaults to latest. |

## Behavior

`export json` parses and validates the bundle before serialization. Validation
errors and warnings are included in the top-level `issues` array and attached to
matching files. When `--out` is omitted, the JSON is printed to stdout. The
HTML-only `--plain` flag is rejected for JSON.

The `files` array contains parsed `.md` and `.markdown` documents only.
Non-Markdown assets remain visible through `openknowledge list --json` and are
preserved by `openknowledge export tar`.

The top-level object declares `schemaVersion: "1"` for the normalized CLI JSON
contract and `specVersion` for the selected Open Knowledge Format version.
These versions are independent. The v1 JSON Schema is available at
`packages/cli/schemas/v1/bundle.schema.json` and is protected by a golden
contract test.

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

## Exporter Change History

### 2026-07-15 - Versioned normalized JSON

Normalized bundle JSON now declares `schemaVersion: "1"` and ships with a
Draft 2020-12 JSON Schema and golden snapshot.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/bundle.go`
> * `packages/cli/internal/okf/bundle_types.go`
> * `packages/cli/internal/okf/export_test.go`
> * `packages/cli/schemas/v1/bundle.schema.json`
> * `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> When JSON fields, issue inclusion, link metadata, frontmatter handling, or
> stdout versus file behavior changes, update this page and [CLI changelog](/changelog/cli.md).
