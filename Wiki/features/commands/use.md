---
type: Command Documentation
title: openknowledge use
description: Prints an entrypoint, bundle file, metadata, or query briefing from a local or connected OKF bundle.
tags: [openknowledge, cli, command, registry, agent]
timestamp: 2026-06-20T00:00:00Z
---

# `openknowledge use`

`openknowledge use` prints an entrypoint, a bundle-relative file, metadata, or
a source-grounded query briefing from a local or connected Open Knowledge bundle. It
resolves a registry key or path, then prints the selected Markdown body,
metadata, or token-bounded briefing plus original excerpts.

The metadata layer is optional. Plain OKF bundles without declared entrypoints
fall back to root `index.md`.

## Usage

```sh
openknowledge use <name-or-path>
openknowledge use <name-or-path> <entry>
openknowledge use <name-or-path> --info
openknowledge use <name-or-path> <entry> --info
openknowledge use <name-or-path> --query <text>
openknowledge use <name-or-path> --query <text> --budget <tokens>
openknowledge use <name-or-path> --query <text> --format json
openknowledge use --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `name-or-path` | argument | Registry key or local bundle path. |
| `entry` | argument | Optional entrypoint name declared as `okf_bundle_entry_<name>` in the root index, or a bundle-relative file path. |
| `--info` | flag | Print bundle and entrypoint metadata instead of the Markdown body. |
| `--query` | flag | Select relevant bundle sections and print a source-grounded briefing. |
| `--budget` | flag | Approximate query output token budget. Defaults to `2400`. |
| `--limit` | flag | Maximum number of query sections. Defaults to `12`. |
| `--format` | flag | Query output format, `markdown` or `json`. Defaults to `markdown`. |
| `--spec` | flag | OKF spec version for query mode. Defaults to `latest`. |

`--info` can appear after the target or after a named entry.

## Bundle Metadata Layer

Bundle metadata lives in the bundle-root `index.md` frontmatter as flat
`okf_bundle_*` keys:

```md
---
okf_version: "0.1"
okf_bundle_name: accessibility
okf_bundle_title: Accessibility Review
okf_bundle_entry_default: agents/accessibility-checker.md
okf_bundle_entry_review: agents/accessibility-review.md
---

# Accessibility Review
```

Entrypoints are ordinary Markdown files. Their frontmatter may include `type`,
`title`, `description`, `tags`, and `use_when`; `use --info` reads those fields
when present.

## Behavior

Without an entry argument, `use` prints `okf_bundle_entry_default` when it is
declared. If no default entrypoint exists, it prints root `index.md`.

With an entry argument, `use` first checks for a matching
`okf_bundle_entry_<name>`. If no declared entrypoint matches, it treats the
argument as a bundle-relative file path. Direct file paths do not require root
metadata.

Entrypoint paths must stay inside the bundle. Missing files, directories, and
paths that escape the bundle fail before output.

`--info` prints a compact bundle metadata block. With a named entry, it prints
that entrypoint's path and frontmatter summary. Without a named entry, it lists
all declared entrypoints; when none are declared, it prints the root `index.md`
fallback metadata.

`--query` switches `use` into token-bounded briefing mode. It builds a
section-level index from Markdown headings, scores original sections with
lexical matches over metadata, paths, headings, and body text, then packs the
highest-scoring excerpts into an approximate token budget.

Query mode does not use embeddings and does not generate summaries. Markdown
output starts with query and budget metadata, then prints a deterministic
briefing with selected key points, linked-neighbor context, gaps, source ranges,
and original excerpts for verification. JSON output returns the same result
model plus a `briefing` object for tools that want to pack or inspect context
themselves.

When a selected section links to another local Markdown file and budget remains,
query mode may include that target's first section as a neighbor result.
Neighbor sections are marked in JSON and in the Markdown source list.

## Quick Examples

```sh
openknowledge connect ./accessibility --as accessibility
openknowledge use accessibility --info
openknowledge use accessibility
openknowledge use accessibility review
openknowledge use accessibility agents/accessibility-review.md
openknowledge use ./project-memory
openknowledge use accessibility --query "validation workflow"
openknowledge use personal --query "release checklist" --budget 1200
openknowledge use personal --query "release checklist" --format json
```

## Command Change History

### 2026-06-28

Query mode now prints answer-ready, source-grounded briefing metadata before
the original excerpts. Markdown output includes key points with citations,
related linked-neighbor context, and gaps; JSON output includes the same data in
the additive `briefing` field.

### 2026-06-20

`openknowledge use` shipped with key/path resolution, default entrypoint
selection, root `index.md` fallback, named entrypoints, `--info`, entrypoint
frontmatter summaries, and bundle-contained path checks.

Direct bundle-relative file paths are now accepted in the optional entry
argument after declared entrypoint names are checked.

Query mode shipped on `openknowledge use` with section-level lexical retrieval,
approximate token budgeting, Markdown and JSON output, local linked-neighbor
inclusion, and `--query`, `--budget`, `--limit`, `--format`, and `--spec`
flags.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/cmd/openknowledge/main_test.go`
> * `packages/cli/internal/okf/context.go`
> * `packages/cli/internal/okf/context_briefing.go`
> * `packages/cli/internal/okf/context_test.go`
> * `packages/cli/internal/okf/metadata.go`
> * `packages/cli/internal/okf/metadata_test.go`
>
> **Update notes**
>
> Update this page when entrypoint selection, supported metadata fields, `--info`
> output, fallback behavior, path-safety checks, query scoring, output fields,
> or token budget behavior change. CLI behavior changes also require
> [CLI changelog](/changelog/cli.md) updates.
