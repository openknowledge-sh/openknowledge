# Open Knowledge eval protocol schemas v1

`dataset.schema.json` defines the strict YAML-compatible dataset contract for
`openknowledge eval run`. A dataset uses `type: openknowledge.eval` and numeric
`version: 1`.

`retrieval-dataset.schema.json` defines graded relevance judgments for
`openknowledge eval retrieval`. The command measures section and document
ranking with MRR, Recall@k, and nDCG@k. Optional quality gates enforce absolute
metrics or embedding uplift from the local hash baseline.

`claim-replay-dataset.schema.json` defines recorded claim truth at ordered Git
checkpoints for `openknowledge eval claims`. The command classifies observed
active claims as supported, stale, hallucinated, or unverified.

The dataset stores questions, retrieval expectations, and optional answer
expectations. Answer expectations include answer text, decision, conflict
disclosure, cited sources, minimum valid and entailed citations, and minimum
groundedness. Retrieval policy expectations can
require a trust tier, reject stale evidence, restrict lifecycle statuses, and
require structured provenance on every selected source.

Cases can declare bounded `agents` IDs. Comparison reports map changed bundle
paths to affected questions and agents and list changed paths with no eval
coverage.

`answer-request.schema.json` defines the JSON document that the CLI writes to
an answer command stdin. It contains the dataset identity, target revision,
questions, and retrieved source Markdown with content-bound locators.

`answer-response.schema.json` defines the strict JSON document that the command
writes to stdout. It contains one answer per case. Each answer preserves an
answer or abstain decision, applicability metadata, conflicts, and claims.
Citation entailment is an attestation from this trusted command; the CLI
validates and scores it but does not independently prove semantic entailment.

The CLI starts the configured executable directly. It does not use a shell or
provide a sandbox. The process inherits the CLI environment and system access.
Use a trusted command. Retrieval is deterministic, but answer reproducibility
depends on the command and its external services.
