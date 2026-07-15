# Open Knowledge Go API

This package is the supported read-only Go facade over the same OKF core used
by the `openknowledge` CLI.

```go
import "github.com/openknowledge-sh/openknowledge/packages/cli/okf"

report, err := okf.ValidateWithVersion("./Wiki", "0.1")
if err != nil {
    return err
}
if err := okf.RequireValidBundle(report); err != nil {
    return err
}

context, err := okf.ResolveContextWithVersion("./Wiki", "0.1", okf.ContextOptions{
    Query:  "release workflow",
    Budget: 1200,
    Limit:  8,
})

federated, err := okf.ResolveFederatedContextWithVersion(
    []okf.FederatedTarget{{Name: "team", Root: "./TeamWiki"}},
    "0.1",
    okf.ContextOptions{Query: "release workflow", Budget: 1200, Limit: 8},
)

entries, err := okf.RegistryEntries()
root, err := okf.ResolveKnowledgeRoot("team-docs")
canWrite, err := okf.RegistryPathCanWrite(root)
```

The facade covers parsing, validation, inventory, deterministic search,
budget-bounded context, source/search graphs, metadata, frontmatter, portable
manifest decoding, embedded spec discovery, and strict bounded read-only
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
