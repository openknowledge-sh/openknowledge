---
type: Command Documentation
title: openknowledge review
description: Create a content review task and optionally run an installed agent.
tags: [openknowledge, cli, command, review]
timestamp: 2026-09-01T00:00:00Z
---

# `openknowledge review`

Use `okn review` for an optional agent review of managed knowledge.

## Usage

```sh
okn review [path]
okn review [path] --scope changed
okn review [path] --scope changed --base main
okn review [path] --scope full
okn review [path] --scope full --all-rules
okn review content [path] --scope full
okn review rules [path]
```

The top-level command builds a deterministic content review task. On a
terminal, it offers installed agents first. You can also copy the task or save
it to `.openknowledge/review-task.md`.

Without terminal input, the command prints the portable task. Review findings
are advisory. The command does not change validation status.

Changed scope is the default. It includes staged, unstaged, untracked, and
deleted Markdown. It also includes one level of local Markdown dependencies.
Changed scope requires Git. Full scope selects all Markdown and works without
Git.

Use `--concerns <ids>` to limit review concerns. Use `--rules <ids>` for an
exact rule selection. Use `--all-rules` for the complete rule catalog.

The review records source digests, the bundle digest, selected rules, and
validation counts. Its ID is deterministic for the selected review contract.

The `content` and `rules` subcommands remain available for compatibility.
`okn prompt review` also remains available as an advanced prompt interface.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/main.go`
> - `packages/cli/internal/okf/content_review.go`
> - `packages/cli/internal/okf/rules.go`
