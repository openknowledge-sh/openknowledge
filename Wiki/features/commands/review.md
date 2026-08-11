---
type: Command Documentation
title: openknowledge prompt review
description: Prints advisory AI review prompts.
tags: [openknowledge, cli, command, prompt, review]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge prompt review`

Use `okn prompt review` to print advisory prompts. The command does
not call a model, edit files, or change validation status.

## Usage

```sh
okn prompt review content Wiki --scope changed
okn prompt review content Wiki --scope changed --base main
okn prompt review content Wiki --scope full
okn prompt review content Wiki --scope full --all-rules
okn prompt review rules Wiki
okn prompt review rules --path Wiki
okn prompt review rules --rules docs,changelog --path Wiki
okn prompt review rules --all Wiki
```

### Content review

`content` builds a portable content-health prompt. It records:

- the deterministic bundle digest;
- the Git head and resolved comparison-base commits when they are available;
- each selected page and its byte digest;
- the ordered rule IDs and a digest of their resolved instructions;
- the selected content-health concerns; and
- deterministic validation error and warning counts.

Changed scope is the default. It compares the wiki with `HEAD`, includes
staged, unstaged, untracked, and deleted Markdown paths, and adds one hop of
incoming and outgoing local Markdown dependencies. Use `--base <ref>` to
compare a proposed revision with another Git ref. Changed scope requires a Git
repository. Full scope selects every Markdown page and also works outside Git.

Use `--concerns <ids>` to limit the review. The stable concern IDs are
`duplication-conflicts`, `stale-orphaned`, `information-architecture`,
`task-usefulness`, `rule-compliance`, and `maintenance-priority`.

The prompt uses configured rules by default. Use `--rules <ids>` for an exact
selection or `--all-rules` for the complete built-in and local catalog.

The review ID is deterministic for the selected source and review contract. It
does not depend on the machine-local absolute Wiki path, the spelling of a Git
base ref, or the displayed creation time.

### Rule review

The `rules` workflow loads the same catalog as `okn prompt rules`.
The catalog contains built-in rules and wiki-local rules.

The prompt asks an external agent to inspect evidence for the selected
maintenance obligations. The findings are advisory. Use
`okn validate` to validate the OKF bundle.

Open Knowledge removed the old top-level `openknowledge review` form before
1.0.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/rules.go`
> * `packages/cli/internal/okf/content_review.go`
