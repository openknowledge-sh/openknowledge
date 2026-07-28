---
type: Decision
title: Wiki Structure
description: Setup decisions for the Open Knowledge CLI developer wiki.
tags: [openknowledge, cli, decision, wiki]
timestamp: 2026-07-21T00:00:00Z
---

# Wiki Structure

## Decision

The knowledge base uses the name `Wiki` and lives inside the repository at
`Wiki/`.
It supports two maintenance tasks:

- Maintain the CLI changelog.
- Maintain developer documentation, especially command and exporter pages.

## Rationale

The wiki stays near the code. Thus, agents can update the wiki with a related
CLI change.

Each command has a separate page. This structure prevents one large command
document.

## Agent Integration

The repository root contains `AGENTS.md`. It directs agents to this wiki and
to `.codex/skills/openknowledge-wiki/SKILL.md`.

## Boundaries

Do not store copied source material in the wiki by default.

The source code and tests define product behavior. This wiki is the canonical
CLI reference. The README is the product overview.

In an Open Knowledge bundle, the OKF Markdown is the canonical corpus. You can
rebuild search indexes, graphs, exports, and runtime artifacts from this corpus.
