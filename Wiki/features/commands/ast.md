---
type: Command Documentation
title: openknowledge ast
description: Print the parsed Open Knowledge Format document model as JSON.
tags: [openknowledge, cli, command, ast, parser]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge ast`

Inspect the parser model shared by validation, search, listing, rendering, and
exporters. Use [`validate`](validate.md) for ordinary pass/fail checks.

## Usage

```sh
openknowledge ast [key-or-path]
openknowledge ast Wiki --spec 0.1
openknowledge ast Wiki --out ast.json
```

| Option | Default | Description |
| --- | --- | --- |
| `key-or-path` | `.` | Registry key or bundle root. |
| `--spec <version>` | `latest` | OKF spec version. |
| `--out <file>` | stdout | Atomically write the JSON document. |

## Output

The v1 AST contains the resolved root, spec version, and path-sorted Markdown
documents. Each document may include:

- source identity and classification (`rel`, `id`, `kind`, `reserved`);
- complete content and body;
- typed YAML frontmatter and compatible scalar values;
- derived title, type, description, tags, resource, and bundle metadata;
- Markdown blocks, sections, headings, links, and code blocks;
- read, UTF-8, frontmatter, and Markdown diagnostics.

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

The contract is published as `ast.schema.json`. This is an intentionally
verbose diagnostics surface; [`export json`](/features/exporters/json.md)
provides a smaller normalized bundle model.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/ast_command.go`
> - `packages/cli/internal/okf/ast_bundle_parse.go`
> - `packages/cli/internal/okf/ast_document_types.go`
> - `packages/cli/schemas/v1/ast.schema.json`
