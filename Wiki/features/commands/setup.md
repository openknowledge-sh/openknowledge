---
type: Command Documentation
title: openknowledge setup
description: Runs the managed agent onboarding workflow for a knowledge base.
tags: [openknowledge, cli, command, setup]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge setup`

`openknowledge setup [wiki]` is the canonical onboarding command. It runs the
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
| `--depth <n>` | Supply a positive traversal hint. Requires `--from`. |
| `--help` | Print setup-specific help. |

Built-in canonical rules are `project`, `docs`, `decisions`, `changelog`,
`research`, `bugs`, `schemas`, `summary`, and `agents`.

## Completion Contract

Setup succeeds only when all three stages succeed: the selected agent harness
finishes, the target is a valid OKF bundle, and project integration installs.
Agent failure, a missing target, validation findings, or integration failure
produce a nonzero exit. Existing uncommitted repository changes remain visible
to the agent.

## Use Cases

* Start a project wiki through Codex, Claude Code, or OpenCode.
* Refresh a wiki from an existing repository, folder, or website with one
  `--from` mode instead of a separate command.
* Preselect known maintenance loops, for example docs plus changelog, while
  still letting the setup agent inspect context before creating files.
* Let setup interviews adapt to the existing workspace, project docs, and
  available agent memory instead of repeating the same generic questionnaire.
* Choose concrete maintenance rules for future agents, such as docs,
  changelog, decisions, research, bugs, schemas, summary, or project memory.
* Seed repo-scoped or user-scoped skills with guidance for spawning focused
  lower-reasoning subagents for bounded wiki maintenance tasks.
* Leave the user with the use/navigation commands for the created bundle:
  `openknowledge list`, `openknowledge search`, `openknowledge get`, and open
  the finished wiki with `openknowledge view`.

## Command Change History

### 2026-07-17 - Canonical managed onboarding

`openknowledge setup [wiki]` now owns executable onboarding, including the
`--from` source mode, validation, and project integration. Print-only setup and
source workflows moved under `openknowledge prompt`; `agent init` and
`agent from` were removed before 1.0.

### 2026-07-06 - Use/navigation loop

The setup prompt, generated `SETUP.MD`, README setup prompt, and landing page
prompt now tell agents to show users how to inspect a finished wiki with
`openknowledge list`, `openknowledge search`, and `openknowledge get`, and open
it with `openknowledge view`.

### 2026-07-05 - Maintenance rules

`openknowledge setup` gained `--rules <rules>` support. Selected
comma-separated rules are inserted into the generated setup prompt as the
starting point for
`AGENTS.md`, workflow docs, and agent instruction files. The default setup
prompt also lists the available built-in canonical rules and points the user
toward the same maintenance-loop vocabulary exposed by
`openknowledge prompt rules --list`.

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
