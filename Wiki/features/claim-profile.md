---
type: Feature Documentation
title: Typed Claims v1
description: Define evidence-backed, typed, and lifecycle-safe knowledge assertions.
tags: [openknowledge, claims, provenance, validation, ontology]
timestamp: 2026-08-24T00:00:00Z
profile: openknowledge.claims/v1
owner: team:openknowledge
status: stable
---

# Typed Claims v1

Use claims for facts that need deterministic validation, conflict detection,
provenance, impact analysis, or runtime policy. Keep explanations in Markdown.

The canonical authoring format is YAML in Markdown frontmatter. The data model
uses established semantic-web concepts without requiring authors to write RDF.

## Standards alignment

| Open Knowledge field | Standards concept |
| --- | --- |
| `subject`, `predicate`, `object` | RDF subject-predicate-object statement and IRI identity |
| ontology labels and aliases | SKOS preferred and alternative labels |
| evidence and derivation | PROV-O provenance concepts |
| evidence `selector` | W3C Web Annotation selectors |
| `datatype` | RDF and XML Schema datatypes |
| `unit`, `quantity_kind` | QUDT units and quantity kinds |
| `valid_time` | RFC 3339 interval with an OWL-Time-compatible projection |
| predicate constraints | Deterministic SHACL-style domain, range, cardinality, and required-scope checks |

The CLI validates this model directly. It does not run a general OWL reasoner
or expose an RDF query endpoint.

## Ontology registry

Declare bundle terms in `claim_ontology`. Built-in prefixes include `rdf`,
`rdfs`, `xsd`, `skos`, `prov`, `sh`, `oa`, `time`, `qudt`, `unit`,
`quantitykind`, and `okn`.

```yaml
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces:
    auth: https://example.com/auth/
  entities:
    - id: auth:token-service
      types: [auth:Service]
      pref_label: Token service
      alt_labels: [Authentication service]
    - id: auth:legacy-token-service
      deprecated: true
      replaced_by: auth:token-service
  predicates:
    - id: auth:tokenFormat
      pref_label: token format
      subject_types: [auth:Service]
      object_kind: literal
      datatype: xsd:string
      maximum_count: 1
  evidence_roles:
    - id: auth:contract
      pref_label: API contract
```

Use absolute IRIs or CURIEs with a declared prefix. A predicate defines the
object kind and can define a datatype, quantity kind, canonical unit, required
scope dimensions, and maximum count.

Entity merges preserve stable IDs. The old entity stays in the registry with
`deprecated: true` and `replaced_by`. Claims must reference the active
replacement. This is a deterministic lightweight migration record, not an OWL
identity inference.

## Claim occurrence

Each `id` identifies one immutable occurrence. Never reuse it for a new value.
The `slot` groups occurrences that answer the same question.

```yaml
sources:
  - id: identity-openapi
    resource: ./openapi/identity.yaml
    observe: pinned
    sha256: 9252f8d43e1456b9678c8a84c5c09fbf26ca43a893f9910cb575f6b0fa403c08
    role: authoritative
    access: [profile:support, team:identity]
claims:
  - id: auth:claim/token-format/2026-08-22
    slot: auth:slot/token-format
    subject: auth:token-service
    predicate: auth:tokenFormat
    object:
      value: JWT
      datatype: xsd:string
    scope:
      auth:environment:
        value: production
        datatype: xsd:string
    evidence:
      - id: auth:evidence/token-format/openapi
        source_ref: identity-openapi
        stance: supports
        role: auth:contract
        observed_at: 2026-08-22T08:00:00Z
        selector:
          type: text_quote
          exact: Access tokens use JWT.
    status: verified
    valid_time:
      from: 2026-08-22
    verification:
      method: human-review
      by: human:identity-lead
      at: 2026-08-22T09:00:00Z
      evidence_refs: [auth:evidence/token-format/openapi]
    section_ref: "#token-format"
```

The object contains exactly one of `ref` or `value`. A literal can also contain
`datatype`, `language`, `unit`, and `quantity_kind`. Unit and quantity kind
must occur together.

Scope dimensions also use typed objects. Predicates can require selected scope
dimensions.

## Evidence selectors

Evidence is a list. Each item has a stable `id`, a same-document `source_ref`,
a `stance`, and a `role`.

A source that has a selector must use `observe: pinned` and contain the
lowercase SHA-256 digest of the exact artifact bytes. The `resource` path is
relative to the document that declares the source. Validation resolves the
path inside the bundle, refuses symbolic links, reads at most 8 MiB, and
compares the digest before it checks the selector. It does not use the
Markdown body as a substitute for the declared artifact.

Source access labels use `profile:`, `agent:`, `team:`, or `use_case:`. Empty
access is public. Runtime retrieval rejects the whole candidate when any
restricted source has no label in common with the active access identity.

Supported stances are `supports`, `opposes`, and `contextualizes`. Supported
selector types are:

- `text_quote` with `exact` and optional `prefix` or `suffix`;
- `text_position` with `start` and `end`;
- `fragment` with `value`;
- `page` with a one-based `page`;
- `media_fragment` with `value`;
- `data_position` with `start` and `end`.

For UTF-8 text, `text_quote.exact` must occur exactly once. Optional `prefix`
and `suffix` must match the adjacent text. `text_position` uses half-open
Unicode code-point offsets. `data_position` uses half-open byte offsets.
`fragment` resolves Markdown heading anchors or explicit IDs, and HTML IDs.

Validation never fetches evidence. A remote resource, a content-addressed
identifier without local bytes, an artifact over the read limit, or a selector
without a deterministic local resolver produces an `unverifiable` error.
`page` and `media_fragment` remain in the authoring contract, but the current
validator cannot resolve them. Publication therefore fails closed instead of
recording a false verification. Materialize the exact artifact locally before
using a selector.

`section_ref` binds a claim to a Markdown retrieval section. It is not an
evidence selector.

## Lifecycle and relations

Statuses are `extracted`, `proposed`, `supported`, `verified`, `disputed`,
`rejected`, `superseded`, and `archived`.

Only `verified` requires a `verification` record. Extraction confidence belongs
to a proposal envelope. It is not a truth score.

Use explicit occurrence relations:

```yaml
relations:
  supersedes: [auth:claim/token-format/2026-01-01]
  contradicts: [auth:claim/token-format/vendor-report]
  derived_from: [auth:claim/token-format/source-extraction]
```

The CLI does not infer supersession from timestamps or document order. A
superseded occurrence stays in the bundle and a successor explicitly names it.
Supersession cycles and missing relation targets are invalid.

Reviewed rejection, supersession, and archival append a `decisions` event with
the action, actor, and time. This event does not replace the original
`verification` record.

## References and impact

`claim_refs` contains exact occurrence IDs:

```yaml
openknowledge_claim_profile: "1"
claim_refs:
  - auth:claim/token-format/2026-08-22
```

A reference does not follow a slot automatically. This preserves reproducible
dependencies. Impact analysis also includes source sharing, Markdown links,
and eval cases.

## Conflict policy

The audit compares active claims with the same:

- `slot`;
- `subject`;
- `predicate`;
- normalized typed scope;
- overlapping half-open `valid_time` interval.

For a predicate with `maximum_count: 1`, incompatible normalized objects are a
conflict. Equal objects with equal evidence sources and validity are duplicates.
Occurrence IDs, status, evidence, and document order are not part of the
comparison key.

The CLI reports a conflict. It does not choose which evidence is true.

## Runtime behavior

List, graph, search, context, MCP, audit, and runtime outputs include the typed
claim projection. The graph includes declaration, reference, supersession,
contradiction, and derivation edges.

Runtime retrieval blocks active `extracted`, `proposed`, `supported`, or
`disputed` claims. It can serve an active verified successor while keeping
rejected, superseded, or archived history.

Production publication rejects unresolved active claims by default. The
runtime serves an immutable accepted generation and never selects a value by
"newest timestamp."

Use [`okn claims`](/features/commands/claims.md) for deterministic operations.

## Viewer representation

The viewer keeps YAML frontmatter as the canonical claim representation. A
collapsed **Claims** panel gives each document a read-only semantic view, and
the Claims workspace provides a knowledge-base-wide view.

The panel uses ontology labels for the subject, predicate, object, and scope.
For a scoped metric, the compact summary uses the value of the scope dimension
whose local name is `metric`. The summary shows only the subject, metric, and
formatted quantity. The Claims workspace keeps the other scope values,
lifecycle metadata, evidence, and provenance visible in **Evidence and
metadata**. Quantity kind remains technical metadata.

Authored incoming and outgoing relations appear in a collapsed
**Relationships** section below the selected claim. The section is omitted
when the claim has no authored relations.

Labeled subjects, object references, metric identities, predicates, and
evidence sources are contextual links inside the selected claim. They replace
the detail pane with a read-only inspector and keep the claim list in place:

- entity inspectors show types, aliases, deprecation and replacement metadata,
  and every claim that uses the entity as a subject, object, or scope value;
- predicate inspectors show object and cardinality constraints, datatype or
  quantity constraints, required scope, and every claim using the predicate;
- source inspectors show the declared resource, observation mode, authority,
  digest, access metadata, evidence use, and a link to the declaring document.

The selected claim also gets a collapsed **History** section when its slot has
multiple occurrences. A collapsed **Impact** section appears when another
document references the claim or another claim uses the same declared source
resource. Empty History, Impact, and Relationships sections are omitted. These
are viewer projections of canonical ontology, source, claim-reference, slot,
and relation data; they do not create new persisted objects.

Claim details show status, scope, evidence, provenance, time, and occurrence
relations. A `section_ref` adds a claim count to the matching Markdown heading.

The **Claims** workspace supports bundle-wide claim review and filters. The
**Graph** workspace preserves claim occurrence nodes and their typed relations.
Neither view changes claim YAML or lifecycle state.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/claims_*.go`
> - `packages/cli/internal/claimops/`
> - `packages/cli/internal/audit/audit.go`
> - `packages/cli/schemas/claims/v1/frontmatter.schema.json`
> - `packages/cli/cmd/openknowledge/viewer_claims.go`
> - `packages/cli/cmd/openknowledge/viewer_export.go`
> - `packages/web/src/viewer/`
>
> **Update notes**
>
> Update this page when the claim contract or a claim consumer changes.
