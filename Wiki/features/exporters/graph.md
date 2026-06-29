---
type: Exporter Documentation
title: Graph Exporter
description: JSON graph export target for Open Knowledge bundle link structure.
tags: [openknowledge, cli, exporter, graph]
timestamp: 2026-06-18T00:00:00Z
status: shipped
---

# Graph Exporter

`openknowledge to graph` writes an AST-backed node and edge graph for an Open
Knowledge bundle.

## Usage

```sh
openknowledge to graph [path]
openknowledge to graph --out <file> [path]
openknowledge to graph --spec <version> [path]
openknowledge to graph --help
```

## Output

The graph JSON includes:

* `root` and `specVersion` for bundle context.
* `nodes` for every parsed bundle file, including reserved files such as
  `index.md` and `log.md`.
* `edges` for deduplicated, existing, non-self local Markdown links.
* bundle and node `issues` when validation produced warnings or errors.

Edges use source and target Markdown paths, include source and target document
IDs, and preserve link labels, hrefs, and line numbers when available. Missing
local link targets remain validation issues instead of becoming dangling graph
nodes.

## Behavior

`to graph` uses the same AST-backed bundle parser as `to json` and the viewer
knowledge graph. Markdown links inside fenced code blocks are ignored by the
AST parser and therefore do not become graph edges.

The command prints graph JSON to stdout by default. `--out <file>` writes the
same JSON to disk. `--plain` is not valid for graph output.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/graph.go`
> * `packages/cli/internal/okf/graph_types.go`
> * `packages/cli/internal/okf/bundle.go`
> * `packages/cli/internal/okf/ast_links.go`
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/cmd/openknowledge/viewer.go`
>
> **Update notes**
>
> Graph output changes should update [openknowledge to](/features/commands/to.md),
> README command tables, root help, and [CLI changelog](/changelog/cli.md).
