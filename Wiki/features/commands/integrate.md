---
type: Command Documentation
title: openknowledge agent integrate compatibility alias
description: Use the deprecated agent integrate alias for integration install.
tags: [openknowledge, cli, command, integration, compatibility]
timestamp: 2026-07-28T00:00:00Z
---

# `openknowledge agent integrate`

`okn agent integrate` is a deprecated alias for
[`okn integration install`](integration.md).

## Usage

```sh
okn agent integrate Wiki --runtime codex
okn agent integrate Wiki --runtime claude --observe
okn agent integrate --global --runtime opencode
```

The alias has the same runtime selection, opt-in observation, file safety, and
global discovery behavior as the canonical command. New scripts must use
`okn integration`.


---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/integrate_command.go`
> * `Wiki/features/commands/integration.md`
