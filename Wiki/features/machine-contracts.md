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

## Covered Outputs

| Schema | CLI surface |
| --- | --- |
| `ast.schema.json` | `openknowledge ast` |
| `bundle.schema.json` | `openknowledge to json` |
| `graph.schema.json` | `openknowledge to graph`, including source and search graph types |
| `list.schema.json` | `openknowledge list --json` |
| `registry-list.schema.json` | `openknowledge registry list --json` |
| `registry-status.schema.json` | `openknowledge registry status --json` |
| `search-context.schema.json` | `openknowledge search --format json` and MCP search structured content |
| `search-results.schema.json` | `openknowledge search --matches --format json` |
| `validation.schema.json` | `openknowledge validate --format json` and MCP validation structured content |

Shared issue, link, and recursively typed frontmatter definitions live in
`common.schema.json`.

## Public Schema URLs

The canonical Draft 2020-12 schemas live under
`packages/cli/schemas/v1/`. The production website build copies them to
`/schemas/cli/v1/`, so each declared identifier is also its fetchable public
location. For example:

```text
https://openknowledge.sh/schemas/cli/v1/validation.schema.json
https://openknowledge.sh/schemas/cli/v1/common.schema.json
```

The build fails when a schema `$id` does not match its public route. Relative
references to `common.schema.json` therefore resolve identically from the
repository and website.

## Compatibility Policy

Version 1 permits additive fields that preserve existing field meanings and
types. Removing a field, changing a type, narrowing accepted values in a way
that rejects valid v1 output, or changing semantics incompatibly requires a
new schema-version directory and a new `schemaVersion` value.

Schemas are closed at defined object boundaries with
`additionalProperties: false`. Integrations can therefore distinguish a
deliberate additive contract change from accidental encoder/schema drift.
Nested AST, graph node and edge, search source, validation check, registry,
bundle, list, link, and issue shapes are explicit rather than open-ended.

## Enforcement

`go test ./packages/cli/...` performs three complementary checks:

* compiles every schema as Draft 2020-12 with all `$id` resources registered
  locally, so tests never depend on the network
* validates every golden JSON fixture and representative non-empty output from
  the real AST, bundle, graph, list, search, context, and validation builders
* mutates top-level and nested objects with undeclared fields and requires the
  corresponding schema to reject them

The pinned `github.com/santhosh-tekuri/jsonschema/v6` dependency is imported
only by tests. It does not become part of the CLI runtime binary. Web tests
independently copy all schemas into an isolated distribution tree, verify the
route-to-`$id` mapping, and exercise the negative mismatched-ID path.

Golden fixtures remain useful for detecting exact serialized changes; schema
validation adds semantic coverage for non-empty and nested output that an
empty snapshot cannot provide by itself.

## Source Anchors

* `packages/cli/schemas/v1/`
* `packages/cli/internal/okf/schema_contract_test.go`
* `packages/cli/internal/okf/machine_contract_test.go`
* `packages/cli/go.mod`
* `packages/web/scripts/schema-distribution.mjs`
* `packages/web/scripts/schema-distribution.test.mjs`
* `packages/web/scripts/build.mjs`
