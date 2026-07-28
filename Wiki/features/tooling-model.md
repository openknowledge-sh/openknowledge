---
type: Feature Documentation
title: Tooling Model
description: Product-level map of the Open Knowledge CLI workflows.
tags: [openknowledge, cli, tooling, registry, viewer, export]
timestamp: 2026-07-17T00:00:00Z
---

# Tooling Model

Open Knowledge provides one lifecycle for a Git-native OKF knowledge base.
Search, the viewer, MCP, agents, jobs, exports, and services use the same knowledge base.
They do not use separate knowledge models.

## Workflow surface

| Workflow | Commands | Outcome |
| --- | --- | --- |
| Start here | `setup`, `search`, `validate` | Create and integrate a wiki, retrieve useful context, and verify its source. |
| Maintain and automate | `agent`, `insights`, `jobs` | Maintain knowledge interactively, capture gaps, and schedule repeatable work. |
| Browse and publish | `get`, `list`, `view`, `mcp`, `export` | Read exact knowledge, browse, integrate clients, and publish portable views. |
| Connect and operate | `connect`, `disconnect`, `registry`, `runtime`, `deploy` | Resolve bundles, serve immutable generations, and provision hosted runtimes. |

`okn setup` is the primary project activation flow.
Run it from the project repository.
The CLI uses the current repository as its source and writes `Wiki`.
It starts the selected interactive agent.
Then, it validates the result and installs the project integration.
An explicit wiki path without `--from` starts a guided setup.
Use this form for a new or open-ended knowledge base.
Use `--from` only for a different repository, folder, or website.
`scaffold` is an advanced, deterministic operation without an agent.
`prompt` provides portable workflows that only print output.

Each connection change has one command.
`connect` materializes and registers local, manifest, archive, or Git sources.
`disconnect` removes the registration.
`registry` provides only `refresh`, `list`, `status`, and `where`.

All key-or-path consumers use the same resolver.
Thus, `get`, `search`, `view`, `mcp`, `validate`, `list`, and `export` use the same source rules.
These rules apply to direct folders and registered sources.
`export html` publishes a portable archive and manifest.
You can connect this output again.

Deterministic validation does not require a model.
`okn prompt review` provides advice and does not affect validation status.
Machine-readable views use versioned Draft 2020-12 contracts.
These views include AST, bundle, graph, list, registry, search, and validation output.

## Typical local loop

```sh
okn setup
okn search Wiki "release workflow" --budget 1200
okn validate Wiki
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
