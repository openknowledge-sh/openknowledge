# Open Knowledge eval protocol schemas v1

`dataset.schema.json` defines the strict YAML-compatible dataset contract for
`openknowledge eval run`. A dataset uses `type: openknowledge.eval` and numeric
`version: 1`.

The dataset stores questions, retrieval expectations, and optional answer
expectations. Answer expectations include answer text, cited sources, minimum
valid citations, and minimum groundedness. Retrieval policy expectations can
require a trust tier, reject stale evidence, restrict lifecycle statuses, and
require structured provenance on every selected source.

`answer-request.schema.json` defines the JSON document that the CLI writes to
an answer command stdin. It contains the dataset identity, target revision,
questions, and retrieved source Markdown with content-bound locators.

`answer-response.schema.json` defines the strict JSON document that the command
writes to stdout. It contains one answer per case. Each answer contains claims,
and each claim contains a list of cited request locators.

The CLI starts the configured executable directly. It does not use a shell or
provide a sandbox. The process inherits the CLI environment and system access.
Use a trusted command. Retrieval is deterministic, but answer reproducibility
depends on the command and its external services.
