# Open Knowledge Go API

This package is the supported read-only Go facade over the same OKF core used
by the `openknowledge` CLI.

```go
import "github.com/openknowledge-sh/openknowledge/packages/cli/okf"

ctx := context.Background()

report, err := okf.ValidateWithVersion("./Wiki", "0.2")
if err != nil {
    return err
}
if err := okf.RequireValidBundle(report); err != nil {
    return err
}

context, err := okf.ResolveContextWithVersion("./Wiki", "0.2", okf.ContextOptions{
    Query:  "release workflow",
    Budget: 1200,
    Limit:  8,
})

facts, err := okf.BuildSemanticFactsWithVersion("./Wiki", "0.2")

rdf, err := okf.BuildRDFDatasetWithVersion("./Wiki", "0.2")
nquads, err := rdf.NQuads()

sparql, err := okf.QuerySPARQLWithVersion(ctx, "./Wiki", "0.2", query, okf.SPARQLQueryOptions{})

datalog, err := okf.QueryDatalogWithVersion(ctx, "./Wiki", "0.2", okf.DatalogQuery{
    Query: "claim(ID, Subject, Predicate, Object)",
}, okf.DatalogQueryOptions{})

hybrid, err := okf.QueryHybridWithVersion(ctx, "./Wiki", "0.2", okf.HybridQuery{
    Text: "release workflow",
    Limit: 8,
}, okf.HybridQueryOptions{})

federated, err := okf.ResolveFederatedContextWithVersion(
    []okf.FederatedTarget{{Name: "team", Root: "./TeamWiki"}},
    "0.2",
    okf.ContextOptions{Query: "release workflow", Budget: 1200, Limit: 8},
)

entries, err := okf.RegistryEntries()
root, err := okf.ResolveKnowledgeRoot("team-docs")
canWrite, err := okf.RegistryPathCanWrite(root)
```

The facade covers parsing, validation, inventory, deterministic search,
budget-bounded context, normalized semantic facts, pluggable local embeddings,
RDF/N-Quads, bounded SPARQL and Datalog with proof paths, explicit hybrid
retrieval, source/search graphs, metadata, frontmatter, portable manifest
decoding, embedded spec discovery, and strict bounded read-only
registry discovery/resolution and capability checks. It intentionally excludes
registry mutation, remote downloads, archive extraction, HTML generation, and
viewer process lifecycle; use the CLI for those operational workflows.

Functions without a version select `LatestSpecVersion`. Persisted integrations
should use the explicit `WithVersion` forms and retain returned
`SchemaVersion` and `SpecVersion` identities. Search and context callers can
persist `RetrievalRevision` and each result locator to detect stale evidence
after edits or refreshes. The module is still pre-v1, so
Go source compatibility follows module semantic versioning; serialized output
compatibility follows the separately documented machine-schema policy.

Validation rules are also version-bound. Use
`KnownValidationRulesForVersion`, `ParseValidationRuleOverrideForVersion`,
and `SetValidationRuleSeverityForVersion` with an explicit spec selection.
The known-rule list includes mandatory rules;
`IsValidationRuleOverrideableForVersion` distinguishes fixed severities.

Build `LocalVectorIndex`, `SPARQLSnapshot`, `DatalogSnapshot`, or
`HybridSnapshot` when serving repeated requests. These indexes are immutable
and revision-bound. Structured snapshots apply access filtering before query
evaluation. An optional embedding cache reuses vectors only when both exact
input content and the provider model fingerprint match.

Use `NewHTTPEmbeddingProvider` for an OpenAI-compatible endpoint. A base URL
uses `/v1/embeddings`. `DefaultEmbeddingCachePath` returns a private per-user
cache path for one knowledge root and provider model.
