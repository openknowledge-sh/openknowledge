---
type: Command Documentation
title: openknowledge prompt
description: Advanced portable prompt and maintenance-rule tools.
tags: [openknowledge, cli, command, prompt, advanced]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge prompt`

`openknowledge prompt` groups portable workflows that print agent instructions
instead of starting a harness. Normal onboarding uses
[`openknowledge setup`](setup.md); this namespace is for copying prompts into an
external agent, inspecting the canonical rule catalog, or updating a managed
instruction block.

## Usage

```sh
openknowledge prompt setup --rules docs,changelog
openknowledge prompt from <source> --out Wiki
openknowledge prompt rules --list
openknowledge prompt rules docs,changelog --path Wiki
openknowledge prompt rules apply docs --path Wiki --file AGENTS.md
openknowledge prompt review rules Wiki
```

## Subcommands

| Subcommand | Effect |
| --- | --- |
| `setup` | Print the canonical setup interview prompt. |
| [`from`](from.md) | Print a source-to-wiki prompt. |
| [`rules`](rules.md) | List or render maintenance rules; `rules apply` updates one managed instruction block. |
| [`review`](review.md) | Print advisory AI review prompts. |

The former top-level `from`, `rules`, and `review` commands were removed before
1.0. They are not retained as aliases, so scripts fail loudly instead of
silently depending on a transitional interface.


---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/prompt_command.go`
> * `packages/cli/internal/okf/setup.go`
> * `packages/cli/internal/okf/rules.go`
> * `packages/cli/internal/okf/from.go`
