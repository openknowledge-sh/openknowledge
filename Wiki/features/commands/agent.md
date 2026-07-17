---
type: Command Documentation
title: openknowledge agent
description: Experimental steered Open Knowledge sessions through Codex, Claude Code, or OpenCode.
tags: [openknowledge, cli, command, agent, codex, claude, opencode]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge agent`

`openknowledge agent` is the human-facing interface over supported agent
harnesses. It launches Codex by default, or Claude Code/OpenCode through
`--runtime`, and prepends the versioned `openknowledge-agent/v1` contract.
That contract tells the agent to treat workspace files as source of truth,
preserve provenance, respect publication gates, validate its work, and leave
Git publication to the surrounding runtime.

When the repository has been connected with
[`openknowledge agent integrate`](integrate.md), the native project hooks also observe
sessions launched through this proxy. The proxy does not have a separate
insight implementation; direct Codex, Claude Code, OpenCode, and
`openknowledge agent` sessions all feed the same project observer.

Unlike declarative [`jobs`](jobs.md), local sessions need no Markdown job,
schedule, run record, or commit policy. They edit the selected filesystem
directly unless `--isolate` creates a retained branch and worktree.

## Usage

```sh
openknowledge agent
openknowledge agent --runtime claude
openknowledge agent --runtime opencode --model provider/model
openknowledge agent exec "Update the whitepaper"
openknowledge agent exec --runtime claude "Repair citations"
openknowledge agent integrate Wiki
openknowledge agent doctor
openknowledge agent doctor --runtime opencode --json
openknowledge agent exec --isolate "Update the wiki"
```

## Commands And Flags

| Input | Default | Effect |
| --- | --- | --- |
| initial prompt | none | Starts an interactive harness session with the steered task. With no task, the contract asks the agent to wait for one. |
| `exec <prompt>` | required | Runs one non-interactive task and returns the harness exit status. |
| `doctor` | all runtimes | Probes harness executables without starting a model session. Accepts only `--runtime` and `--json`. |
| `integrate <wiki>` | - | Installs project-scoped discovery skills and observation hooks. `--global` installs discovery-only user skills. |
| `--runtime` | `codex` | Selects `codex`, `claude`, or `opencode`. |
| `--model` | harness default | Passes a harness-specific model override. |
| `--path` | current directory | Selects the editable workspace. |
| `--isolate` | false | Creates and retains a branch/worktree at `HEAD`. |
| `--no-steer` | false | Passes only the user or generated workflow prompt. |

Executable overrides are `OPENKNOWLEDGE_CODEX`, `OPENKNOWLEDGE_CLAUDE`, and
`OPENKNOWLEDGE_OPENCODE`. Each explicit override is version-probed and fails
closed. Codex discovery additionally skips broken `PATH` wrappers and checks
supported macOS Codex.app and ChatGPT.app bundles.

`doctor --json` returns `schemaVersion: "1"` and a `runtimes` array containing
`runtime`, `available`, and optional `executable` or `error` fields. A probe of
all runtimes succeeds when at least one is available; it exits `1` only when
none are available. This diagnostic JSON is versioned but does not currently
have a published schema.

## Harness Contracts

| Runtime | Interactive | Non-interactive |
| --- | --- | --- |
| Codex | `codex --sandbox workspace-write` | `codex exec --sandbox workspace-write <prompt>` locally; scheduled jobs use the explicit stdin form `codex exec ... -`. |
| Claude Code | `claude` | `claude --print --no-session-persistence` with `acceptEdits` and a narrow allowlist for Open Knowledge validation and read-only Git inspection. |
| OpenCode | `opencode` | `opencode run --auto`; explicit deny rules in project configuration still apply. |

Codex documents `exec` as its non-interactive CI surface, Claude Code documents
`--print`, and OpenCode documents `run`. Open Knowledge owns the common task,
workspace, and publication contract while each adapter owns the vendor CLI
arguments.

Upstream references: [Codex CLI](https://developers.openai.com/codex/cli/),
[Claude Code CLI](https://code.claude.com/docs/en/cli-usage),
[OpenCode CLI](https://opencode.ai/docs/cli/), and
[OpenCode providers](https://opencode.ai/docs/providers/).

Local OpenCode follows the user's normal provider configuration. A headless
OpenCode worker receives `OPENCODE_API_KEY`; repository `opencode.json` may bind
that placeholder to any provider and should choose a default model or set
`agent.model`.

## Direct And Isolated Modes

Direct mode does not require Git and creates no branch, commit, pull request,
or private run record. Existing uncommitted changes remain visible.

`--isolate` requires a Git repository and creates:

```text
branch:   agent/<UTC timestamp>-<random suffix>
worktree: <user-config>/openknowledge/jobs/<repo>/interactive-worktrees/<id>
base:     HEAD
```

The worktree is retained after the harness exits. Open Knowledge never commits
or opens a pull request for this human-facing mode.

Knowledge-maintenance observations use the adjacent, harness-independent
[`openknowledge insights`](insights.md) interface. `insights create` captures a
durable gap without starting a model; `insights run` sends one or all pending
items through the selected local agent runtime.


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
