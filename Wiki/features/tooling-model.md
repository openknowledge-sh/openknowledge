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
| Start here | `setup`, `search`, `validate` | Create and integrate a wiki, retrieve useful context, and verify its source. |
| Maintain and automate | `agent`, `insights`, `jobs` | Maintain knowledge interactively, capture gaps, and schedule repeatable work. |
| Browse and publish | `get`, `list`, `view`, `mcp`, `export` | Read exact knowledge, browse, integrate clients, and publish portable views. |
| Connect and operate | `connect`, `disconnect`, `registry`, `runtime`, `deploy` | Resolve bundles, serve immutable generations, and provision hosted runtimes. |

`openknowledge setup` is the primary project activation flow. Run it directly
from the project repository: the CLI uses the current repository as its source,
writes `Wiki`, launches the selected interactive agent, validates the result,
and installs project integration. An explicit wiki path without `--from` starts
a guided setup for a new or open-ended knowledge base. Use `--from` only for
another repository, folder, or website. `scaffold` remains an advanced
deterministic, agent-free primitive; `prompt` exposes portable print-only
workflows.

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
openknowledge setup
openknowledge search Wiki "release workflow" --budget 1200
openknowledge validate Wiki
```

Optional exact reads, browsing, integrations, and publishing remain separate:

```sh
openknowledge list Wiki
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
