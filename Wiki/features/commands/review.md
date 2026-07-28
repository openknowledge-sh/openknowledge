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
okn prompt review rules Wiki
okn prompt review rules --path Wiki
okn prompt review rules --rules docs,changelog --path Wiki
okn prompt review rules --all Wiki
```

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
