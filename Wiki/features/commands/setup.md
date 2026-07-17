---
type: Command Documentation
title: openknowledge setup
description: Runs the managed agent onboarding workflow for a knowledge base.
tags: [openknowledge, cli, command, setup]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge setup`

`openknowledge setup [wiki]` is the canonical CLI-led onboarding command. Run
it directly in the Git repository that should own the wiki. It launches the
setup workflow through Codex by default, or Claude Code or OpenCode via
`--runtime`. After the agent finishes, the CLI requires the target bundle to
exist, validates it, and installs the repository-scoped discovery skills and
observation hooks.

With `--from <source>`, the same command executes the source-to-wiki workflow.
The source may be a repository, local folder, or website. This replaces the
former public `agent init`, `agent from`, and top-level `from` surfaces.
Portable print-only variants live under [`openknowledge prompt`](prompt.md).

## Usage

```sh
openknowledge setup Wiki --from .
openknowledge setup Wiki
openknowledge setup Wiki --rules docs,changelog
openknowledge setup Wiki --runtime claude
openknowledge setup Wiki --from https://example.com/docs
openknowledge setup Wiki --from ./existing-repo --type custom --about "Release operations"
openknowledge setup --help
```

## Arguments And Flags

The optional positional argument selects the target wiki and defaults to
`Wiki`. Setup must run inside a Git repository so project integration has a
stable repository root.

| Flag | Description |
| --- | --- |
| `--from <source>` | Run the source-to-wiki workflow instead of a new setup interview. |
| `--runtime <runtime>` | Select `codex`, `claude`, or `opencode`. |
| `--model <model>` | Override the harness model. |
| `--rules <rules>` | Preselect comma-separated maintenance rules for a new setup. Incompatible with `--from`. |
| `--type <type>` | Select `understanding` or `custom` for `--from`. |
| `--about <goal>` | Supply the custom source-to-wiki goal. Requires `--from`. |
| `--depth <n>` | Supply a non-negative traversal hint. `0` lets the agent choose the minimum depth. Requires `--from`. |
| `--help` | Print setup-specific help. |

Built-in canonical rules are `project`, `docs`, `decisions`, `changelog`,
`research`, `bugs`, `schemas`, `summary`, and `agents`.

## Completion Contract

Setup succeeds only when all three stages succeed: the selected agent harness
finishes, the target is a valid OKF bundle, and project integration installs.
Agent failure, a missing target, validation errors, or integration failure
produce a nonzero exit. Existing uncommitted repository changes remain visible
to the agent.

Setup is the workflow controller: it starts an interactive process for the
selected agent runtime. Do not treat `scaffold` as an equivalent onboarding
path; it is the advanced deterministic primitive for creating bundle files
without an agent or project integration.

After setup, inspect the result with `list`, `search`, and `get`, then open it
with `view`.


---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/setup_command.go`
> * `packages/cli/internal/okf/setup.go`
> * `packages/cli/internal/okf/new.go`
> * `packages/cli/internal/okf/rules.go`
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/setup_test.go`
> * `packages/cli/internal/okf/rules_test.go`
> * `README.md`
> * `packages/web/index.html`
>
> **Update notes**
>
> The setup prompt is a product workflow, not only help text. Update
> [Feature docs workflow](/workflows/feature-docs.md) and [CLI changelog](/changelog/cli.md)
> when the interview, expected outputs, or validation loop changes.
