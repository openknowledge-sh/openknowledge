# Open Knowledge CLI JSON schemas v1

These Draft 2020-12 schemas describe the versioned machine-readable output of
the Open Knowledge CLI. Every covered top-level document contains
`"schemaVersion": "1"`.

The website build publishes this directory at
`https://openknowledge.sh/schemas/cli/v1/`, matching every schema's `$id`.
Relative references such as `common.schema.json` therefore resolve both from
the source tree and from the public URL.

| Schema | CLI output |
| --- | --- |
| `job-list.schema.json` | `openknowledge jobs list --json` |
| `job-status.schema.json` | `openknowledge jobs status --json` |
| `job-runs.schema.json` | `openknowledge jobs runs --json` |
| `job-start.schema.json` | `openknowledge jobs start --json` |
| `job-control.schema.json` | `openknowledge jobs stop|kill --json` |
| `job-run-summary.schema.json` | Shared run summary used by agent management outputs |
| `job-validation.schema.json` | `openknowledge jobs validate --json` |
| `job-run-plan.schema.json` | `openknowledge jobs run --dry-run` and persisted `plan.json` |
| `job-run-record.schema.json` | Persisted agent `run.json`, including cancellation and kill outcomes |
| `ast.schema.json` | `openknowledge ast` |
| `bundle.schema.json` | `openknowledge to json` |
| `cli-error.schema.json` | `openknowledge --error-format json <command> ...` failures on stderr |
| `federated-search-context.schema.json` | `openknowledge search --all <query> --format json` |
| `federated-search-results.schema.json` | `openknowledge search --all <query> --matches --format json` |
| `graph.schema.json` | `openknowledge to graph` |
| `list.schema.json` | `openknowledge list --json` |
| `registry-list.schema.json` | `openknowledge registry list --json` |
| `registry-status.schema.json` | `openknowledge registry status --json` |
| `search-results.schema.json` | `openknowledge search --matches --format json` |
| `search-context.schema.json` | `openknowledge search --format json` |
| `validation.schema.json` | `openknowledge validate --format json` |

Additive fields may be added to v1 outputs. Removing a field, changing its JSON
type, or changing its meaning incompatibly requires a new schema version and a
new directory. `specVersion` is independent: it identifies the selected Open
Knowledge Format version, not the CLI JSON contract.

The pre-1.0 `agents` command group is experimental and exempt from that
compatibility rule. Its job, plan, run-record, and management schemas are the
single current contract and may change in place without legacy copies or
migrations until the feature is stabilized.

The CLI test suite compiles every schema as Draft 2020-12 without network
access, validates all golden contracts and representative non-empty outputs,
and verifies that undeclared top-level and nested fields are rejected. The
validator dependency is test-only and is not linked into the distributed CLI.

Portable `openknowledge.json` discovery manifests use an independent protocol
schema under [`../manifest/v1/`](../manifest/v1/). They do not contain the CLI
output `schemaVersion` field; their numeric `version` and concrete OKF `spec`
identify separate compatibility dimensions.

CLI-owned registry and managed-cache provenance use independent persistence
schemas under [`../storage/v1/`](../storage/v1/).
