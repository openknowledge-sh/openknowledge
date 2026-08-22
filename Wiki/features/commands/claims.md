---
type: Command Documentation
title: openknowledge claims
description: Maintain evidence-backed typed claims through deterministic operations.
tags: [openknowledge, cli, claims, provenance, agents]
timestamp: 2026-08-22T00:00:00Z
---

# `openknowledge claims`

Use this command to find, propose, link, validate, and change typed claim
occurrences. Each write is validated before the CLI keeps it.

## Agent workflow

Find existing terms, slots, and occurrences before authoring:

```sh
okn claims find "token format" --path Wiki
okn claims suggest auth.md --path Wiki --out claim-candidates.json
okn claims entities find "Identity API" --path Wiki --json
```

Create a complete claim JSON object and pass it to `propose`:

```sh
okn claims propose --path Wiki --from auth.md \
  --claim-json '{
    "id":"auth:claim/token-format/2026-08-22",
    "slot":"auth:slot/token-format",
    "subject":"auth:token-service",
    "predicate":"auth:tokenFormat",
    "object":{"value":"JWT","datatype":"xsd:string"},
    "evidence":[{
      "id":"auth:evidence/token-format/openapi",
      "sourceRef":"identity-openapi",
      "stance":"supports",
      "role":"auth:contract",
      "selector":{"type":"text_quote","exact":"Access tokens use JWT."}
    }],
    "status":"proposed",
    "sectionRef":"#token-format"
  }' \
  --reason "The API contract defines the production format." \
  --confidence 0.92 --out /tmp/token-format.json
```

`confidence` is extraction confidence. It is not truth confidence.
The referenced `identity-openapi` source must name a local artifact with
`observe: pinned` and its exact lowercase `sha256` digest. `propose` verifies
the artifact and selector before it writes the read-only proposal.

Apply and inspect the digest-bound proposal:

```sh
okn claims apply /tmp/token-format.json --path Wiki
okn claims link auth:claim/token-format/2026-08-22 runbook.md --path Wiki
okn claims impact auth:claim/token-format/2026-08-22 --path Wiki
okn claims validate --path Wiki
```

`apply` refuses the proposal when the document changed after `propose`. It also
refuses an occurrence ID that already exists. A new value needs a new ID and
can name the old occurrence in `relations.supersedes`.

`suggest` reports uncovered sections, excerpts, source IDs, and similar
occurrences. It does not decide the subject, predicate, object, or truth.

## Stable entities

Entity IDs are absolute IRIs or CURIEs and do not depend on document paths or
slugs. `entities find` searches ID, preferred label, and alternate labels. It
also reports claim occurrence IDs that reference each entity.

Create a digest-bound resolution proposal:

```sh
okn claims entities propose --path Wiki --document ontology.md \
  --entity auth:token-service --alias "Identity API" \
  --reason "The product name refers to the same service." \
  --confidence 0.91 --out /tmp/entity-alias.json

okn claims entities propose --path Wiki --document ontology.md \
  --entity auth:token-service --merge-from auth:legacy-token-service \
  --reason "Both IDs resolve to one deployed service." \
  --confidence 0.88 --out /tmp/entity-merge.json
```

The CLI verifies that the IDs exist. An alias proposal binds the target
ontology document. A merge proposal binds both declaration documents.

Preview every affected claim field before approval, and then apply the exact
proposal:

```sh
okn claims entities impact /tmp/entity-merge.json --path Wiki --json
okn claims entities apply /tmp/entity-merge.json --path Wiki \
  --approved-by human:alice --json
```

`apply` requires a `human:` or `github:` approval identity. It refuses stale
digests, rewrites subject, object, and scope references as one validated
transaction, and rolls back all edited documents when validation adds an
error. A merge does not delete the old stable ID. It marks that entity
`deprecated: true` with `replaced_by: <canonical-id>` and moves its labels and
types to the canonical entity. Validation refuses new claim references to a
deprecated entity.

Validation also rejects a preferred or alternate label shared by active
entity IDs.

## Lifecycle operations

```sh
okn claims dispute auth:claim/token-format/vendor --path Wiki
okn claims verify auth:claim/token-format/2026-08-22 \
  --approved-by human:alice --path Wiki
okn claims reject auth:claim/token-format/vendor \
  --approved-by github:alice --path Wiki
okn claims supersede auth:claim/token-format/2026-01-01 \
  --by auth:claim/token-format/2026-08-22 \
  --approved-by human:alice --path Wiki
okn claims archive auth:claim/token-format/2025-01-01 \
  --approved-by human:alice --path Wiki
okn claims approve-authority identity-openapi --document auth.md \
  --approved-by human:alice --path Wiki
```

`verify` requires a `human:`, `github:`, or `process:` identity. The CLI writes
a claim-level verification record. Source authority alone is not verification.

`supersede` requires the successor occurrence to contain an explicit
`relations.supersedes` reference to the old occurrence.

`reject`, `supersede`, and protected archival require a human or GitHub
approval identity. `dispute` preserves the object and evidence.

`approve-authority` records who approved a source as authoritative. Hosted
maintenance always routes authority changes to human review.

## Impact and validation

`impact` returns the exact occurrence, evidence sources, `claim_refs`
dependents, linked documents, shared-source documents, and eval cases.

`validate` checks:

- global occurrence ID uniqueness;
- namespaces, entities, predicates, object kinds, datatypes, and units;
- predicate cardinality and required scope;
- evidence IDs, sources, roles, stances, pinned artifact digests, and selector
  resolution;
- verification references and actor identities;
- validity intervals and section bindings;
- relation targets and supersession cycles;
- exact `claim_refs` targets.

Use `--against <base-path>` to protect supported, verified, and disputed
history and source-authority decisions.

Selector validation never accesses the network. It reports `unverifiable`
when exact source bytes or a deterministic resolver are unavailable. A digest
mismatch is a tampering error.

Use `--json` for strict machine output. Invalid claims return exit status `1`.
Invalid command use returns exit status `2`.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/claims_command.go`
> - `packages/cli/internal/claimops/`
> - `packages/cli/schemas/claims/v1/proposal.schema.json`
> - `packages/cli/schemas/claims/v1/entity-proposal.schema.json`
> - `packages/cli/schemas/v1/claims-entity-*.schema.json`
>
> **Update notes**
>
> Update this page when claim operations, gates, or machine output changes.
