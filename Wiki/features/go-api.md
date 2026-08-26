---
type: Feature Documentation
title: Go API
description: Embed the read-only Open Knowledge parser, validation, retrieval, and graph core.
tags: [openknowledge, go, api, sdk, integration]
timestamp: 2026-08-12T00:00:00Z
---

# Go API

Import the supported read-only facade used by the CLI:

```go
import "github.com/openknowledge-sh/openknowledge/packages/cli/okf"
```

## Example

```go
report, err := okf.ValidateWithVersion("./Wiki", "0.2")
if err != nil {
    return err
}
if err := okf.RequireValidBundle(report); err != nil {
    return err
}

packet, err := okf.ResolveContextWithVersion(
    "./Wiki",
    "0.2",
    okf.ContextOptions{Query: "release workflow", Budget: 1200, Limit: 8},
)
```

## Reusable retrieval index

Build one immutable snapshot for multiple retrieval requests:

```go
index, err := okf.BuildContextIndexWithVersion("./Wiki", "0.2")
if err != nil {
    return err
}

results := index.Search(okf.SearchOptions{Query: "release workflow", Limit: 8})
packet, err := index.Resolve(okf.ContextOptions{
    Query: "release workflow",
    Budget: 1200,
    Limit: 8,
})
```

Set `SearchFilters` to restrict candidates before ranking:

```go
results := index.Search(okf.SearchOptions{
    Query: "release workflow",
    Limit: 8,
    Filters: okf.SearchFilters{Types: []string{"Guide"}, Tags: []string{"operations"}},
})
```

Search combines BM25 with a local deterministic vector. The vector uses hashed
word and character features. It does not use an embedding service.

## Semantic facts and local embeddings

Build the normalized, revision-bound semantic fact model once:

```go
facts, err := okf.BuildSemanticFactsWithVersion("./Wiki", "0.2")
```

`facts.Valid` gates every structured projection. Claims, evidence, relations,
references, ontology terms, lifecycle, access labels, typed objects, and
source provenance share one contract.

The default local vector index uses the deterministic hashed provider. Supply
an `EmbeddingProvider` to use an in-process model and a cache path to reuse
unchanged section vectors:

```go
vectors, err := okf.BuildLocalVectorIndex(
    context.Background(), "./Wiki", "0.2", provider, "./cache/vectors.json",
)
results, err := vectors.Search(context.Background(), "release policy", 8)
```

The provider declares a model identity and emits normalized `float32` vectors.
Cache keys bind the exact input digest and model fingerprint. An empty cache
path disables persistence.

Connect an OpenAI-compatible HTTP endpoint when the process does not host the
model:

```go
provider, err := okf.NewHTTPEmbeddingProvider(ctx, okf.HTTPEmbeddingOptions{
    URL: "http://127.0.0.1:11434",
    Model: "embeddinggemma",
})
cachePath, err := okf.DefaultEmbeddingCachePath("./Wiki", provider.Model())
vectors, err := okf.BuildLocalVectorIndex(
    ctx, "./Wiki", "0.2", provider, cachePath,
)
```

The constructor probes the endpoint to bind dimensions and model behavior.
The provider supports OpenAI-compatible responses and Ollama `/api/embed`
responses. It permits plain HTTP only for loopback hosts.

## SPARQL, Datalog, and hybrid query

One-shot helpers parse, validate, project, and query:

```go
sparqlResult, err := okf.QuerySPARQLWithVersion(ctx, "./Wiki", "0.2", sparql, okf.SPARQLQueryOptions{
    AllowedAccess: []string{"team:identity"},
})

datalogResult, err := okf.QueryDatalogWithVersion(ctx, "./Wiki", "0.2", okf.DatalogQuery{
    Query: "depends_on(Source, Target)",
    Rules: rules,
}, okf.DatalogQueryOptions{})

hybridResult, err := okf.QueryHybridWithVersion(ctx, "./Wiki", "0.2", okf.HybridQuery{
    Text: "production dependency",
    SPARQL: sparql,
    Datalog: &okf.DatalogQuery{Query: "depends_on(Source, Target)", Rules: rules},
    Limit: 12,
}, okf.HybridQueryOptions{})
```

For repeated queries, build `SPARQLSnapshot`, `DatalogSnapshot`, or
`HybridSnapshot`. Snapshots are immutable, revision-bound, safe for concurrent
reads, and do not reread source files. Access policy is fixed when the
structured snapshot is built.

`BuildRDFDatasetWithVersion` returns the lossless RDF model.
`RDFDataset.NQuads` serializes deterministic RDF 1.1 N-Quads.

SPARQL permits only bounded `SELECT` and `ASK`. Datalog rules use the safe
Mangle profile unless closed-world negation is selected explicitly. Derived
facts carry proof trees to source-backed asserted inputs. Hybrid routing is
explicit and uses reciprocal-rank fusion.

`BuildContextIndex` uses `LatestSpecVersion`. `BuildContextIndexWithVersion`
uses the selected spec version. Both functions parse, validate, and index the
bundle once.

`ContextIndex.Search` and `ContextIndex.Resolve` use the stored snapshot. These
methods do not read the source files. Reuse one index for concurrent requests.
Build a new index after the bundle changes.

`Search`, `SearchWithVersion`, `ResolveContext`, and
`ResolveContextWithVersion` remain compatible one-shot helpers. Each helper
builds a new index before retrieval.

## Surface

The package exposes these functions and models:

- AST and normalized bundle parsing
- validation policies, known rules, and the valid-bundle gate
- inventory and bundle metadata
- deterministic search, bounded context, local embeddings, and caller-supplied RRF federation
- normalized semantic facts, source and retrieval graphs, and RDF projection
- bounded SPARQL and Datalog snapshots with source provenance and proofs
- explicit hybrid BM25/vector/structured orchestration
- OKF 0.2 derived trust, lifecycle, source, and Attested Computation signals
- strict frontmatter and portable-manifest decoding
- supported spec discovery and the embedded spec
- bounded registry inventory, key or path resolution, and authoring capability

Returned types alias the core models. Therefore, Go results, CLI JSON, MCP
structured content, and published schemas use the same field definitions.
Retrieval results include corpus revisions and content-addressed locators.
Use these values to detect stale evidence.

Functions without a version use `LatestSpecVersion`. For persistent
integrations, prefer `WithVersion` functions. Store `SpecVersion`,
`SchemaVersion`, the retrieval revision, and the selected locators. See
[Machine-readable contracts](machine-contracts.md) for more information.

Validation rule discovery and override parsing follow the same contract. Use
`KnownValidationRulesForVersion`, `IsKnownValidationRuleForVersion`,
`ParseValidationRuleOverrideForVersion`, and
`SetValidationRuleSeverityForVersion` when selecting a spec explicitly.
The known-rule catalog includes fixed-severity rules;
`IsValidationRuleOverrideableForVersion` distinguishes rules that accept
policy overrides. `ValidationOptions` can contain configurable rules from
multiple profiles. Validation applies only options from its selected profile.
The unversioned helpers use `LatestSpecVersion`.

The supported spec versions are `0.1` and `0.2`. `LatestSpecVersion` is
`0.2`. The 0.2 AST normalizes a bare `verified` mapping to a one-item list.
Use `DeriveOKFV02Signals` to derive the same `okf02` contract used by list and
graph output. `OKFV02SourceFootnotes` maps source IDs to stable HTML anchors.
Signal derivation does not execute computation, executor, or attester
resources.

The API is read-only. It does not connect sources, refresh sources, or change
the registry. It does not extract archives, render HTML, or manage processes.
Registry reads do not migrate or rewrite local storage.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/okf/doc.go`
> - `packages/cli/okf/context_index.go`
> - `packages/cli/okf/vector_index.go`
> - `packages/cli/okf/read.go`
> - `packages/cli/okf/types.go`
> - `packages/cli/okf/read.go`
> - `packages/cli/okf/registry.go`
> - `packages/cli/okf/read_test.go`
