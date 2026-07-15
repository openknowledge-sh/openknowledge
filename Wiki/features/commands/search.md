---
type: Command Documentation
title: openknowledge search
description: Builds source-preserving Markdown context from a local or connected OKF bundle.
tags: [openknowledge, cli, command, search, context, graph]
timestamp: 2026-07-06T00:00:00Z
---

# `openknowledge search`

`openknowledge search` builds a bounded, source-grounded context packet from a
local or connected Open Knowledge bundle. It resolves the target the same way
as other key-or-path commands, parses Markdown with the AST-backed bundle
reader, splits content at H1-H3 heading sections, and ranks those sections with
the canonical BM25-style search scorer.

The default Markdown output preserves the selected source sections and records
their file paths, headings, line ranges, scores, and direct or linked
relationship to the query. Use [`get`](get.md) when the caller already knows
which entrypoint or bundle file to load. Use `--matches` when a human or agent
needs to inspect ranked snippets instead of consuming a context packet.

## Usage

```sh
openknowledge search <name-or-path> <query>
openknowledge search <name-or-path> <query> --budget <tokens>
openknowledge search <name-or-path> <query> --no-expand
openknowledge search <name-or-path> <query> --matches
openknowledge search <name-or-path> <query> --format json
openknowledge search <name-or-path> <query> --limit <count>
openknowledge search <name-or-path> <query> --spec <version>
openknowledge search --all <query>
openknowledge search --all <query> --matches --format json
openknowledge search --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `name-or-path` | argument | Registry key or local bundle path. |
| `query` | argument | Search text. Shell users should quote multi-word queries. |
| `--budget` | flag | Approximate maximum token count for a context packet. Defaults to `2400`; cannot be combined with `--matches`. |
| `--all` | flag | Replace the single target with every current registry entry and fuse their local ranks. |
| `--no-expand` | flag | Return only sections that directly match the query; do not add outgoing-link or backlink context. |
| `--matches` | flag | Print the ranked match-list inspection view instead of a source-preserving context packet. |
| `--format` | flag | Output format, `markdown` or `json`. Defaults to `markdown`. |
| `--limit` | flag | Maximum number of selected context sources or displayed matches. Defaults to `12`. |
| `--spec` | flag | OKF spec version. Defaults to `latest`. |

## Behavior

Search chunks are Markdown sections split at content-bearing H1, H2, and H3
headings, not arbitrary fixed-size token windows. H4-H6 headings remain inside
their surrounding H1-H3 chunk. Heading-only parent sections are omitted so
selected sources contain useful prose, lists, code, or other authored content.
Each source keeps its bundle file, section ID, heading and heading path, source
line range, estimated token count, original Markdown, content digest, and
content-addressed locator.

Every retrieval computes a deterministic SHA-256 revision over the indexed
Markdown paths and bytes and records the concrete OKF spec version. Context and
match output from the same source generation therefore share one revision.
Each section separately hashes its normalized Markdown and exposes an opaque
`okf+sha256://` locator containing the index revision, escaped bundle path, and
section digest. A stored locator changes when either the corpus generation or
the cited section changes, even if a duplicate heading shifts the legacy
human-readable section ID.

Ranking is lexical and deterministic. The canonical scorer uses BM25-style
term saturation and length normalization across weighted fields:

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

After ranking direct matches, search performs one shallow graph-expansion hop
by default. It considers existing local Markdown links from a directly matched
section and sections that link back to the matched file. External, missing,
self-referential, and deeper transitive links are not expanded. Related
sections are labeled `outgoing-link` or `backlink`, receive relation-weighted
scores, and are included only when they fit the remaining token budget and
source limit. `--no-expand` disables this step.

The context packer selects direct evidence first, then related sections. It is
deterministic, deduplicates sections, stops at `--limit`, and respects the
approximate `--budget`; if needed, only the final selected section is
truncated. Search does not generate a summary or answer. Its default output is
transparent context for the caller's chosen agent or other downstream tool.

### Registry-wide federation

`--all` reads the current local registry snapshot and searches every entry; it
does not contact or refresh managed remotes. BM25 scores from independently
indexed bundles are not directly comparable, so federation ranks candidates
inside each knowledge base and then applies reciprocal-rank fusion (RRF) with
rank constant `60`. Integer local rank is the primary ordering key; registry
key, path, and line provide deterministic tie-breaks. The source limit and
context token budget apply once after fusion, not separately per bundle.

Each wrapper records the registry key, local rank, fusion score, and unchanged
single-bundle source/result. Item identity is `(knowledgeBase, locator)`;
mirrored bundles remain separate evidence and are neither deduplicated nor
score-summed. One unavailable or invalid entry appears with `status: "error"`
while healthy results remain usable. An empty registry is a successful empty
result. If every entry in a non-empty registry fails, the complete report is
still printed and the command exits `1`.

## Output

### Markdown context

The default output is a source-preserving Markdown context packet. It includes
query and budget metadata followed by each selected source, its provenance,
relationship, and Markdown excerpt:

```md
# Open Knowledge Context

Query: validation workflow
Root: `/work/project-memory`
Revision: `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa` (OKF 0.1)
Context: 412 / 2400 estimated tokens
Sources: 2
Validation issues: 0

## 1. Validation Workflow

Source: `guides/validation.md:7-10`
Locator: `okf+sha256://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/guides%2Fvalidation.md#bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb`
Relation: `direct`
Score: `527.86`

## Validation Workflow

Run `openknowledge validate` before sharing the wiki.

## 2. Release Checklist

Source: `workflows/release.md:20-27`
Relation: `outgoing-link`
Score: `290.32`

## Release Checklist

Validate the wiki before publishing release documentation.
```

The excerpt body remains authored Markdown. The surrounding context headings
and provenance lines identify where each section came from when output is
piped directly to an agent or stored in a file.

### JSON context

`--format json` returns the same context packet as structured data:

```json
{
  "schemaVersion": "1",
  "root": "/work/project-memory",
  "revision": {
    "specVersion": "0.1",
    "indexSha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  },
  "query": "validation workflow",
  "budget": 2400,
  "estimatedTokens": 412,
  "limit": 12,
  "sources": [
    {
      "path": "guides/validation.md",
      "id": "guides/validation#validation-workflow",
      "locator": "okf+sha256://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/guides%2Fvalidation.md#bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "contentSha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "kind": "concept",
      "type": "Guide",
      "title": "Validation",
      "heading": "Validation Workflow",
      "headingPath": ["Validation Workflow"],
      "headingLevel": 2,
      "lineStart": 7,
      "lineEnd": 10,
      "score": 527.86,
      "estimatedTokens": 32,
      "relation": "direct",
      "markdown": "## Validation Workflow\n\nRun `openknowledge validate` before sharing the wiki."
    }
  ],
  "issues": []
}
```

Each source includes `path`, `id`, `locator`, `contentSha256`, `kind`, `title`,
and the source fields `heading`,
`headingPath`, `headingLevel`, `lineStart`, `lineEnd`, `score`,
`estimatedTokens`, `relation`, and `markdown`. The top level reports the
resolved root, query, budget, estimated token use, source limit, selected
sources, concrete `revision`, and any validation issues encountered while building the AST-backed
context index. Both JSON search shapes declare `schemaVersion: "1"`; their
contracts are described by `search-context.schema.json` and
`search-results.schema.json` under `packages/cli/schemas/v1/`.

### Ranked matches

`--matches` selects the ranked match-list inspection view. Markdown output
shows result blocks with source location, title or heading, heading path, type,
score, relation, and snippet. Related results are merged into this diagnostic
ranking with a relationship penalty unless `--no-expand` is set.
`--matches --format json` returns the ranked search result model with snippets,
highlights, matched fields, and neighbor relations. In match-list mode,
`--limit` caps displayed matches; the token budget applies to context packets
rather than snippet inspection.

## Quick Examples

```sh
# Default agent-ready Markdown context with linked supporting sections.
openknowledge search Wiki "validation workflow"

# Fit the context into a smaller prompt budget.
openknowledge search personal "release checklist" --budget 1200

# Include only lexical matches.
openknowledge search personal "MCP auth" --no-expand

# Inspect the underlying ranked snippets.
openknowledge search personal "MCP auth" --matches

# Consume a structured context packet.
openknowledge search personal "MCP auth" --format json

# Search all current registry entries under one global budget.
openknowledge search --all "MCP auth" --budget 1600
```

## Caveats

Search does not use embeddings and does not call an LLM. Graph expansion uses
authored local Markdown links and backlinks only. Semantic entity or
relationship extraction belongs in future derivative graph artifacts, not in
the authored OKF Markdown source.

The budget is an estimate rather than a tokenizer-specific guarantee because
different model families count Markdown tokens differently.

## Command Change History

### 2026-07-15 - Registry-wide federated search

Added `--all` for context and ranked-match modes. Per-bundle canonical ranks
are fused with documented RRF metadata, results are namespaced by registry key,
budget and limit are global, and per-entry failures are isolated in the
versioned federated report. Source anchors:
`packages/cli/cmd/openknowledge/main.go`,
`packages/cli/internal/okf/federated_search.go`, and
`packages/cli/internal/okf/federated_search_test.go`.

### 2026-07-15 - Revision-bound retrieval provenance

Context and match outputs now bind every response to the concrete OKF spec and
a deterministic indexed-Markdown SHA-256. Every returned section includes its
own content digest and opaque revision-bound locator; Markdown output exposes
the same provenance alongside source line ranges. The CLI, Go API, and MCP
structured content share this model.

### 2026-07-15 - Versioned search JSON

Context packets and ranked match JSON now declare `schemaVersion: "1"` and
have checked JSON Schemas plus golden snapshots.

### 2026-07-09

`openknowledge search` changed its pre-v1 default from a ranked text match list
to a source-preserving Markdown context packet. BM25 section ranking remains
the canonical retrieval layer. One-hop outgoing-link and backlink expansion
is now on by default and fills only the remaining token budget; `--no-expand`
opts out. Added `--budget` with a `2400` default and `--matches` for the prior
ranked result-list presentation. `--format` is now `markdown|json` with
`markdown` as the default. Removed `--expand graph` and the `text` format name;
`--limit` continues to default to `12` and now caps context sources as well as
displayed matches.

Source anchors: `packages/cli/cmd/openknowledge/main.go`,
`packages/cli/cmd/openknowledge/main_test.go`,
`packages/cli/internal/okf/search_knowledge.go`,
`packages/cli/internal/okf/search_types.go`,
`packages/cli/internal/okf/context.go`,
`packages/cli/internal/okf/context_selection.go`,
`packages/cli/internal/okf/context_types.go`,
`packages/cli/internal/okf/search_test.go`, and
`packages/cli/internal/okf/context_test.go`.

### 2026-07-06

`openknowledge search` shipped as the query retrieval command. It replaced
the previous query mode, added section-level BM25-style ranking, JSON output,
and opt-in graph expansion through local links and backlinks.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/cmd/openknowledge/main_test.go`
> * `packages/cli/internal/okf/search_knowledge.go`
> * `packages/cli/internal/okf/search_types.go`
> * `packages/cli/internal/okf/context.go`
> * `packages/cli/internal/okf/context_selection.go`
> * `packages/cli/internal/okf/context_types.go`
> * `packages/cli/internal/okf/context_sections.go`
> * `packages/cli/internal/okf/search_test.go`
> * `packages/cli/internal/okf/context_test.go`
> * `packages/cli/schemas/v1/search-context.schema.json`
> * `packages/cli/schemas/v1/search-results.schema.json`
>
> **Update notes**
>
> Update this page when search flags, context fields, chunking, ranking,
> packing, expansion behavior, or key/path resolution semantics change. CLI
> behavior changes also require [CLI changelog](/changelog/cli.md) updates.
