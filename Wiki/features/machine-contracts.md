---
type: Feature Documentation
title: Machine-Readable Contracts
description: Versioning, validation, and public distribution for Open Knowledge CLI JSON outputs.
tags: [openknowledge, cli, json, schema, api, compatibility]
timestamp: 2026-07-15T00:00:00Z
---

# Machine-Readable Contracts

Open Knowledge CLI exposes versioned JSON contracts for automation, MCP tools,
editors, CI, and other knowledge-base ecosystem integrations. Every covered
top-level object declares `schemaVersion: "1"`; the independent `specVersion`
field identifies the selected Open Knowledge Format revision where applicable.
The experimental job contracts currently have one schema only and may change
in place before the CLI reaches 1.0.

## Covered Outputs

| Schema | CLI surface |
| --- | --- |
| `job-list.schema.json` | `openknowledge jobs list --json` |
| `job-status.schema.json` | `openknowledge jobs status --json` |
| `job-runs.schema.json` | `openknowledge jobs runs --json` |
| `job-start.schema.json` | `openknowledge jobs start --json` |
| `job-control.schema.json` | `openknowledge jobs stop|kill --json` |
| `job-run-summary.schema.json` | Shared privacy-minimized run summary for management outputs |
| `job-validation.schema.json` | `openknowledge jobs validate --json` |
| `job-run-plan.schema.json` | `openknowledge jobs run --dry-run` and persisted `plan.json` |
| `job-run-record.schema.json` | Persisted agent `run.json` lifecycle records, including `cancelled` and `killed` |
| `ast.schema.json` | `openknowledge ast` |
| `bundle.schema.json` | `openknowledge to json` |
| `cli-error.schema.json` | `openknowledge --error-format json <command> ...` failures on stderr |
| `federated-search-context.schema.json` | `openknowledge search --all <query> --format json` |
| `federated-search-results.schema.json` | `openknowledge search --all <query> --matches --format json` |
| `graph.schema.json` | `openknowledge to graph`, including source and search graph types |
| `list.schema.json` | `openknowledge list --json` |
| `registry-list.schema.json` | `openknowledge registry list --json` |
| `registry-status.schema.json` | `openknowledge registry status --json` |
| `search-context.schema.json` | `openknowledge search --format json` and MCP search structured content |
| `search-results.schema.json` | `openknowledge search --matches --format json` |
| `validation.schema.json` | `openknowledge validate --format json` and MCP validation structured content |

Shared issue, link, and recursively typed frontmatter definitions live in
`common.schema.json`.

Search context and ranked-match contracts share a closed `retrievalRevision`
definition containing the concrete `specVersion` and lowercase SHA-256 of the
indexed Markdown corpus. Their non-empty source/result objects require a
lowercase section `contentSha256` and an `okf+sha256://` locator bound to that
revision. These fields let CLI, Go, and MCP consumers reject stale citations
after a local edit or managed-source refresh.

Federated search uses separate envelopes so the existing single-bundle
root/revision contracts remain unchanged. Both envelopes declare
`fusion.method: "rrf"`, `rankConstant: 60`, and a sorted knowledge-base status
inventory. Successful bases require a revision; failed bases require an error
and cannot claim a revision. Federated result wrappers namespace the existing
closed source/result object with a registry key, local rank, and fusion score.

## Command Error Envelope

Place the global `--error-format json` option before a command to make usage
and operational failures machine-readable. The CLI buffers its diagnostic
stderr and, when the command returns nonzero with a diagnostic, emits exactly
one JSON document on stderr:

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

`kind` is `usage` for exit status `2` and `runtime` for other command
failures. `command` identifies only a recognized root or subcommand path; the
envelope never copies the full argument vector. Diagnostic capture is capped at
256 KiB and reports `truncated: true` if the command exceeded that boundary.

The option does not change stdout or any command-specific JSON success/result
contract. A semantic result can validly use a nonzero status without being a
CLI failure: for example, invalid `validate --format json` output remains the
complete validation report on stdout and does not gain a second stderr
envelope. Successful warnings are replayed as text on stderr. Human-readable
stderr remains the default when the global option is absent or set to `text`.
JSON error mode buffers stderr until the command finishes; interactive,
long-running commands such as `view` should use the default text mode when
immediate warning display matters.

## Public Schema URLs

The current Draft 2020-12 CLI output schemas live under
`packages/cli/schemas/v1/`. The production website build copies them to
`/schemas/cli/v1/`, so each declared identifier is also its fetchable public
location. For example:

```text
https://openknowledge.sh/schemas/cli/v1/validation.schema.json
https://openknowledge.sh/schemas/cli/v1/cli-error.schema.json
https://openknowledge.sh/schemas/cli/v1/common.schema.json
```

The build fails when a schema `$id` does not match its public route. Relative
references to `common.schema.json` therefore resolve identically from the
repository and website.

Portable `openknowledge.json` discovery documents use a separate versioned
protocol schema:

```text
https://openknowledge.sh/schemas/cli/manifest/v1/bundle.schema.json
```

Its numeric `version` identifies the manifest/archive protocol, while its
required concrete `spec` identifies the OKF revision expected inside the
archive. It deliberately does not reuse the CLI output `schemaVersion` field.
Remote consumers reject unknown fields, duplicate object keys, trailing JSON,
non-canonical spec versions, and non-lowercase SHA-256 identities.

CLI-owned local persistence has a third independent version domain:

```text
https://openknowledge.sh/schemas/cli/storage/v1/registry.schema.json
https://openknowledge.sh/schemas/cli/storage/v1/cache-source.schema.json
```

The registry and managed-cache sidecar share `source.schema.json`. Current
writes declare `schemaVersion: "1"`; an unversioned registry remains readable
only as a legacy migration input and is upgraded by the next atomic mutation.
Readers reject unsupported versions, unknown fields, duplicate keys, trailing
JSON, ambiguous duplicate registry names, invalid path/key/access invariants,
and a sidecar whose recorded managed root differs from its actual cache root.
Reads are capped at 8 MiB for the registry and 1 MiB per sidecar before JSON
decoding.

## Compatibility Policy

Version 1 permits additive fields that preserve existing field meanings and
types. Removing a field, changing a type, narrowing accepted values in a way
that rejects valid output, or changing semantics incompatibly normally
requires a new schema-version directory and `schemaVersion` value. The
pre-1.0 experimental agent command surface is explicitly exempt: its job,
plan, run-record, and management contracts may change in place until the
feature is stabilized.

The current job plan's `agent` command is closed over `runtime`, executable
arguments, `prompt_mode`, and credential names. Verification commands use the
generic command shape and do not receive the agent credential list.

Schemas are closed at defined object boundaries with
`additionalProperties: false`. Integrations can therefore distinguish a
deliberate additive contract change from accidental encoder/schema drift.
Nested AST, graph node and edge, search source, validation check, registry,
bundle, list, link, and issue shapes are explicit rather than open-ended.

Runtime generation manifests use a separate closed v1 contract at
`schemas/runtime/v1/generation.schema.json`. It binds a concrete source commit,
knowledge-base ID, OKF spec, and complete sorted SHA-256/byte inventory for the
only permitted roots, `public/`, `source/`, `search/`, and `mcp/`. Runtime
decoding additionally rejects duplicate keys and trailing JSON before digest
verification.

## Enforcement

`go test ./packages/cli/...` performs three complementary checks:

* compiles every CLI-output, portable-manifest, and persistence schema as Draft
  2020-12 with all `$id` resources registered
  locally, so tests never depend on the network
* validates every golden JSON fixture and representative non-empty output from
  the real agent plan/run, AST, bundle, graph, list, search, context, and
  validation builders
* mutates top-level and nested objects with undeclared fields and requires the
  corresponding schema to reject them
* validates a runtime-produced portable manifest and rejects invalid identity,
  version, archive, checksum, and extension variants against its public schema
* validates current registry and remote-cache provenance objects against their
  public persistence schemas

The pinned `github.com/santhosh-tekuri/jsonschema/v6` dependency is imported
only by tests. It does not become part of the CLI runtime binary. Web tests
independently copy all schemas into an isolated distribution tree, verify the
route-to-`$id` mapping, and exercise the negative mismatched-ID path.

Golden fixtures remain useful for detecting exact serialized changes; schema
validation adds semantic coverage for non-empty and nested output that an
empty snapshot cannot provide by itself.

---

<!-- okf-footer: job-maintenance -->

> **Source anchors**
>
> * `packages/cli/schemas/`
> * `packages/cli/internal/okf/schema_contract_test.go`
> * `packages/cli/internal/okf/machine_contract_test.go`
> * `packages/cli/internal/agents/schema_contract_test.go`
> * `packages/cli/go.mod`
> * `packages/web/scripts/schema-distribution.mjs`
> * `packages/web/scripts/schema-distribution.test.mjs`
> * `packages/web/scripts/build.mjs`
>
> **Update notes**
>
> Update this page when a published machine schema, version domain, runtime
> encoder, fixture, or distribution route changes.
