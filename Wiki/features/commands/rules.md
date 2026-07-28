---
type: Command Documentation
title: openknowledge prompt rules
description: Lists, renders, and explicitly applies canonical maintenance rules.
tags: [openknowledge, cli, command, prompt, rules]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge prompt rules`

This advanced command gives access to the maintenance rule catalog. The
default mode prints Markdown instructions. `--list` shows built-in and
wiki-local rules.

The `apply` subcommand inserts or replaces a marked block in an agent
instruction file. Repeated application produces the same block.

## Usage

```sh
okn prompt rules --list --path Wiki
okn prompt rules asd-ste100 --path Wiki
okn prompt rules docs,changelog --path Wiki
okn prompt rules docs --target codex
okn prompt rules apply docs,changelog --path Wiki --file AGENTS.md
okn prompt rules apply docs --path Wiki --dry-run
```

| Option | Default | Description |
| --- | --- | --- |
| `--path <wiki>` | `.openknowledge` | Bundle used for configured and custom rules. |
| `--target <target>` | inferred/generic | Render for `generic`, `codex`, `claude`, or `cursor`. |
| `--list` | off | List the resolved rule catalog. |
| `apply --file <file>` | discovered/prompted | Instruction file to update. |
| `apply --yes` | off | Auto-select or create `AGENTS.md` and skip confirmation. |
| `apply --dry-run` | off | Print the proposed file. Do not write the file. |

Built-in IDs are `project`, `docs`, `decisions`, `changelog`, `research`,
`bugs`, `schemas`, `summary`, and `agents`. A wiki can define additional rules
under `rules/`. Command-line selections override configured defaults.

This wiki defines the `asd-ste100` rule. Its `[rules].enabled` configuration
selects `project` and `asd-ste100` by default.

When possible, target inference selects the instruction format from the
destination. Otherwise, it uses `generic`. In a terminal, warnings follow the
rendered output. When a caller redirects stdout, warnings go to stderr.

Rule review is separate and advisory. Use
`okn prompt review rules Wiki`. Deterministic validation verifies
structure and configured validation policies. It does not evaluate subjective
compliance.

Open Knowledge removed the old top-level `openknowledge rules` form before
1.0.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/rules.go`
> * `packages/cli/internal/okf/rule_catalog.go`
> * `packages/cli/cmd/openknowledge/prompt_command.go`
