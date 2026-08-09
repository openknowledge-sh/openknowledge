---
type: Decision
title: Keep the demo limited to structural citation checks
description: The demo validates input shape and leaves editorial decisions to people.
status: stable
tags: ["decision", "scope", "citations"]
---

# Keep the demo limited to structural citation checks

## Context

A code example makes project documentation easier to verify. A complete
Wikipedia editing assistant would require policy interpretation and community
judgment.

## Decision

Keep the code limited to claim presence, URL parsing, and HTTPS enforcement.
Leave source reliability, due weight, and consensus to human review.

## Consequences

- Tests can verify every code outcome.
- Architecture boundaries remain visible.
- The demo cannot approve a citation or a Wikipedia edit.

## Alternatives

We rejected a simulated reliability score because it would make an unsupported
editorial judgment.

## Related pages

- [Editorial workflow](../architecture/editorial-workflow.md)
- [Citation and source conventions](../conventions/citations-and-sources.md)
