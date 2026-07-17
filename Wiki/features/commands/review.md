---
type: Command Documentation
title: openknowledge prompt review
description: Prints advisory AI review prompts.
tags: [openknowledge, cli, command, prompt, review]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge prompt review`

`openknowledge prompt review` produces advisory prompts and never calls a
model, edits files, or changes deterministic validation status.

## Usage

```sh
openknowledge prompt review rules Wiki
openknowledge prompt review rules --path Wiki
openknowledge prompt review rules --rules docs,changelog --path Wiki
openknowledge prompt review rules --all Wiki
```

The `rules` workflow loads the same built-in and wiki-local catalog as
`openknowledge prompt rules`. It asks an external agent to inspect evidence for
the selected maintenance obligations. Findings remain advisory; use
`openknowledge validate` for deterministic OKF validity.

The old top-level `openknowledge review` form was removed before 1.0.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/rules.go`
