---
type: Feature Documentation
title: Machine-Readable Contracts
description: JSON schemas, version domains, and compatibility for CLI automation.
tags: [openknowledge, cli, json, schema, api, compatibility]
timestamp: 2026-07-18T00:00:00Z
---

# Machine-Readable Contracts

Stable CLI JSON objects declare `schemaVersion: "1"`. `specVersion`, when
present, identifies the independently versioned OKF document format.

## CLI output schemas

| Schema | Surface |
| --- | --- |
| `ast.schema.json` | `ast` |
| `bundle.schema.json` | `export json` |
| `cli-error.schema.json` | global `--error-format json` failures |
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

Shared issue, link, retrieval, and typed-frontmatter definitions live in
`common.schema.json`. Job contracts remain experimental and may change in place
before 1.0.

`agent doctor --json`, runtime plan/build, and Railway deploy results also
declare `schemaVersion: "1"`, but do not currently have published schemas.
Treat these diagnostic and operational shapes as provisional.

## Error envelope

Place the global option before the command:

```sh
openknowledge --error-format json search
```

Failures produce one JSON document on stderr while preserving the original
exit status:

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

`kind` is `usage` for exit status `2` and `runtime` otherwise. Diagnostics are
capped at 256 KiB. Command-specific semantic JSON remains on stdout: an invalid
validation report, for example, is not wrapped as a CLI error.

## Schema locations

Draft 2020-12 CLI schemas live in `packages/cli/schemas/v1/` and are published
at:

```text
https://openknowledge.sh/schemas/cli/v1/<schema>.json
```

Two other version domains are independent:

| Contract | Repository | Public route |
| --- | --- | --- |
| Portable `openknowledge.json` | `schemas/manifest/v1/` | `/schemas/cli/manifest/v1/` |
| Registry and cache persistence | `schemas/storage/v1/` | `/schemas/cli/storage/v1/` |
| Runtime generation manifest | `schemas/runtime/v1/` | not a CLI output contract |

Portable manifests use numeric `version` plus concrete `spec`; local storage
and runtime manifests use their own `schemaVersion` values. These domains do
not imply one another.

## Compatibility

Version 1 may add fields while preserving existing field meanings and types;
the closed v1 schema is then updated in place. Consumers that validate against
a downloaded schema must refresh it before accepting such output. Removing a
field, changing its type or meaning, or rejecting previously valid output
normally requires a new schema version.

Schemas use `additionalProperties: false` at defined object boundaries to catch
encoder drift. Retrieval results bind evidence to a concrete corpus revision,
section digest, and `okf+sha256://` locator. Federated search wraps the existing
single-bundle objects with registry identity, local rank, and RRF score.

Repository tests compile every schema offline, validate golden and runtime
objects, and verify that undeclared top-level and nested fields fail. The web
build checks each `$id` against its public route before copying schemas.

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
> Update this page when a schema, version domain, provisional JSON surface, or
> distribution route changes.
