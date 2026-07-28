---
type: Command Documentation
title: openknowledge setup
description: Runs the managed agent onboarding workflow for a knowledge base.
tags: [openknowledge, cli, command, setup]
timestamp: 2026-07-28T00:00:00Z
---

# `openknowledge setup`

Use `okn setup` to create and integrate a wiki. Run the command in
the Git repository that contains the wiki.

With no arguments, the command uses the current repository as the source. It
writes `Wiki` and starts Codex. After Codex finishes, the CLI verifies that the
target bundle exists. The CLI then validates the bundle and installs
repository-level discovery skills and observation hooks.

Specify `[wiki]` without `--from` to start a guided workflow for a new
knowledge base. Use `--from <source>` for a different repository, local
folder, or website. Use [`okn prompt`](prompt.md) to print portable
prompts. That command does not start an agent.

## Usage

```sh
okn setup
okn setup --runtime claude
okn setup Wiki
okn setup Wiki --rules docs,changelog
okn setup Wiki --from https://example.com/docs
okn setup Wiki --from ./existing-repo --type custom --about "Release operations"
okn setup --help
```

## Arguments and flags

The optional positional argument selects the target wiki. The default target
is `Wiki`. The default source is the current repository.

Run setup inside a Git repository. Project integration requires a stable
repository root.

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

## Completion contract

Before setup starts an interactive process, it resolves the selected runtime
executable. A missing or unusable executable stops setup before agent work
starts. Setup then prints this recovery command:
`okn agent doctor --runtime <runtime>`.

Setup uses the resolved executable for the complete run.

Setup succeeds only when all three stages succeed:

1. The selected agent harness finishes.
2. The target is a valid OKF bundle.
3. Project integration installs.

An agent failure prints the runtime, exit status, and a recovery hint. A
missing target, validation error, or integration failure produces a nonzero
exit status. The agent can see existing uncommitted repository changes.

Setup controls the workflow and starts an interactive agent process.
`scaffold` is not an equivalent onboarding path. It creates bundle files
without an agent or project integration.

The knowledge base is ready when setup succeeds. You do not need a second
onboarding command. Use search when you need context:

```sh
okn search Wiki "release workflow"
```

Setup validates the bundle. Use `get`, `list`, or `view` only when you need
their output. Publishing, registry, runtime, scaffold, and prompt commands are
separate advanced workflows.


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
