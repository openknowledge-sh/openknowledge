---
type: Exporter Documentation
title: Graph Exporter
description: JSON graph export target for Open Knowledge source and search graph structure.
tags: [openknowledge, cli, exporter, graph]
timestamp: 2026-06-18T00:00:00Z
status: shipped
---

# Graph Exporter

`openknowledge to graph` writes AST-backed graph JSON for an Open Knowledge
bundle. The default source graph describes authored files and local links.
The search graph is a derivative retrieval layer built from Markdown heading
chunks.

## Usage

```sh
openknowledge to graph [path]
openknowledge to graph --out <file> [path]
openknowledge to graph --type source [path]
openknowledge to graph --type search [path]
openknowledge to graph --spec <version> [path]
openknowledge to graph --help
```

## Types

| Type | Description |
| --- | --- |
| `source` | Default graph. Nodes are parsed bundle files; edges are deduplicated existing local Markdown links. |
| `search` | Derivative search graph. Nodes include bundle files and Markdown heading chunks; edges include containment, reading order, and chunk-level local links. |

## Output

All graph JSON includes:

* `schemaVersion`, currently `"1"`, for the CLI graph contract.
* `root` and `specVersion` for bundle context.
* `type`, currently `source` or `search`.
* `nodes`.
* `edges`.
* bundle and node `issues` when validation produced warnings or errors.

Source graph nodes represent parsed bundle files, including reserved files such
as `index.md` and `log.md`. Source graph edges use source and target Markdown
paths, include source and target document IDs, and preserve link labels, hrefs,
and line numbers when available. Missing local link targets remain validation
issues instead of becoming dangling graph nodes.

Search graph output includes the source graph plus content-bearing heading
chunk nodes where `kind: "chunk"`. Heading-only parent sections are omitted.
Chunk nodes preserve `path`, `heading`, `headingPath`, `lineStart`, and
`lineEnd`. Search graph edge kinds include:

* `contains` from a source file to one of its chunks.
* `next` between adjacent chunks in source order.
* `local-link` from the source chunk containing an existing local Markdown link
  to the first chunk of the linked target file.

## Behavior

`to graph` uses the same AST-backed bundle parser as `to json`, the viewer
knowledge graph, and CLI search chunking. Markdown links inside fenced code
blocks are ignored by the AST parser and therefore do not become graph edges.

The command prints graph JSON to stdout by default. `--out <file>` writes the
same JSON to disk. `--plain` is not valid for graph output. Unknown graph types
exit with status `2`. The v1 contract is described by
`packages/cli/schemas/v1/graph.schema.json`.

## Use Cases

* Export a source-grounded link graph for visualization.
* Export a search graph for retrieval tooling, graph-expanded search, or MCP
  search adapters.
* Inspect orphaned or weakly connected bundle areas without changing authored
  Markdown.

## Command Change History

### 2026-07-15 - Versioned graph JSON

Source and search graph JSON now declare `schemaVersion: "1"` and share a
checked JSON Schema plus golden snapshot.

### 2026-07-06

`openknowledge to graph` added `--type source|search`. The default source graph
keeps the existing file/link model. The search graph adds derivative chunk
nodes and typed graph edges for source-grounded retrieval.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/graph.go`
> * `packages/cli/internal/okf/graph_types.go`
> * `packages/cli/schemas/v1/graph.schema.json`
> * `packages/cli/internal/okf/bundle.go`
> * `packages/cli/internal/okf/ast_links.go`
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/cmd/openknowledge/viewer.go`
>
> **Update notes**
>
> Graph output changes should update [openknowledge to](/features/commands/to.md),
> README command tables, root help, and [CLI changelog](/changelog/cli.md).
