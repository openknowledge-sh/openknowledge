---
type: Command Documentation
title: openknowledge search
description: Searches source-grounded Markdown chunks in a local or connected OKF bundle.
tags: [openknowledge, cli, command, search, graph]
timestamp: 2026-07-06T00:00:00Z
---

# `openknowledge search`

`openknowledge search` searches Markdown chunks from a local or connected Open
Knowledge bundle. It resolves the target the same way as other key-or-path
commands, parses Markdown with the AST-backed bundle reader, splits content by
heading sections, and returns source-grounded matches with file paths, line
ranges, heading paths, snippets, and scores.

Use `search` when an agent or human needs to find relevant knowledge. Use
[`use`](use.md) when the caller already knows which entrypoint or bundle file
to load.

## Usage

```sh
openknowledge search <name-or-path> <query>
openknowledge search <name-or-path> <query> --format json
openknowledge search <name-or-path> <query> --expand graph
openknowledge search <name-or-path> <query> --limit <count>
openknowledge search <name-or-path> <query> --spec <version>
openknowledge search --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `name-or-path` | argument | Registry key or local bundle path. |
| `query` | argument | Search text. Shell users should quote multi-word queries. |
| `--expand` | flag | Optional expansion mode. `graph` includes outgoing links and backlinks as lower-ranked neighbor results. |
| `--format` | flag | Output format, `text` or `json`. Defaults to `text`. |
| `--limit` | flag | Maximum result count. Defaults to `12`. |
| `--spec` | flag | OKF spec version. Defaults to `latest`. |

## Behavior

Search chunks are Markdown heading sections with content, not arbitrary
fixed-size token windows. Heading-only parent sections are omitted so results
prefer snippets with source prose, lists, code, or other useful content. Each
result records the bundle file, section ID, heading, heading path, source line
range, estimated token count, snippet, score, and matched field names.

Ranking is lexical and deterministic. The scorer uses BM25-style term
saturation and length normalization across weighted fields:

* title
* heading
* heading path
* path and section ID
* type and kind
* description
* frontmatter
* section body

Exact phrase matches, all-query-term coverage, prefixes, fuzzy matches, and
diacritic-insensitive normalization affect the score. `index.md` sections are
downweighted so focused concept pages can outrank broad index pages.

With `--expand graph`, direct matches are followed by graph-neighbor results
when room remains under `--limit`. Outgoing neighbors come from existing local
Markdown links inside a matched section. Backlink neighbors come from sections
that link to the matched file. Neighbor results are marked with
`neighbor: true` and a `relation` such as `outgoing-link` or `backlink` in JSON
output.

## Output

Text output is designed for terminals and agent logs. It prints the query,
root, result count, then ranked result blocks with `path:line-line`, heading,
heading path, type, score, relation, and snippet.

JSON output returns:

* `root`, `query`, and `limit` metadata.
* `results`, each with source path, section ID, kind, type, title, heading,
  heading path, line range, estimated token count, snippet, highlight text,
  score, matched fields, and optional neighbor relation.
* `issues` when bundle validation produced warnings or errors while building
  the AST-backed context index.

## Quick Examples

```sh
openknowledge search Wiki "validation workflow"
openknowledge search personal "release checklist" --limit 5
openknowledge search personal "MCP auth" --expand graph
openknowledge search personal "MCP auth" --format json
```

## Caveats

Search does not use embeddings and does not call an LLM. Semantic entity or
relationship extraction belongs in future derivative graph artifacts, not in
the authored OKF Markdown source.

## Command Change History

### 2026-07-06

`openknowledge search` shipped as the query retrieval command. It replaces
`openknowledge use --query`, adds section-level BM25-style ranking, JSON output,
and optional graph expansion through local links and backlinks.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/cmd/openknowledge/main_test.go`
> * `packages/cli/internal/okf/search_knowledge.go`
> * `packages/cli/internal/okf/search_types.go`
> * `packages/cli/internal/okf/context_sections.go`
> * `packages/cli/internal/okf/search_test.go`
>
> **Update notes**
>
> Update this page when search flags, output fields, chunking, ranking,
> expansion behavior, or key/path resolution semantics change. CLI behavior
> changes also require [CLI changelog](/changelog/cli.md) updates.
