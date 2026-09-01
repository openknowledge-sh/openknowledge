---
type: Command Documentation
title: openknowledge search
description: Build source-preserving context from local or connected knowledge bases.
tags: [openknowledge, cli, command, search, context, graph]
timestamp: 2026-07-21T00:00:00Z
---

# `openknowledge search`

Search one managed knowledge base, an ordinary Markdown directory, or all
connected knowledge bases. Search is local and deterministic. It does not call
an LLM or an embedding service.

## Usage

```sh
okn search <key-or-path> <query>
okn search <key-or-path> <query> --budget 1200
okn search <key-or-path> <query> --matches
okn search <key-or-path> <query> --format json
okn search <key-or-path> <query> --filter type=Guide --filter tag=operations
okn search --all <query>
```

| Option | Default | Description |
| --- | --- | --- |
| `--all` | off | Search the current local registry instead of one target. |
| `--budget <tokens>` | `2400` | Approximate context budget. Incompatible with `--matches`. |
| `--limit <count>` | `12` | Maximum selected sources or displayed matches. |
| `--no-expand` | off | Exclude document, linked, and backlink context expansion. |
| `--filter <key=value>` | none | Restrict candidates. Use `type` or `tag`. Repeat as needed. |
| `--matches` | off | Show ranked snippets instead of a context packet. |
| `--format <format>` | `markdown` | `markdown` or `json`. |
| `--spec <version>` | `latest` | OKF version used to read the bundle. |

## Output modes

The default context packet contains the query, root, content revision, token
estimate, validation issues, and selected Markdown sections. Each source
contains its file, heading, line range, score, relationship, and content hash.
It also contains an `okf+sha256://` locator.

Output contains `status: managed` for an OKF bundle. It contains
`status: unmanaged` for an ordinary Markdown directory. Unmanaged search does
not require YAML frontmatter or authored links. Use `okn setup` before
publication or managed validation.

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

Use `--matches` to inspect ranked snippets and matched fields. JSON output uses
`schemaVersion: "1"` and the same `status` values. The
`search-context.schema.json` and `search-results.schema.json` files define the
contracts.

## How selection works

- Search divides Markdown into content sections at H1 through H3 headings.
  Lower headings stay in their parent section.
- BM25 and local vector retrieval create one candidate set. The local vector uses hashed word and character features. It does not use a model or network service.
- A metadata filter runs before candidate selection. Values for one filter use OR matching. Different filters use AND matching.
- A deterministic reranker keeps lexical evidence as the primary score. The vector score breaks close lexical ranks and supplies vector-only candidates.
- BM25-style ranking combines section evidence with a document signal.
  Filenames, titles, headings, paths, frontmatter, metadata, and bodies affect
  the score. The section with the most query terms receives the document
  boost.
- Context mode keeps the five strongest lexical sections. It adds up to two
  sections from the strongest document. It then adds related parent or child
  sections. It also adds evidence for missing query terms. A selected
  nonlexical section uses the `document-context` relation.
- Search adds one level of authored links and backlinks. Use `--no-expand` to
  prevent this expansion. A fragment selects the section that contains its
  heading. A lower heading resolves to its H1 through H3 parent section. A
  heading-only target resolves to its first content section. A link without a
  fragment selects the first content section. Search does not expand a missing
  fragment. It does not follow external, missing, or transitive links.
- An outgoing target receives 55 percent of the seed score. A backlink
  receives 45 percent. A lexical target keeps the higher lexical or graph
  score. It stays one direct result with its lexical snippet and highlights.
  Multiple links do not add their scores.
- The command packs direct evidence first. Document and link context follow.
  It tries the five strongest lexical seeds before it truncates a large seed.
  It truncates a prioritized document context section before it selects a
  lower-ranked short section. Only the final selected section can be
  incomplete.
- `--all` searches the current registry snapshot. It does not refresh remotes.
  Reciprocal-rank fusion combines ranks under one global limit and budget.
  Partial failures stay visible. The command exits with status `1` only when
  all entries in a non-empty registry fail.

The token budget is an estimate. It is not a model-specific tokenizer
guarantee. Use
[`okn get`](get.md) when you already know the exact file to read.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/search_knowledge.go`
> - `packages/cli/internal/okf/loose_markdown.go`
> - `packages/cli/internal/okf/search_vector.go`
> - `packages/cli/internal/okf/search_filters.go`
> - `packages/cli/internal/okf/context.go`
> - `packages/cli/internal/okf/context_selection.go`
> - `packages/cli/internal/okf/federated_search.go`
> - `packages/cli/schemas/v1/search-context.schema.json`
> - `packages/cli/schemas/v1/search-results.schema.json`
>
> **Update notes**
>
> Update this page when search flags, ranking, chunking, expansion, packing, or
> output contracts change.
