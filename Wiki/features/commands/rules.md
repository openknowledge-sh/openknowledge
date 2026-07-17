---
type: Command Documentation
title: openknowledge prompt rules
description: Lists, renders, and explicitly applies canonical maintenance rules.
tags: [openknowledge, cli, command, prompt, rules]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge prompt rules`

This advanced command exposes the canonical maintenance-rule catalog. Its
default mode prints Markdown instructions; `--list` inventories built-in and
wiki-local rules. The explicit `apply` subcommand inserts or replaces a marked,
idempotent block in an agent instruction file.

## Usage

```sh
openknowledge prompt rules --list --path Wiki
openknowledge prompt rules docs,changelog --path Wiki
openknowledge prompt rules docs --target codex
openknowledge prompt rules apply docs,changelog --path Wiki --file AGENTS.md
openknowledge prompt rules apply docs --path Wiki --dry-run
```

| Option | Default | Description |
| --- | --- | --- |
| `--path <wiki>` | `.openknowledge` | Bundle used for configured and custom rules. |
| `--target <target>` | inferred/generic | Render for `generic`, `codex`, `claude`, or `cursor`. |
| `--list` | off | List the resolved rule catalog. |
| `apply --file <file>` | discovered/prompted | Instruction file to update. |
| `apply --yes` | off | Auto-select or create `AGENTS.md` and skip confirmation. |
| `apply --dry-run` | off | Print the proposed file without writing it. |

Built-in IDs are `project`, `docs`, `decisions`, `changelog`, `research`,
`bugs`, `schemas`, `summary`, and `agents`. A wiki may define additional rules
under `rules/`; explicit command-line selections override configured defaults.
Target inference selects the known instruction format from the destination
when possible and otherwise uses `generic`. Warnings follow rendered output in
a terminal and move to stderr when stdout is piped or redirected.

Rule review is separate and advisory:
`openknowledge prompt review rules Wiki`. Deterministic validation continues to
check structure and configured validation policies, not subjective compliance.

The old top-level `openknowledge rules` form was removed before 1.0.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/rules.go`
> * `packages/cli/internal/okf/rule_catalog.go`
> * `packages/cli/cmd/openknowledge/prompt_command.go`
