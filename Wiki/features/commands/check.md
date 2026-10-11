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
okn check [path] --config runtime.toml --id <knowledge-base>
```

The default path is the current directory. The default format is `text`.
For MCP checks, `--config` defaults to `runtime.toml`. Use `--id` to select a
runtime knowledge base.

Use `okn check` after setup or content changes. Use
[`okn validate`](validate.md) for detailed format and link diagnostics.

## Status

| Status | Meaning |
| --- | --- |
| `READY` | All configured checks passed. Optional layers may still be `NOT CONFIGURED`. |
| `NEEDS ATTENTION` | The knowledge base works, but one configured check needs attention. |
| `BLOCKED` | Structure, configuration, claims, or publication integrity blocks the workflow. |
| `UNMANAGED` | Markdown is searchable, but setup has not created an OKF bundle. |
| `NOT CONFIGURED` | A specific optional layer has no configuration. |

`NOT CONFIGURED` is a layer status. It does not reduce the overall status.
A `READY` result does not certify freshness or retrieval when those layers
are `NOT CONFIGURED`.

## Check layers

Structure validation and local link checks always run for a managed bundle.
The **Imported copy** layer compares copied files and their originals with the
recorded import digests. Changed files or unavailable originals require attention.
The check reports counts and never synchronizes files. Review differences before
updates. Freshness runs when an audit baseline exists. Retrieval runs when an eval
dataset exists. Claims run when the bundle contains claim documents.
Publication checks content eligibility for each output in `release.outputs`.
A `READY` Publication layer does not confirm that every target is ready.
Separate `Target viewer` and `Target mcp` layers report configured target readiness.

MCP readiness requires runtime configuration that selects this knowledge base.
Missing or mismatched runtime configuration sets `Target mcp` and Overall to
`NEEDS ATTENTION`. Configure the runtime, or use `okn publish --target viewer`
for viewer output. Pass the same `--config` and `--id` values to `check` and
`publish` when you select a runtime.

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
> - `packages/cli/internal/okf/setup_import_status.go`
> - `packages/cli/schemas/v1/check.schema.json`
