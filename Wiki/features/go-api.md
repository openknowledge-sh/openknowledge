---
type: Feature Documentation
title: Go API
description: Public read-only Go package for embedding Open Knowledge parsing, validation, retrieval, and graph views.
tags: [openknowledge, go, api, sdk, integration]
timestamp: 2026-07-15T00:00:00Z
---

# Go API

Go applications can import a supported read-only facade over the exact core
used by the CLI:

```go
import "github.com/openknowledge-sh/openknowledge/packages/cli/okf"
```

Before this package existed, the implementation lived only under Go's
`internal/` boundary. External tools therefore had to launch a subprocess and
decode CLI JSON even when they needed in-process parsing or retrieval.

## Example

```go
report, err := okf.ValidateWithVersion("./Wiki", "0.1")
if err != nil {
    return err
}
if err := okf.RequireValidBundle(report); err != nil {
    return err
}

packet, err := okf.ResolveContextWithVersion(
    "./Wiki",
    "0.1",
    okf.ContextOptions{Query: "release workflow", Budget: 1200, Limit: 8},
)

entries, err := okf.RegistryEntries()
root, err := okf.ResolveKnowledgeRoot("team-docs")
canWrite, err := okf.RegistryPathCanWrite(root)
```

## Surface

The package exposes:

* AST and normalized-bundle parsing
* validation, validation policy options, known rules, and the valid-bundle gate
* inventory listing and bundle metadata
* deterministic match search and budget-bounded source context
* deterministic RRF federation across caller-provided named bundle targets
* source and retrieval graph construction
* strict frontmatter and portable-manifest decoding
* supported spec discovery and the pinned spec document
* strict bounded registry inventory, exact key/path resolution, and effective
  local authoring-capability checks

Returned structures are aliases of the core models, not copied SDK-specific
models. CLI behavior, public Go results, JSON schemas, and MCP structured
content therefore share one implementation and one set of JSON field tags.
Search and context results include a concrete `RetrievalRevision` plus
content-addressed section locators. Callers that persist evidence can compare
`revision.indexSha256` after refresh instead of assuming a path and line range
still identify the same content.

## Versioning

Calls without an explicit version use `LatestSpecVersion`. Integrations that
persist data should prefer `WithVersion` functions and record both
`SpecVersion` and `SchemaVersion` from returned models. Retrieval callers
should additionally persist the top-level revision and each selected locator.
Federated helpers accept explicit `FederatedTarget` values rather than reading
the user registry implicitly, keeping the public package deterministic and
read-only. Both default-version and `WithVersion` forms are available.

The Go module is pre-v1, so source compatibility follows module semantic
versioning. Serialized compatibility is a separate contract documented under
[machine-readable contracts](machine-contracts.md). The portable manifest also
has its own independent protocol version and schema.

## Boundary

The public package is intentionally read-only. Registry discovery reads the
same bounded fail-closed storage as the CLI but never rewrites or migrates it.
Registry mutation, network
materialization and refresh, archive extraction, HTML/viewer generation, and
process lifecycle remain operational CLI responsibilities. This keeps an
embedded parser or retrieval service from silently mutating user registry or
cache state.

## Verification

An external-package test imports only the public path and exercises validation,
AST and normalized parsing, inventory, search, context, graph, metadata,
validation options, strict manifest decoding, spec discovery, registry
inventory and resolution, effective capabilities, and the no-mutation registry
invariant against real temporary bundles.

## Source Anchors

* `packages/cli/okf/doc.go`
* `packages/cli/okf/types.go`
* `packages/cli/okf/read.go`
* `packages/cli/okf/registry.go`
* `packages/cli/okf/read_test.go`
* `packages/cli/okf/README.md`
* `packages/cli/internal/okf/`
