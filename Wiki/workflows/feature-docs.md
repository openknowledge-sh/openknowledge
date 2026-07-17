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
wiki content that explains CLI behavior.

## Inspect

* Read [Agent Rules](/AGENTS.md).
* Read the relevant page under [commands](/features/commands/) or [exporters](/features/exporters/).
* Inspect source files and tests for the changed behavior.
* Check `README.md`, [installation](/features/installation.md), and
  [CLI operations](/features/operations.md) when user-facing examples or
  operational docs are involved.

## Update

* Update the smallest relevant feature page.
* Add or revise usage, arguments, flags, examples, and use cases.
* Keep source anchors, update notes, and similar agent-maintenance metadata in
  the end-of-page footer block marked `<!-- okf-footer: agent-maintenance -->`
  instead of rendering them as prominent `##` sections.
* For command pages, update the command change history when a major command-surface change is made.
* Keep candidate work clearly labeled as candidate until shipped.
* If a new command or exporter exists, add a page and update the section index.

## Command Page Pattern

Use a progressive-disclosure order inspired by high-quality developer docs:
purpose, usage, options, examples, behavior, caveats, source anchors, and update
notes.

For command pages, prefer these sections when they add signal:

* `Usage` with copyable commands and `--help`.
* `Arguments And Flags` with kind, required/default state, and effect.
* `Quick Examples` covering common, scripting, and edge-case workflows.
* `Behavior` for input resolution, output format, exit codes, files
  read/written, network or process side effects, and target-specific modes.
* `Caveats` for surprising defaults, CI/headless differences, registry/viewer
  differences, and unsupported flags.
* `Command Change History` with dated entries for major command-surface changes:
  added, removed, renamed, or behavior-changing arguments, flags, subcommands,
  frontmatter/config properties, output fields, and exit-code semantics. Each
  entry should include the date, a concise change summary, and source anchors.

Keep short commands short. Use deeper behavior sections for complex commands
such as `openknowledge view`, `openknowledge validate`, and `openknowledge export`.

## Agent Maintenance Footer

Use a footer block for grounding metadata that agents need but readers should
not scan as primary documentation:

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

Source anchors should name the command entrypoint and focused tests when they
exist. Update notes should say when docs and CLI changelog memory must change.
Do not put these labels in `##` headings unless the page is itself a workflow
or rule document.

## Reference Patterns

These public docs are useful style references when evolving this wiki:

* [React `useEffect`](https://react.dev/reference/react/useEffect) - reference,
  usage, caveats, and troubleshooting.
* [TanStack Query `useQuery`](https://tanstack.com/query/latest/docs/framework/react/reference/useQuery)
  and [Important Defaults](https://tanstack.com/query/latest/docs/framework/react/guides/important-defaults)
  - separate exhaustive reference from mental-model guidance.
* [Next.js CLI](https://nextjs.org/docs/app/api-reference/cli/next) and
  [Vite CLI](https://vite.dev/guide/cli.html) - command usage and options by
  subcommand.
* [pnpm install](https://pnpm.io/cli/install),
  [npm install](https://docs.npmjs.com/cli/v11/commands/npm-install/), and
  [GitHub CLI `gh pr create`](https://cli.github.com/manual/gh_pr_create) -
  CLI examples, defaults, aliases, and edge cases.

## Do Not Update

Do not rewrite broad documentation for unrelated refactors. Do not claim a
feature exists unless the command surface or implementation supports it.

## Verify

Run:

```sh
openknowledge validate "Wiki"
```

Fix validation errors and avoidable warnings before finishing.
