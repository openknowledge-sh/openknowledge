---
type: Feature Documentation
title: Tooling Model
description: Product-level map of the Open Knowledge CLI workflows.
tags: [openknowledge, cli, tooling, registry, viewer, export]
timestamp: 2026-07-17T00:00:00Z
---

# Tooling Model

Open Knowledge provides one lifecycle for an OKF knowledge base.
Search, the viewer, MCP, agents, jobs, exports, and services use the same knowledge base.
They do not use separate knowledge models.

## Workflow surface

| Workflow | Commands | Outcome |
| --- | --- | --- |
| Start here | `setup`, `search`, `validate`, `view` | Create, retrieve, verify, and browse local knowledge. |
| Trust and govern | `audit`, `claims`, `evidence`, `eval`, `quality` | Add evidence, lifecycle checks, and quality controls when required. |
| Query and interchange | `query`, `export` | Run explicit semantic queries and create portable output. |
| Publish and operate | `mcp`, `connect`, `disconnect`, `registry`, `automation` | Connect sources, serve tools, and run managed processes. |
| Advanced internals | `agent`, `get`, `list`, `scaffold`, `prompt`, `ast`, `spec`, `version`, `telemetry` | Inspect or control lower-level CLI behavior. |

`okn setup` prints the primary setup prompt.
The prompt uses the current directory as its source and writes `Wiki`.
Copy the printed prompt into an agent that already works in the project.
An explicit wiki path without `--from` prints a guided setup.
Use this form for a new or open-ended knowledge base.
Use `--from` only for a different repository, folder, or website.
`scaffold` is an advanced, deterministic operation without an agent.
`prompt` provides additional portable workflows that only print output.

Each connection change has one command.
`connect` materializes and registers local, manifest, archive, or Git sources.
`disconnect` removes the registration.
`registry` provides only `refresh`, `list`, `status`, and `where`.

All key-or-path consumers use the same resolver.
Thus, `get`, `search`, `query`, `view`, `mcp`, `validate`, `list`, and `export` use the same source rules.
These rules apply to direct folders and registered sources.
`export html` publishes a portable archive and manifest.
You can connect this output again.

Deterministic validation does not require a model.
`okn prompt review` provides advice and does not affect validation status.
Machine-readable views use versioned Draft 2020-12 contracts.
These views include AST, bundle, graph, semantic query, list, registry, search,
and validation output.

## Typical local loop

```sh
okn setup
okn validate Wiki
okn search Wiki "release workflow" --budget 1200
okn view Wiki
```

Use separate commands for exact reads, browsing, integrations, and publication:

```sh
okn list Wiki
okn get Wiki
okn mcp Wiki
okn view Wiki
okn export html --out ./site Wiki
```

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/cmd/openknowledge/setup_command.go`
> * `packages/cli/internal/okf/registry.go`
