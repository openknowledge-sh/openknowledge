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
openknowledge prompt rules apply docs,changelog --path Wiki --file AGENTS.md
openknowledge prompt rules apply docs --path Wiki --dry-run
```

Built-in IDs are `project`, `docs`, `decisions`, `changelog`, `research`,
`bugs`, `schemas`, `summary`, and `agents`. A wiki may define additional rules
under `rules/`; explicit command-line selections override configured defaults.
`apply` requires confirmation when it would replace an existing managed block,
unless `--yes` is present, and supports `--dry-run`.

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
