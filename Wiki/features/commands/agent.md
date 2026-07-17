---
type: Command Documentation
title: openknowledge agent
description: Experimental human-facing Codex sessions with direct or isolated filesystem editing.
tags: [openknowledge, cli, command, agent, codex]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge agent`

`openknowledge agent` is the human-facing agent interface. It launches Codex
with the selected directory as its working filesystem. Unlike declarative
[`openknowledge jobs`](jobs.md), it needs no Markdown job file, schedule, run
record, commit policy, or daemon.

Direct filesystem editing is the default for local work. Add `--isolate` when
the task should get a dedicated Git branch and worktree.

## Usage

```sh
openknowledge agent
openknowledge agent "<initial prompt>"
openknowledge agent --path <directory>
openknowledge agent --model <model> "<initial prompt>"
openknowledge agent --isolate "<initial prompt>"
openknowledge agent exec "<prompt>"
openknowledge agent exec --path <directory> "<prompt>"
openknowledge agent exec --model <model> "<prompt>"
openknowledge agent exec --isolate "<prompt>"
openknowledge agent --help
openknowledge agent exec --help
```

With no `exec` subcommand, Open Knowledge starts the interactive Codex terminal
UI and may pass an initial prompt. `agent exec` runs one non-interactive Codex
task and requires a prompt.

## Arguments And Flags

| Input | Required | Effect |
| --- | --- | --- |
| initial prompt | no | Starts the interactive session with a task. Remaining positional words are joined with spaces. |
| `exec <prompt>` | prompt required | Runs one non-interactive task and exits with the Codex process status. |
| `--path <directory>` | no | Selects the editable directory. Defaults to the current directory. |
| `--model <model>` | no | Passes an explicit model override to Codex. |
| `--isolate` | no | Creates and retains a branch and worktree at the repository's current `HEAD`. |
| `OPENKNOWLEDGE_CODEX` | no | Requires one explicit Codex executable name or path instead of automatic discovery. |

Both modes explicitly request Codex's `workspace-write` sandbox. The Codex
process inherits the current terminal, environment, and authentication state.

Before creating an isolated worktree, Open Knowledge runs a bounded
`--version` probe against Codex candidates. It tries every `codex` executable
found through `PATH`; on macOS it also checks binaries bundled with Codex.app
and ChatGPT.app. A broken wrapper is skipped when a later candidate works.
When `OPENKNOWLEDGE_CODEX` is set, that exact executable is probed and failure
is reported without fallback.

## Direct Mode

Without `--isolate`, the selected directory is passed directly to Codex. Open
Knowledge does not require Git and does not create a branch, worktree, commit,
pull request, job file, or private run record. Existing uncommitted changes are
visible to the agent, and its writes remain in the same filesystem.

This mode is intended for local editing, interactive exploration, and using the
agent as a sub-agent inside an already managed workspace.

## Isolated Mode

`--isolate` requires that `--path` resolve inside a Git repository. Open
Knowledge creates:

```text
branch:   agent/<UTC timestamp>-<random suffix>
worktree: <user-config>/openknowledge/jobs/<repo>/interactive-worktrees/<id>
base:     HEAD
```

If `--path` selects a subdirectory, the Codex process starts in the matching
subdirectory inside the worktree. The branch and worktree are deliberately
retained after Codex exits so the user can inspect, continue, commit, or remove
them. Open Knowledge does not automatically commit or open a pull request.

Uncommitted source-worktree changes are not part of `HEAD`, so they are not
copied into the isolated worktree.

## Command Change History

### 2026-07-17

Added the experimental human-facing `agent` command with interactive and
one-shot `exec` modes. Direct filesystem editing is the default and
`--isolate` opts into a retained branch and worktree. The former automation
group named `agents` was removed rather than retained as an alias; declarative
automation now lives under [`jobs`](jobs.md).

Codex executable discovery now probes candidates before creating isolated
state, skips broken wrappers, checks supported macOS app bundles, and supports
the fail-closed `OPENKNOWLEDGE_CODEX` override.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/agent_command.go`
> * `packages/cli/cmd/openknowledge/agent_command_test.go`
> * `packages/cli/cmd/openknowledge/codex_resolver.go`
> * `packages/cli/internal/agents/adhoc.go`
>
> **Update notes**
>
> Update this page when agent modes, Codex arguments, isolation behavior, or
> retained-worktree semantics change. Record release-facing changes in the
> [CLI changelog](../../changelog/cli.md).
