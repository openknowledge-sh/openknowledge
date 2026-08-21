# Open Knowledge CLI JSON schemas v1

These Draft 2020-12 schemas describe versioned CLI and runtime documents.
Most top-level output documents contain `"schemaVersion": "1"`. Private usage
events use `type: openknowledge.usage` and numeric `version: 1`.

The website build publishes this directory at
`https://openknowledge.sh/schemas/cli/v1/`, matching every schema's `$id`.
Relative references such as `common.schema.json` therefore resolve both from
the source tree and from the public URL.

| Schema | CLI output |
| --- | --- |
| `audit-report.schema.json` | `openknowledge audit --format json` |
| `audit-source-baseline.schema.json` | `openknowledge audit --baseline <file> --update-baseline` |
| `agent-doctor.schema.json` | `openknowledge agent doctor --json` |
| `runtime-plan.schema.json` | `openknowledge automation runtime plan` |
| `runtime-build.schema.json` | `openknowledge automation runtime build` |
| `runtime-search.schema.json` | Runtime `GET <route>/_search` response |
| `runtime-context.schema.json` | Runtime MCP `openknowledge_search` result |
| `usage-event.schema.json` | Private runtime HTTP and MCP search event |
| `deploy-plan.schema.json` | `openknowledge automation deploy railway --dry-run` |
| `deploy-result.schema.json` | Successful `openknowledge automation deploy railway` result |
| `deploy-runtime-scaffold.schema.json` | `openknowledge automation deploy railway init` |
| `job-list.schema.json` | `openknowledge automation jobs list --json` |
| `job-status.schema.json` | `openknowledge automation jobs status --json` |
| `job-runs.schema.json` | `openknowledge automation jobs runs --json` |
| `job-start.schema.json` | `openknowledge automation jobs start --json` |
| `job-control.schema.json` | `openknowledge automation jobs stop|kill --json` |
| `job-run-summary.schema.json` | Shared run summary used by agent management outputs |
| `job-validation.schema.json` | `openknowledge automation jobs validate --json` |
| `job-run-plan.schema.json` | `openknowledge automation jobs run --dry-run` and persisted `plan.json` |
| `job-run-record.schema.json` | Persisted agent `run.json`, including cancellation and kill outcomes |
| `ast.schema.json` | `openknowledge ast` |
| `bundle.schema.json` | `openknowledge export json` |
| `cli-error.schema.json` | `openknowledge --error-format json <command> ...` failures on stderr |
| `eval-comparison.schema.json` | `openknowledge eval run --base <git-ref> --format json` |
| `eval-report.schema.json` | `openknowledge eval run --format json` |
| `federated-search-context.schema.json` | `openknowledge search --all <query> --format json` |
| `federated-search-results.schema.json` | `openknowledge search --all <query> --matches --format json` |
| `graph.schema.json` | `openknowledge export graph` |
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

Job run plans can contain a resolved native eval comparison. The object records
dataset, target, spec, gate, base SHA, and optional answer runner settings.
Job run records can contain eval status, identity, private report paths,
regressions, and proposed failures. These objects are part of
`job-run-plan.schema.json` and `job-run-record.schema.json`.

Runtime plans contain normalized retrieval policy and usage event settings.
Their GitHub settings also expose the low-risk auto-merge switch; validation
requires enabled check publishing and at least one required check when it is on.
Runtime search and context results identify the active generation and
retrieval revision. They also contain policy, trust, freshness, provenance,
selection, and rejection metadata.

Runtime generation identity includes a sorted `checks` array. The generation
`contentDigest` binds these successful GitHub check names to the generation.

Private usage events contain generation identity, a search channel, an HMAC
query fingerprint, a query length range, an outcome, selected evidence, and
policy rejection counts. Query text is optional. The runtime writes these
strict objects as JSONL. Usage events use the same generation `checks` array.

The CLI test suite compiles every schema as Draft 2020-12 without network
access, validates all golden contracts and representative non-empty outputs,
and verifies that undeclared top-level and nested fields are rejected. The
validator dependency is test-only and is not linked into the distributed CLI.

Portable `openknowledge.json` discovery manifests use an independent protocol
schema under [`../manifest/v1/`](../manifest/v1/). They do not contain the CLI
output `schemaVersion` field; their numeric `version` and concrete OKF `spec`
identify separate compatibility dimensions.

Strict eval datasets use an independent input schema under
[`../eval/v1/`](../eval/v1/). They use `type: openknowledge.eval` and numeric
`version: 1`. Eval reports and comparison reports use the CLI
`schemaVersion: "1"` output contract. Eval case results can contain answer
text, claim-level citation validity, cited sources, and groundedness metrics.
Eval checks can also assert the minimum trust tier, freshness allowance,
allowed lifecycle statuses, and structured provenance of every selected source.
Comparison reports include changed paths, affected questions and declared
agents, plus changed paths that are not covered by any case.

CLI-owned registry and managed-cache provenance use independent persistence
schemas under [`../storage/v1/`](../storage/v1/).
