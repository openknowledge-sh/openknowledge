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

For command-surface work, start from the matching page under
[features/commands](features/commands/). Agent setup and maintenance-rule
changes should read both [setup](features/commands/setup.md) and
[rules](features/commands/rules.md), because `openknowledge setup --rules` and
`openknowledge prompt rules` shares the same canonical rule catalog.

The repo-local Codex skill is `.codex/skills/openknowledge-wiki/SKILL.md`.

## Update Rules

* Update [changelog/cli.md](changelog/cli.md) when a package change affects CLI behavior, command output, flags, setup, exporters, validation, viewer behavior, release packaging, or user-facing docs.
* Update the relevant page under [features/commands](features/commands/) or [features/exporters](features/exporters/) when behavior, arguments, examples, or use cases change.
* Update [features/commands/rules.md](features/commands/rules.md) when agent maintenance rule IDs, descriptions, generated instructions, `--path`, `--target`, `rules apply`, or setup rule selection changes.
* Keep release history in [changelog/cli.md](changelog/cli.md), not duplicated on
  command pages. Command pages describe the current surface.
* Keep shipped behavior separate from planned work. Do not place candidate
  commands or exporters in the shipped command index.
* Preserve source paths in prose or code spans when a page depends on specific files.
* Write reference documentation, not product copy. Lead with the task, prefer
  tables and copyable examples, state defaults once, and remove rationale that
  does not change how a reader uses the feature.
* Keep pages concise and scan-friendly. A simple command page should usually
  fit within 80 lines; complex runtime pages should rarely exceed 200.
* Put agent-maintenance material at the end of concept pages in a footer block
  instead of top-level Markdown headings. Use this shape for source anchors,
  update notes, and similar grounding metadata:

  ```md
  ---

  <!-- okf-footer: agent-maintenance -->

  > **Source anchors**
  >
  > - `packages/...`
  >
  > **Update notes**
  >
  > Update this page when shipped behavior changes.
  ```

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
