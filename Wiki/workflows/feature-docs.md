---
type: Workflow
title: Feature Docs Workflow
description: How agents maintain CLI feature and command documentation.
tags: [openknowledge, cli, workflow, docs]
timestamp: 2026-06-18T00:00:00Z
---

# Feature Docs Workflow

## Trigger

Use this workflow when touching CLI commands, flags, help text, exporters,
validation, setup, registry behavior, the local viewer, README content, or
`docs/cli.md` content that explains CLI behavior.

## Inspect

* Read [Agent Rules](/AGENTS.md).
* Read the relevant page under [commands](/features/commands/) or [exporters](/features/exporters/).
* Inspect source files and tests for the changed behavior.
* Check `README.md` and `docs/cli.md` when user-facing examples or operational docs are involved.

## Update

* Update the smallest relevant feature page.
* Add or revise usage, arguments, flags, examples, use cases, source anchors, and update notes.
* Keep candidate work clearly labeled as candidate until shipped.
* If a new command or exporter exists, add a page and update the section index.

## Do Not Update

Do not rewrite broad documentation for unrelated refactors. Do not claim a
feature exists unless the command surface or implementation supports it.

## Verify

Run:

```sh
openknowledge validate "Wiki"
```

Fix validation errors and avoidable warnings before finishing.
