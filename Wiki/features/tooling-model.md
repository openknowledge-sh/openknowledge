---
type: Feature Documentation
title: Tooling Model
description: Product-level map of the Open Knowledge CLI workflows.
tags: [openknowledge, cli, tooling, registry, viewer, export]
timestamp: 2026-07-17T00:00:00Z
---

# Tooling Model

Open Knowledge provides one lifecycle for repository knowledge. Setup converts
ordinary Markdown into an OKF knowledge base. Search can also read unmanaged
Markdown before adoption.

## Workflow surface

| Workflow | Commands | Outcome |
| --- | --- | --- |
| Start here | `setup`, `check`, `search`, `view`, `review`, `publish`, `upgrade` | Adopt, verify, use, improve, publish, and migrate knowledge. |
| Advanced | `validate`, `audit`, `claims`, `evidence`, `eval`, `quality`, `query`, `export`, `mcp`, `connect`, `registry`, `automation`, and direct tools | Control a specific layer or integration. |

`okn setup` discovers generic Markdown without a required folder structure,
frontmatter, or link graph. The interactive flow selects paths and creates a
managed copy or adopts one complete directory. Both modes use a deterministic
plan before writes.

If no Markdown exists, setup creates a tailored task. The user can run an
installed agent, copy the task, or save it. Existing OKF bundles continue with
`check`, `review`, or `upgrade` instead of setup.

`okn check` combines configured deterministic layers into one status. Search
works for managed bundles and unmanaged Markdown. Review supplies optional
agent judgment. Publish remains strict and accepts only checked, configured,
managed bundles. Upgrade handles explicit OKF version transitions.

Each connection change has one command.
`connect` materializes and registers local, manifest, archive, or Git sources.
`disconnect` removes the registration.
`registry` provides only `refresh`, `list`, `status`, and `where`.

All key-or-path consumers use the same resolver.
Thus, `check`, `search`, `view`, `review`, `publish`, `upgrade`, and advanced
bundle commands use the same source rules.
These rules apply to direct folders and registered sources.
`export html` publishes a portable archive and manifest.
You can connect this output again.

Deterministic setup, check, search, upgrade, and publication planning do not
require a model. `okn review` provides advice and does not affect check status.
Machine-readable views use versioned Draft 2020-12 contracts.
These views include AST, bundle, graph, semantic query, list, registry, search,
and validation output.

## Typical local loop

```sh
okn setup
okn check Wiki
okn search Wiki "release workflow" --budget 1200
okn review Wiki
okn view Wiki
```

Use separate commands for exact reads, browsing, integrations, and publication:

```sh
okn list Wiki
okn get Wiki
okn validate Wiki
okn export html --out ./site Wiki
okn mcp Wiki
```

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/cmd/openknowledge/setup_command.go`
> * `packages/cli/internal/okf/registry.go`
