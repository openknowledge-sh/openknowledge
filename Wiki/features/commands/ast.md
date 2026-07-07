---
type: Command Documentation
title: openknowledge ast
description: Prints the parsed Open Knowledge Format AST as JSON.
tags: [openknowledge, cli, command, ast, parser]
timestamp: 2026-06-20T00:00:00Z
---

# `openknowledge ast`

`openknowledge ast` shows the parser view of a knowledge base as formatted
JSON. Use it to explore the structure the CLI builds from Markdown files:
frontmatter, derived metadata, body content, headings, links, Markdown blocks,
sections, and parser diagnostics.

This parser model is the shared base for downstream CLI behavior. Validation,
listing, bundle/export projections, search, context selection, and the local
viewer all work from the AST or from data derived from it. For everyday
pass/fail checks, start with [`openknowledge validate`](validate.md); use `ast`
when you want to inspect the underlying document model directly.

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

## Quick Examples

Print the AST for the current directory:

```sh
openknowledge ast
```

Inspect another knowledge base:

```sh
openknowledge ast ./project-memory
```

Save a parser snapshot for comparison:

```sh
openknowledge ast ./project-memory --out ast.json
```

Use a specific OKF spec version:

```sh
openknowledge ast --spec 0.1 ./project-memory
```

## Example Output

`openknowledge ast ./project-memory` prints formatted JSON. The top-level shape
starts like:

```json
{
  "root": "/work/project-memory",
  "specVersion": "0.1",
  "documents": [
    {
      "rel": "AGENTS.md",
      "id": "AGENTS",
      "kind": "concept",
      "frontmatter": {
        "has": true
      },
      "metadata": {
        "type": "Agent Rules",
        "title": "Project Memory Agent Rules"
      }
    }
  ]
}
```

When `--out ast.json` is used, stdout is a short write summary:

```text
OK Wrote AST
root /work/project-memory
out ast.json
```

## Behavior

The command resolves `path` with the same knowledge-root resolver used by other
bundle commands. It then parses Markdown files, skips `.git`, sorts documents
by bundle-relative path, and prints a lower-camel-case JSON `ASTBundle`.

The JSON is a parser diagnostics surface, not the normalized exporter contract.
Its top-level fields are:

| Field | Meaning |
| --- | --- |
| `root` | Absolute path to the resolved knowledge root. |
| `specVersion` | OKF spec version used for parsing. |
| `documents` | Parsed Markdown documents in bundle-relative path order. |

Each `documents[]` item contains the main pieces the CLI derives from a file:

| Field | Use it to inspect |
| --- | --- |
| `rel`, `id`, `kind`, `reserved` | Which file was parsed and how the CLI classified it. |
| `frontmatter` | Raw parsed top-level frontmatter values and formatting warnings. |
| `metadata` | Derived fields such as `type`, `title`, and `description`. |
| `body` | Markdown content after frontmatter. |
| `markdown` | Parsed Markdown blocks, sections, headings, links, code blocks, and syntax diagnostics. |
| `links` | Local links resolved from the document. |
| `readDiagnostic`, `utf8Diagnostic`, `frontmatterDiagnostic` | Parser-level failures attached to the document. |

`markdown.sections` is the section tree used by context export and related
features that need heading boundaries. `markdown.blocks` is the block model
used by rendering and diagnostics. Search, context, link resolution,
validation, bundle generation, and HTML rendering consume this parsed model or
projections derived from it instead of each command inventing its own Markdown
parse.

When `--out` is omitted, JSON is written to stdout. When `--out` is present,
the command writes the JSON file and prints a short success summary.

## Use Cases

* Explore the parsed structure of a knowledge base before building on top of
  it.
* Understand what downstream commands receive after Markdown has been parsed
  into the shared CLI model.
* Check whether a file is being treated as an index, log, reserved file, or
  concept document.
* Confirm that frontmatter produced the expected `type`, `title`, and
  `description` metadata.
* Debug why search, context selection, the local viewer, or an exporter is
  missing a heading, link, table, list, or code block.
* Inspect parser diagnostics before deciding whether the issue belongs in
  Markdown content, frontmatter formatting, or validation rules.
* Save an AST snapshot with `--out` when comparing parser changes.

## Caveats

`openknowledge ast` is intentionally verbose. Prefer `openknowledge validate`
for pass/fail checks and user-facing validation messages.

The AST JSON follows the CLI's internal parser model. It is useful for
debugging and tests, but exporters may expose smaller, normalized output
contracts.

The command does not modify the knowledge base. The only write side effect is
the file passed to `--out`.

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
