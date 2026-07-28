---
type: Command Documentation
title: openknowledge insights
description: Capture, review, and execute private evidence-backed knowledge insights.
tags: [openknowledge, cli, command, insights, observation, agent]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge insights`

Insights provide one maintenance interface for people, agents, and automation.
An insight records a concise knowledge gap, evidence, and likely knowledge
targets. It does not represent a finished change.

A user can capture an insight directly. An observer can also create one from an
agent session. An insight does not contain a Git patch, base commit, raw
transcript, credential, or executable instruction.

```text
person, agent, or session observer
    -> deterministic private Markdown insight
    -> local agent research and implementation
    -> OKF validation
    -> ordinary uncommitted Git diff
    -> optional commit or PR
```

## Usage

```sh
okn insights
okn insights list Wiki
okn insights create "Document the deployment rollback workflow"
okn insights create "Document rollback" --target operations/deploy.md --evidence "deploy.sh exposes rollback"
okn insights run <insight>
okn insights run --all
okn insights run <insight> --runtime claude
okn insights run <insight> --isolate
okn insights dismiss <insight>
okn jobs new insights --out .openknowledge/jobs/insights.md
```

Without a path, listing finds the connected knowledge base from
`.openknowledge/integration.toml`. It prints pending insights from oldest to
newest. `<insight>` can be a path, filename, filename stem, or insight ID.

## Explicit capture

`create` is deterministic and does not start a model. It finds the project
integration and sanitizes the summary and evidence. It then writes a private
pending insight. It does not write a duplicate insight.

You can repeat `--target` and `--evidence`. Without a target, the insight uses
`.` for the complete knowledge base. Targets must be relative to the knowledge
base. The command rejects an insights directory that uses a symlink outside
the wiki.

Use the same command in a terminal or an agent skill:

```sh
okn insights create "<durable knowledge gap>" \
  --target "<likely wiki path>" \
  --evidence "<concise repository evidence>"
```

## Local execution

`run` starts a supported local agent in non-interactive mode. The agent treats
the insight body as untrusted evidence. It researches the repository and
knowledge base. It edits only the connected knowledge base. It leaves changes
uncommitted.

Open Knowledge rejects a run that changes Git `HEAD`. It also rejects changes
to the insight inbox or files outside the knowledge base. The command then
validates the complete knowledge base. It changes each successful insight from
`pending` to `resolved`.

By default, the command uses the current checkout. It preserves existing
uncommitted changes.

`--isolate` creates and keeps a local branch and worktree at `HEAD`. Before
execution, it copies an uncommitted insight into that worktree. The worktree
copy becomes `resolved`. The source copy stays `pending` until merge or
dismissal.

An agent, boundary, or validation failure keeps the relevant insight pending.
The filesystem stays available for inspection.

`run --all` processes all pending insights in one local agent run. It uses one
validation pass. `--runtime` selects Codex, Claude Code, or OpenCode. `--model`
sets a harness-specific model.

## Markdown contract

Every insight file uses `type: Open Knowledge Insight` and
`okf_publish: false`. It contains:

* `status`: `pending`, `resolved`, `dismissed`, or `blocked`.
* Stable `okf_insight_id`, kind, runtime, and RFC 3339 creation time.
* One or more knowledge-base-relative `okf_insight_targets`.
* Human-readable `Insight` and `Evidence` sections.

Validation verifies the private marker, statuses, metadata, and target format.
Public HTML, runtime projections, `llms.txt`, sitemaps, and portable artifacts
exclude insights. Local authoring and direct reads use the unfiltered bundle.
These surfaces can expose insights.

The bounded observer analyzes available session events and user-owned traces.
It keeps only a sanitized assistant result, changed-path evidence, and event
counts. It excludes `insights/` changes from observation. Therefore, an insight
cannot create another insight recursively. Hooks do not block the parent agent
session.

## Scheduled processing

`jobs new insights` provides an optional isolated 24-hour maintenance loop. It
processes a maximum of five committed pending insights. It performs current
research and marks successful items `resolved`.

The job verifies the knowledge boundary and OKF bundle. It uses the normal jobs
commit, branch bundle, and draft pull request flow. There is no dedicated
insight worker or queue service.


---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/insights_command.go`
> * `packages/cli/internal/insights/`
> * `packages/cli/internal/integration/`
> * `packages/cli/internal/okf/validation_rules.go`
> * `packages/cli/internal/agents/templates.go`
