---
type: Command Documentation
title: openknowledge setup
description: Prints portable setup instructions or runs them with an agent.
tags: [openknowledge, cli, command, setup]
timestamp: 2026-07-28T00:00:00Z
---

# `openknowledge setup`

Use `okn setup` to print portable instructions for a wiki.

With no arguments, the command uses the current directory as the source. The
default target is `Wiki`. The command prints the instructions to standard
output. It does not start an agent.

Run `okn setup` yourself. Copy its complete output into an agent that already
has access to the project. This is the primary setup flow.

`--agent` is an advanced option. It starts an installed agent runtime from the
CLI. Run agent mode in the Git repository that contains the wiki.

Specify `[wiki]` without `--from` for a new knowledge base. Use
`--from <source>` for a different repository, local folder, or website.

## Usage

```sh
okn setup
okn setup --agent
okn setup --agent --runtime claude
okn setup Wiki
okn setup Wiki --rules docs,changelog
okn setup Wiki --from https://example.com/docs
okn setup Wiki --from ./existing-repo --type custom --about "Release operations"
okn setup --help
```

## Arguments and flags

The optional positional argument selects the target wiki. The default target
is `Wiki`. The default source is the current directory.

Run agent mode inside a Git repository. Project integration requires a stable
repository root.

| Flag | Description |
| --- | --- |
| `--from <source>` | Run the source-to-wiki workflow instead of a new setup interview. |
| `--agent` | Run the instructions with an agent. By default, setup prints the instructions. |
| `--runtime <runtime>` | Select `codex`, `claude`, or `opencode`. Requires `--agent`. |
| `--model <model>` | Override the harness model. Requires `--agent`. |
| `--rules <rules>` | Preselect comma-separated maintenance rules for a new setup. Incompatible with `--from`. |
| `--type <type>` | Select `understanding` or `custom` for `--from`. |
| `--about <goal>` | Supply the custom source-to-wiki goal. Requires `--from`. |
| `--depth <n>` | Supply a non-negative traversal hint. `0` lets the agent choose the minimum depth. Requires `--from`. |
| `--help` | Print setup-specific help. |

Built-in canonical rules are `project`, `docs`, `decisions`, `changelog`,
`research`, `bugs`, `schemas`, `summary`, and `agents`.

## Agent mode

If you omit `--runtime`, setup detects the installed supported runtimes. It
asks you to select one before it starts the agent.

For non-interactive input, specify `--runtime`. Without this flag, setup lists
the available runtimes and stops.

If setup finds no supported runtime, it stops. Install `codex`, `claude`, or
`opencode`. Then, run the command again.

Before agent work starts, setup resolves the selected runtime executable. A
missing executable stops setup. Use this command to diagnose the installation:

```sh
okn agent doctor --runtime <runtime>
```

Setup succeeds only when all three stages succeed:

1. The selected agent harness finishes.
2. The target is a valid OKF bundle.
3. The selected runtime project skill installs.

An agent failure prints the runtime, exit status, and a recovery hint. A
missing target, validation error, or integration failure produces a nonzero
exit status. The agent can see existing uncommitted repository changes.

Agent mode controls the workflow and starts an interactive agent process. It
does not install observation hooks. Use `okn integration install` with
`--observe` if you explicitly want session observation.

`scaffold` is not an equivalent onboarding path. It creates bundle files
without an agent or project integration.

The knowledge base is ready when agent mode succeeds. Use search when you need
context:

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
