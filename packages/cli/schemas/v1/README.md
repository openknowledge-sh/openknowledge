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
| `ast.schema.json` | `openknowledge ast` |
| `bundle.schema.json` | `openknowledge to json` |
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
