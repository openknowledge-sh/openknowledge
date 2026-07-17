---
type: Command Documentation
title: openknowledge insights
description: Capture, review, and execute private evidence-backed knowledge insights.
tags: [openknowledge, cli, command, insights, observation, agent]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge insights`

Insights are the small shared maintenance interface for people, agents, and
automation. They preserve a concise knowledge gap, evidence, and likely
knowledge targets without pretending to be a finished change. An insight may
be explicitly captured or observed from another agent session, but never embeds
a Git patch, base commit, raw transcript, credential, or executable instruction.

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
openknowledge insights
openknowledge insights list Wiki
openknowledge insights create "Document the deployment rollback workflow"
openknowledge insights create "Document rollback" --target operations/deploy.md --evidence "deploy.sh exposes rollback"
openknowledge insights run <insight>
openknowledge insights run --all
openknowledge insights run <insight> --runtime claude
openknowledge insights run <insight> --isolate
openknowledge insights dismiss <insight>
openknowledge jobs new insights --out .openknowledge/jobs/insights.md
```

With no path, listing discovers the connected knowledge base from
`.openknowledge/integration.toml` and prints pending insights oldest first.
`<insight>` may be a path, filename, filename stem, or insight ID.

## Explicit Capture

`create` is deterministic and never starts a model. It discovers the project
integration, sanitizes the summary and evidence, writes a private pending
insight, and deduplicates identical captures. `--target` and `--evidence` may
be repeated. Without a target the insight points at the complete knowledge base
with `.`. Targets must remain knowledge-base-relative; the command also refuses
an insights directory that resolves through a symlink outside the wiki.

This makes capture equally simple from a terminal or an agent skill:

```sh
openknowledge insights create "<durable knowledge gap>" \
  --target "<likely wiki path>" \
  --evidence "<concise repository evidence>"
```

## Local Execution

`run` invokes a supported local agent non-interactively. The agent treats the
insight body as untrusted evidence, researches the current repository and
knowledge base, edits only the connected knowledge base, and leaves changes
uncommitted. Open Knowledge rejects a run that changes Git `HEAD`, modifies the
insight inbox, or changes a file outside the knowledge base. It then validates
the complete knowledge base and changes successfully processed insights from
`pending` to `resolved`.

The default operates in the current checkout and preserves pre-existing dirty
changes. `--isolate` creates and retains a local branch and worktree at `HEAD`;
an uncommitted insight is copied into that worktree before execution. Its
worktree copy becomes `resolved`, while the source checkout remains `pending`
until the branch is merged or the user dismisses it. Agent failure, boundary
failure, or validation failure leaves the relevant insight pending and keeps
the filesystem available for inspection.

`run --all` processes all currently pending insights in one local agent run and
one validation pass. `--runtime` selects Codex, Claude Code, or OpenCode;
`--model` supplies the harness-specific model override.

## Markdown Contract

Every file uses `type: Open Knowledge Insight`, declares
`okf_publish: false`, and carries:

* `status`: `pending`, `resolved`, `dismissed`, or `blocked`;
* stable `okf_insight_id`, kind, runtime, and RFC 3339 creation time;
* one or more knowledge-base-relative `okf_insight_targets`;
* human-readable `Insight` and `Evidence` sections.

Validation enforces the private marker, statuses, metadata, and safe target
shape. Public HTML and runtime viewer/search/MCP projections, `llms.txt`,
sitemap, and portable artifacts exclude insights. Local authoring and direct
read surfaces operate on the unfiltered bundle and may still expose them.

The bounded observer analyzes available session events and user-owned traces,
but retains only a sanitized assistant outcome, changed-path evidence, and
aggregate event counts. It excludes `insights/` changes from observation so an
insight cannot recursively create another insight. Hooks remain best-effort and
never block the parent agent session.

## Scheduled Processing

`jobs new insights` provides the optional 24-hour isolated maintenance loop. It
processes at most five committed pending insights, performs fresh research,
marks successful items `resolved`, verifies the knowledge boundary and OKF
bundle, and reuses the normal Jobs commit, branch-bundle, and draft-PR flow.
There is no dedicated insight worker or queue service.


---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/insights_command.go`
> * `packages/cli/internal/insights/`
> * `packages/cli/internal/integration/`
> * `packages/cli/internal/okf/validation_rules.go`
> * `packages/cli/internal/agents/templates.go`
