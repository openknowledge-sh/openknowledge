---
type: Command Documentation
title: openknowledge agent suggestions
description: List, apply, dismiss, observe, and verify private Markdown knowledge suggestions.
tags: [openknowledge, cli, command, suggestions, observation, security]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge agent suggestions`

Suggestions are the narrow file interface between an agent session and normal
Open Knowledge maintenance. A project observer writes a pending Markdown file;
the user can edit, delete, apply, dismiss, commit, or leave it uncommitted.

```text
agent session
    -> Wiki/suggestions/*.md
    -> ordinary Open Knowledge Job
    -> isolated worktree and validation
    -> commit and draft PR
```

There is no suggestion worker role, persistent reconciler, queue schema, or
special cloud lifecycle.

## Usage

```sh
openknowledge agent suggestions
openknowledge agent suggestions Wiki
openknowledge agent suggestions apply Wiki/suggestions/<file>.md
openknowledge agent suggestions dismiss Wiki/suggestions/<file>.md
openknowledge jobs new suggestions \
  --out .openknowledge/jobs/suggestions.md
```

With no path, the list command reads `.openknowledge/integration.toml` and
prints pending suggestions from the connected knowledge base oldest first. An
explicit path overrides discovery. `apply` parses and
preflights the embedded unified diff, rejects edits outside the connected
knowledge base or declared targets, applies the patch as an uncommitted change,
and atomically changes the status to `applied`. A conflict changes neither the
target filesystem nor the suggestion. `dismiss` atomically changes a pending
suggestion to `dismissed`.

## Markdown Contract

Every suggestion is an OKF concept with `type: Open Knowledge Suggestion`, a
semantic title and description, one of `pending`, `applied`, `dismissed`, or
`blocked`, stable suggestion metadata, at least one knowledge-base-relative
target, evidence, and a fenced unified diff. It must declare
`okf_publish: false`.

The validator enforces that private marker and the required status, metadata,
and target shape. The publication-set builder therefore omits suggestions from
viewer, search, MCP, `llms.txt`, sitemap, and copied public source artifacts.
The observer also excludes the suggestions directory from its Git diff, so a
new or updated suggestion cannot recursively generate another suggestion.

Raw transcripts, credentials, and agent logs are not written into a suggestion.
The observer analyzes the bounded hook event and available session trace,
including user/assistant messages, tool calls/results, errors, retries, and
validation events. It keeps only a sanitized final assistant outcome and
aggregate event counts, then combines those with session metadata and the
current Git diff to create semantic intent, evidence, declared targets, and the
proposed patch.
Evidence may name repository files changed by the session, but the embedded
diff is filtered to the connected knowledge base. Source or application diffs
outside the Wiki are never copied into suggestion Markdown. If the filtered
diff appears to contain a credential, the observer omits the entire patch and
leaves only semantic intent, targets, and a warning for manual review.

## Maintenance Job

The `suggestions` Job template is an ordinary current-schema Markdown Job. It
runs every 24 hours, uses `agent.runtime: codex`, discovers the connected
knowledge base, processes at most five pending files oldest first, validates
the result, commits verified changes, and requests a draft PR. Change the
schedule, batch instruction, base branch, or runtime by editing the generated
Markdown.

The prompt first attempts a clean patch and falls back to semantic intent when
the base is stale. It marks incorporated suggestions `applied` and malformed or
obsolete suggestions `blocked`. `concurrency.key: knowledge-suggestions`
prevents simultaneous local runs, but v1 does not lock for the lifetime of an
open PR; a later schedule can therefore create an overlapping draft PR.

The template runs `openknowledge agent suggestions verify`. This single
verification step discovers the integration, rejects edits outside the
connected knowledge base,
edits outside targets belonging to suggestions changed from `pending` to
`applied`, rejects newly created publishable pages, and performs normal OKF
validation. The existing Jobs runner then
owns worktree isolation, verification, commit creation, branch bundling, and
publisher handoff. A no-change run creates neither a commit nor a PR.

## Command Change History

### 2026-07-17 - Markdown suggestions

Added project observation suggestions, atomic local list/apply/dismiss
operations, strict validation/publication exclusion, target verification, and
the ordinary `jobs new suggestions` maintenance template.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/suggestions_command.go`
> * `packages/cli/internal/suggestions/`
> * `packages/cli/internal/okf/validation_rules.go`
> * `packages/cli/internal/agents/templates.go`
