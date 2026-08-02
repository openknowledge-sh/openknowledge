---
type: Decision
title: Product Interface Direction
description: Organize the CLI around one primary path and optional workflows over one OKF knowledge base.
tags: [openknowledge, product, cli, interface, runtime]
timestamp: 2026-07-18T00:00:00Z
status: accepted
---

# Product Interface Direction

## Decision

Open Knowledge gives one lifecycle for an OKF knowledge base. All product
interfaces use the same Git-native Markdown object.

These interfaces include search, the viewer, MCP, agents, jobs, exports, and
runtime generations.

The CLI exposes one primary path and three secondary workflow groups:

| Workflow | Commands |
| --- | --- |
| Start here | `setup`, `search`, `validate` |
| Work locally | `agent`, `get`, `list`, `view` |
| Share and connect | `export`, `mcp`, `connect`, `disconnect`, `registry` |
| Automate and operate | `automation` |

The advanced section contains the deterministic `scaffold`, `prompt`, `ast`,
and `spec` tools.

## Interface Rules

1. Root help starts with user outcomes.
2. Command pages contain the command details.
3. Each capability has one canonical command.
4. Deterministic OKF operations do not require a model, network, or service.
5. Service roles use the same validation, publication, retrieval, and export
   contracts.
6. Provider configuration stays under `automation deploy`.
7. Runtime operations stay under `automation runtime`.

The primary activation flow is:

```sh
okn setup
```

The command starts a terminal wizard. Without terminal input, it prints a
complete agent task. Use `--prompt` or `--interactive` to select a mode.

The agent also demonstrates one search that uses source evidence. Thus,
activation does not require another command.

Use `search` independently after activation. Use `validate` independently
after activation. `scaffold` remains an agent-free primitive, not a second
activation path.

Use `setup --prompt` for portable setup instructions. Use
`prompt rules|review` for maintenance and review instructions.
Use `export html|json|graph|tar` for publication.
Use `automation jobs|insights|runtime|deploy` for unattended and hosted work.

Use the top-level `connect` command to add a connection. Use the top-level
`disconnect` command to remove a connection.

Commands remain separate when they have different process or security
boundaries. The local `view`, stdio `mcp`, and hosted runtime are separate
interfaces.

---

<!-- okf-footer: agent-maintenance -->

> **Update notes**
>
> Extend these workflows. Keep compatibility aliases hidden from root help.
