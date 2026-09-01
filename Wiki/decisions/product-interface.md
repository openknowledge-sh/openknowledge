---
type: Decision
title: Product Interface Direction
description: Organize the CLI around one primary path and optional workflows over one OKF knowledge base.
tags: [openknowledge, product, cli, interface, runtime]
timestamp: 2026-07-18T00:00:00Z
decision_status: accepted
---

# Product Interface Direction

## Decision

Open Knowledge gives one lifecycle from repository Markdown to published OKF
knowledge. Managed product interfaces use the same OKF Markdown bundle.

These interfaces include search, the viewer, MCP, agents, jobs, exports, and
runtime generations.

The CLI exposes one primary workflow and one advanced group:

| Workflow | Commands |
| --- | --- |
| Start here | `setup`, `check`, `search`, `view`, `review`, `publish`, `upgrade` |
| Advanced | Direct validation, governance, query, export, runtime, connection, automation, and inspection commands |

Existing commands remain available. Root help places the lower-level commands
under **Advanced**.

## Interface Rules

1. Root help starts with user outcomes.
2. Command pages contain the command details.
3. Each capability has one canonical command.
4. Deterministic OKF operations do not require a model, network, or service.
5. Service roles use the same validation, publication, retrieval, and export
   contracts.
6. Provider configuration stays under `automation deploy`.
7. Runtime operations stay under `automation runtime`.

The primary lifecycle is:

```sh
okn setup
okn check Wiki
okn search Wiki "release workflow"
okn view Wiki
okn review Wiki
okn publish Wiki --plan
```

Setup accepts ordinary Markdown without YAML frontmatter or an authored link
graph. It provides path selection and two results: a managed copy or in-place
adoption. The plan shows all writes before apply.

If the source has no Markdown, setup creates a tailored agent task. The
continuation offers installed agents first, then copy and save choices.

Search accepts unmanaged Markdown and marks it as `unmanaged`. Check reports
`UNMANAGED`, and publish refuses unmanaged input. Review is optional agent work
after deterministic checks. Upgrade is the explicit format migration path.

Use advanced commands when direct control of a layer is necessary. These
commands include `validate`, `audit`, `eval`, `query`, `export`, and `mcp`.

Use the top-level `connect` command to add a connection. Use the top-level
`disconnect` command to remove a connection.

Commands remain separate when they have different process or security
boundaries. The local `view`, stdio `mcp`, and hosted runtime are separate
interfaces.

---

<!-- okf-footer: agent-maintenance -->

> **Update notes**
>
> Keep the lifecycle commands in **Start here**. Keep lower-level commands in
> **Advanced**.
