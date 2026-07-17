---
type: Decision
title: Product Interface Direction
description: Organize the CLI around four workflows over one OKF knowledge base.
tags: [openknowledge, product, cli, interface, runtime]
timestamp: 2026-07-18T00:00:00Z
status: accepted
---

# Product Interface Direction

## Decision

Open Knowledge is one lifecycle around an OKF knowledge base. Search, viewer,
MCP, agents, jobs, exports, and runtime generations are views or maintenance
loops over the same Git-native Markdown object.

The CLI is organized around four workflows:

| Workflow | Commands |
| --- | --- |
| Create and maintain | `setup`, `agent`, `insights`, `jobs` |
| Use and publish | `get`, `search`, `list`, `view`, `mcp`, `export` |
| Run as a service | `runtime`, `deploy` |
| Validate and connect | `validate`, `connect`, `disconnect`, `registry` |

Low-level deterministic tools—`scaffold`, `prompt`, `ast`, and `spec`—remain
available under an advanced section.

## Interface rules

1. Root help starts with user outcomes; command references own details.
2. Each capability has one canonical command home.
3. Deterministic OKF operations do not require a model, network, or service.
4. Service roles reuse validation, publication, retrieval, and export contracts.
5. Provider provisioning stays under `deploy`; runtime mechanics stay under
   `runtime`.

The primary activation flow is:

```sh
openknowledge setup Wiki --from .
```

The CLI launches the selected agent, validates the resulting wiki, and installs
project integration. `scaffold` remains an explicit agent-free primitive, not a
second onboarding path.

Portable instructions live under `prompt setup|from|rules|review`. Publishing
lives under `export html|json|graph|tar`. Connection mutation lives at
top-level `connect` and `disconnect`.

Commands with materially different process or security boundaries remain
separate even when they present the same knowledge. In particular, local
`view`, stdio `mcp`, and hosted runtime serving are distinct surfaces.

---

<!-- okf-footer: agent-maintenance -->

> **Update notes**
>
> Extend these workflows instead of introducing parallel aliases.
