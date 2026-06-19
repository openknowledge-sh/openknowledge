---
type: Changelog
title: CLI Changelog
description: Maintained changelog memory for Open Knowledge CLI package changes.
tags: [openknowledge, cli, changelog]
timestamp: 2026-06-18T00:00:00Z
---

# CLI Changelog

This page records CLI-facing package changes in a developer-focused format.
Entries should summarize what changed, why it matters, source anchors, and docs
that were updated.

## Unreleased

### 2026-06-19 - Viewer file tree system badges

* The viewer file explorer now shows only the filename in each file row instead
  of repeating the full relative path on the right.
* Removed the generic `md` badge and replaced it with a right-aligned `system`
  badge only for reserved Markdown files such as `index.md` and `log.md`.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer search dropdown focus and keyboard controls

* The viewer search dropdown now opens on focus with top file entries for an
  empty query and stays open while typed search requests are pending, avoiding
  flicker between keystrokes.
* The dropdown closes when a result is activated, including after pending search
  requests resolve.
* Search now gives `index.md` files lower priority than comparable regular
  pages in both the local search API and exported static HTML.
* The document viewer header keeps its vertical padding so the top-bar search
  aligns with the logo.
* Search results can be selected with `ArrowDown`/`ArrowUp` and opened with
  `Enter` while focus stays in the search field.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `packages/cli/internal/okf/search.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer search moved out of sidebar

* Removed the duplicate search box from the file explorer sidebar; viewer
  search now lives only in the top bar.
* `Command+K` on macOS and `Ctrl+K` elsewhere still focus the top-bar search,
  and exported static HTML keeps the same search behavior.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Empty workspace graph overview

* The panel viewer empty workspace now uses a 50/50 overview with the file tree
  on the left and a connected graph of Markdown files on the right.
* The graph is built from local Markdown links and graph nodes open files as
  panels, including in exported static HTML.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Sidebar search restored in viewer

* The panel viewer file explorer now includes a search box above the file tree.
* The top bar now includes the primary search field, focused by `Command+K` on
  macOS and `Ctrl+K` elsewhere.
* Local `openknowledge open` pages use the existing `/api/search` endpoint, and
  exported static HTML searches the embedded note manifest in-browser.
* Search result clicks open as panels and keep the sidebar open.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Local viewer always uses panel stack

* Removed the local viewer focus-mode toggle so document browsing always uses
  the horizontally scrollable panel stack.
* File-tree and Markdown link navigation now consistently append or replace
  panels instead of switching into a single-page layout.
* Stack View Transitions now clear fallback panel-entry animation classes before
  the live DOM is shown again, avoiding a second flash after the transition.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Reachable local viewer URL

* `openknowledge open` now prints and opens the actual listener URL as the
  `Open Knowledge view` line, defaulting to `127.0.0.1`, so direct path aliases
  such as `/wiki/` remain reachable without local DNS setup.
* The optional local-domain URL is still printed as `Open Knowledge alias` for
  environments that map names such as `open.knowledge` to loopback.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`, `docs/cli.md`,
  `Wiki/features/commands/open.md`, `Wiki/changelog/cli.md`.

### 2026-06-18 - Markdown and frontmatter validation warnings

* `openknowledge validate` now checks Markdown syntax for malformed links,
  unclosed code spans, invalid table separators, and unclosed fenced code blocks.
* Parseable frontmatter formatting issues, such as duplicate keys or delimiter
  whitespace, are reported as warnings; frontmatter that cannot be parsed
  remains an error.
* Source anchors: `packages/cli/internal/okf/validate.go`,
  `packages/cli/internal/okf/frontmatter.go`.
* Docs updated: `README.md`, `docs/cli.md`,
  `Wiki/features/commands/validate.md`, `Wiki/changelog/cli.md`.

### 2026-06-18 - Registry-backed local viewer

* Changed `openknowledge open` without a path to open the Open Knowledge
  Registry viewer, with a left workspace selector for registered knowledge
  bases.
* Kept `openknowledge open <path-or-name>` as the direct viewer for one
  knowledge base.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`, `docs/cli.md`,
  `Wiki/features/commands/open.md`, `Wiki/features/commands/registry.md`.

### 2026-06-18 - Context-aware setup interview prompt

* Updated `openknowledge setup` so agents inspect the current workspace or
  target folder and relevant runtime-exposed memories before asking questions.
* The setup prompt and generated `SETUP.MD` now tell agents to ask only missing,
  context-specific questions instead of repeating a fixed questionnaire.
* Source anchors: `packages/cli/internal/okf/setup.go`,
  `packages/cli/internal/okf/new.go`, `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`, `docs/cli.md`, `packages/web/index.html`,
  `Wiki/features/commands/setup.md`.

### 2026-06-18 - Wiki maintenance loop initialized

* Created a colocated Open Knowledge wiki at `Wiki/`.
* Added command, exporter, installation, workflow, and changelog seed pages.
* Added root `AGENTS.md` and repo skill `.codex/skills/openknowledge-wiki/SKILL.md`
  so future agents update this wiki when touching CLI behavior.

## Baseline Command Surface

As of the wiki setup, the CLI exposes:

* `openknowledge setup`
* `openknowledge new`
* `openknowledge registry list`
* `openknowledge registry add`
* `openknowledge where`
* `openknowledge open`
* `openknowledge to html`
* `openknowledge to json`
* `openknowledge spec`
* `openknowledge validate`
* `openknowledge list`
* `openknowledge version`

## Entry Template

```md
### YYYY-MM-DD - Short change title

* What changed:
* Why it matters:
* Source anchors:
* Docs updated:
```

## Update Rules

Add an entry when a change affects command behavior, arguments or flags, help
text, validation rules, export output, viewer behavior, setup prompts, registry
semantics, release packaging, npm wrapper behavior, or developer-facing docs.

Do not add entries for purely internal refactors unless they alter user-visible
or developer-relevant behavior.
