---
type: Command Documentation
title: openknowledge prompt
description: Print maintenance rules and advisory review tasks.
tags: [openknowledge, cli, command, prompt, advanced]
timestamp: 2026-08-02T00:00:00Z
---

# `openknowledge prompt`

Use `okn prompt` for maintenance rules and advisory review tasks.

Use [`okn setup`](setup.md) for all knowledge-base setup tasks. Setup owns the
current agent task and source-to-wiki workflow.

## Usage

```sh
okn prompt rules --list
okn prompt rules docs,changelog --path Wiki
okn prompt rules apply docs,changelog --path Wiki --file AGENTS.md
okn prompt review content Wiki --scope changed
okn prompt review rules Wiki
```

## Subcommands

| Subcommand | Effect |
| --- | --- |
| [`rules`](rules.md) | List or render maintenance rules. `rules apply` updates one managed instruction block. |
| [`review`](review.md) | Print advisory AI review tasks. |

`prompt setup` and `prompt from` are not supported. Use `okn setup --prompt`
or `okn setup --from <source>` instead.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/prompt_command.go`
> * `packages/cli/internal/okf/rules.go`
