# Open Knowledge CLI JSON schemas v1

These Draft 2020-12 schemas describe the versioned machine-readable output of
the Open Knowledge CLI. Every covered top-level document contains
`"schemaVersion": "1"`.

| Schema | CLI output |
| --- | --- |
| `ast.schema.json` | `openknowledge ast` |
| `bundle.schema.json` | `openknowledge to json` |
| `graph.schema.json` | `openknowledge to graph` |
| `list.schema.json` | `openknowledge list --json` |
| `search-results.schema.json` | `openknowledge search --matches --format json` |
| `search-context.schema.json` | `openknowledge search --format json` |
| `validation.schema.json` | `openknowledge validate --format json` |

Additive fields may be added to v1 outputs. Removing a field, changing its JSON
type, or changing its meaning incompatibly requires a new schema version and a
new directory. `specVersion` is independent: it identifies the selected Open
Knowledge Format version, not the CLI JSON contract.
