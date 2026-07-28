---
type: Command Documentation
title: openknowledge automation
description: Run unattended maintenance and hosted runtime workflows.
tags: [openknowledge, cli, command, automation, jobs, insights, runtime, deploy]
timestamp: 2026-07-28T00:00:00Z
---

# `openknowledge automation`

Use `okn automation` for workflows that run outside the direct local
knowledge loop.

## Usage

```sh
okn automation jobs <command>
okn automation insights <command>
okn automation runtime <command>
okn automation deploy <provider>
```

## Commands

| Command | Purpose |
| --- | --- |
| [`jobs`](jobs.md) | Run repeatable isolated maintenance jobs from Markdown specifications. |
| [`insights`](insights.md) | Capture and resolve maintenance insights. |
| [`runtime`](runtime.md) | Build and serve immutable knowledge generations. |
| [`deploy`](deploy.md) | Provision the runtime on a supported provider. |

Local commands remain at the root. For example, use `okn setup`,
`okn search`, `okn validate`, `okn view`, and `okn integration` without the
automation prefix.

The old root forms, such as `okn jobs` and `okn runtime`, remain compatible.
They are hidden from root help. New scripts and documentation must use the
`automation` namespace.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/automation_command.go`
> * `packages/cli/cmd/openknowledge/command_catalog.go`
