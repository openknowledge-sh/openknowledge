---
type: Command Documentation
title: openknowledge ast
description: Prints the parsed Open Knowledge Format AST as JSON.
tags: [openknowledge, cli, command, ast, parser]
timestamp: 2026-06-20T00:00:00Z
---

# `openknowledge ast`

`openknowledge ast` prints the parser's Open Knowledge Format AST as formatted
JSON. Use it to inspect how files, frontmatter, metadata, body content, links,
and parser diagnostics are represented before validation reports or exporters
convert the parsed model into their own output shapes.

## Usage

```sh
openknowledge ast [path]
openknowledge ast --spec <version> [path]
openknowledge ast --out <file> [path]
openknowledge ast --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `path` | argument | Knowledge base root. Defaults to the current directory. |
| `--spec` | flag | OKF spec version. Defaults to latest. |
| `--out` | flag | Output file. Defaults to stdout. |

## Behavior

The command resolves registry keys and local paths with the same root resolver
used by other bundle commands. It then runs the OKF AST parser directly and
prints the parsed `ASTBundle` as lower-camel-case JSON.

The output is a diagnostics surface, not the normalized JSON exporter contract.
For each document, it includes bundle-relative identity, parser classification,
raw content, parsed frontmatter values, derived metadata, Markdown body, a
structural Markdown tree, resolved links, and read/UTF-8/frontmatter diagnostics
when present. Validation errors and warnings remain available through
`openknowledge validate`.

The `markdown` field is the parser-owned Markdown structure for a document. It
currently includes ordered block nodes, headings with source lines and anchors,
Markdown links/images, and fenced code blocks with Mermaid detection. The raw
`body` field remains available for debugging and compatibility while linter and
exporter paths migrate onto the structural tree.

## Use Cases

* Inspect how a Markdown file is classified before debugging validation output.
* Confirm frontmatter and derived metadata before wiring a linter or exporter
  to the AST model.
* Save a parser snapshot with `--out` when comparing AST refactors.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/ast_command.go`
> * `packages/cli/internal/okf/ast_bundle_parse.go`
> * `packages/cli/internal/okf/ast_document_types.go`
> * `packages/cli/internal/okf/ast_markdown_types.go`
>
> **Update notes**
>
> When AST JSON fields, parser diagnostics, command flags, or exit behavior
> change, update this page and [CLI changelog](/changelog/cli.md).
