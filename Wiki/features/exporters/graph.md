---
type: Candidate Feature
title: Graph Exporter
description: Candidate requirements page for a future graph export target.
tags: [openknowledge, cli, exporter, graph, candidate]
timestamp: 2026-06-18T00:00:00Z
status: candidate
---

# Graph Exporter

`openknowledge to graph` is not currently part of the shipped command surface.
This page is a requirements and design landing page for future graph export
work.

## Candidate Goal

Expose the wiki as nodes and edges so downstream tools can visualize concept
relationships, find orphan pages, or analyze link structure.

## Likely Inputs

* Bundle files and metadata from the same parse path used by the JSON exporter.
* Local links extracted from Markdown pages.
* Reserved file kinds such as `index.md` and `log.md`.
* Validation issues when graph consumers need quality signals.

## Open Questions

* Should output be JSON graph, DOT, GraphML, Mermaid, or multiple targets?
* Should edges be typed only by link context, or remain untyped as OKF v0.1 suggests?
* Should reserved files appear as nodes?
* Should missing local link targets become dangling nodes?

---

<!-- okf-footer: agent-maintenance -->

> **Update notes**
>
> When this feature is implemented, change the status, add command usage, update
> [openknowledge to](/features/commands/to.md), root help, README command tables,
> tests, and [CLI changelog](/changelog/cli.md).
