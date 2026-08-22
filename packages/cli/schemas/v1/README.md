# Open Knowledge CLI JSON schemas v1

These Draft 2020-12 schemas describe versioned CLI and runtime documents.
Most top-level output documents contain `"schemaVersion": "1"`. Private usage,
feedback, and intervention events use a namespaced `type` and numeric
`version: 1`.

The website build publishes this directory at
`https://openknowledge.sh/schemas/cli/v1/`, matching every schema's `$id`.
Relative references such as `common.schema.json` therefore resolve both from
the source tree and from the public URL.

| Schema | CLI output |
| --- | --- |
| `artifact.schema.json` | Durable report bundle manifest under `.openknowledge/reports/<run-id>/artifact.json` |
| `audit-report.schema.json` | `openknowledge audit --format json` |
| `audit-source-baseline.schema.json` | `openknowledge audit --baseline <file> --update-baseline` |
| `evidence-pin.schema.json` | `openknowledge evidence pin --json` |
| `agent-doctor.schema.json` | `openknowledge agent doctor --json` |
| `runtime-plan.schema.json` | `openknowledge automation runtime plan` |
| `runtime-build.schema.json` | `openknowledge automation runtime build` |
| `runtime-releases.schema.json` | `openknowledge automation runtime releases` |
| `runtime-release-action.schema.json` | `openknowledge automation runtime preview --check`, `pin`, and `rollback` |
| `runtime-cache.schema.json` | `openknowledge automation runtime cache status`, `rebuild`, and `prune` |
| `runtime-search.schema.json` | Runtime `GET <route>/_search` response |
| `runtime-context.schema.json` | Runtime MCP `openknowledge_search` result |
| `usage-event.schema.json` | Private runtime HTTP and MCP search event |
| `feedback-event.schema.json` | Runtime `POST <route>/_feedback` response and private feedback event |
| `quality-report.schema.json` | `openknowledge quality report --format json` |
| `intervention-event.schema.json` | `openknowledge quality interventions append`, private intervention JSONL |
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
| `claims-entities.schema.json` | `openknowledge claims entities find --json` |
| `claims-entity-impact.schema.json` | `openknowledge claims entities impact --json` |
| `claims-entity-mutation.schema.json` | `openknowledge claims entities apply --json` |

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
before and proposed pass counts, regressions, and proposed failures. These objects are part of
`job-run-plan.schema.json` and `job-run-record.schema.json`.

Runtime plans contain normalized retrieval policy, usage event settings, and
permission-aware access profiles that route knowledge bases to declared agents,
teams, and use cases. Their GitHub settings also expose the low-risk auto-merge
switch; validation requires enabled check publishing and at least one required
check when it is on. Runtime search and context results identify the active
generation, access profile, and retrieval revision. They also contain policy,
trust, freshness, provenance, selection, and rejection metadata. The explicit
`decision` and `refusalReasons` fields prevent consumers from treating an empty
result as permission to answer without sufficiently trusted evidence.

Runtime generation identity includes a sorted `checks` array. The generation
`contentDigest` binds successful GitHub or runtime-local publication check
names to the generation.
Runtime builds can mark a generation as staged without changing production.
Release inventories identify the active and immediately previous generation;
preview, pin, and rollback actions have a separate strict result contract.
Runtime index caches are private, generation-bound, content-digested files.
Their management command reports strict `runtime-cache.schema.json` results.

Private usage events contain generation identity, a search channel, an HMAC
query fingerprint, a query length range, an outcome, selected evidence, and
policy rejection counts. Query text is optional. The runtime writes these
strict objects as JSONL. Usage events use the same generation `checks` array.
When usage recording is enabled, runtime retrieval responses expose an opaque
`usageEventId`. Feedback events bind that ID to the original generation,
query fingerprint, outcome, access route, and selected evidence without
copying raw query text.

Private intervention events form a strict agentic-maintenance lifecycle. They
bind detection, proposal, review, verified publication, dismissal, failure,
and rollback stages to one intervention ID, evidence set, target set, and
risk route. Hosted maintenance records detection and proposal automatically.
Low-risk automation reaches publication only after the exact GitHub squash
commit becomes an active runtime generation with successful checks.

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

Claim command results use `claims-find.schema.json`,
`claims-impact.schema.json`, `claims-validation.schema.json`, and
`claims-mutation.schema.json`. Entity search, impact, and approved apply use
the `claims-entities` and `claims-entity-*` schemas. Deterministic claim candidates use
`claims-suggestions.schema.json`. Digest-bound authored proposals use the
independent contracts under [`../claims/v1/`](../claims/v1/).

CLI-owned registry and managed-cache provenance use independent persistence
schemas under [`../storage/v1/`](../storage/v1/).
