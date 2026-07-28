---
type: Command Documentation
title: openknowledge get
description: Read an exact Markdown file, bundle entrypoint, or metadata.
tags: [openknowledge, cli, command, registry]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge get`

Read an exact file, entrypoint, or metadata record. Use
[`search`](search.md) when you need ranked or budget-limited retrieval.

## Usage

```sh
okn get <key-or-path>
okn get <key-or-path> <entry-or-file>
okn get <key-or-path> --info
```

`key-or-path` can be a local Markdown file, registry key, or bundle directory.
`entry-or-file` can be a named bundle entrypoint or bundle-relative Markdown
path. `--info` prints metadata instead of content.

## Selection

1. A standalone Markdown path reads that exact file.
2. A bundle with one argument reads `okf_bundle_entry_default`, when declared,
   or root `index.md`.
3. A second argument first matches `okf_bundle_entry_<name>`, then falls back
   to a relative Markdown path.

Relative selections must stay inside the bundle. They cannot traverse
symlinks. A missing file, directory selection, or path escape stops the
command before output.

Bundle metadata is optional root `index.md` frontmatter:

```yaml
okf_bundle_name: accessibility
okf_bundle_title: Accessibility Review
okf_bundle_entry_default: agents/accessibility-checker.md
okf_bundle_entry_review: agents/accessibility-review.md
```

Entrypoints are Markdown files. `--info` reports available bundle and page
metadata. This metadata includes titles, purpose, tags, and entrypoint paths.
An OKF bundle without this metadata remains valid. It uses the root index.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/main.go`
> - `packages/cli/internal/okf/metadata.go`
> - `packages/cli/internal/okf/metadata_test.go`
