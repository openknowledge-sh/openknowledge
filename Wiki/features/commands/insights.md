---
type: Command Documentation
title: openknowledge automation insights
description: Capture, review, and execute private evidence-backed knowledge insights.
tags: [openknowledge, cli, command, insights, observation, agent]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge automation insights`

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
okn automation insights
okn automation insights list Wiki
okn automation insights create "Document the deployment rollback workflow"
okn automation insights create "Document rollback" --target operations/deploy.md --evidence "deploy.sh exposes rollback"
okn automation insights create "Confirm a policy" --risk medium --confidence 0.8 --owner github:docs-reviewer
okn automation insights run <insight>
okn automation insights run --all
okn automation insights run <insight> --runtime claude
okn automation insights run <insight> --isolate
okn automation insights dismiss <insight>
okn automation insights from-usage <file-or-dir> --min-occurrences 2
okn automation insights from-usage <file-or-dir> --eval-out .openknowledge/evals/usage-gaps.yaml
okn automation insights from-audit audit-report.json
okn automation jobs new insights --out .openknowledge/jobs/insights.md
```

Without a path, listing finds the connected knowledge base from
`.openknowledge/integration.toml`. It prints pending insights from oldest to
newest. `<insight>` can be a path, filename, filename stem, or insight ID.

## Explicit capture

`create` is deterministic and does not start a model. It finds the project
integration and sanitizes the summary and evidence. It then writes a private
pending insight. It reports the new file with a slash-separated project path.
It does not write a duplicate insight.

You can repeat `--target` and `--evidence`. Without a target, the insight uses
`.` for the complete knowledge base. Targets must be relative to the knowledge
base. The command rejects an insights directory that uses a symlink outside
the wiki.

Use `--risk`, `--confidence`, and repeatable `--owner` flags to set a
maintenance route. Without these flags, the route is `medium`, `human`, 0.75,
and `unassigned`.

Use the same command in a terminal or an agent skill:

```sh
okn automation insights create "<durable knowledge gap>" \
  --target "<likely wiki path>" \
  --evidence "<concise repository evidence>"
```

## Maintenance routes

Each insight contains an `okf_insight_route` mapping with `risk`, `approval`,
`confidence`, and `owners`. Approval is derived from risk:

| Risk | Approval | Confidence rule | Processing |
| --- | --- | --- | --- |
| `low` | `auto` | At least 0.95 | An agent can propose and verify the declared change. |
| `medium` | `human` | At least 0.60 | A pull request requires human review. |
| `high` | `expert` | From 0 through 1 | Automation can add current evidence and block the insight. It cannot edit a declared knowledge target. |

When `risk` is absent, confidence selects the route. A value of at least 0.95
selects low risk. A value from 0.60 through 0.94 selects medium risk. A lower
value selects high risk. Owner values are bounded identifiers. Use
`github:<login>` for a GitHub user and `github-team:<slug>` for a GitHub team.
Other owner values remain metadata.

Local `insights run` reports a high-risk insight to its owners and does not
start an agent for it. The scheduled insights job can add current evidence and
set the insight to `blocked`. Runtime exchange validation rejects an expert
proposal that changes a declared target.

## Audit findings

Convert every finding in a strict audit report to a private insight:

```sh
okn automation insights from-audit <audit-report.json>
```

The command preserves the finding ID for stable deduplication. It copies the
finding impact and evidence and reads `owner` and `owners` from target
frontmatter. A route uses `unassigned` when no target supplies an owner.

Claim conflicts, changed sources, frequently used unverified knowledge, and
missing owners use `high/expert`. An unanswered question uses `medium/human`
with 0.95 confidence. Other audit categories use `medium/human` with 1.0
confidence.

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

## Runtime usage gaps

Use private runtime events to create stable knowledge gap insights:

```sh
okn automation insights from-usage <file-or-dir> \
  [--min-occurrences <n>] \
  [--eval-out <path>]
```

The input can be one event file or a directory of `*.jsonl` files. You can
give more than one input. The command strictly validates each v1 usage event.

`--min-occurrences` defaults to `2`. The command ignores clusters below this
count. It groups `no-evidence` and `policy-rejected` events by knowledge base
and keyed query fingerprint.

Each group has a stable `runtime-usage-gap` identity. A repeated run reuses an
existing insight instead of creating a duplicate. The insight records counts,
time range, channels, and policy rejection totals.

`--eval-out` creates a strict eval v1 dataset. The command adds a case only
when query capture supplied a question. Each case requires at least one
source.

The dataset uses private file permissions. The command uses exclusive file
creation and does not replace an existing path. Without captured queries, the
command creates insights but does not create an eval dataset.

## Markdown contract

Every insight file uses `type: Open Knowledge Insight` and
`okf_publish: false`. It contains:

* `status: draft` for a pending insight.
* `status: stable` for a resolved insight.
* `status: deprecated` for a dismissed insight.
* `okf_insight_status: blocked` with `status: draft` for a blocked insight.
* `generated.by` with the CLI or observer process actor.
* `generated.at` with the RFC 3339 creation time.
* Stable `okf_insight_id` and kind values.
* One or more knowledge-base-relative `okf_insight_targets`.
* One normalized `okf_insight_route` with risk, approval, confidence, and
  owners.
* Optional `okf_insight_finding_id` for an audit-derived insight.
* Human-readable `Insight` and `Evidence` sections.

Validation verifies the private marker, statuses, metadata, and target format.
The reader accepts legacy insight status, runtime, and creation fields. A
status change converts those legacy fields to the OKF 0.2 contract.
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
commit and branch bundle flow. Risk controls pull request review and optional
low-risk auto-merge in the runtime publisher. There is no dedicated insight
worker or queue service.


---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/insights_command.go`
> * `packages/cli/internal/insights/`
> * `packages/cli/internal/integration/`
> * `packages/cli/internal/okf/validation_rules.go`
> * `packages/cli/internal/usage/`
> * `packages/cli/internal/eval/dataset.go`
> * `packages/cli/internal/agents/templates.go`
> * `packages/cli/cmd/openknowledge/runtime_worker.go`
