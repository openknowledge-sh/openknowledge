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
openknowledge new --bundle-name <id> --bundle-purpose <text> [folder]
openknowledge new --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `folder` | argument | Destination folder. Defaults to a slug derived from the knowledge base name. |
| `--name` | flag | Knowledge base display name. If omitted, the CLI prompts. |
| `--bundle-name` | flag | Optional stable bundle id written to `openknowledge.toml` as `[bundle].name`. |
| `--bundle-title` | flag | Optional display title written as `[bundle].title`. |
| `--bundle-purpose` | flag | Optional bundle purpose written as `[bundle].purpose`. |
| `--bundle-tag` | repeatable flag | Optional bundle tag written into `[bundle].tags`. |
| `--bundle-entry` | repeatable flag | Optional entrypoint mapping as `name=path`, written under `[bundle.entries]`. |

## Behavior

The command creates the destination directory when it does not exist and refuses
to write into an existing non-empty directory. When `folder` is omitted, the CLI
uses `--name` or the interactive name answer to derive a lowercase slug. When
`folder` is provided and `--name` is omitted, the prompt default is a title
derived from the folder name.

The default scaffold writes the core handoff files only: `index.md`, `log.md`,
`AGENTS.md`, `SETUP.MD`, and `SPEC.md`.

When bundle metadata flags are provided, `new` writes optional Open Knowledge
CLI metadata into `openknowledge.toml` under `[bundle]` and `[bundle.entries]`.
This metadata is a tooling layer for discovery, `connect`, and `use` entrypoint
routing; it is not required for OKF conformance. Root `index.md` keeps only
`okf_version: "0.1"` frontmatter.

`--bundle-entry` accepts repeatable `name=path` values. For example,
`--bundle-entry default=agents/checker.md` writes
`default = "agents/checker.md"` under `[bundle.entries]`. The command records the
mapping only; setup or later authoring should create and maintain the target
entrypoint document.

Generated `openknowledge.toml` metadata uses this shape:

```toml
[bundle]
name = "accessibility"
title = "Accessibility Review"
purpose = "Accessibility review guidance."
tags = ["accessibility", "review"]

[bundle.entries]
default = "agents/accessibility-checker.md"
```

## Quick Examples

```sh
openknowledge new ./project-memory
openknowledge new --name "Project Memory" ./project-memory
openknowledge new --name "Accessibility Review" \
  --bundle-name accessibility \
  --bundle-purpose "Accessibility review guidance." \
  --bundle-tag accessibility \
  --bundle-tag review \
  --bundle-entry default=agents/accessibility-checker.md \
  ./accessibility
```

## Use Cases

* Create the initial bundle for a project wiki.
* Seed optional bundle metadata for local registration and future agent
  entrypoints.
* Generate a local pinned copy of the OKF spec.
* Produce an agent handoff file for post-scaffold customization.

## Source Anchors

* `packages/cli/internal/okf/new.go`
* `packages/cli/cmd/openknowledge/main.go`

## Update Notes

When scaffold files, default frontmatter, path rules, or terminal output change,
update this page and [CLI changelog](/changelog/cli.md).
