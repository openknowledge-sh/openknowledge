---
type: Feature Documentation
title: Machine-Readable Contracts
description: JSON schemas, version domains, and compatibility for CLI automation.
tags: [openknowledge, cli, json, schema, api, compatibility]
timestamp: 2026-07-28T00:00:00Z
---

# Machine-Readable Contracts

Most stable CLI JSON objects declare `schemaVersion: "1"`.
When present, `specVersion` identifies the independently versioned OKF document format.
Audit contracts and private usage events use a type and numeric version.

## CLI output schemas

| Schema | Surface |
| --- | --- |
| `ast.schema.json` | `ast` |
| `bundle.schema.json` | `export json` |
| `cli-error.schema.json` | global `--error-format json` failures |
| `audit-report.schema.json` | `audit --format json` |
| `audit-source-baseline.schema.json` | `audit --baseline <file> --update-baseline` |
| `eval-comparison.schema.json` | `eval run --base <git-ref> --format json` |
| `eval-report.schema.json` | `eval run --format json` |
| `quality-report.schema.json` | `quality report --format json` |
| `intervention-event.schema.json` | `quality interventions append`, private intervention JSONL |
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
| `runtime-releases.schema.json` | `runtime releases` |
| `runtime-release-action.schema.json` | `runtime preview --check`, `runtime pin`, `runtime rollback` |
| `runtime-cache.schema.json` | `runtime cache status`, `runtime cache rebuild`, `runtime cache prune` |
| `runtime-search.schema.json` | runtime `GET <route>/_search` |
| `runtime-context.schema.json` | runtime MCP `openknowledge_search` |
| `usage-event.schema.json` | private runtime HTTP and MCP search events |
| `feedback-event.schema.json` | runtime `POST <route>/_feedback` response and private feedback event |
| `deploy-plan.schema.json` | `deploy railway --dry-run` |
| `deploy-result.schema.json` | successful `deploy railway` result |
| `deploy-runtime-scaffold.schema.json` | `deploy railway init` |

`common.schema.json` contains shared issue, link, retrieval, typed-frontmatter,
and OKF 0.2 signal definitions. `list.schema.json` and `graph.schema.json`
reference its `okf02` contract. This contract contains derived trust,
lifecycle and staleness, generation and verification events, structured
sources, and optional Attested Computation data.

Audit reports use `type: openknowledge.audit-report` and numeric `version: 1`.
Each finding records category, severity, impact, targets, and evidence. Source
baselines use `type: openknowledge.audit-source-baseline` and numeric
`version: 1`.

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

Eval dataset v1 policy expectations are `minimum_trust`, `allow_stale`,
`allowed_statuses`, and `require_sources`. They test every selected source
against derived OKF 0.2 trust, freshness, lifecycle, and provenance signals.

`eval-report.schema.json` accepts these names as check kinds. Each check
records `expected`, optional rejected source paths in `actual`, and `passed`.
Markdown reports include failed policy checks and include all policy checks in
the totals.

Eval dataset v1 cases can contain an optional `agents` list. Eval case results
contain the normalized agent IDs, or an empty list when no agent is declared.

`eval-comparison.schema.json` requires an `impact` object. The object contains
`changedPaths`, `affectedAgents`, `affectedQuestions`, and `uncoveredPaths`.
Each affected question contains its case ID, question, agents, paths, and
impact reasons.

Expected, retrieved, and valid cited sources attribute changed paths to cases.
Changes to retrieval results, case outcomes, or answer text can also affect a
question. An uncovered path is a changed path with no case source link.

Runtime plans include the normalized `serve.retrieval_policy` object.
`runtime-search.schema.json` and `runtime-context.schema.json` bind responses
to the active generation and retrieval revision.

Runtime plans also include normalized `accessProfiles`. Each profile contains
an environment token name, published knowledge base allowlists, routing
labels, and an optional complete retrieval policy override.

Runtime build results can contain `staged: true`. A staged result has no
`published` object. A published active pointer can contain
`previousGeneration`.

`runtime-releases.schema.json` defines the verified generation inventory. It
records active and previous generation names plus commit, spec, digest, check,
file count, and active status for each stored release.

`runtime-release-action.schema.json` defines `preview`, `pin`, and `rollback`
descriptors. Each descriptor binds the action, knowledge base, generation, and
content digest. Preview descriptors also contain the listen address.

`runtime-cache.schema.json` defines persistent index cache command results.
Its `action` is `status`, `rebuild`, or `prune`. Every result contains
`entries` and `removed` arrays. Prune results also contain `applied`.

Each entry identifies the knowledge base, generation, `search` or `mcp`
target, cache path, and state. States are `ready`, `missing`, `invalid`, and
`rebuilt`. Valid and rebuilt entries can include the index digest and section
count. Invalid entries can include an error. Each removal identifies one
knowledge base and cache generation.

Runtime search and context responses contain `access`, `decision`, and
`refusalReasons`. Access identifies the profile and its `agents`, `teams`, and
`useCases` labels. Decision is `answer` or `refuse`.

A refusal has no selected evidence. Its reason is
`no_relevant_evidence`, `no_policy_compliant_evidence`, or
`insufficient_budget`. The response preserves rejected candidates for review.

Runtime generation identity includes sorted `checks`. These names identify
the successful GitHub checks bound into the generation `contentDigest`.
Runtime plans include normalized `github.required_checks` names and the
`github.auto_merge_low_risk` switch. Auto-merge configuration requires GitHub
integration, check publishing, and at least one required check.

The private hosted exchange can contain a bounded maintenance attestation.
It records normalized risk, approval, confidence, owners, insight and finding
IDs, insight paths, expert targets, and proposal status. This exchange object
is an internal runtime boundary and is not a published CLI schema.

Both retrieval contracts include the effective policy and rejected candidates.
Selected items add trust, freshness, provenance, and selection metadata.
Rejected reasons identify trust, staleness, status, or source policy failures.

Runtime plans also include the normalized `serve.usage_events` object.
`usage-event.schema.json` defines strict local JSONL event records. Each event
uses `type: openknowledge.usage` and numeric `version: 1`.

The event records generation, channel, HMAC query fingerprint, query length,
outcome, selected evidence, and policy rejection counts. Query text is
optional and requires explicit runtime configuration. Its generation identity
uses the same `checks` array.

Runtime search and context responses can contain an optional `usageEventId`.
It is a 32-character lowercase hexadecimal ID. The runtime emits it only after
successful private usage event persistence.

`feedback-event.schema.json` defines `openknowledge.feedback` version 1. Each
event binds its ID, time, knowledge base, generation, usage event ID, query
fingerprint, channel, outcome, access, sentiment, reasons, and selected
evidence. The contract does not contain raw query text.

Feedback sentiment is `positive` or `negative`. Positive events have no
reasons. Negative events have one to six unique reasons from the schema enum.

`quality-report.schema.json` defines `openknowledge.quality-report` version 1.
Its input summary includes intervention events. Intervention-backed timing,
review, audit outcome, and safe-automation metrics are measured only when the
required lifecycle stages are present.

`intervention-event.schema.json` defines `openknowledge.intervention` version
1. Each event binds a stable intervention ID to its time, knowledge base,
actor, source, fixed risk route, targets, and evidence. Stages are `detected`,
`proposed`, `reviewed`, `published`, `dismissed`, `failed`, and `rolled-back`.

Reviewed events add decision and duration. Published events add a generation,
content digest, non-empty successful checks, automated flag, and required
verification. Automated publication is restricted to `low/auto` routing.
Terminal audit-finding events can classify their result as `confirmed` or
`false-positive`. The CLI additionally validates ordered cross-event
lifecycle transitions, which a per-event JSON Schema cannot express.
It binds an evaluation time to the current bundle path, specification, and
SHA-256. It also records the observation window and input counts.

Each metric has an ID, `measured` or `unavailable` status, unit, and evidence
note. Measured metrics contain a value. Ratio metrics can contain a numerator
and denominator. Trend metrics can contain previous and change values.

Generation records contain usage outcomes and feedback counts. Concept records
contain current metadata, sources, `evalCoverageStatus`, eval coverage, use,
feedback, audit findings, priority, and risk reasons. Coverage status is
`unavailable` when no current-revision eval input was supplied. Change records
contain base and proposed eval accuracy plus improved and regressed case
counts.

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
