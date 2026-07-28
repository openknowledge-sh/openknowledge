---
type: Command Documentation
title: openknowledge ast
description: Print the parsed Open Knowledge Format document model as JSON.
tags: [openknowledge, cli, command, ast, parser]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge ast`

Inspect the parser model that validation, search, listing, rendering, and
exporters use. Use [`validate`](validate.md) for pass or fail checks.

## Usage

```sh
okn ast [key-or-path]
okn ast Wiki --spec 0.1
okn ast Wiki --out ast.json
```

| Option | Default | Description |
| --- | --- | --- |
| `key-or-path` | `.` | Registry key or bundle root. |
| `--spec <version>` | `latest` | OKF spec version. |
| `--out <file>` | stdout | Atomically write the JSON document. |

## Output

The v1 AST contains the resolved root and spec version. It also contains
Markdown documents in path order. Each document can include:

- Source identity and classification: `rel`, `id`, `kind`, and `reserved`.
- Complete content and body.
- Typed YAML frontmatter and compatible scalar values.
- Derived title, type, description, tags, resource, and bundle metadata.
- Markdown blocks, sections, headings, links, and code blocks.
- Read, UTF-8, frontmatter, and Markdown diagnostics.

```json
{
  "schemaVersion": "1",
  "root": "/work/project-memory",
  "specVersion": "0.1",
  "documents": [
    {
      "rel": "AGENTS.md",
      "id": "AGENTS",
      "kind": "concept",
      "metadata": {"type": "Agent Rules"}
    }
  ]
}
```

The `ast.schema.json` file defines the contract. AST output is a detailed
diagnostic format. [`export json`](/features/exporters/json.md) provides a
smaller normalized bundle model.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/ast_command.go`
> - `packages/cli/internal/okf/ast_bundle_parse.go`
> - `packages/cli/internal/okf/ast_document_types.go`
> - `packages/cli/schemas/v1/ast.schema.json`
