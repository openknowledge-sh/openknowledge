---
type: Command Documentation
title: openknowledge check
description: Run configured knowledge checks and report one status.
tags: [openknowledge, cli, command, check, validation]
timestamp: 2026-09-01T00:00:00Z
---

# `openknowledge check`

Use `okn check` to inspect the current state of one knowledge base.

## Usage

```sh
okn check [path]
okn check [path] --format json
okn check [path] --gate
```

The default path is the current directory. The default format is `text`.

## Status

| Status | Meaning |
| --- | --- |
| `READY` | All configured checks passed. Optional layers may still be `NOT CONFIGURED`. |
| `NEEDS ATTENTION` | The knowledge base works, but one configured check needs attention. |
| `BLOCKED` | Structure, configuration, claims, or publication integrity blocks the workflow. |
| `UNMANAGED` | Markdown is searchable, but setup has not created an OKF bundle. |
| `NOT CONFIGURED` | A specific optional layer has no configuration. |

`NOT CONFIGURED` is a layer status. It does not reduce the overall status.

## Check layers

Structure validation and local link checks always run for a managed bundle.
Freshness runs when an audit baseline exists. Retrieval runs when an eval
dataset exists. Claims run when the bundle contains claim documents.
Publication evaluates each output in `release.outputs`.

For ordinary unmanaged Markdown, check reports `UNMANAGED` and blocks
publication. If the directory has no Markdown, check reports `BLOCKED`.

The command exits with status `1` for `BLOCKED`. Use `--gate` to also fail for
`NEEDS ATTENTION` and `UNMANAGED`.

JSON output uses `schemaVersion: "1"`. The `check.schema.json` file defines
the report, overall status, and ordered layer objects.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/check_command.go`
> - `packages/cli/schemas/v1/check.schema.json`
