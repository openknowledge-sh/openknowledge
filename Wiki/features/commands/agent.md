---
type: Command Documentation
title: openknowledge agent
description: Experimental steered Open Knowledge sessions through Codex, Claude Code, or OpenCode.
tags: [openknowledge, cli, command, agent, codex, claude, opencode]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge agent`

Use `okn agent` to start a supported agent harness. The command
starts Codex by default. Use `--runtime` to select Claude Code or OpenCode.

The command adds the versioned `openknowledge-agent/v1` contract. The contract
defines workspace files as the source of truth. It also requires provenance,
publication controls, and validation. The surrounding runtime controls Git
publication.

Use [`okn integration`](integration.md) to install a project skill for one
runtime. Add `--observe` only when you want the selected runtime to observe
sessions.

Local sessions do not require a Markdown job, schedule, run record, or commit
policy. [`jobs`](jobs.md) uses these declarative controls. A local session
edits the selected filesystem directly. `--isolate` creates a retained branch
and worktree instead.

## Usage

```sh
okn agent
okn agent --runtime claude
okn agent --runtime opencode --model provider/model
okn agent exec "Update the whitepaper"
okn agent exec --runtime claude "Repair citations"
okn agent doctor
okn agent doctor --runtime opencode --json
okn agent exec --isolate "Update the wiki"
```

## Commands and flags

| Input | Default | Effect |
| --- | --- | --- |
| initial prompt | none | Starts an interactive harness session with the steered task. With no task, the contract asks the agent to wait for one. |
| `exec <prompt>` | required | Runs one non-interactive task and returns the harness exit status. |
| `doctor` | all runtimes | Probes harness executables. It does not start a model session. Accepts only `--runtime` and `--json`. |
| `--runtime` | `codex` | Selects `codex`, `claude`, or `opencode`. |
| `--model` | harness default | Passes a harness-specific model override. |
| `--path` | current directory | Selects the editable workspace. |
| `--isolate` | false | Creates and retains a branch/worktree at `HEAD`. |
| `--no-steer` | false | Passes only the user or generated workflow prompt. |

Executable overrides are `OPENKNOWLEDGE_CODEX`, `OPENKNOWLEDGE_CLAUDE`, and
`OPENKNOWLEDGE_OPENCODE`. The command verifies the version of each explicit
override. It stops if the version is not valid.

Codex discovery skips broken `PATH` wrappers. It also inspects supported macOS
Codex.app and ChatGPT.app bundles.

`doctor --json` returns `schemaVersion: "1"` and a `runtimes` array. Each item
contains `runtime`, `available`, and optional `executable` or `error` fields.

A probe of all runtimes succeeds when one or more runtimes are available. It
exits with status `1` when no runtime is available. This versioned diagnostic
JSON does not have a published schema.

## Harness contracts

| Runtime | Interactive | Non-interactive |
| --- | --- | --- |
| Codex | `codex --sandbox workspace-write` | `codex exec --sandbox workspace-write <prompt>` locally. Scheduled jobs use the explicit stdin form `codex exec ... -`. |
| Claude Code | `claude` | `claude --print --no-session-persistence` with `acceptEdits` and a narrow allowlist for Open Knowledge validation and read-only Git inspection. |
| OpenCode | `opencode` | `opencode run --auto`. Explicit deny rules in project configuration still apply. |

Codex uses `exec` for non-interactive CI tasks. Claude Code uses `--print`.
OpenCode uses `run`. Open Knowledge defines the common task, workspace, and
publication contract. Each adapter defines its vendor CLI arguments.

Upstream references: [Codex CLI](https://developers.openai.com/codex/cli/),
[Claude Code CLI](https://code.claude.com/docs/en/cli-usage),
[OpenCode CLI](https://opencode.ai/docs/cli/), and
[OpenCode providers](https://opencode.ai/docs/providers/).

Local OpenCode uses the normal provider configuration. A headless OpenCode
worker receives `OPENCODE_API_KEY`. Repository `opencode.json` can bind this
placeholder to a provider. Select a default model or set `agent.model`.

## Direct and isolated modes

Direct mode does not require Git. It creates no branch, commit, pull request,
or private run record. The agent can see existing uncommitted changes.

`--isolate` requires a Git repository and creates:

```text
branch:   agent/<UTC timestamp>-<random suffix>
worktree: <user-config>/openknowledge/jobs/<repo>/interactive-worktrees/<id>
base:     HEAD
```

Open Knowledge keeps the worktree after the harness exits. It does not create a
commit or pull request for this mode.

Knowledge maintenance observations use
[`okn automation insights`](insights.md). This interface does not depend on a
harness. `insights create` records a durable gap and does not start a model.
`insights run` sends pending items to the selected local agent runtime.


---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/agent_command.go`
> * `packages/cli/cmd/openknowledge/agent_command_test.go`
> * `packages/cli/cmd/openknowledge/codex_resolver.go`
> * `packages/cli/internal/agents/harness.go`
> * `packages/cli/internal/agents/adhoc.go`
>
> **Update notes**
>
> Update this page when harnesses, steering, generated workflows, isolation,
> or executable discovery change. Record release-facing changes in the
> [CLI changelog](../../changelog/cli.md).
