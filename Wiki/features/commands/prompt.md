---
type: Command Documentation
title: openknowledge prompt
description: Advanced portable prompt and maintenance-rule tools.
tags: [openknowledge, cli, command, prompt, advanced]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge prompt`

`openknowledge prompt` groups portable workflows that print agent
instructions. These workflows do not start an agent harness.

Use [`openknowledge setup`](setup.md) for normal onboarding. Use this command
to copy a prompt, inspect the rule catalog, or update a managed instruction
block.

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
| [`rules`](rules.md) | List or render maintenance rules. `rules apply` updates one managed instruction block. |
| [`review`](review.md) | Print advisory AI review prompts. |

Open Knowledge removed the former top-level `from`, `rules`, and `review`
commands before 1.0. They are not aliases. Scripts that use them return an
error.


---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/prompt_command.go`
> * `packages/cli/internal/okf/setup.go`
> * `packages/cli/internal/okf/rules.go`
> * `packages/cli/internal/okf/from.go`
