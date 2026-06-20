---
type: Command Documentation
title: openknowledge use
description: Prints agent entrypoint Markdown or metadata from a local or connected OKF bundle.
tags: [openknowledge, cli, command, registry, agent]
timestamp: 2026-06-20T00:00:00Z
---

# `openknowledge use`

`openknowledge use` prints an agent-facing entrypoint from a local or connected
Open Knowledge bundle. It resolves a registry key or path, reads optional root
`okf_bundle_*` metadata, and prints either the selected Markdown body or
entrypoint metadata.

The metadata layer is optional. Plain OKF bundles without declared entrypoints
fall back to root `index.md`.

## Usage

```sh
openknowledge use <name-or-path>
openknowledge use <name-or-path> <entry>
openknowledge use <name-or-path> --info
openknowledge use <name-or-path> <entry> --info
openknowledge use --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `name-or-path` | argument | Registry key or local bundle path. |
| `entry` | argument | Optional entrypoint name declared as `okf_bundle_entry_<name>` in the root index. |
| `--info` | flag | Print bundle and entrypoint metadata instead of the Markdown body. |

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

With an entry argument, `use` requires a matching `okf_bundle_entry_<name>`.
Missing named entries fail and print available entry names when any exist.

Entrypoint paths must stay inside the bundle. Missing files, directories, and
paths that escape the bundle fail before output.

`--info` prints a compact bundle metadata block. With a named entry, it prints
that entrypoint's path and frontmatter summary. Without a named entry, it lists
all declared entrypoints; when none are declared, it prints the root `index.md`
fallback metadata.

## Quick Examples

```sh
openknowledge connect ./accessibility --as accessibility
openknowledge use accessibility --info
openknowledge use accessibility
openknowledge use accessibility review
openknowledge use ./project-memory
```

## Source Anchors

* `packages/cli/cmd/openknowledge/main.go`
* `packages/cli/cmd/openknowledge/main_test.go`
* `packages/cli/internal/okf/metadata.go`
* `packages/cli/internal/okf/metadata_test.go`

## Command Change History

### 2026-06-20

`openknowledge use` shipped with key/path resolution, default entrypoint
selection, root `index.md` fallback, named entrypoints, `--info`, entrypoint
frontmatter summaries, and bundle-contained path checks.

## Update Notes

Update this page when entrypoint selection, supported metadata fields, `--info`
output, fallback behavior, or path-safety checks change. CLI behavior changes
also require [CLI changelog](/changelog/cli.md) updates.
