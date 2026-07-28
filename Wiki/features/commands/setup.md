---
type: Command Documentation
title: openknowledge setup
description: Runs the managed agent onboarding workflow for a knowledge base.
tags: [openknowledge, cli, command, setup]
timestamp: 2026-07-28T00:00:00Z
---

# `openknowledge setup`

`openknowledge setup` is the canonical CLI-led onboarding command. Run it in
the Git repository that should own the wiki. With no arguments, it uses the
current repository as its source, writes `Wiki`, and launches Codex. After the
agent finishes, the CLI requires the target bundle to exist, validates it, and
installs repository-scoped discovery skills and observation hooks.

An explicit `[wiki]` path without `--from` starts a guided workflow for a new
or open-ended knowledge base. Use `--from <source>` for another repository,
local folder, or website. Portable print-only variants live under
[`openknowledge prompt`](prompt.md).

## Usage

```sh
openknowledge setup
openknowledge setup --runtime claude
openknowledge setup Wiki
openknowledge setup Wiki --rules docs,changelog
openknowledge setup Wiki --from https://example.com/docs
openknowledge setup Wiki --from ./existing-repo --type custom --about "Release operations"
openknowledge setup --help
```

## Arguments And Flags

The optional positional argument selects the target wiki. With no positional
argument, the target defaults to `Wiki` and the source defaults to the current
repository. Setup must run inside a Git repository so project integration has
a stable repository root.

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

Before launching an interactive process, setup resolves the selected runtime
executable. A missing or unusable executable fails before agent work starts and
prints an exact `openknowledge agent doctor --runtime <runtime>` recovery
command. The resolved executable is reused for the run, avoiding a second
discovery pass.

Setup succeeds only when all three execution stages succeed: the selected agent
harness finishes, the target is a valid OKF bundle, and project integration
installs. Agent failure prints the runtime and exit status plus an
authentication-and-rerun hint. A missing target, validation errors, or
integration failure also produce a nonzero exit. Existing uncommitted
repository changes remain visible to the agent.

Setup is the workflow controller: it starts an interactive process for the
selected agent runtime. Do not treat `scaffold` as an equivalent onboarding
path; it is the advanced deterministic primitive for creating bundle files
without an agent or project integration.

The knowledge base is ready when setup succeeds; no second onboarding command
is required. Search it later when useful:

```sh
openknowledge search Wiki "release workflow"
```

Setup already validates the bundle. `get`, `list`, and `view` remain optional
follow-up tools for exact reads, structural inspection, and human browsing;
publishing, registry, runtime, scaffold, and prompt commands are separate
advanced workflows.


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
