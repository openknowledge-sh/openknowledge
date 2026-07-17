---
type: Command Documentation
title: openknowledge agent
description: Experimental steered Open Knowledge sessions through Codex, Claude Code, Grok, or OpenCode.
tags: [openknowledge, cli, command, agent, codex, claude, grok, opencode]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge agent`

`openknowledge agent` is the human-facing interface over supported agent
harnesses. It launches Codex by default, or Claude Code/Grok/OpenCode through
`--runtime`, and prepends the versioned `openknowledge-agent/v1` contract.
That contract tells the agent to treat workspace files as source of truth,
preserve provenance, respect publication gates, validate its work, and leave
Git publication to the surrounding runtime.

When the repository has been connected with
[`openknowledge agent integrate`](integrate.md), the native project hooks also observe
sessions launched through this proxy. The proxy does not have a separate
suggestion implementation; direct Codex, Claude Code, OpenCode, and
`openknowledge agent` sessions all feed the same project observer.

Unlike declarative [`jobs`](jobs.md), local sessions need no Markdown job,
schedule, run record, or commit policy. They edit the selected filesystem
directly unless `--isolate` creates a retained branch and worktree.

## Usage

```sh
openknowledge agent
openknowledge agent --runtime claude
openknowledge agent --runtime grok --model grok-4.5
openknowledge agent --runtime opencode --model provider/model
openknowledge agent exec "Update the whitepaper"
openknowledge agent exec --runtime claude "Repair citations"
openknowledge agent integrate Wiki
openknowledge agent suggestions
openknowledge agent doctor
openknowledge agent doctor --runtime opencode --json
openknowledge agent exec --isolate "Update the wiki"
```

## Commands And Flags

| Input | Default | Effect |
| --- | --- | --- |
| initial prompt | none | Starts an interactive harness session with the steered task. With no task, the contract asks the agent to wait for one. |
| `exec <prompt>` | required | Runs one non-interactive task and returns the harness exit status. |
| `doctor` | all runtimes | Probes harness executables without starting a model session. An explicit unavailable `--runtime` exits nonzero. |
| `integrate <wiki>` | - | Installs project-scoped discovery skills and observation hooks. `--global` installs discovery-only user skills. |
| `suggestions [wiki]` | `Wiki` | Lists the private maintenance inbox; nested `apply` and `dismiss` manage individual suggestions. |
| `--runtime` | `codex` | Selects `codex`, `claude`, `grok`, or `opencode`. |
| `--model` | harness default | Passes a harness-specific model override. |
| `--path` | current directory | Selects the editable workspace. |
| `--isolate` | false | Creates and retains a branch/worktree at `HEAD`. |
| `--no-steer` | false | Passes only the user or generated workflow prompt. |

Executable overrides are `OPENKNOWLEDGE_CODEX`, `OPENKNOWLEDGE_CLAUDE`, and
`OPENKNOWLEDGE_GROK`, and `OPENKNOWLEDGE_OPENCODE`. Each explicit override is version-probed and fails
closed. Codex discovery additionally skips broken `PATH` wrappers and checks
supported macOS Codex.app and ChatGPT.app bundles.

## Harness Contracts

| Runtime | Interactive | Non-interactive |
| --- | --- | --- |
| Codex | `codex --sandbox workspace-write` | `codex exec --sandbox workspace-write <prompt>` locally; scheduled jobs use the explicit stdin form `codex exec ... -`. |
| Claude Code | `claude` | `claude --print --no-session-persistence` with `acceptEdits` and a narrow allowlist for Open Knowledge validation and read-only Git inspection. |
| Grok Build | `grok` | `grok --no-auto-update --always-approve --single <prompt>`; optional `--model` selects an xAI or configured custom model. |
| OpenCode | `opencode` | `opencode run --auto`; explicit deny rules in project configuration still apply. |

Codex documents `exec` as its non-interactive CI surface, Claude Code documents
`--print`, Grok documents `--single`, and OpenCode documents `run`. Open Knowledge owns the common task,
workspace, and publication contract while each adapter owns the vendor CLI
arguments.

Upstream references: [Codex CLI](https://developers.openai.com/codex/cli/),
[Claude Code CLI](https://code.claude.com/docs/en/cli-usage),
[Grok Build CLI](https://docs.x.ai/build/cli/headless-scripting),
[OpenCode CLI](https://opencode.ai/docs/cli/), and
[OpenCode providers](https://opencode.ai/docs/providers/).

Local OpenCode follows the user's normal provider configuration. A headless
OpenCode worker receives `OPENCODE_API_KEY`; repository `opencode.json` may bind
that placeholder to any provider and should choose a default model or set
`agent.model`. The separate official Grok worker receives `XAI_API_KEY` with no
credential file required.

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

## Command Change History

### 2026-07-17 - Multi-harness Open Knowledge agent

Added Codex, Claude Code, Grok Build, and OpenCode adapters; the default versioned steering
contract; `--runtime`, `--no-steer`, executable overrides, setup/source
workflow adapters, and `doctor`. Setup adapters are now invoked through the
canonical [`openknowledge setup`](setup.md) command.

### 2026-07-17 - Project observation integration

Documented that project-scoped hooks installed by `openknowledge agent integrate`
observe proxy and directly launched harness sessions through one suggestion
format.

### 2026-07-17 - Unified agent-maintenance namespace

Grouped project integration and the private suggestion inbox under the existing
`agent` command.

### 2026-07-17 - Initial human-facing command

Added interactive and one-shot modes, direct editing by default, retained
`--isolate` worktrees, and resilient Codex executable discovery. The former
automation group named `agents` was removed; automation lives under `jobs`.

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
