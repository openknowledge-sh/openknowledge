---
type: Workflow
title: Changelog Update Workflow
description: How agents maintain CLI changelog memory.
tags: [openknowledge, cli, workflow, changelog]
timestamp: 2026-06-18T00:00:00Z
---

# Changelog Update Workflow

## Trigger

Use this workflow when changes affect CLI behavior, command output, flags, help
text, validation rules, export output, viewer behavior, setup prompts, registry
semantics, release packaging, npm wrapper behavior, or user-facing docs.

## Inspect

* Read [CLI changelog](/changelog/cli.md).
* Inspect changed files with `git diff --stat` and targeted diffs.
* Check related command or exporter pages under [features](/features/).

## Update

Add an entry under `## Unreleased` in [CLI changelog](/changelog/cli.md) with:

* what changed
* why it matters
* source anchors
* docs updated

Keep entries concise and grouped by date.

## Do Not Update

Do not add changelog entries for formatting-only edits, internal cleanup with no
behavioral impact, or dependency churn that does not affect CLI users or
developers.

## Verify

Run:

```sh
openknowledge validate "Wiki"
```

Fix validation errors and avoidable warnings before finishing.
