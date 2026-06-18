---
type: Command Documentation
title: openknowledge new
description: Scaffolds a minimal Open Knowledge bundle.
tags: [openknowledge, cli, command, scaffold]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge new`

`openknowledge new` creates a minimal OKF bundle with `index.md`, `log.md`,
`AGENTS.md`, `SETUP.MD`, and `SPEC.md`. The scaffold is intentionally small so
an agent can shape the final wiki around the user's domain.

## Usage

```sh
openknowledge new [folder]
openknowledge new --name <name> [folder]
openknowledge new --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `folder` | argument | Destination folder. Defaults to a slug derived from the knowledge base name. |
| `--name` | flag | Knowledge base display name. If omitted, the CLI prompts. |

## Use Cases

* Create the initial bundle for a project wiki.
* Generate a local pinned copy of the OKF spec.
* Produce an agent handoff file for post-scaffold customization.

## Source Anchors

* `packages/cli/internal/okf/new.go`
* `packages/cli/cmd/openknowledge/main.go`

## Update Notes

When scaffold files, default frontmatter, path rules, or terminal output change,
update this page and [CLI changelog](/changelog/cli.md).
