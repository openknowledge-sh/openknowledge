---
type: Decision
title: Product Interface Direction
description: Organize the complete self-maintaining knowledge-base stack around four user workflows and a progressively disclosed CLI.
tags: [openknowledge, product, cli, interface, runtime]
timestamp: 2026-07-17T00:00:00Z
status: accepted
---

# Product Interface Direction

Open Knowledge is one product for the full lifecycle of an OKF knowledge base,
not a collection of unrelated format, viewer, agent, and hosting tools. Running
a self-maintaining knowledge base legitimately requires deterministic content,
human and machine access, maintenance automation, and an operational runtime.
The interface should expose those capabilities through a small number of user
workflows instead of presenting every subsystem as a separate product concept.

## Core Object

The stable object is an **OKF knowledge base**: a Git-native set of Markdown
documents with explicit publication boundaries. Viewer, search, MCP, agents,
jobs, exports, and runtime generations are different ways to use or maintain
that same object. They should not introduce competing knowledge models.

## Four Primary Use Cases

| Workflow | User outcome | Primary commands |
| --- | --- | --- |
| Create and maintain | Create a knowledge base, connect agent sessions, review captured insights, and automate repeated maintenance. | `setup`, `agent`, `jobs` |
| Use and publish | Read exact knowledge, retrieve context, browse it, connect MCP clients, and produce portable views. | `get`, `search`, `list`, `view`, `mcp`, `export` |
| Run as a service | Build immutable generations, serve public knowledge, reconcile private maintenance, and provision infrastructure. | `runtime`, `deploy` |
| Validate and connect | Check OKF independently of the managed product and connect local or remote knowledge bases. | `validate`, `connect`, `disconnect`, `registry` |

The first two workflows work locally. The service workflow adds long-running
roles and infrastructure without changing the authored knowledge model. The
validation workflow remains useful as an independent OKF toolchain even when a
user does not adopt agents or the hosted runtime.

## Interface Rules

1. Root help starts with outcomes and workflows, not an exhaustive flag matrix.
2. Command-specific help owns detailed variants and machine-contract options.
3. A capability has one canonical user-facing home. This pre-1.0 interface does
   not retain compatibility aliases for removed command forms.
4. Operational implementation details stay below `runtime`; provider-specific
   provisioning stays below `deploy`.
5. Deterministic OKF operations never require an agent, network, or runtime.
6. Agent and service layers reuse validation, publication, retrieval, and export
   contracts instead of creating parallel knowledge semantics.

## Shipped Shape

Project integration and the suggestion inbox belong to the agent-maintenance
workflow:

```sh
openknowledge agent integrate Wiki
openknowledge agent suggestions
openknowledge agent suggestions apply Wiki/suggestions/<file>.md
```

There are no parallel top-level `integrate` or `suggestions` forms because the
feature has not shipped yet. `jobs` remains top-level because it is the
declarative automation and scheduling primitive, not a mode of one interactive
session.

Root help groups every command under the four workflows and places portable
prompt generators and low-level scaffolding in an advanced section.

The canonical onboarding command is:

```sh
openknowledge setup [wiki] [--from <source>]
```

It starts the selected agent harness, validates the resulting wiki, and
installs project integration. `--from` switches the prompt recipe without
creating a second onboarding command. `scaffold` remains the explicit deterministic,
agent-free scaffold.

Portable prompt generation lives only under `prompt setup|from|rules|review`.
The former top-level prompt commands and `agent init|from` were removed.

Publishing lives only under `export html|json|graph|tar`; the former `to` name
was removed. Connection mutation lives only at top-level `connect` and
`disconnect`; `registry` retains `refresh`, `list`, `status`, and `where`.

Do not merge commands merely to reduce their count. `view`, stdio `mcp`, and
HTTP runtime serving have different process and security contracts; grouping
them is useful only if their modes remain obvious at invocation time.

## Product Test

The primary activation flow should fit in one explanation:

```text
Create or connect an OKF knowledge base, use it locally or as a service, let
agents capture evidence-backed suggestions, and merge validated maintenance
through normal Git review.
```

The suggestion loop should be measured by accepted useful suggestions, not by
the number of observations created. Runtime breadth should be judged by whether
it makes that same knowledge lifecycle safely operable, not as a separate
feature-count goal.

---

<!-- okf-footer: agent-maintenance -->

> **Update notes**
>
> Keep the root surface aligned with these canonical homes. New capabilities
> should extend a workflow instead of reintroducing parallel aliases.
