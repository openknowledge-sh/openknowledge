---
type: Exporter Documentation
title: Graph Exporter
description: JSON graph export target for Open Knowledge source and search graph structure.
tags: [openknowledge, cli, exporter, graph]
timestamp: 2026-07-21T00:00:00Z
status: shipped
---

# Graph Exporter

`openknowledge export graph` writes AST-backed graph JSON for an Open Knowledge bundle.
The default source graph describes authored files and local links.
The search graph is a retrieval layer.
The CLI builds this layer from Markdown heading chunks.

These outputs are structural document and chunk link graphs.
They are not entity-resolved semantic knowledge graphs.
They do not infer domain entities or relationship predicates from prose.

## Usage

```sh
openknowledge export graph [key-or-path]
openknowledge export graph --out <file> [key-or-path]
openknowledge export graph --type source [key-or-path]
openknowledge export graph --type search [key-or-path]
openknowledge export graph --spec <version> [key-or-path]
openknowledge export graph --help
```

## Types

| Type | Description |
| --- | --- |
| `source` | Default graph. Each node represents a parsed bundle file. Each existing non-self local Markdown link occurrence is an edge. This includes parallel links. |
| `search` | Derivative search graph. Nodes include bundle files and content-bearing H1-H3 Markdown chunks. Edges include containment, reading order, and local links. |

## Output

All graph JSON includes:

* `schemaVersion` for the CLI graph contract. The current value is `"1"`.
* `root` and `specVersion` for the bundle context.
* `type`. The current value is `source` or `search`.
* `nodes`.
* `edges`.
* Bundle and node `issues` when validation finds warnings or errors.

Source graph nodes represent parsed bundle files.
The nodes include reserved files such as `index.md` and `log.md`.
Source graph edges use source and target Markdown paths.
Each edge includes source and target document IDs.
It preserves link labels, hrefs, target anchors, and available line numbers.
Parallel links between the same files remain separate edges.
A missing local link target causes a validation issue.
It does not create a dangling graph node.

Search graph output includes the source graph.
It also includes content-bearing H1-H3 heading chunk nodes with `kind: "chunk"`.
H4-H6 headings remain in their surrounding chunk.
The output omits heading-only parent sections.
Chunk nodes preserve `path`, `heading`, `headingPath`, `lineStart`, and `lineEnd`.
Search graph edges have these kinds:

* `contains` links a source file to one of its chunks.
* `next` links adjacent chunks in source order.
* `local-link` links a source chunk to the addressed content-bearing target chunk.
  The source chunk must contain an existing local Markdown link.

A fragment selects the chunk that owns the canonical target heading. H4-H6
headings select their containing H1-H3 chunk.
A heading-only H1-H3 target selects its first content-bearing descendant.
A link without a fragment selects the first content-bearing chunk.
An unresolved fragment does not create a chunk edge.
Repeated authored links remain distinct chunk edges.

## Behavior

`export graph` uses the same AST-backed bundle parser as `export json`.
The viewer knowledge graph and CLI search chunking use this parser too.
The AST parser ignores Markdown links in fenced code blocks.
Therefore, these links do not become graph edges.

The command accepts a registry key or bundle path.
By default, it prints graph JSON to stdout.
`--out <file>` writes the same JSON to a file.
The command does not accept `--plain`.
An unsupported `--type` exits with status `1`.
An unknown flag is a usage error with status `2`.
`packages/cli/schemas/v1/graph.schema.json` defines the v1 contract.

## Use cases

* Export a source-grounded link graph for visualization.
* Export a search graph for retrieval tools, graph-expanded search, or MCP adapters.
* Find isolated or weakly connected bundle areas. Do not change authored
  Markdown.


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
> After a graph output change, update [openknowledge export](/features/commands/export.md).
> Also update the README command tables, root help, and [CLI changelog](/changelog/cli.md).
