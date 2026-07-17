---
type: Feature Documentation
title: Tooling Model
description: Product-level map of the Open Knowledge CLI workflows.
tags: [openknowledge, cli, tooling, registry, viewer, export]
timestamp: 2026-07-17T00:00:00Z
---

# Tooling Model

Open Knowledge is one lifecycle around a Git-native OKF knowledge base. Search,
viewer, MCP, agents, jobs, exports, and services are projections or maintenance
loops over that same object, not separate knowledge models.

## Workflow Surface

| Workflow | Commands | Outcome |
| --- | --- | --- |
| Create and maintain | `setup`, `agent`, `insights`, `jobs`; advanced `scaffold` and `prompt` | Onboard a wiki, maintain it interactively, capture and execute insights, and schedule repeatable work. |
| Use and publish | `get`, `search`, `list`, `view`, `mcp`, `export` | Read exact knowledge, retrieve context, browse, integrate clients, and publish portable views. |
| Run as a service | `runtime`, `deploy` | Build immutable generations, serve public knowledge, and reconcile private maintenance. |
| Validate and connect | `validate`, `connect`, `disconnect`, `registry` | Check OKF independently and resolve local or remote bundles. |

`openknowledge setup Wiki` is the primary activation flow. Add `--from` for an
existing repository, folder, or website. It runs an agent, validates the
result, and installs project integration. `scaffold` remains the deterministic
agent-free scaffold; `prompt` exposes portable print-only workflows.

Connection mutation has one entry point per action. `connect` materializes and
registers local, manifest, archive, or Git sources; `disconnect` removes the
registration. `registry` owns only `refresh`, `list`, `status`, and `where`.

All key-or-path consumers share the same resolver. `get`, `search`, `view`,
`mcp`, `validate`, `list`, and `export` therefore work the same way for direct
folders and registered sources. `export html` publishes a portable archive and
manifest that can be connected again.

Deterministic validation never requires a model. `openknowledge prompt review`
is advisory and does not affect validation status. Machine-readable AST,
bundle, graph, list, registry, search, and validation views share versioned
Draft 2020-12 contracts.

## Typical Local Loop

```sh
openknowledge setup Wiki --from ./docs
openknowledge list Wiki
openknowledge search Wiki "release workflow" --budget 1200
openknowledge get Wiki
openknowledge mcp Wiki
openknowledge view Wiki
openknowledge export html --out ./site Wiki
```

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/cmd/openknowledge/setup_command.go`
> * `packages/cli/internal/okf/registry.go`
