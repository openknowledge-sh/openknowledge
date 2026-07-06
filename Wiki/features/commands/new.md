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
| `--bundle-name` | flag | Optional stable bundle id written to root `index.md` as `okf_bundle_name`. |
| `--bundle-title` | flag | Optional display title written as `okf_bundle_title`. |
| `--bundle-purpose` | flag | Optional bundle purpose written as `okf_bundle_purpose`. |
| `--bundle-tag` | repeatable flag | Optional bundle tag written into `okf_bundle_tags`. |
| `--bundle-entry` | repeatable flag | Optional entrypoint mapping as `name=path`, written as `okf_bundle_entry_<name>`. |

## Behavior

The command creates the destination directory when it does not exist and refuses
to write into an existing non-empty directory. When `folder` is omitted, the CLI
uses `--name` or the interactive name answer to derive a lowercase slug. When
`folder` is provided and `--name` is omitted, the prompt default is a title
derived from the folder name.

The scaffold writes the core handoff files only: `index.md`, `log.md`,
`AGENTS.md`, `SETUP.MD`, and `SPEC.md`.

When bundle metadata flags are provided, `new` writes optional Open Knowledge
CLI metadata into the root `index.md` frontmatter as flat `okf_bundle_*` keys.
This metadata is a tooling layer for discovery, future `connect`, and candidate
`get` entrypoint routing; it is not required for OKF conformance. Without these
flags, `new` writes only `okf_version: "0.1"` in the root index frontmatter.

`--bundle-entry` accepts repeatable `name=path` values. For example,
`--bundle-entry default=agents/checker.md` writes
`okf_bundle_entry_default: "agents/checker.md"`. The command records the
mapping only; setup or later authoring should create and maintain the target
entrypoint document.

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

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/new.go`
> * `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> When scaffold files, default frontmatter, path rules, or terminal output change,
> update this page and [CLI changelog](/changelog/cli.md).
