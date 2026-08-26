---
type: Command Documentation
title: openknowledge query
description: Run bounded SPARQL, Datalog, and hybrid queries over typed knowledge.
tags: [openknowledge, cli, command, query, sparql, datalog, retrieval]
timestamp: 2026-08-25T00:00:00Z
---

# `openknowledge query`

Run read-only semantic or logical queries over one validated knowledge base.
Markdown and YAML remain canonical. The command builds immutable,
revision-bound projections and never writes asserted or derived facts back to
the bundle.

## Engines

| Engine | Use it for | Query surface |
| --- | --- | --- |
| `sparql` | RDF graph patterns, joins, aggregates, and property paths. | SPARQL 1.1 `SELECT` or `ASK` |
| `datalog` | Recursive rules and inspectable derivations. | Mangle atoms and rules |
| `hybrid` | One ranked result set from text plus explicit structured queries. | BM25, local vectors, SPARQL, and Datalog |

The hybrid command runs only the engines named by the caller. A text query
does not get translated into SPARQL or Datalog automatically.

## SPARQL

```sh
okn query sparql --query 'SELECT ?claim ?status WHERE {
  ?claim <https://openknowledge.sh/ns/status> ?status
}' Wiki
okn query sparql --query-file ./query.rq --out ./result.json Wiki
```

The embedded engine supports `SELECT`, `ASK`, aggregates, property paths, and
the revision named graph. `UPDATE`, `CONSTRUCT`, remote loading, and federation
are disabled. Results preserve RDF literal datatypes and language tags and
include source provenance for claim-backed bindings.

## Datalog

The base projection contains these relations:

```text
claim(ID, Subject, Predicate, Object)
status(ID, Status)
trust(ID, TrustTier)
stale(ID, Boolean)
scope(ID, Dimension, Value)
valid_time(ID, From, Until)
relation(ID, Kind, TargetID)
evidence(ID, EvidenceID, Stance)
source(ID, Document, Locator)
object_metadata(ID, Datatype, Language, Unit, QuantityKind)
```

Query an asserted fact directly:

```sh
okn query datalog --query 'claim(ID, Subject, Predicate, Object)' Wiki
```

Add recursive Mangle rules from a file:

```text
depends_on(X, Y) :- relation(X, "derived_from", Y).
depends_on(X, Z) :- relation(X, "derived_from", Y), depends_on(Y, Z).
```

```sh
okn query datalog \
  --query 'depends_on(Source, Target)' \
  --rules ./rules.mg Wiki
```

The default `openknowledge.safe/v1` profile accepts positive safe rules.
Negation requires the explicit `openknowledge.closed-world/v1` profile because
absence is then interpreted against the filtered local snapshot. Rules cannot
inject new base facts. Every derived result contains a proof tree and the
source provenance of its asserted inputs.

## Hybrid

```sh
okn query hybrid --text 'production token format' Wiki
okn query hybrid --text 'vehicle inspection schedule' \
  --embedding-url http://127.0.0.1:11434 \
  --embedding-model embeddinggemma Wiki
okn query hybrid --text 'production token format' \
  --sparql-file ./constraints.rq Wiki
okn query hybrid --text 'downstream derivations' \
  --datalog-query 'depends_on(Source, Target)' --rules ./rules.mg Wiki
```

Text uses BM25, vector, and section-focus candidates. The section-focus route
selects one answer-focused section from each retrieved document. It uses term
coverage and a specific heading match.

Explicit structured routes remain independent. Reciprocal-rank fusion combines
their ranks and joins structured bindings or proof paths back to source text.
Result kinds distinguish `retrieved-text`, `asserted-fact`, and `derived-fact`.

Without an embedding URL, the vector route uses the deterministic local hash
provider. Set `--embedding-url` to use an OpenAI-compatible HTTP endpoint. A
base URL uses `/v1/embeddings`. The endpoint can also use the Ollama
`/api/embed` response format.

| Embedding option | Default |
| --- | --- |
| `--embedding-url` or `OPENKNOWLEDGE_EMBEDDING_URL` | Not set. Use the local hash provider. |
| `--embedding-model` or `OPENKNOWLEDGE_EMBEDDING_MODEL` | `embeddinggemma` |
| `--embedding-cache` or `OPENKNOWLEDGE_EMBEDDING_CACHE` | Private per-user cache file. |
| `OPENKNOWLEDGE_EMBEDDING_TOKEN` | Not set. Send no authorization header. |

The provider probes the model identity before index construction. The cache
binds each exact section input to that identity. A later run embeds only new
or changed sections. The cache file has user-only permissions.

The command sends section text and query text to a configured endpoint. HTTP
is valid only for a loopback endpoint. Use HTTPS for a non-loopback endpoint.
Redirects are disabled. A configured endpoint failure stops the query. It does
not silently select the hash provider.

By default, hybrid retrieval excludes rejected, superseded, archived, and
stale claims. Access filtering runs before graph or rule evaluation.

## Common limits and output

| Option | Default | Description |
| --- | --- | --- |
| `key-or-path` | `.` | Local root or connected registry key. |
| `--access <label>` | public only | Grant a `profile:`, `agent:`, `team:`, or `use_case:` source label. Repeatable. |
| `--limit <count>` | `12` | Maximum returned bindings, facts, or fused results. Range: 1–1000. |
| `--timeout <duration>` | `2s` | Per-engine deadline. Maximum: `30s`. |
| `--out <file>` | stdout | Atomically write the JSON result. |
| `--spec <version>` | `latest` | OKF version used to read the bundle. |

The engines also enforce query or rule byte limits, dataset or fact limits,
created-fact and proof-depth limits, and a process-wide concurrency bound.
Invalid bundles do not enter the semantic projections.

JSON uses the v1 `sparql-query.schema.json`, `datalog-query.schema.json`, or
`hybrid-query.schema.json` contract.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/query_command.go`
> - `packages/cli/internal/okf/semantic_facts.go`
> - `packages/cli/internal/okf/sparql.go`
> - `packages/cli/internal/okf/datalog.go`
> - `packages/cli/internal/okf/hybrid.go`
> - `packages/cli/internal/okf/embedding_http.go`
> - `packages/cli/schemas/v1/{sparql-query,datalog-query,hybrid-query}.schema.json`
>
> **Update notes**
>
> Update this page when query languages, projections, access policy, limits,
> proof behavior, routing, fusion, or output contracts change.
