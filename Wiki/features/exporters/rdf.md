---
type: Feature Documentation
title: RDF exporter
description: Export typed semantic facts as deterministic RDF 1.1 N-Quads.
tags: [openknowledge, exporter, rdf, n-quads, provenance]
timestamp: 2026-08-25T00:00:00Z
---

# RDF exporter

Export the normalized semantic fact model as deterministic RDF 1.1 N-Quads:

```sh
okn export rdf Wiki
okn export rdf --out ./knowledge.nq Wiki
```

The exporter creates one named graph bound to the corpus revision. It preserves
the direct subject-predicate-object statement and a claim occurrence resource
for lifecycle, scope, evidence, selectors, access labels, relations, and
provenance. Typed literals retain XML Schema datatypes or language tags.
Quantity metadata uses QUDT identifiers and temporal metadata has an
OWL-Time-compatible representation.

The projection is lossless with respect to the normalized semantic fact
contract. It is rebuildable from Markdown and YAML and is not canonical state.
An invalid bundle fails before serialization. `--out` replaces the destination
only after the complete dataset is serialized.

Use [`okn query sparql`](/features/commands/query.md) to query the same
revision-bound graph in memory.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/semantic_facts.go`
> - `packages/cli/internal/okf/rdf.go`
> - `packages/cli/schemas/v1/semantic-facts.schema.json`
> - `packages/cli/schemas/v1/rdf-dataset.schema.json`
