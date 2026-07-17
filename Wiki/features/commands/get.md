---
type: Command Documentation
title: openknowledge get
description: Read an exact Markdown file, bundle entrypoint, or metadata.
tags: [openknowledge, cli, command, registry]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge get`

Read deterministic, exact knowledge. Use [`search`](search.md) when you need
ranked or budget-bounded retrieval.

## Usage

```sh
openknowledge get <key-or-path>
openknowledge get <key-or-path> <entry-or-file>
openknowledge get <key-or-path> --info
```

`key-or-path` may be a standalone local Markdown file, registry key, or bundle
directory. `entry-or-file` may be a named bundle entrypoint or a
bundle-relative Markdown path. `--info` prints metadata instead of content.

## Selection

1. A standalone Markdown path reads that exact file.
2. A bundle with one argument reads `okf_bundle_entry_default`, when declared,
   or root `index.md`.
3. A second argument first matches `okf_bundle_entry_<name>`, then falls back
   to a relative Markdown path.

Relative selections must remain inside the bundle and cannot traverse
symlinks. Missing files, directories, and escapes fail before output.

Bundle metadata is optional root `index.md` frontmatter:

```yaml
okf_bundle_name: accessibility
okf_bundle_title: Accessibility Review
okf_bundle_entry_default: agents/accessibility-checker.md
okf_bundle_entry_review: agents/accessibility-review.md
```

Entrypoints are ordinary Markdown files. `--info` reports bundle title,
purpose, tags, entrypoint paths, and selected-page metadata when present. Plain
OKF bundles without metadata remain valid and use the root index fallback.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/main.go`
> - `packages/cli/internal/okf/metadata.go`
> - `packages/cli/internal/okf/metadata_test.go`
