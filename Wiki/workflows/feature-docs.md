---
type: Workflow
title: Feature Docs Workflow
description: Maintain current, concise CLI feature documentation.
tags: [openknowledge, cli, workflow, docs]
timestamp: 2026-07-18T00:00:00Z
---

# Feature Docs Workflow

## When to use this workflow

Use this workflow for these documentation changes:

- CLI commands, flags, or help text
- Exporters and validation
- Setup or registry behavior
- Viewer behavior
- Configuration
- README examples

## Process

1. Read [Agent Rules](/AGENTS.md).
2. Read the applicable command or exporter page.
3. Inspect the implementation and focused tests.
4. Use the source code as the authority for behavior.
5. Update the smallest current-state reference page.
6. For a user-visible change, update the
   [CLI changelog](/changelog/cli.md).
7. Run `openknowledge validate Wiki`.
8. Fix all errors and avoidable warnings.

## Page structure

Use progressive disclosure. Put information in this order:

1. One sentence that gives the purpose
2. Copyable usage
3. Options and defaults
4. Behavior that affects results, files, processes, the network, or exit status
5. Necessary cautions

Do not repeat the root help or product positioning.
Do not repeat implementation history or security explanations from another
page.

Keep a simple command page to approximately 80 lines or fewer. Keep a complex
runtime page to approximately 200 lines or fewer.

Put source anchors and update notes in the footer:

```md
---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/...`
```

Do not put candidate work in the released command index.

When a new command or exporter is available, add its page. Then, add an index
entry.

## Language

Write project-authored Wiki text with ASD-STE100 Issue 9 rules. Use the
technical terms that [Agent Rules](/AGENTS.md) identifies.

Use active voice and simple verb tenses. Do not use contractions, phrasal
verbs, or semicolons.

Keep descriptive sentences to a maximum of 25 words. Keep instructions to a
maximum of 20 words.

Put only one instruction in each procedural sentence. Start the instruction
with an imperative verb.

Use one term for one item. Make each pronoun reference clear.

## Boundaries

Do not rewrite broad documentation for an unrelated refactor.
Do not describe behavior that the source code or tests do not support.
Keep release history in the changelog, not on command pages.
