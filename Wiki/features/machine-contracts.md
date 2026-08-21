---
type: Feature Documentation
title: Machine-Readable Contracts
description: JSON schemas, version domains, and compatibility for CLI automation.
tags: [openknowledge, cli, json, schema, api, compatibility]
timestamp: 2026-07-28T00:00:00Z
---

# Machine-Readable Contracts

Stable CLI JSON objects declare `schemaVersion: "1"`.
When present, `specVersion` identifies the independently versioned OKF document format.

## CLI output schemas

| Schema | Surface |
| --- | --- |
| `ast.schema.json` | `ast` |
| `bundle.schema.json` | `export json` |
| `cli-error.schema.json` | global `--error-format json` failures |
| `eval-comparison.schema.json` | `eval run --base <git-ref> --format json` |
| `eval-report.schema.json` | `eval run --format json` |
| `list.schema.json` | `list --json` |
| `validation.schema.json` | `validate --format json`, MCP validation |
| `graph.schema.json` | `export graph` |
| `search-context.schema.json` | single-bundle context, MCP search |
| `search-results.schema.json` | single-bundle ranked matches |
| `federated-search-context.schema.json` | registry-wide context |
| `federated-search-results.schema.json` | registry-wide matches |
| `registry-list.schema.json` | `registry list --json` |
| `registry-status.schema.json` | `registry status --json` |
| `job-list.schema.json` | `jobs list --json` |
| `job-status.schema.json` | `jobs status --json` |
| `job-runs.schema.json` | `jobs runs --json` |
| `job-start.schema.json` | `jobs start --json` |
| `job-control.schema.json` | `jobs stop|kill --json` |
| `job-run-summary.schema.json` | Privacy-minimized management summary shared by job outputs. |
| `job-validation.schema.json` | `jobs validate --json` |
| `job-run-plan.schema.json` | `jobs run --dry-run`, persisted plan |
| `job-run-record.schema.json` | persisted lifecycle record |
| `agent-doctor.schema.json` | `agent doctor --json` |
| `runtime-plan.schema.json` | `runtime plan` |
| `runtime-build.schema.json` | `runtime build` |
| `runtime-search.schema.json` | runtime `GET <route>/_search` |
| `runtime-context.schema.json` | runtime MCP `openknowledge_search` |
| `deploy-plan.schema.json` | `deploy railway --dry-run` |
| `deploy-result.schema.json` | successful `deploy railway` result |
| `deploy-runtime-scaffold.schema.json` | `deploy railway init` |

`common.schema.json` contains shared issue, link, retrieval, typed-frontmatter,
and OKF 0.2 signal definitions. `list.schema.json` and `graph.schema.json`
reference its `okf02` contract. This contract contains derived trust,
lifecycle and staleness, generation and verification events, structured
sources, and optional Attested Computation data.
Job contracts are experimental.
They can change without a new version before version 1.0.
Run plans can contain deterministic `preflight` commands and can omit `agent`
for empty-prompt jobs that contain verification. Run records preserve
preflight results and use `preflight_failed` when this phase stops execution.

`job-run-plan.schema.json` can contain an `eval` object. It records the dataset,
target, spec, gate, resolved base SHA, and optional answer runner settings.
`job-run-record.schema.json` can contain the eval result. It records status,
dataset, target, base SHA, gate, report paths, regression count, and proposed
failure count.

Eval result status is `pass`, `fail`, or `error`. The containing run uses
`verification_failed` when an eval gate or eval operation fails.

Runtime plans include the normalized `serve.retrieval_policy` object.
`runtime-search.schema.json` and `runtime-context.schema.json` bind responses
to the active generation and retrieval revision.

Both retrieval contracts include the effective policy and rejected candidates.
Selected items add trust, freshness, provenance, and selection metadata.
Rejected reasons identify trust, staleness, status, or source policy failures.

The published v1 schema distribution includes eval, diagnostic, runtime, and Railway deployment outputs.
Golden tests marshal the current Go result types.
The shared schema suite compiles and validates these fixtures.

## Error envelope

Place the global option before the command:

```sh
okn --error-format json search
```

A failure produces one JSON document on stderr.
The command preserves the original exit status:

```json
{
  "schemaVersion": "1",
  "error": {
    "kind": "usage",
    "command": "search",
    "exitCode": 2,
    "message": "search requires a key or path and a query",
    "truncated": false
  }
}
```

`kind` is `usage` for exit status `2`.
For all other exit status values, `kind` is `runtime`.
The maximum diagnostic size is 256 KiB.
Command-specific semantic JSON remains on stdout.
For example, the CLI does not wrap an invalid validation report as a CLI error.

## Schema locations

Draft 2020-12 CLI schemas are in `packages/cli/schemas/v1/`.
The project publishes them at:

```text
https://openknowledge.sh/schemas/cli/v1/<schema>.json
```

Other version domains are independent:

| Contract | Repository | Public route |
| --- | --- | --- |
| Eval protocol v1 | `schemas/eval/v1/` | `/schemas/cli/eval/v1/` |
| Portable `openknowledge.json` | `schemas/manifest/v1/` | `/schemas/cli/manifest/v1/` |
| Registry and cache persistence | `schemas/storage/v1/` | `/schemas/cli/storage/v1/` |
| Runtime generation manifest | `schemas/runtime/v1/` | not a CLI output contract |

Eval datasets use `type: openknowledge.eval` and numeric `version: 1`.
The dataset schema defines strict YAML-compatible questions, context settings,
retrieval expectations, and answer expectations.

`answer-request.schema.json` defines the stdin document for an answer command.
`answer-response.schema.json` defines its stdout document. Both protocols use
`schemaVersion: "1"` in the eval v1 domain.

Eval reports use the CLI `schemaVersion: "1"` contract. They bind results to
the dataset digest and corpus revision. Answer results contain answer text,
claims, citation validity, cited sources, and groundedness.
Comparison reports bind base and proposed results to their retrieval revisions.
They also record each case classification and the selected gate.

Portable manifests use a numeric `version` and a concrete `spec`.
Local storage and runtime manifests use their own `schemaVersion` values.
These domains are independent.

## Compatibility

Version 1 can add fields when existing field meanings and types do not change.
The project then updates the closed v1 schema.
A consumer that uses a downloaded schema must refresh it before it accepts this output.
A new schema version is usually necessary when the project removes a field.
It is also usually necessary when a field type or meaning changes.
The project also uses a new version when new output rules reject previously valid output.

Schemas use `additionalProperties: false` at defined object boundaries.
This rule detects encoder drift.
Retrieval results bind evidence to a corpus revision, section digest, and `okf+sha256://` locator.
Federated search adds registry identity, local rank, and RRF score to single-bundle objects.

Repository tests compile each schema offline.
They validate golden objects and runtime objects.
They verify that undeclared top-level and nested fields fail.
The web build verifies each `$id` against its public route before it copies schemas.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/schemas/`
> - `packages/cli/internal/okf/schema_contract_test.go`
> - `packages/cli/internal/agents/schema_contract_test.go`
> - `packages/web/scripts/schema-distribution.mjs`
>
> **Update notes**
>
> Update this page after a change to a schema, version domain, machine-readable
> surface, or distribution route.
