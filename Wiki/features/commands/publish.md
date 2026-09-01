---
type: Command Documentation
title: openknowledge publish
description: Build configured viewer or MCP artifacts from checked knowledge.
tags: [openknowledge, cli, command, publish, viewer, mcp]
timestamp: 2026-09-01T00:00:00Z
---

# `openknowledge publish`

Use `okn publish` to build configured output from one managed knowledge base.

## Usage

```sh
okn publish [path] --plan
okn publish [path] --target viewer
okn publish [path] --target viewer --out ./site
okn publish [path] --target mcp --config runtime.toml
okn publish [path] --target mcp --config runtime.toml --id docs
```

Without `--target`, the command builds every output in `release.outputs`.
Supported targets are `viewer` and `mcp`.

Use `--plan` to inspect target file counts and destinations without writes.
The default viewer destination is
`<path>/.openknowledge/publish/viewer`. MCP publication uses `runtime.toml` by
default. Use `--id` to select a runtime knowledge base.

Publish requires a managed OKF bundle. It runs the unified knowledge check and
refuses `BLOCKED` or `UNMANAGED` input. Publication must also have a `READY`
layer and a configured output.

The viewer target uses the static HTML exporter. The MCP target builds a
runtime generation through the selected runtime configuration. That
configuration must select the same path with MCP output enabled.

The advanced `okn export` and runtime commands remain available for direct
control.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/publish_command.go`
> - `packages/cli/cmd/openknowledge/main.go`
> - `packages/cli/cmd/openknowledge/runtime_command.go`
