---
type: Agent Rules
title: Wiki Agent Rules
description: Rules for agents that maintain the Open Knowledge CLI developer wiki.
tags: [openknowledge, agents, cli, docs]
timestamp: 2026-06-18T00:00:00Z
---

# Agent Rules

This wiki is in the `Wiki/` directory of the Open Knowledge CLI repository.
It keeps CLI documentation and changelog records near the code.

## Read first

Before you change CLI behavior, read the applicable page under
[features](features/). Then, read the workflow that applies to the change:

- [Feature docs workflow](workflows/feature-docs.md)
- [Changelog update workflow](workflows/changelog-updates.md)

This requirement applies to these changes:

- Commands and flags
- Exporters and validation
- The viewer and setup flow
- CLI content in the README or other documentation
- Package behavior that affects a release

For a command change, start with its page under
[features/commands](features/commands/).

For an agent rule change, read the
[setup](features/commands/setup.md) and [rules](features/commands/rules.md)
pages. Both command surfaces use the same rule catalog.

The repo-local Codex skill is `.codex/skills/openknowledge-wiki/SKILL.md`.

## Update rules

- Update [changelog/cli.md](changelog/cli.md) when a package change affects
  users.
- Include changes to CLI behavior, output, flags, setup, exporters, validation,
  the viewer, release packages, and user documentation.
- Update the applicable page under
  [commands](features/commands/) or [exporters](features/exporters/) when its
  behavior changes.
- Include changes to arguments, examples, and use cases.
- Update [features/commands/rules.md](features/commands/rules.md) when the rule
  catalog or rule commands change.
- Include changes to `okn setup --rules`, `okn prompt rules`, and
  `rules apply`.
- Include changes to the `--path` and `--target` flags.
- Keep release history in the changelog. Command pages describe only the
  current interface.
- Keep released behavior separate from planned work.
- Do not put candidate commands or exporters in the released command index.
- Preserve source paths when a page depends on specific files.
- Use `okn` in user-facing command examples.
- Use `openknowledge` only for canonical binary, package, or historical
  references.
- Write reference documentation, not product copy.
- Start with the task. Give options and defaults one time.
- Use tables only when they make comparison easier.
- Give copyable examples.
- Remove explanations that do not change how a reader uses the feature.
- Keep each page concise and easy to scan.
- Keep a simple command page to approximately 80 lines or fewer.
- Keep a complex runtime page to approximately 200 lines or fewer.
- Put agent-maintenance information in a footer block.
- Use this form for source anchors, update notes, and related metadata:

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

- When the agent runtime supports subagents, use them for bounded Wiki tasks.
- Give each subagent a narrow inspection, documentation, or validation task.

## Writing standard

Write all project-authored Wiki text with the rules in ASD-STE100 Issue 9.
Use American English.

Follow the wiki-local [ASD-STE100 rule](rules/asd-ste100.md).

Use the official
[ASD-STE100 Issue 9](https://www.asd-ste100.org/assets/files/ASD-STE100_ISSUE9.pdf)
for the complete rules and dictionary.

Apply these rules:

- Use approved words, or use established Open Knowledge technical terms.
- Use one technical term for one item or action.
- Use the active voice.
- Use the simple present, simple past, or simple future tense.
- Use an `-ing` form only in an established technical term.
- Do not use contractions or semicolons.
- Keep a descriptive sentence to a maximum of 25 words.
- Keep an instruction to a maximum of 20 words.
- Give only one instruction in each procedural sentence.
- Start each instruction with an imperative verb.
- Put a necessary condition before the instruction.
- Keep each paragraph on one topic.
- Keep each paragraph to a maximum of six sentences.
- Use a vertical list for complex information.
- Keep multi-word nouns to three words when an established technical term does
  not require more words.
- Make each pronoun reference clear.

Treat product names, command names, flags, paths, schema fields, protocol names,
and programming terms as technical terms. Do not change their spelling.

`SPEC.md` is a pinned upstream document. Keep its upstream text unchanged.
Apply this writing standard only to the local notice around that text.

## Do not update

Do not update the wiki for an unrelated refactor or a formatting-only change.
Do not update it for dependency changes that do not affect users.

Do not store secrets, tokens, private credentials, or unverified claims in the
wiki. Do not claim that an automation exists until the agent runtime creates
it.

## Validation

Follow the local [Open Knowledge Format specification](SPEC.md).

Keep each non-reserved Markdown concept valid for OKF. Each concept must have
YAML frontmatter and a non-empty `type` field.

Use `index.md` files as progressive-disclosure indexes. Use `log.md` files as
chronological logs.

After a meaningful Wiki edit, run:

```sh
okn validate "Wiki"
```

Fix all errors and avoidable warnings.
