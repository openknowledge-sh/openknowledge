---
type: Feature Documentation
title: Claim Freshness
description: Derive exact typed claim staleness from changed evidence and reconcile it.
tags: [openknowledge, claims, provenance, freshness]
timestamp: 2026-08-26T00:00:00Z
status: stable
owner: team:openknowledge
verified: {by: human:openknowledge-maintainers, at: 2026-08-26T17:15:00Z}
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces: {okn: https://openknowledge.dev/ns/}
  entities:
    - {id: okn:feature/claim-freshness, pref_label: Claim freshness}
  predicates:
    - {id: okn:derivesFreshnessFrom, object_kind: literal, datatype: xsd:string, maximum_count: 1}
    - {id: okn:providesOperation, object_kind: literal, datatype: xsd:string}
sources:
  - id: typed-claims
    resource: claim-profile.md
    observe: manual
    role: authoritative
  - id: claims-command
    resource: commands/claims.md
    observe: manual
    role: authoritative
  - id: knowledge-audit
    resource: commands/audit.md
    observe: manual
    role: authoritative
claims:
  - id: okn:claim/claim-freshness/evidence-digest
    slot: okn:slot/claim-freshness/evidence-digest
    subject: okn:feature/claim-freshness
    predicate: okn:derivesFreshnessFrom
    object: {value: latest local evidence SHA-256 observation, datatype: xsd:string}
    evidence:
      - {id: okn:evidence/claim-freshness/profile, source_ref: typed-claims, stance: supports, role: primary}
    owners: [team:openknowledge]
    status: verified
    section_ref: "#derived-state"
    verification:
      method: documentation-review
      by: human:openknowledge-maintainers
      at: 2026-08-26T17:15:00Z
      evidence_refs: [okn:evidence/claim-freshness/profile]
      evidence_versions:
        - evidence_ref: okn:evidence/claim-freshness/profile
          source_ref: typed-claims
          resource: claim-profile.md
          sha256: 5249e8e5fb7446695214cdefd85421d74fc858a3fa5d6267ad749b7783310e17
          by: human:openknowledge-maintainers
          at: 2026-08-26T17:15:00Z
  - id: okn:claim/claim-freshness/reconcile-command
    slot: okn:slot/claim-freshness/reconcile-command
    subject: okn:feature/claim-freshness
    predicate: okn:providesOperation
    object: {value: claims stale and claims reconcile, datatype: xsd:string}
    evidence:
      - {id: okn:evidence/claim-freshness/commands, source_ref: claims-command, stance: supports, role: primary}
    owners: [team:openknowledge]
    status: verified
    section_ref: "#reconciliation"
    verification:
      method: documentation-review
      by: human:openknowledge-maintainers
      at: 2026-08-26T17:15:00Z
      evidence_refs: [okn:evidence/claim-freshness/commands]
      evidence_versions:
        - evidence_ref: okn:evidence/claim-freshness/commands
          source_ref: claims-command
          resource: commands/claims.md
          sha256: 3a9afeb197446c097492e61bc687c382bf8b6af226aa77cfa8a0276f8601df37
          by: human:openknowledge-maintainers
          at: 2026-08-26T17:15:00Z
  - id: okn:claim/claim-freshness/audit-gate
    slot: okn:slot/claim-freshness/audit-gate
    subject: okn:feature/claim-freshness
    predicate: okn:providesOperation
    object: {value: source baseline advancement gate, datatype: xsd:string}
    evidence:
      - {id: okn:evidence/claim-freshness/audit, source_ref: knowledge-audit, stance: supports, role: primary}
    owners: [team:openknowledge]
    status: verified
    section_ref: "#reconciliation"
    verification:
      method: documentation-review
      by: human:openknowledge-maintainers
      at: 2026-08-26T17:15:00Z
      evidence_refs: [okn:evidence/claim-freshness/audit]
      evidence_versions:
        - evidence_ref: okn:evidence/claim-freshness/audit
          source_ref: knowledge-audit
          resource: commands/audit.md
          sha256: 349c857347cc394764826a842563961a0c9aa5a2e76d2d2a711d9d5fbabf4447
          by: human:openknowledge-maintainers
          at: 2026-08-26T17:15:00Z
---

# Claim Freshness

Open Knowledge derives claim freshness from evidence state. It does not store
a second authored stale flag.

## Observation contract

Claim verification records the SHA-256 identity of each local live evidence
resource in `verification.evidence_versions`. The ordered list is append-only.
The last entry for an evidence ID is the current reviewed observation.

A pinned source uses two identities. `resource` points to the immutable
content-addressed artifact. `live_resource` points to the upstream local file.
This separation prevents the artifact from hiding later upstream changes.

Remote evidence stays network-free during validation and claim maintenance.
Remote-only verification can have no local evidence observation.

<a id="derived-state"></a>

## Derived state

The parser hashes an observable local live resource. When that digest differs
from the latest reviewed observation, it sets `stale: true` on the machine
claim projection and lists the exact evidence IDs in `staleEvidence`.

The derived state reaches search context, runtime policy, MCP, the viewer, and
the audit. It remains stale across unrelated edits because the stored evidence
history does not change.

<a id="reconciliation"></a>

## Reconciliation

Use this review loop:

```sh
okn claims stale --path Wiki
okn claims reconcile <claim-id> --document <path> \
  --approved-by human:<id> --path Wiki
okn audit Wiki --baseline .openknowledge/audit-sources.json \
  --update-baseline --fail-on high
```

`reconcile` appends a current local evidence observation in one validated file
write. It preserves the previous claim verification and observation history.
It does not decide whether the claim is semantically true. Correct or replace
the claim before reconciliation when the evidence no longer supports it.

The command writes page-level `verified` only when all active claims on the
page are verified and current. It leaves `generated` unchanged because an
evidence-only reconciliation does not change the body.

The audit refuses to advance its source baseline while a changed source has an
unresolved typed claim. A source without typed claims keeps its page-level
`source-changed` finding until review.

## History evaluation

`okn eval claims` reads a strict dataset and checks active claim IDs at
immutable Git checkpoints. Recorded states are `supported`, `refuted`, and
`unverified`. The report classifies observed claims as `supported`, `stale`,
`hallucinated`, or `unverified`.

This evaluation tests memory behavior across repository history. It does not
use a model as a truth oracle.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/claims_evidence.go`
> - `packages/cli/internal/claimops/observation.go`
> - `packages/cli/internal/audit/audit.go`
> - `packages/cli/internal/eval/claim_replay.go`
>
> **Update notes**
>
> Update this page when evidence observation, stale derivation,
> reconciliation, or claim replay behavior changes.
