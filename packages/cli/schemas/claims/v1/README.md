# Typed claim contracts v1

`frontmatter.schema.json` defines the authored claim ontology and typed claim graph.
`proposal.schema.json` defines a digest-bound, read-only claim proposal.
`entity-proposal.schema.json` defines a digest-bound alias or merge proposal
for stable ontology entity IDs. A merge binds both declaration documents.

Each occurrence has a globally unique `id`. A `slot` groups comparable
subject-predicate-object assertions. Evidence, selectors, verification, time,
units, scope, and claim relations remain structured.

The proposal contract creates a new occurrence with `status: proposed`.
Confidence describes extraction confidence. It does not describe truth.

A claim evidence selector requires its same-document source to use
`observe: pinned` with the SHA-256 of the exact artifact bytes. Validation
resolves available local artifacts without a network fetch. Missing remote
bytes and selector types without deterministic local resolvers are
`unverifiable` and fail closed.

`claims entities impact` previews affected claim fields. `claims entities
apply` requires a human or GitHub approval identity, rewrites canonical
Markdown transactionally, and keeps the former ID as `deprecated` with
`replaced_by`.
