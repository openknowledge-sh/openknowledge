---
type: Workflow
title: Changelog Update Workflow
description: How agents maintain CLI changelog memory.
tags: [openknowledge, cli, workflow, changelog]
timestamp: 2026-06-18T00:00:00Z
---

# Changelog Update Workflow

## Trigger

Use this workflow when a change affects users. This requirement applies to
these changes:

- CLI behavior, output, flags, or help text
- Validation rules or export output
- Viewer behavior or setup prompts
- Registry behavior or release packages
- npm wrapper behavior
- User documentation

## Inspect

1. Read the [CLI changelog](/changelog/cli.md).
2. Inspect the changed files with `git diff --stat`.
3. Inspect the applicable file diffs.
4. Read the related command or exporter pages under [features](/features/).

## Update

Add an entry under `## Unreleased` in the
[CLI changelog](/changelog/cli.md). Include this information:

- The change
- Its effect on users
- Source anchors
- Updated documentation

Keep entries short. Group entries by product area.

Use headings such as `Viewer`, `Setup`, `Automation`, `Registry`,
`Compatibility`, or the applicable product feature.

Do not add dates to entry headings. Put the release date only in the released
version heading.

## Do not update

Do not add an entry for a formatting-only edit.

Do not add an entry for an internal cleanup that does not change behavior.
Do not add an entry for a dependency change that does not affect users.

## Verify

Run:

```sh
okn validate "Wiki"
```

Fix all errors and avoidable warnings.
