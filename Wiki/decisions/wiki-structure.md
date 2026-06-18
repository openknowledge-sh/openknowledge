---
type: Decision
title: Wiki Structure
description: Setup decisions for the Open Knowledge CLI developer wiki.
tags: [openknowledge, cli, decision, wiki]
timestamp: 2026-06-18T00:00:00Z
---

# Wiki Structure

## Decision

The knowledge base is named `Wiki` and lives inside the repository at `Wiki/`.
It focuses on two maintenance loops:

* CLI package changelog memory.
* Developer-focused feature documentation, especially command and exporter pages.

## Rationale

The wiki should be close to the code so agents can update it in the same change
that touches CLI behavior. Command pages are split by command so long Markdown
docs do not become a single hard-to-maintain file.

## Agent Integration

The repository root has `AGENTS.md` pointing agents to this wiki and to the
repo-local skill at `.codex/skills/openknowledge-wiki/SKILL.md`.

## Boundaries

The wiki should not store raw copied source material by default. Source files,
tests, README content, and release docs remain the source of truth.
