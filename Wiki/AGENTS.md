---
type: Agent Rules
title: Wiki Agent Rules
description: Rules for agents maintaining the Open Knowledge CLI developer wiki.
tags: [openknowledge, agents, cli, docs]
timestamp: 2026-06-18T00:00:00Z
---

# Agent Rules

This wiki lives at `Wiki/` in the Open Knowledge CLI repository. It exists to
keep developer-facing CLI feature documentation and changelog memory close to
the code.

## Read First

Before changing CLI behavior, command flags, exporters, validation, viewer
behavior, setup flow, README/docs that describe CLI behavior, or release-facing
package behavior, read the relevant page under [features](features/) and the
workflow that matches the work:

* [Feature docs workflow](workflows/feature-docs.md)
* [Changelog update workflow](workflows/changelog-updates.md)

The repo-local Codex skill is `.codex/skills/openknowledge-wiki/SKILL.md`.

## Update Rules

* Update [changelog/cli.md](changelog/cli.md) when a package change affects CLI behavior, command output, flags, setup, exporters, validation, viewer behavior, release packaging, or user-facing docs.
* Update the relevant page under [features/commands](features/commands/) or [features/exporters](features/exporters/) when behavior, arguments, examples, or use cases change.
* For each command page, maintain a dated command change history for major command-surface changes, including added, removed, renamed, or behavior-changing arguments, flags, subcommands, frontmatter/config properties, output fields, and exit-code semantics.
* Keep shipped behavior separate from planned work. Planned `openknowledge to graph` work belongs on [features/exporters/graph.md](features/exporters/graph.md) until implemented.
* Preserve source paths in prose or code spans when a page depends on specific files.
* Keep pages concise and scan-friendly. Prefer sections for purpose, usage, arguments, use cases, implementation notes, and update notes.
* When the current agent runtime supports subagents, use focused lower-reasoning
  subagents for bounded wiki maintenance tasks such as narrow source inspection,
  targeted docs checks, or validation-focused review.

## Do Not Update

Do not update the wiki for unrelated refactors, formatting-only changes,
dependency noise, or changes that do not affect CLI behavior, docs, workflows,
or release-facing behavior.

Do not store secrets, tokens, private credentials, or unverified claims in the
wiki. Do not claim native automations exist unless they have actually been
created in the current agent runtime.

## Validation

Follow the local [Open Knowledge Format spec](SPEC.md). Keep non-reserved
Markdown concept documents OKF-valid with YAML frontmatter and a non-empty
`type` field. Treat `index.md` as progressive-disclosure indexes and `log.md`
as chronological logs.

After meaningful wiki edits, run:

```sh
openknowledge validate "Wiki"
```

Fix validation errors and avoidable warnings before finishing.
