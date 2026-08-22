---
type: Feature Documentation
title: Corpus Schema v1
description: Optional bundle-wide contracts for document types, paths, metadata, and typed links.
tags: [openknowledge, validation, schema, ontology, links]
timestamp: 2026-08-22T00:00:00Z
---

# Corpus Schema v1

`corpus_schema` is an optional extension on the root `index.md`. Without it,
existing bundles keep their current document type behavior. Once present,
`okn validate` applies its rules fail-closed through the fixed-severity
`corpus-schema` rule.

```yaml
corpus_schema:
  version: "1"
  document_types:
    - id: Runbook
      aliases: [Run Book]
      paths: [runbooks/**]
      required: [owner, service]
    - id: Service
      paths: [services/**]
      required: [owner]
    - id: Migration
      paths: [migrations/**]
  link_predicates:
    - id: depends_on
      source_types: [Runbook]
      target_types: [Service]
  migrations:
    - from: "0"
      to: "1"
      document: migrations/corpus-v1.md
```

A concept can then declare a typed local link:

```yaml
typed_links:
  - predicate: depends_on
    target: ../services/auth.md
```

Validation enforces unique type IDs and aliases, clean bundle-relative glob
patterns with `**`, allowed type paths, required non-empty metadata, declared
predicates, permitted source and target types, resolving local targets, and
existing Markdown migration records. Unknown fields are errors.

Document type identity comes from the declared type ID, not a filename or
slug. Claim entity identity remains a separate absolute IRI or CURIE in
`claim_ontology`. The corpus schema does not implement RDF, OWL, SHACL,
general graph queries, or an automatic migration engine.

The authored extension schema is
`schemas/corpus/v1/frontmatter.schema.json`. Deterministic validation performs
the cross-document checks that JSON Schema cannot perform.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/corpus_schema.go`
> - `packages/cli/schemas/corpus/v1/frontmatter.schema.json`
> - `packages/cli/internal/okf/corpus_schema_test.go`
>
> **Update notes**
>
> Update this page when the corpus extension or its cross-document validation
> changes.
