---
name: openknowledge-wiki
description: Maintain this repository's Open Knowledge wiki for CLI feature documentation and changelog memory. Use when Codex touches Open Knowledge CLI behavior, command flags, exporters, validation, viewer behavior, setup flow, README or docs that describe CLI behavior, release-impacting package changes, or any feature work that should update Wiki/.
---

# OpenKnowledge Wiki

## Overview

Use this skill to keep the colocated wiki at `Wiki/` in sync with CLI behavior and developer-facing feature documentation.

## Workflow

1. Read `Wiki/AGENTS.md` first, then open the workflow that matches the change:
   - `Wiki/workflows/feature-docs.md` for command, exporter, viewer, setup, validation, registry, or docs changes.
   - `Wiki/workflows/changelog-updates.md` for package or CLI behavior changes that should be remembered.
2. Inspect source files and existing docs before editing wiki pages. Prefer `rg` and source-backed summaries.
3. Update the smallest relevant wiki pages:
   - `Wiki/features/commands/*.md` for command behavior and use cases.
   - `Wiki/features/exporters/*.md` for exporter behavior and output contracts.
   - `Wiki/changelog/cli.md` for user-visible or developer-relevant CLI changes.
4. Keep agent-maintenance material at the end of concept pages in the footer
   block marked `<!-- okf-footer: agent-maintenance -->`. Use that footer for
   source anchors, update notes, and similar grounding metadata instead of
   prominent `##` headings.
5. Keep shipped behavior separate from planned work. For example, `openknowledge to graph` belongs on the graph exporter candidate page until implemented.
6. For bounded wiki maintenance tasks, spawn focused subagents when the current runtime supports them. Prefer lower reasoning effort for those subagents because they should inspect narrow source/doc areas, draft concise updates, or validate focused assumptions rather than own broad architecture decisions.
7. After meaningful wiki edits, run `openknowledge validate "Wiki"` and fix errors or avoidable warnings before finishing.

## Boundaries

Do not rewrite the wiki for unrelated refactors, dependency updates, formatting-only edits, or changes that do not alter CLI behavior, docs, workflows, or release-facing behavior. Do not store secrets, private tokens, or unsupported claims in the wiki.
