---
type: Command Documentation
title: openknowledge list
description: Prints a bundle tree and optional machine-readable inventory.
tags: [openknowledge, cli, command, inventory]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge list`

`openknowledge list` prints the bundle tree with inline validation issues. It
can also emit JSON inventory output for tools and agents.

The candidate connections model adds a second no-argument mode:
`openknowledge list` lists connected knowledge bundles, while
`openknowledge list <key-or-path>` prints the tree for a specific bundle. This
would align `list` with `open`, whose no-argument mode already starts from the
registry/workspace selector.

## Usage

```sh
openknowledge list [path]
openknowledge list --spec <version> [path]
openknowledge list --json [path]
openknowledge list --help
```

Candidate connection usage:

```sh
openknowledge list
openknowledge list <key-or-path>
openknowledge list --json
openknowledge list <key-or-path> --json
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `path` | argument | Knowledge base root. Defaults to the current directory. |
| `--spec` | flag | OKF spec version. Defaults to latest. |
| `--json` | flag | Print machine-readable inventory entries. |

In the candidate connections model, the optional argument becomes
`key-or-path`. Without it, `list` prints the connected bundle overview instead
of the current working directory tree.

## Candidate Connected Bundle Overview

No-argument `list` should print one compact block per connected bundle:

```text
Open Knowledge Connections

accessibility
  name    Accessibility Review
  path    /Users/me/.openknowledge/bundles/accessibility
  access  read
  source  github: openknowledge-sh/accessibility
  status  valid
  entries default, review
```

The overview should derive display fields from both registry state and bundle
content:

* `key`, `path`, `access`, `source`, and `managed` come from the local
  connection registry.
* `name`, `purpose`, `tags`, and entrypoint names are read from root
  `okf_bundle_*` metadata when present.
* If metadata is absent, `name` falls back to the root `index.md` H1, then the
  folder basename.
* `status` comes from the latest validation run or an on-demand lightweight
  validation pass.

JSON output should expose the same data so generic agent skills can discover
available knowledge packages before choosing whether to call the shipped
`openknowledge use` entrypoint reader.

## Use Cases

* Inspect a wiki from the terminal.
* Discover connected specialized knowledge bundles available to agents.
* Give agents a compact bundle map before opening files.
* Check validation issues in context.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/list.go`
> * `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> Update this page when tree formatting, JSON fields, validation attachment, or
> sorting behavior changes.
