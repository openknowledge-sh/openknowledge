---
type: Exporter Documentation
title: JSON Exporter
description: Normalized JSON export behavior for Open Knowledge bundles.
tags: [openknowledge, cli, exporter, json]
timestamp: 2026-06-18T00:00:00Z
---

# JSON Exporter

The JSON exporter serializes a normalized model of parsed Markdown documents.
The model includes document metadata and typed YAML frontmatter values.
It also includes Markdown body content, links, and validation issues.
This projection does not include non-Markdown assets.

## Command

```sh
openknowledge export json [key-or-path]
openknowledge export json --out <file> [key-or-path]
openknowledge export json --spec <version> [key-or-path]
```

## Arguments and flags

| Name | Kind | Description |
| --- | --- | --- |
| `key-or-path` | argument | Registry key or knowledge base root. Defaults to the current directory. |
| `--out` | flag | Optional output file. Defaults to stdout. |
| `--spec` | flag | OKF spec version. Defaults to latest. |

## Behavior

`export json` parses and validates the bundle before serialization.
The top-level `issues` array contains validation errors and warnings.
Each applicable file also contains its validation issues.
Without `--out`, the command prints the JSON to stdout.
The command does not accept the HTML-only `--plain` flag.

The `files` array contains parsed `.md` and `.markdown` documents only.
Non-Markdown assets remain visible through `openknowledge list --json` and are
preserved by `openknowledge export tar`.

The top-level object declares `schemaVersion: "1"` for the normalized CLI JSON contract.
It declares `specVersion` for the selected Open Knowledge Format version.
These versions are independent.
The v1 JSON Schema is at `packages/cli/schemas/v1/bundle.schema.json`.
A golden contract test protects this schema.

Each link entry includes its kind and source line.
It also includes the local target path, target ID, and target status.
A directory link exists when it resolves to an `index.md` file in that directory.

Each file's `frontmatter` preserves YAML mappings and sequences as JSON objects and arrays.
It preserves scalar types as JSON-compatible values.
It preserves block scalar content as strings.
The exporter does not flatten nested values.
It does not replace decoded content with YAML syntax markers such as `|`.

## Use cases

* Provide bundle content to tools and agents.
* Examine parsed frontmatter and extracted links.
* Validate output contracts in tests.


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
> Update this page after a change to JSON fields, issues, links, frontmatter,
> or output destinations.
> Also update the [CLI changelog](/changelog/cli.md).
