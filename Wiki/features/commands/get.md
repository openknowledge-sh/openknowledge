---
type: Command Documentation
title: openknowledge get
description: Prints an exact Markdown file, bundle entrypoint, or metadata from a local or connected OKF bundle.
tags: [openknowledge, cli, command, registry, agent]
timestamp: 2026-07-06T00:00:00Z
---

# `openknowledge get`

`openknowledge get` prints an exact local Markdown file, a bundle-relative
Markdown file, a declared entrypoint, or bundle metadata. It is the
deterministic read command: use it when the caller already knows which Markdown
file or entrypoint it wants.

The metadata layer is optional. Plain OKF bundles without declared entrypoints
fall back to root `index.md`.

## Usage

```sh
openknowledge get <name-or-path>
openknowledge get <name-or-path> <entry-or-file>
openknowledge get <name-or-path> --info
openknowledge get <name-or-path> <entry-or-file> --info
openknowledge get --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `name-or-path` | argument | Local Markdown file, registry key, or local bundle path. |
| `entry-or-file` | argument | Optional entrypoint name declared as `okf_bundle_entry_<name>` in the root index, or a bundle-relative Markdown file path. |
| `--info` | flag | Print bundle and selected-file metadata instead of the Markdown body. |

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
`title`, `description`, `tags`, and `use_when`; `get --info` reads those fields
when present.

## Behavior

With one argument that points at a local Markdown file, `get` prints that exact
file.

With one argument that resolves to a registry key or bundle folder, `get`
prints `okf_bundle_entry_default` when declared. If no default entrypoint
exists, it prints root `index.md`.

With a second argument, `get` first checks for a matching
`okf_bundle_entry_<name>`. If no declared entrypoint matches, it treats the
argument as a bundle-relative Markdown file path. Direct Markdown file paths do
not require root metadata.

Selected bundle-relative paths must stay inside the bundle. Missing files,
directories, and paths that escape the bundle fail before output.

`--info` prints a compact bundle metadata block. With a named entry or path, it
prints that file's path and frontmatter summary. Without a named entry, it
lists all declared entrypoints; when none are declared, it prints the root
`index.md` fallback metadata.

Query retrieval belongs to [`openknowledge search`](search.md), not `get`.
Search owns ranked retrieval, source snippets, JSON output, and graph-expanded
results.

## Quick Examples

```sh
openknowledge connect ./accessibility --as accessibility
openknowledge get README.md
openknowledge get accessibility --info
openknowledge get accessibility
openknowledge get accessibility review
openknowledge get accessibility agents/accessibility-review.md
openknowledge get ./project-memory
```

## Example Output

`openknowledge get personal --info` prints bundle metadata and declared
entrypoints:

```text
Open Knowledge Get
entrypoint and file metadata

name      Project Memory
root      /work/project-memory
purpose   Durable project context.
tags      project

Entrypoints
  default      agents/default.md  Default Agent Guide
```

`openknowledge get personal` prints the selected Markdown body exactly:

```md
---
type: Agent Entrypoint
title: Default Agent Guide
---

# Default Agent Guide

Read the wiki before non-trivial work.
```

## Command Change History

### 2026-07-06

`openknowledge get` replaced the previous entrypoint/file-loading surface. The
command keeps deterministic Markdown retrieval separate from
[`openknowledge search`](search.md) and from the interactive
[`openknowledge view`](view.md) viewer.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/cmd/openknowledge/main_test.go`
> * `packages/cli/internal/okf/metadata.go`
> * `packages/cli/internal/okf/metadata_test.go`
>
> **Update notes**
>
> Update this page when entrypoint selection, supported metadata fields, `--info`
> output, fallback behavior, direct-file behavior, or path-safety checks change.
> Search retrieval belongs on [search](search.md). CLI behavior changes also
> require [CLI changelog](/changelog/cli.md) updates.
