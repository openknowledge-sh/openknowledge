---
type: Command Documentation
title: openknowledge review
description: Prints advisory AI review prompts for Open Knowledge maintenance workflows.
tags: [openknowledge, cli, command, review, agents, rules]
timestamp: 2026-07-07T00:00:00Z
---

# `openknowledge review`

`openknowledge review` prints advisory AI review prompts. It does not call a
model, edit files, or decide validation status. Use `openknowledge validate`
for deterministic CI-safe checks.

The first shipped review workflow is `openknowledge review rules`, which asks
an agent to inspect evidence and report whether selected maintenance rules
appear to have been followed.

## Usage

```sh
openknowledge review rules Wiki
openknowledge review rules --path Wiki
openknowledge review rules --rules docs,changelog --path Wiki
openknowledge review rules --all Wiki
openknowledge review --help
openknowledge review rules --help
```

## Arguments And Flags

`openknowledge review` currently accepts the `rules` subcommand.

`openknowledge review rules` supports:

| Name | Kind | Description |
| --- | --- | --- |
| `path` | argument | Open Knowledge wiki path. Defaults to `.openknowledge`. |
| `--path <path>` | flag | Open Knowledge wiki path. |
| `--rules <rules>` | flag | Comma-separated built-in or custom maintenance rule IDs to review. Defaults to `[rules].enabled`, then `project`. |
| `--all` | flag | Review every built-in and wiki-local custom rule. Cannot be combined with `--rules`. |
| `--help` | flag | Print command-specific help. |

## Behavior

`review rules` loads the same rule catalog as `openknowledge rules`: built-in
rules plus valid custom rule documents from the configured `[rules].paths`
directories, defaulting to `<wiki>/rules/`. When `--rules` and `--all` are
omitted, `[rules].enabled` supplies the default selection before falling back
to `project`.

The command then prints a Markdown prompt for an AI agent. The prompt tells the
agent to run `openknowledge validate "<wiki>"`, inspect source-backed evidence
such as the working tree and relevant wiki pages, and report findings with rule
IDs, evidence, impact, and concrete suggested fixes.

The command is advisory. It does not perform an LLM call, does not mutate the
wiki or agent instruction files, and does not turn AI judgment into validation
errors. Missing or invalid custom rule catalog structure, including invalid
`[rules]` configuration, remains a deterministic `rule-catalog` validation
concern.

If the wiki path is missing, empty, or invalid OKF, `review rules` prints the
same non-blocking wiki path warnings used by `openknowledge rules`. Malformed
custom rule documents make the command exit with usage status `2` because the
selected review prompt cannot be rendered reliably.

## Example Output

`openknowledge review rules --rules docs --path Wiki` prints an advisory prompt
for another agent. The beginning looks like:

```text
# Open Knowledge Rule Review

You are reviewing whether this workspace follows its Open Knowledge maintenance rules.

Wiki path: `Wiki`

This is an advisory AI review, not deterministic validation. Run deterministic validation first:
- `openknowledge validate "Wiki"`

Review scope:
- Inspect the working tree, recent diffs, existing agent instructions, and only the wiki pages relevant to the selected rules.
- Treat missing evidence as uncertainty, not proof.
```

## Use Cases

* Ask an agent to review whether docs and changelog rules were followed after a
  feature branch.
* Review repository-specific custom rules such as security, data boundaries, or
  release hygiene.
* Keep `openknowledge validate` deterministic while still providing an
  AI-assisted review loop.

## Command Change History

### 2026-07-07 - Rule review prompt

Added `openknowledge review rules` as a prompt-producing, advisory AI review
workflow for built-in and wiki-local custom maintenance rules. It honors
`[rules].paths` and `[rules].enabled` from `openknowledge.toml`.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/rule_catalog.go`
> * `packages/cli/cmd/openknowledge/main_test.go`
> * `packages/cli/internal/okf/rules_test.go`
>
> **Update notes**
>
> Update this page when review subcommands, prompt content, rule selection
> flags, or review exit behavior changes.
