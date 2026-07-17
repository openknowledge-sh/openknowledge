---
type: Workflow
title: Feature Docs Workflow
description: Maintain current, concise CLI feature documentation.
tags: [openknowledge, cli, workflow, docs]
timestamp: 2026-07-18T00:00:00Z
---

# Feature Docs Workflow

## When to use it

Use this workflow for CLI commands, flags, help, exporters, validation, setup,
registry behavior, viewer behavior, configuration, or README examples.

## Process

1. Read [Agent Rules](/AGENTS.md) and the relevant command or exporter page.
2. Inspect the implementation and focused tests. Treat source as authoritative.
3. Update the smallest current-state reference page.
4. Add a concise release-level entry to the
   [CLI changelog](/changelog/cli.md) only for user-visible behavior.
5. Run `openknowledge validate Wiki` and fix errors and avoidable warnings.

## Page style

Use progressive disclosure:

1. one-sentence purpose;
2. copyable usage;
3. options with defaults;
4. behavior that affects outcomes, files, processes, network, or exit status;
5. caveats only when surprising.

Do not repeat root help, product positioning, implementation chronology, or
security rationale owned by another page. A simple command page should usually
fit within 80 lines; complex runtime references should rarely exceed 200.

Keep source anchors and update notes in the footer:

```md
---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/...`
```

Keep candidate work out of the shipped command index. Add a page and index
entry only when a new command or exporter is shipped.

## Boundaries

Do not rewrite broad documentation for unrelated refactors. Do not claim
behavior that source or tests do not support. Keep release history in the
changelog, not on command pages.
