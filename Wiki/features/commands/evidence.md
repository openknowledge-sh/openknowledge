---
type: Command Documentation
title: openknowledge evidence
description: Capture exact evidence bytes and bind a source to an immutable local artifact.
tags: [openknowledge, cli, evidence, provenance, claims]
timestamp: 2026-08-22T00:00:00Z
---

# `openknowledge evidence`

Use `evidence pin` to materialize source bytes before a claim selector relies
on them.

```sh
okn evidence pin ./captures/auth-openapi.yaml \
  --document auth.md --source identity-openapi --path Wiki

okn evidence pin https://example.test/policy.html \
  --document policy.md --source policy-primary --path Wiki --json
```

When the input is omitted, the command reads the current
`sources[].resource`. It fetches the network only when the explicit or
declared input is an HTTP or HTTPS URL. HTTP capture requires a successful 2xx
response, has a 30-second timeout, and rejects content larger than 8 MiB.

The command:

1. Computes the SHA-256 of the exact captured bytes.
2. writes a read-only artifact under
   `.openknowledge/evidence/sha256/<digest>/artifact<extension>`;
3. writes an immutable, source-specific receipt under the digest directory;
4. updates the source `resource`, `observe: pinned`, and lowercase `sha256`;
5. validates the edited bundle and rolls back the document edit when it adds a
   claim validation error.

The receipt records the artifact, digest, byte count, media type, original and
final resource, capture time, declaring document, and source ID. It also copies
available source type, author, publisher, license, and access metadata.

Use `sources[].access` when the source must only contribute to selected
runtime profiles. It accepts one label or a list of labels. Labels use
`profile:<id>`, `agent:<id>`, `team:<id>`, or `use_case:<id>`. The receipt
stores the normalized list.

Repeating the command without an input verifies the artifact and receipt. It
does not recapture bytes or change the original capture time. A changed
artifact or receipt fails closed.

Runtime build validates this store again and copies it into the immutable
generation under `evidence/`. This directory is private runtime state. It is
not part of the viewer, source archive, search projection, or MCP projection,
and the HTTP service does not expose it. Runtime claim validation resolves the
published source path against this generation-bound private evidence layer.

The JSON result follows `evidence-pin.schema.json`. Receipts use the independent
`openknowledge.evidence-receipt` version 1 contract.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/evidence_command.go`
> - `packages/cli/internal/claimops/evidence.go`
> - `packages/cli/schemas/v1/evidence-pin.schema.json`
> - `packages/cli/schemas/evidence/v1/receipt.schema.json`
>
> **Update notes**
>
> Update this page when capture limits, storage paths, receipt fields, or
> source binding behavior changes.
