---
type: Command Documentation
title: openknowledge search
description: Build source-preserving context from local or connected knowledge bases.
tags: [openknowledge, cli, command, search, context, graph]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge search`

Search one knowledge base—or every connected knowledge base—and return
source-grounded Markdown context. Search is local, lexical, deterministic, and
does not call an LLM.

## Usage

```sh
openknowledge search <key-or-path> <query>
openknowledge search <key-or-path> <query> --budget 1200
openknowledge search <key-or-path> <query> --matches
openknowledge search <key-or-path> <query> --format json
openknowledge search --all <query>
```

| Option | Default | Description |
| --- | --- | --- |
| `--all` | off | Search the current local registry instead of one target. |
| `--budget <tokens>` | `2400` | Approximate context budget; incompatible with `--matches`. |
| `--limit <count>` | `12` | Maximum selected sources or displayed matches. |
| `--no-expand` | off | Exclude linked and backlink context. |
| `--matches` | off | Show ranked snippets instead of a context packet. |
| `--format <format>` | `markdown` | `markdown` or `json`. |
| `--spec <version>` | `latest` | OKF version used to read the bundle. |

## Output modes

The default context packet contains the query, resolved root, content revision,
estimated token use, validation issues, and selected Markdown sections. Each
source includes its file, heading, line range, score, relationship, content
hash, and content-addressed `okf+sha256://` locator.

```text
# Open Knowledge Context

Query: validation workflow
Root: `/work/project-memory`
Context: 412 / 2400 estimated tokens
Sources: 2

## 1. Validation Workflow
Source: `guides/validation.md:7-10`
Relation: `direct`
```

Use `--matches` to inspect ranked snippets and matched fields. JSON versions of
both modes use `schemaVersion: "1"` and the published
`search-context.schema.json` and `search-results.schema.json` contracts.

## How selection works

- Markdown is split at content-bearing H1–H3 sections. Lower headings stay
  within their parent section.
- BM25-style ranking weighs titles, headings, paths, frontmatter, metadata, and
  section bodies. Exact phrases, term coverage, prefixes, fuzzy matches, and
  normalized diacritics affect the score.
- Search adds one hop of authored outgoing links and backlinks unless
  `--no-expand` is set. It never follows external, missing, self, or transitive
  links.
- Direct evidence is packed first, followed by related sections. Only the last
  selected section may be truncated to fit the approximate token budget.
- `--all` searches the current registry snapshot without refreshing remotes.
  Per-bundle ranks are combined with reciprocal-rank fusion under one global
  limit and budget. Partial failures remain visible; the command exits `1` only
  when every entry in a non-empty registry fails.

The token budget is an estimate, not a model-specific tokenizer guarantee. Use
[`openknowledge get`](get.md) when you already know the exact file to read.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/search_knowledge.go`
> - `packages/cli/internal/okf/context.go`
> - `packages/cli/internal/okf/federated_search.go`
> - `packages/cli/schemas/v1/search-context.schema.json`
> - `packages/cli/schemas/v1/search-results.schema.json`
>
> **Update notes**
>
> Update this page when search flags, ranking, chunking, expansion, packing, or
> output contracts change.
