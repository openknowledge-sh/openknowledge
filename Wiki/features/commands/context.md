---
type: Command Documentation
title: openknowledge context
description: Prints query-focused Markdown sections from an OKF bundle for token-bounded agent context.
tags: [openknowledge, cli, command, context, agents]
timestamp: 2026-06-20T00:00:00Z
---

# `openknowledge context`

`openknowledge context` prints query-focused excerpts from a local or connected
Open Knowledge bundle. It is the token-bounded reading path for agents that need
relevant source material without loading whole Markdown files.

The command does not use embeddings or generate summaries. It builds a
section-level index from Markdown headings, scores original sections with
lexical matches over metadata, paths, headings, and body text, then packs the
highest-scoring excerpts into an approximate token budget.

## Usage

```sh
openknowledge context --query <text>
openknowledge context <name-or-path> --query <text>
openknowledge context <name-or-path> --query <text> --budget <tokens>
openknowledge context <name-or-path> --query <text> --format json
openknowledge context --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `name-or-path` | argument | Registry key or local bundle path. Defaults to the current directory. |
| `--query` | flag | Required search query used to select sections. |
| `--budget` | flag | Approximate output token budget. Defaults to `2400`. |
| `--limit` | flag | Maximum number of sections. Defaults to `12`. |
| `--format` | flag | `markdown` or `json`. Defaults to `markdown`. |
| `--spec` | flag | OKF spec version. Defaults to `latest`. |

## Behavior

The context index splits Markdown files on `#`, `##`, and `###` headings while
ignoring headings inside fenced code blocks. Content before the first heading is
treated as a `Top` section. Each result keeps the original Markdown excerpt,
file path, source line range, score, and estimated token count.

Markdown output starts with query and budget metadata, then lists source ranges
before printing excerpts. JSON output returns the same result model for tools
that want to pack or inspect context themselves.

When a selected section links to another local Markdown file and budget remains,
`context` may include that target's first section as a neighbor result. Neighbor
sections are marked in JSON and in the Markdown source list.

## Quick Examples

```sh
openknowledge context Wiki --query "validation workflow"
openknowledge context personal --query "release checklist" --budget 1200
openknowledge context personal --query "release checklist" --format json
```

## Use Cases

* Give an agent relevant bundle excerpts without printing whole files.
* Inspect which sections the lexical context resolver considers relevant.
* Feed source-cited excerpts into another tool through JSON.
* Keep token usage predictable before adding optional embedding providers.

## Command Change History

### 2026-06-20

`openknowledge context` shipped with section-level lexical retrieval,
approximate token budgeting, Markdown and JSON output, local linked-neighbor
inclusion, and `--query`, `--budget`, `--limit`, `--format`, and `--spec`
flags.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/context.go`
> - `packages/cli/internal/okf/context_test.go`
> - `packages/cli/cmd/openknowledge/main.go`
> - `packages/cli/cmd/openknowledge/main_test.go`
>
> **Update notes**
>
> Update this page when context scoring, section splitting, output fields,
> command flags, or token budget behavior changes. CLI behavior changes also
> require [CLI changelog](/changelog/cli.md) updates.
